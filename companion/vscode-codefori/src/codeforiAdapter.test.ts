import { describe, expect, it } from "vitest";

import {
  createCodeForIAdapter,
  type CodeForIExports,
  type CodeForIConnection,
  type CodeForIInstance,
} from "./codeforiAdapter.js";
import { CANONICAL_PROOF_QUERY } from "./query.js";

class Deferred<T> {
  readonly promise: Promise<T>;
  private resolve!: (value: T) => void;

  constructor() {
    this.promise = new Promise<T>((resolve) => {
      this.resolve = resolve;
    });
  }

  complete(value: T): void {
    this.resolve(value);
  }
}

function createFakeExports(
  runSQL: CodeForIConnection["runSQL"],
): { exports: CodeForIExports; emit: (event: "connected" | "disconnected") => void } {
  const callbacks = new Map<string, () => void>();
  let connectionAvailable = true;
  const instance: CodeForIInstance = {
    getConnection: () => connectionAvailable
      ? { runSQL }
      : (undefined as unknown as CodeForIConnection),
    subscribe: (_context, event, _name, callback) => {
      callbacks.set(event, callback as () => void);
    },
  };

  return {
    exports: { instance },
    emit: (event) => {
      connectionAvailable = event === "connected";
      callbacks.get(event)?.();
    },
  };
}

