import type { QueryResult } from "./codeforiAdapter.js";

export const IMMEDIATE_ADMISSION_CAPACITY = 16;
export const SQL_WAIT_MS = 5_000;

export interface ImmediateAdmissionOptions {
  capacity?: number;
  timeoutMs?: number;
}

export class ImmediateAdmission {
  private active = 0;
  private readonly capacity: number;
  private readonly timeoutMs: number;

  constructor(options: ImmediateAdmissionOptions = {}) {
    this.capacity = options.capacity ?? IMMEDIATE_ADMISSION_CAPACITY;
    this.timeoutMs = options.timeoutMs ?? SQL_WAIT_MS;
  }

  async execute(signal: AbortSignal | undefined, work: () => Promise<QueryResult>): Promise<QueryResult> {
    if (signal?.aborted) {
      return { state: "cancelled" };
    }
    if (this.active >= this.capacity) {
      return { state: "limit_exceeded" };
    }

    this.active += 1;
    const result: Promise<QueryResult> = Promise.resolve()
      .then((): Promise<QueryResult> | QueryResult =>
        signal?.aborted ? { state: "cancelled" } : work(),
      )
      .catch((): QueryResult => ({ state: "failed" }));
    void result.finally(() => {
      this.active -= 1;
    });

    return waitForResult(result, signal, this.timeoutMs);
  }
}

function waitForResult(
  result: Promise<QueryResult>,
  signal: AbortSignal | undefined,
  timeoutMs: number,
): Promise<QueryResult> {
  return new Promise((resolve) => {
    let finished = false;
    const timeout = setTimeout(() => finish({ state: "timeout" }), timeoutMs);
    const onAbort = () => finish({ state: "cancelled" });

    function finish(value: QueryResult): void {
      if (finished) {
        return;
      }
      finished = true;
      clearTimeout(timeout);
      signal?.removeEventListener("abort", onAbort);
      resolve(value);
    }

    signal?.addEventListener("abort", onAbort, { once: true });
    result.then(finish);
  });
}
