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
import type { RequestAuthenticator, TokenPublisher } from "./tokenState.js";
import type { SourceArtifactBroker } from "./sourceArtifactBroker.js";

export const FIXED_LOOPBACK_HOST = "127.0.0.1";
export const DYNAMIC_LOOPBACK_PORT = 0;

export interface BrokerRequest {
  method: string;
  path: string;
  headers: Readonly<Record<string, string | string[] | undefined>>;
  body: Uint8Array;
}

export interface BrokerResponse {
  status: number;
  headers: Readonly<Record<string, string>>;
  body: Uint8Array;
}

export interface FixedLoopbackServer {
  listen(host: string, port: number): Promise<string>;
  close(): Promise<void>;
}

type RequestHandler = (request: BrokerRequest) => Promise<BrokerResponse>;
export type BrokerHandler = (request: RpcRequest) => Promise<BrokerResult>;

export interface BrokerOptions {
  serverFactory: (handler: RequestHandler, authenticate: RequestAuthenticator) => FixedLoopbackServer;
  handler: BrokerHandler;
  tokenPublisher: TokenPublisher;
  eligibility?: () => { connected: boolean; focused: boolean; generation: number };
}

export interface CompanionBroker {
  start(): Promise<boolean>;
  stop(): Promise<void>;
  refreshRegistration(): Promise<void>;
  endpoint(): string | undefined;
}

export function createCodeForIBrokerHandler(
  adapter: CodeForIAdapter,
  admission = new ImmediateAdmission(),
  sourceArtifacts?: SourceArtifactBroker,
): BrokerHandler {
  return async (request): Promise<BrokerResult> => {
    try {
      if (request.method === "session.status") return adapter.sessionStatus();
      if (request.method === "program_inspection.v1.resolve") return await adapter.resolveProgram(request.params as { name: string; library?: string });
      if (request.method === "program_inspection.v1.find_source") return adapter.findProgramSource(request.params as { library: string; name: string; objectType: "*PGM" });
      if (request.method === "catalog.resolve_candidates.v1") return adapter.resolveCatalogCandidates(request.params as { item: string; productionLibrary?: string });
      if (request.method === "source_artifact.page.v1") return sourceArtifacts?.page(request) ?? { state: "unavailable" };
      if (request.method === "source_artifact.dispose.v1") return sourceArtifacts?.dispose((request.params as { cursor: string }).cursor) ?? { state: "unavailable" };
      return admission.execute(undefined, () => adapter.query((request.params as { sql: string }).sql));
    } catch (error) {
      if (request.method === "program_inspection.v1.resolve") {
        adapter.recordOperationFailure("program.resolve", "handler");
      }
      throw error;
    }
  };
}

export function createBroker(options: BrokerOptions): CompanionBroker {
  let server: FixedLoopbackServer | undefined;
  let startAttempted = false;
  let authenticate: RequestAuthenticator = () => false;
  let registration: Awaited<ReturnType<TokenPublisher["publish"]>> | undefined;
  let boundEndpoint: string | undefined;
  let closed = false;
  let lifecycle = Promise.resolve();

  const serialize = <T>(operation: () => Promise<T>): Promise<T> => {
    const result = lifecycle.then(operation, operation);
    lifecycle = result.then(() => undefined, () => undefined);
    return result;
  };

  const handle = async (request: BrokerRequest): Promise<BrokerResponse> => {
    if (!authenticate(request.headers)) {
      return unauthorized();
    }
    if (request.method !== "POST" || request.path !== "/v1/rpc") {
      return sanitized(404);
    }
    const decoded = decodeRequest(request.body);
    if (!decoded) {
      return sanitized(400);
    }
    const normalized = normalizeRequest(decoded);
    if (!normalized) {
      return correlated(decoded, decoded.method === "catalog.resolve_candidates.v1" ? { state: "invalid_request" } : { state: "invalid_query" });
    }

    try {
      return correlated(normalized, await options.handler(normalized));
    } catch {
      return correlated(
        normalized,
        normalized.method === "session.status"
          ? { state: "companion_unavailable" }
          : normalized.method === "sql.query"
            ? { state: "failed" }
            : { state: "unavailable" },
      );
    }
  };

  return {
    start(): Promise<boolean> {
      return serialize(async () => {
      if (closed || startAttempted) {
        return false;
      }
      startAttempted = true;
      const created = options.serverFactory(handle, (headers) => authenticate(headers));
      server = created;
      try {
        const endpoint = await created.listen(FIXED_LOOPBACK_HOST, DYNAMIC_LOOPBACK_PORT);
        boundEndpoint = endpoint;
        if (closed) {
          await created.close().catch(() => undefined);
          server = undefined;
          boundEndpoint = undefined;
          return false;
        }
        const published = await options.tokenPublisher.publish(endpoint, eligibility(options));
        if (closed) {
          await published.remove().catch(() => undefined);
          await created.close().catch(() => undefined);
          server = undefined;
          boundEndpoint = undefined;
          return false;
        }
        registration = published;
        authenticate = registration.authenticate;
        return true;
      } catch {
        if (server === created) {
          server = undefined;
        }
        boundEndpoint = undefined;
        await created.close().catch(() => undefined);
        return false;
      }
      });
    },
    stop(): Promise<void> {
      closed = true;
      return serialize(async () => {
      const active = server;
      server = undefined;
      boundEndpoint = undefined;
      const published = registration;
      registration = undefined;
      authenticate = () => false;
      if (published) await published.remove().catch(() => undefined);
      if (active) {
        await active.close();
      }
      });
    },
    refreshRegistration(): Promise<void> {
      return serialize(async () => {
        if (!closed && registration) await registration.update(eligibility(options));
      });
    },
    endpoint: (): string | undefined => boundEndpoint,
  };
}

function eligibility(options: BrokerOptions): { connected: boolean; focused: boolean; generation: number } {
  return options.eligibility?.() ?? { connected: false, focused: false, generation: 0 };
}

function normalizeRequest(request: RpcRequest): RpcRequest | null {
  if (request.method === "sql.query") {
    const sql = canonicalizeProofQuery((request.params as { sql: string }).sql);
    return sql ? { ...request, params: { sql } } : null;
  }
  if (request.method !== "catalog.resolve_candidates.v1") return request;
  const params = request.params as { item: string; productionLibrary?: string };
  const item = normalizeSystemName(params.item);
  const productionLibrary = params.productionLibrary === undefined ? undefined : normalizeSystemName(params.productionLibrary);
  if (!item || (params.productionLibrary !== undefined && !productionLibrary)) return null;
  return { ...request, params: productionLibrary === undefined ? { item } : { item, productionLibrary } };
}

function normalizeSystemName(value: string): string | undefined {
  const normalized = value.trim().toUpperCase();
  return /^[A-Z$#@][A-Z0-9_$#@]{0,9}$/.test(normalized) ? normalized : undefined;
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

function unauthorized(): BrokerResponse {
  return {
    status: 401,
    headers: { "content-type": "application/json" },
    body: new TextEncoder().encode('{"state":"unauthorized"}'),
  };
}