describe("Code for IBM i public adapter", () => {
  it("uses the canonical proof query and normalizes an arbitrary raw column label", async () => {
    const calls: Array<[string | string[], { rows?: number } | undefined]> = [];
    const fake = createFakeExports(async (sql, options) => {
      calls.push([sql, options]);
      return [{ DIFFERENT_RAW_LABEL: "QUSER" }];
    });
    const adapter = createCodeForIAdapter(fake.exports, {});

    await expect(adapter.query(" select\tcurrent_user\r\nfrom sysibm.sysdummy1 ")).resolves.toEqual({
      state: "ok",
      rows: [{ value: "QUSER" }],
    });
    expect(calls).toEqual([[CANONICAL_PROOF_QUERY, { rows: 1 }]]);
  });

  it("rejects broader SQL before it can reach runSQL", async () => {
    let calls = 0;
    const fake = createFakeExports(async () => {
      calls += 1;
      return [{ VALUE: "QUSER" }];
    });
    const adapter = createCodeForIAdapter(fake.exports, {});

    await expect(adapter.query("SELECT CURRENT_USER FROM SYSIBM.SYSDUMMY1;")).resolves.toEqual({
      state: "invalid_query",
    });
    expect(calls).toBe(0);
  });

  it("suppresses raw errors and malformed rows as failed", async () => {
    const rawFailure = createFakeExports(async () => {
      throw new Error("raw failure for QUSER");
    });
    const malformed = createFakeExports(async () => [{ VALUE: "QUSER", EXTRA: "ignored" }]);
    const symbolColumn = Symbol("unexpected");
    const withExtraEnumerableColumn = { VALUE: "QUSER", [symbolColumn]: "ignored" };
    const nonStringColumn = createFakeExports(async () => [withExtraEnumerableColumn]);

    await expect(
      createCodeForIAdapter(rawFailure.exports, {}).query(CANONICAL_PROOF_QUERY),
    ).resolves.toEqual({ state: "failed" });
    await expect(
      createCodeForIAdapter(malformed.exports, {}).query(CANONICAL_PROOF_QUERY),
    ).resolves.toEqual({ state: "failed" });
    await expect(
      createCodeForIAdapter(nonStringColumn.exports, {}).query(CANONICAL_PROOF_QUERY),
    ).resolves.toEqual({ state: "failed" });
  });

  it("suppresses a result that arrives after a disconnected event", async () => {
    const deferred = new Deferred<Array<Record<string, string>>>();
    const fake = createFakeExports(async () => deferred.promise);
    const adapter = createCodeForIAdapter(fake.exports, {});

    const result = adapter.query(CANONICAL_PROOF_QUERY);
    fake.emit("disconnected");
    deferred.complete([{ UNVERIFIED_LABEL: "QUSER" }]);

    await expect(result).resolves.toEqual({ state: "unavailable" });
    expect(adapter.sessionStatus()).toEqual({ state: "connection_unavailable" });
  });

  it("suppresses in-flight program matches after disconnect or reconnect", async () => {
    for (const reconnect of [false, true]) {
      const deferred = new Deferred<Array<{ library: string; name: string; type: string; text: string }>>();
      const callbacks = new Map<string, () => void>();
      let connection: CodeForIConnection | undefined = {
        runSQL: async () => [],
        getConfig: () => ({ currentLibrary: "LIBA", libraryList: [] }),
        getContent: () => ({ getObjectList: async () => deferred.promise }),
      };
      const adapter = createCodeForIAdapter({ instance: {
        getConnection: () => connection as CodeForIConnection,
        subscribe: (_context, event, _name, callback) => callbacks.set(event, callback as () => void),
      } }, {});
      const result = adapter.resolveProgram({ name: "PISA061" });
      connection = undefined;
      callbacks.get("disconnected")?.();
      if (reconnect) {
        connection = { runSQL: async () => [], getConfig: () => ({ currentLibrary: "LIBB", libraryList: [] }), getContent: () => ({ getObjectList: async () => [] }) };
        callbacks.get("connected")?.();
      }
      deferred.complete([{ library: "LIBA", name: "PISA061", type: "*PGM", text: "" }]);
      await expect(result).resolves.toMatchObject({ state: "unavailable", reason: "stale_session", matches: [] });
    }
  });

  it("reports immutable sanitized diagnostics and refreshes them after connection events", () => {
    const fake = createFakeExports(async () => [{ VALUE: "QUSER" }]);
    const adapter = createCodeForIAdapter(fake.exports, {});
    let refreshes = 0;
    const unsubscribe = adapter.onSessionChange(() => { refreshes += 1; });

    expect(adapter.diagnostics()).toEqual({
      instance: "available",
      subscriptions: "registered",
      getConnection: "available",
    });
    expect(Object.isFrozen(adapter.diagnostics())).toBe(true);

    fake.emit("disconnected");

    expect(refreshes).toBe(1);
    expect(adapter.diagnostics()).toEqual({
      instance: "available",
      subscriptions: "registered",
      getConnection: "unavailable",
    });
    unsubscribe();
  });

  it("reports a getConnection exception without exposing its error", () => {
    const adapter = createCodeForIAdapter({
      instance: {
        getConnection: () => { throw new Error("host.example QUSER secret"); },
        subscribe: () => undefined,
      },
    }, {});

    expect(adapter.diagnostics()).toEqual({
      instance: "available",
      subscriptions: "registered",
      getConnection: "threw",
    });
  });

  it("recovers from a missed connection event by re-probing status, diagnostics, and the query gate", async () => {
    let available = false;
    let queryCalls = 0;
    const runSQL: CodeForIConnection["runSQL"] = async () => {
      queryCalls += 1;
      return [{ VALUE: "QUSER" }];
    };
    const exports: CodeForIExports = {
      instance: {
        getConnection: () => available
          ? { runSQL }
          : (undefined as unknown as CodeForIConnection),
        subscribe: () => undefined,
      },
    };
    const adapter = createCodeForIAdapter(exports, {});

    expect(adapter.sessionStatus()).toEqual({ state: "connection_unavailable" });
    expect(adapter.diagnostics().getConnection).toBe("unavailable");

    available = true;

    expect(adapter.sessionStatus()).toEqual({ state: "connected" });
    expect(adapter.diagnostics().getConnection).toBe("available");

    available = false;
    const queryAdapter = createCodeForIAdapter(exports, {});
    available = true;
    await expect(queryAdapter.query(CANONICAL_PROOF_QUERY)).resolves.toEqual({
      state: "ok",
      rows: [{ value: "QUSER" }],
    });
    expect(queryCalls).toBe(1);
  });
});
