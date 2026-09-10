import { describe, expect, it } from "vitest";
import { writeFile } from "node:fs/promises";

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

  it("runs only fixed Catalogados SQL with normalized bound criteria and preserves row order", async () => {
    const calls: Array<[string | string[], { bindings?: unknown[]; rows?: number } | undefined]> = [];
    const fake = createFakeExports(async (sql, options) => {
      calls.push([sql, options]);
      return [{ ITEM: "PISA061", TIPO_DE_FUENTE: "RPGLE", TIPO_OBJETO: "RPGLE", APLICACION: null, VERSION: null, BIBLIOTECA_PRODUCCION: "PRODLIB", BIBLIOTECA_FUENTES: "SRCLIB", ARCHIVO_FUENTES: "Q", DESCRIPCION: null }];
    });
    const adapter = createCodeForIAdapter(fake.exports, {});

    await expect(adapter.resolveCatalogCandidates({ item: " pisa061 ", productionLibrary: " prodlib " })).resolves.toEqual({
      state: "ok", candidates: [{ item: "PISA061", sourceType: "RPGLE", objectType: "RPGLE", application: "", version: "", productionLibrary: "PRODLIB", sourceLibrary: "SRCLIB", sourceFileBase: "Q", description: "" }],
    });
    expect(calls).toEqual([[expect.stringContaining("UPPER(PDNAME) = UPPER(?)"), { bindings: ["%PISA061%", "PRODLIB"], rows: 51 }]]);
    expect(calls[0]![0]).toContain("ORDER BY SHSNAM, PDSLIB, PDSFIL, SHOTYP, SHSTYP, PDNAME, PDAPPL, PDVERS");
  });

  it("uses the unfiltered fixed Catalogados variant and fails closed on invalid, malformed, or over-limit rows", async () => {
    const calls: Array<{ bindings?: unknown[]; rows?: number } | undefined> = [];
    const valid = { ITEM: "PISA061", TIPO_DE_FUENTE: "RPGLE", TIPO_OBJETO: "RPGLE", APLICACION: null, VERSION: null, BIBLIOTECA_PRODUCCION: null, BIBLIOTECA_FUENTES: "SRCLIB", ARCHIVO_FUENTES: "Q", DESCRIPCION: null };
    let returnedRows: unknown[] = Array.from({ length: 50 }, () => valid);
    const fake = createFakeExports(async (_sql, options) => { calls.push(options); return returnedRows; });
    const adapter = createCodeForIAdapter(fake.exports, {});

    await expect(adapter.resolveCatalogCandidates({ item: "bad name" })).resolves.toEqual({ state: "invalid_request" });
    const accepted = await adapter.resolveCatalogCandidates({ item: "PISA061" });
    expect(accepted.state).toBe("ok");
    if (accepted.state === "ok") expect(accepted.candidates).toHaveLength(50);
    returnedRows = Array.from({ length: 51 }, () => valid);
    await expect(adapter.resolveCatalogCandidates({ item: "PISA061" })).resolves.toEqual({ state: "candidate_limit_exceeded" });
    returnedRows = [{ ...valid, ITEM: null }];
    await expect(adapter.resolveCatalogCandidates({ item: "PISA061" })).resolves.toEqual({ state: "failed" });
    expect(calls).toEqual(Array(3).fill({ bindings: ["%PISA061%"], rows: 51 }));
  });

  it("suppresses Catalogados results after disconnect, reconnect, or deactivation", async () => {
    for (const change of ["disconnected", "connected", "deactivate"] as const) {
      const deferred = new Deferred<unknown[]>();
      const fake = createFakeExports(async () => deferred.promise);
      const adapter = createCodeForIAdapter(fake.exports, {});
      const result = adapter.resolveCatalogCandidates({ item: "PISA061" });
      if (change === "deactivate") adapter.deactivate(); else fake.emit(change);
      deferred.complete([]);
      await expect(result).resolves.toEqual({ state: "unavailable" });
    }
  });

  it("does not expose raw Catalogados failures", async () => {
    const fake = createFakeExports(async () => { throw new Error("host.example QUSER secret binding"); });
    const result = await createCodeForIAdapter(fake.exports, {}).resolveCatalogCandidates({ item: "PISA061" });
    expect(result).toEqual({ state: "failed" });
    expect(JSON.stringify(result)).not.toMatch(/host\.example|QUSER|secret|binding/);
  });

  it("suppresses a whole-member result after the Code for IBM i session disconnects", async () => {
    const deferred = new Deferred<string>();
    const callbacks = new Map<string, () => void>();
    const adapter = createCodeForIAdapter({ instance: {
      getConnection: () => ({
        runSQL: async () => [],
        getContent: () => ({
          getObjectList: async () => [],
          downloadMemberContent: async () => deferred.promise,
        }),
      }),
      subscribe: (_context, event, _name, callback) => callbacks.set(event, callback as () => void),
    } }, {});

    const result = adapter.acquireCatalogSource({ item: "PISA061", sourceLibrary: "SRCLIB", sourceFileBase: "Q", objectType: "RPGLE", sourceType: "RPGLE", application: "", version: "", productionLibrary: "", description: "" });
    callbacks.get("disconnected")?.();
    deferred.complete("source must not escape");

    await expect(result).resolves.toEqual({ state: "stale_session" });
  });

  it("forwards the Nexus-owned fourth local path to the public member download API", async () => {
    const calls: Array<[string, string, string, string | undefined]> = [];
    const adapter = createCodeForIAdapter({ instance: {
      getConnection: () => ({
        runSQL: async () => [],
        getContent: () => ({
          getObjectList: async () => [],
          downloadMemberContent: async (library, file, member, localPath) => {
            calls.push([library, file, member, localPath]);
            await writeFile(localPath!, "line");
            return "provider-owned whole member";
          },
        }),
      }),
      subscribe: () => undefined,
    } }, {});

    const result = await adapter.acquireCatalogSource({ item: "PISA061", sourceLibrary: "SRCLIB", sourceFileBase: "Q", objectType: "RPGLE", sourceType: "RPGLE", application: "", version: "", productionLibrary: "", description: "" });

    expect(calls).toHaveLength(1);
    expect(calls[0]?.slice(0, 3)).toEqual(["SRCLIB", "QRPGLE", "PISA061"]);
    expect(calls[0]?.[3]).toContain("bac-nexus-source-");
    if (result.state === "ok") await result.artifact.dispose();
    else throw new Error("expected artifact");
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

  it("binds one connection snapshot for each program resolution", async () => {
    let connections = 0;
    const connection: CodeForIConnection = {
      runSQL: async () => [],
      enableSQL: true,
      getConfig: () => ({ currentLibrary: "LIBA", libraryList: [] }),
      getContent: () => ({ getObjectList: async () => [{ library: "LIBA", name: "PISA061", type: "*PGM", text: "" }] }),
    };
    const adapter = createCodeForIAdapter({ instance: {
      getConnection: () => { connections += 1; return connection; },
      subscribe: () => undefined,
    } }, {});
    connections = 0;

    await expect(adapter.resolveProgram({ name: "PISA061" })).resolves.toMatchObject({ state: "resolved" });

    expect(connections).toBe(1);
    expect(adapter.diagnostics().sqlCapability).toBe("available");
  });

  const preflightCases: ReadonlyArray<readonly [string, () => CodeForIConnection | undefined, string]> = [
    ["an absent connection", () => undefined, "preflight_connection_unavailable"],
    ["a throwing connection", () => { throw new Error("host.example QUSER secret PISA061"); }, "preflight_connection_threw"],
    ["a missing inspection API", () => ({ runSQL: async () => [] }), "preflight_inspection_api_unavailable"],
    ["a throwing inspection API accessor", () => {
      const connection: CodeForIConnection = { runSQL: async () => [] };
      Object.defineProperty(connection, "getConfig", { get: () => { throw new Error("host.example QUSER secret PISA061"); } });
      return connection;
    }, "preflight_accessor_failed"],
  ];

  it.each(preflightCases)("records a bounded preflight diagnostic for %s", async (_name, getConnection, stage) => {
    const adapter = createCodeForIAdapter({ instance: {
      getConnection: getConnection as CodeForIInstance["getConnection"],
      subscribe: () => undefined,
    } }, {});

    await expect(adapter.resolveProgram({ name: "PISA061", library: "LIBA" })).resolves.toMatchObject({ state: "unavailable", matches: [] });
    expect(adapter.diagnostics().operationFailure).toEqual({ operation: "program.resolve", stage });
    expect(JSON.stringify(adapter.diagnostics())).not.toMatch(/host\.example|QUSER|secret|PISA061/);
  });

  it("records a throwing inspection API invocation and recovers after a successful lookup", async () => {
    let fail = true;
    const adapter = createCodeForIAdapter({ instance: {
      getConnection: () => ({
        runSQL: async () => [],
        getConfig: () => {
          if (fail) throw new Error("host.example QUSER secret PISA061");
          return { currentLibrary: "LIBA", libraryList: [] };
        },
        getContent: () => ({ getObjectList: async () => [{ library: "LIBA", name: "PISA061", type: "*PGM", text: "" }] }),
      }),
      subscribe: () => undefined,
    } }, {});

    await expect(adapter.resolveProgram({ name: "PISA061" })).resolves.toMatchObject({ state: "unavailable" });
    expect(adapter.diagnostics().operationFailure).toEqual({ operation: "program.resolve", stage: "resolve" });
    expect(JSON.stringify(adapter.diagnostics())).not.toMatch(/host\.example|QUSER|secret|PISA061/);

    fail = false;
    await expect(adapter.resolveProgram({ name: "PISA061", library: "LIBA" })).resolves.toMatchObject({ state: "resolved" });
    expect(adapter.diagnostics().operationFailure).toBeUndefined();
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
      sqlCapability: "unknown",
      operationFailure: undefined,
    });
    expect(Object.isFrozen(adapter.diagnostics())).toBe(true);

    fake.emit("disconnected");

    expect(refreshes).toBe(1);
    expect(adapter.diagnostics()).toEqual({
      instance: "available",
      subscriptions: "registered",
      getConnection: "unavailable",
      sqlCapability: "unknown",
      operationFailure: undefined,
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
      sqlCapability: "unknown",
      operationFailure: undefined,
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

  it.each([
    ["available", () => ({ runSQL: async () => [], enableSQL: true }), "available"],
    ["unavailable", () => ({ runSQL: async () => [], enableSQL: false }), "unavailable"],
    ["unknown when missing", () => ({ runSQL: async () => [] }), "unknown"],
    ["unknown when the getter throws", () => {
      const connection: CodeForIConnection = { runSQL: async () => [] };
      Object.defineProperty(connection, "enableSQL", { get: () => { throw new Error("host.example QUSER secret"); } });
      return connection;
    }, "unknown"],
  ] as const)("classifies SQL capability as %s without exposing connection errors", (_name, createConnection, expected) => {
    const adapter = createCodeForIAdapter({ instance: {
      getConnection: createConnection,
      subscribe: () => undefined,
    } }, {});

    const diagnostics = adapter.diagnostics();

    expect(diagnostics.sqlCapability).toBe(expected);
    expect(JSON.stringify(diagnostics)).not.toContain("host.example");
    expect(JSON.stringify(diagnostics)).not.toContain("QUSER");
    expect(JSON.stringify(diagnostics)).not.toContain("secret");
  });

  it("captures a sanitized program inspection failure and clears it after a successful resolution", async () => {
    let fail = true;
    const adapter = createCodeForIAdapter({
      instance: {
        getConnection: () => ({
          runSQL: async () => [],
          enableSQL: false,
          getConfig: () => ({ currentLibrary: "LIBA", libraryList: [] }),
          getContent: () => ({
            getObjectList: async () => {
              if (fail) throw new Error("host.example QUSER secret PISA061");
              return [{ library: "LIBA", name: "PISA061", type: "*PGM", text: "" }];
            },
          }),
        }),
        subscribe: () => undefined,
      },
    }, {});

    await expect(adapter.resolveProgram({ name: "PISA061", library: "LIBA" })).resolves.toMatchObject({ state: "unavailable" });
    expect(adapter.diagnostics().operationFailure).toEqual({ operation: "program.resolve", stage: "get_object_list" });
    expect(adapter.diagnostics().sqlCapability).toBe("unavailable");
    expect(JSON.stringify(adapter.diagnostics())).not.toContain("host.example");
    expect(JSON.stringify(adapter.diagnostics())).not.toContain("QUSER");
    expect(JSON.stringify(adapter.diagnostics())).not.toContain("secret");

    fail = false;
    await expect(adapter.resolveProgram({ name: "PISA061", library: "LIBA" })).resolves.toMatchObject({ state: "resolved" });
    expect(adapter.diagnostics().operationFailure).toBeUndefined();
  });
});
