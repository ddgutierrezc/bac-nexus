import { describe, expect, it } from "vitest";

import { ImmediateAdmission } from "./admission.js";
import type { QueryResult } from "./codeforiAdapter.js";

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

const ok = (value: string): Extract<QueryResult, { state: "ok" }> => ({
  state: "ok",
  rows: [{ value }],
});

describe("immediate admission", () => {
  it("admits ten independently before they settle and preserves each reordered result", async () => {
    const admission = new ImmediateAdmission();
    const entered: number[] = [];
    const deferred = Array.from({ length: 10 }, () => new Deferred<ReturnType<typeof ok>>());
    const results = deferred.map((gate, index) =>
      admission.execute(undefined, async () => {
        entered.push(index);
        return gate.promise;
      }),
    );

    await Promise.resolve();
    expect(entered).toEqual(Array.from({ length: 10 }, (_, index) => index));

    for (let index = deferred.length - 1; index >= 0; index -= 1) {
      deferred[index]!.complete(ok(`value-${index}`));
    }

    await expect(Promise.all(results)).resolves.toEqual(
      Array.from({ length: 10 }, (_, index) => ok(`value-${index}`)),
    );
  });

  it("returns limit_exceeded immediately rather than queueing excess work", async () => {
    const admission = new ImmediateAdmission();
    const gates = Array.from({ length: 16 }, () => new Deferred<ReturnType<typeof ok>>());
    const active = gates.map((gate) => admission.execute(undefined, async () => gate.promise));

    await Promise.resolve();
    await expect(admission.execute(undefined, async () => ok("extra"))).resolves.toEqual({
      state: "limit_exceeded",
    });

    for (const gate of gates) {
      gate.complete(ok("released"));
    }
    await Promise.all(active);
  });

  it("does not call work cancelled before the operation begins", async () => {
    const admission = new ImmediateAdmission();
    const controller = new AbortController();
    let calls = 0;

    const result = admission.execute(controller.signal, async () => {
      calls += 1;
      return ok("unexpected");
    });
    controller.abort();

    await expect(result).resolves.toEqual({ state: "cancelled" });
    expect(calls).toBe(0);
  });

  it("retains started capacity after timeout until the underlying promise settles", async () => {
    const admission = new ImmediateAdmission({ capacity: 1, timeoutMs: 1 });
    const gate = new Deferred<ReturnType<typeof ok>>();
    const started = admission.execute(undefined, async () => gate.promise);

    await expect(started).resolves.toEqual({ state: "timeout" });
    await expect(admission.execute(undefined, async () => ok("extra"))).resolves.toEqual({
      state: "limit_exceeded",
    });

    gate.complete(ok("settled"));
    await new Promise((resolve) => setTimeout(resolve, 0));
    await expect(admission.execute(undefined, async () => ok("after-settlement"))).resolves.toEqual(
      ok("after-settlement"),
    );
  });
});
