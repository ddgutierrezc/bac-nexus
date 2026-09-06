import { canonicalizeProofQuery } from "./query.js";
import { ImmediateAdmission } from "./admission.js";
import type { CodeForIAdapter } from "./codeforiAdapter.js";
import {
  decodeRequest,
  encodeResponse,
  type BrokerResult,
  type RpcRequest,
  unavailableResponse,
} from "./protocol.js";

export const FIXED_LOOPBACK_HOST = "127.0.0.1";
export const FIXED_LOOPBACK_PORT = 64139;

export interface BrokerRequest {
  method: string;
  path: string;
  headers: Readonly<Record<string, string | undefined>>;
  body: Uint8Array;
}

export interface BrokerResponse {
  status: number;
  headers: Readonly<Record<string, string>>;
  body: Uint8Array;
}

export interface FixedLoopbackServer {
  listen(host: string, port: number): Promise<void>;
  close(): Promise<void>;
}

type RequestHandler = (request: BrokerRequest) => Promise<BrokerResponse>;
export type BrokerHandler = (request: RpcRequest) => Promise<BrokerResult>;

export interface BrokerOptions {
  serverFactory: (handler: RequestHandler) => FixedLoopbackServer;
  handler: BrokerHandler;
}

export interface CompanionBroker {
  start(): Promise<boolean>;
  stop(): Promise<void>;
}

export function createCodeForIBrokerHandler(
  adapter: CodeForIAdapter,
  admission = new ImmediateAdmission(),
): BrokerHandler {
  return async (request) =>
    request.method === "session.status"
      ? adapter.sessionStatus()
      : admission.execute(undefined, () => adapter.query(request.params.sql));
}

export function createBroker(options: BrokerOptions): CompanionBroker {
  let server: FixedLoopbackServer | undefined;
  let startAttempted = false;

  const handle = async (request: BrokerRequest): Promise<BrokerResponse> => {
    if (request.method !== "POST" || request.path !== "/v1/rpc") {
      return sanitized(404);
    }
    const decoded = decodeRequest(request.body);
    if (!decoded) {
      return sanitized(400);
    }
    const normalized = normalizeQuery(decoded);
    if (!normalized) {
      return correlated(decoded, { state: "invalid_query" });
    }

    try {
      return correlated(normalized, await options.handler(normalized));
    } catch {
      return correlated(
        normalized,
        normalized.method === "session.status"
          ? { state: "companion_unavailable" }
          : { state: "failed" },
      );
    }
  };

  return {
    async start(): Promise<boolean> {
      if (startAttempted) {
        return false;
      }
      startAttempted = true;
      const created = options.serverFactory(handle);
      server = created;
      try {
        await created.listen(FIXED_LOOPBACK_HOST, FIXED_LOOPBACK_PORT);
        return true;
      } catch {
        if (server === created) {
          server = undefined;
        }
        await created.close().catch(() => undefined);
        return false;
      }
    },
    async stop(): Promise<void> {
      const active = server;
      server = undefined;
      if (active) {
        await active.close();
      }
    },
  };
}

function normalizeQuery(request: RpcRequest): RpcRequest | null {
  if (request.method === "session.status") {
    return request;
  }
  const sql = canonicalizeProofQuery(request.params.sql);
  return sql ? { ...request, params: { sql } } : null;
}

function correlated(request: RpcRequest, result: BrokerResult): BrokerResponse {
  return {
    status: 200,
    headers: { "content-type": "application/json" },
    body: encodeResponse(request, result),
  };
}

function sanitized(status: number): BrokerResponse {
  return {
    status,
    headers: { "content-type": "application/json" },
    body: unavailableResponse(),
  };
}
