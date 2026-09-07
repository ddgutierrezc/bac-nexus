import type { QueryState, SessionState } from "./protocol.js";
import { canonicalizeProofQuery } from "./query.js";

const MAX_VALUE_BYTES = 256;
const encoder = new TextEncoder();
const decoder = new TextDecoder("utf-8", { fatal: true });

export interface CodeForIConnection {
  runSQL(sql: string, options: { rows?: number }): Promise<unknown[]>;
}

export interface CodeForIInstance {
  getConnection(): CodeForIConnection;
  subscribe(context: unknown, event: "connected" | "disconnected", name: string, callback: () => void): void;
}

export interface CodeForIExports {
  instance: CodeForIInstance;
}

export type QueryResult =
  | { state: "ok"; rows: [{ value: string }] }
  | { state: Exclude<QueryState, "ok"> };
export type SessionStatusResult = { state: SessionState };

export interface AdapterDiagnosticSnapshot {
  readonly instance: "available" | "unavailable";
  readonly subscriptions: "registered" | "unavailable";
  readonly getConnection: "available" | "unavailable" | "threw";
}

export interface CodeForIAdapter {
  sessionStatus(): SessionStatusResult;
  query(sql: string): Promise<QueryResult>;
  diagnostics(): AdapterDiagnosticSnapshot;
  onSessionChange(callback: () => void): () => void;
  deactivate(): void;
}

export function createCodeForIAdapter(
  codeForI: CodeForIExports | undefined,
  context: unknown,
): CodeForIAdapter {
  let instance = codeForI?.instance;
  let active = true;
  let connectionAvailable = instance !== undefined;
  let connectionGeneration = 0;
  let subscriptionsRegistered = false;
  let getConnection: AdapterDiagnosticSnapshot["getConnection"] = "unavailable";
  const listeners = new Set<() => void>();

  const refreshConnection = (): boolean => {
    if (!instance) {
      getConnection = "unavailable";
      return false;
    }
    try {
      const connection = instance.getConnection();
      getConnection = connection === undefined ? "unavailable" : "available";
      return connection !== undefined;
    } catch {
      getConnection = "threw";
      return false;
    }
  };
  const notify = (): void => { for (const listener of listeners) listener(); };

  if (instance) {
    try {
      instance.subscribe(context, "connected", "BAC Nexus Companion connected", () => {
        connectionAvailable = refreshConnection();
        connectionGeneration += 1;
        notify();
      });
      instance.subscribe(context, "disconnected", "BAC Nexus Companion disconnected", () => {
        connectionAvailable = false;
        getConnection = "unavailable";
        connectionGeneration += 1;
        notify();
      });
      subscriptionsRegistered = true;
      connectionAvailable = refreshConnection();
    } catch {
      connectionAvailable = false;
      subscriptionsRegistered = false;
      getConnection = "threw";
    }
  }

  return {
    sessionStatus(): SessionStatusResult {
      if (!active || !instance) {
        return { state: "codefori_extension_unavailable" };
      }
      connectionAvailable = refreshConnection();
      return connectionAvailable
        ? { state: "connected" }
        : { state: "connection_unavailable" };
    },
    async query(sql: string): Promise<QueryResult> {
      const canonicalSQL = canonicalizeProofQuery(sql);
      if (!canonicalSQL) {
        return { state: "invalid_query" };
      }
      if (!active || !instance) {
        return { state: "unavailable" };
      }
      connectionAvailable = refreshConnection();
      if (!connectionAvailable) {
        return { state: "unavailable" };
      }

      const startedGeneration = connectionGeneration;
      let connection: CodeForIConnection;
      try {
        connection = instance.getConnection();
      } catch {
        connectionAvailable = false;
        return { state: "unavailable" };
      }

      try {
        const rows = await connection.runSQL(canonicalSQL, { rows: 1 });
        if (!active || !connectionAvailable || connectionGeneration !== startedGeneration) {
          return { state: "unavailable" };
        }
        return normalizeRows(rows);
      } catch {
        return { state: "failed" };
      }
    },
    diagnostics(): AdapterDiagnosticSnapshot {
      if (active && instance) {
        connectionAvailable = refreshConnection();
      }
      return Object.freeze({
        instance: active && instance ? "available" : "unavailable",
        subscriptions: active && subscriptionsRegistered ? "registered" : "unavailable",
        getConnection: active ? getConnection : "unavailable",
      });
    },
    onSessionChange(callback: () => void): () => void {
      listeners.add(callback);
      return () => listeners.delete(callback);
    },
    deactivate(): void {
      active = false;
      connectionAvailable = false;
      instance = undefined;
      listeners.clear();
    },
  };
}

function normalizeRows(rows: unknown): QueryResult {
  if (!Array.isArray(rows) || rows.length !== 1) {
    return { state: "failed" };
  }

  const row = rows[0];
  if (!isRecord(row)) {
    return { state: "failed" };
  }
  const keys = Reflect.ownKeys(row).filter((key) =>
    Object.prototype.propertyIsEnumerable.call(row, key),
  );
  if (keys.length !== 1 || typeof keys[0] !== "string") {
    return { state: "failed" };
  }

  const value = row[keys[0]];
  return isBoundedUTF8String(value)
    ? { state: "ok", rows: [{ value }] }
    : { state: "failed" };
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isBoundedUTF8String(value: unknown): value is string {
  if (typeof value !== "string" || encoder.encode(value).byteLength > MAX_VALUE_BYTES) {
    return false;
  }
  try {
    return decoder.decode(encoder.encode(value)) === value;
  } catch {
    return false;
  }
}
