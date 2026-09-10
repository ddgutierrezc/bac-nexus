import { CANONICAL_PROOF_QUERY } from "./query.js";

export const PROTOCOL_VERSION = 1;
export const MAX_REQUEST_BYTES = 4096;
// Metadata responses are bounded independently from any future source-content API.
export const MAX_RESPONSE_BYTES = 4096;
// 50 candidates × 9 fields × 256 bytes fits below this cap under normal JSON encoding.
export const MAX_CATALOG_RESPONSE_BYTES = 128 * 1024;
export const MAX_SOURCE_RESPONSE_BYTES = 128 * 1024;

export type BrokerMethod = "session.status" | "sql.query" | "program_inspection.v1.resolve" | "program_inspection.v1.find_source" | "catalog.resolve_candidates.v1" | "source_artifact.page.v1" | "source_artifact.dispose.v1";
export type QueryState =
  | "ok"
  | "unavailable"
  | "invalid_query"
  | "limit_exceeded"
  | "timeout"
  | "cancelled"
  | "failed";
export type SessionState =
  | "connected"
  | "companion_unavailable"
  | "codefori_extension_unavailable"
  | "connection_unavailable";

export type BrokerResult =
  | { state: QueryState; rows?: Array<{ value: string }> }
  | { state: SessionState }
  | ResolveProgramResult
  | FindProgramSourceResult
  | CatalogResolveResult
  | SourceArtifactResult;

export interface CatalogCandidate {
  item: string;
  sourceLibrary: string;
  sourceFileBase: string;
  objectType: string;
  sourceType: string;
  application: string;
  version: string;
  productionLibrary: string;
  description: string;
}

export type CatalogResolveResult =
  | { state: "ok"; candidates: CatalogCandidate[] }
  | { state: "invalid_request" | "candidate_limit_exceeded" | "unavailable" | "failed" };

export type SourceArtifactResult =
  | { state: "ok"; cursor: string; page: { content: string; start_line: number; line_count: number; eof: boolean } }
  | { state: "disposed" | "invalid_request" | "not_found" | "ambiguous" | "unavailable" | "expired" | "invalid_source_encoding" | "response_too_large" | "cleanup_failed" };

export type SourcePageParams =
  | { candidate: CatalogCandidate; start_line: number; max_lines: number }
  | { cursor: string; start_line: number; max_lines: number };

type ValidQueryResult =
  | { state: "ok"; rows: [{ value: string }] }
  | { state: Exclude<QueryState, "ok"> };

export interface RpcRequest {
  version: typeof PROTOCOL_VERSION;
  requestID: string;
  method: BrokerMethod;
  params: Record<string, never> | { sql: string } | { name: string; library?: string } | { library: string; name: string; objectType: "*PGM" } | { item: string; productionLibrary?: string } | SourcePageParams | { cursor: string };
}

const encoder = new TextEncoder();
const decoder = new TextDecoder("utf-8", { fatal: true });
const queryStates = new Set<QueryState>([
  "ok",
  "unavailable",
  "invalid_query",
  "limit_exceeded",
  "timeout",
  "cancelled",
  "failed",
]);
const sessionStates = new Set<SessionState>([
  "connected",
  "companion_unavailable",
  "codefori_extension_unavailable",
  "connection_unavailable",
]);

export function decodeRequest(body: Uint8Array): RpcRequest | null {
  if (body.byteLength > MAX_REQUEST_BYTES) {
    return null;
  }

  let value: unknown;
  try {
    value = parseStrictJSON(decoder.decode(body));
  } catch {
    return null;
  }

  if (!hasExactKeys(value, ["version", "request_id", "method", "params"])) {
    return null;
  }
  if (
    value.version !== PROTOCOL_VERSION ||
    !isBoundedASCIIString(value.request_id) ||
    (value.method !== "session.status" && value.method !== "sql.query" && value.method !== "program_inspection.v1.resolve" && value.method !== "program_inspection.v1.find_source" && value.method !== "catalog.resolve_candidates.v1" && value.method !== "source_artifact.page.v1" && value.method !== "source_artifact.dispose.v1")
  ) {
    return null;
  }

  if (value.method === "session.status") {
    if (!hasExactKeys(value.params, [])) {
      return null;
    }
    return {
      version: PROTOCOL_VERSION,
      requestID: value.request_id,
      method: value.method,
      params: {},
    };
  }

  if (value.method === "program_inspection.v1.resolve") {
    if (!isRecord(value.params) || !hasExactKeys(value.params, ["name"]) && !hasExactKeys(value.params, ["name", "library"]) || typeof value.params.name !== "string") return null;
    if ("library" in value.params) {
      if (typeof value.params.library !== "string") return null;
      return {
        version: PROTOCOL_VERSION,
        requestID: value.request_id,
        method: value.method,
        params: { name: value.params.name, library: value.params.library },
      };
    }
    return {
      version: PROTOCOL_VERSION,
      requestID: value.request_id,
      method: value.method,
      params: { name: value.params.name },
    };
  }
  if (value.method === "program_inspection.v1.find_source") {
    if (!isRecord(value.params) || !hasExactKeys(value.params, ["library", "name", "objectType"])) return null;
    const { library, name, objectType } = value.params;
    if (typeof library !== "string" || typeof name !== "string" || objectType !== "*PGM") return null;
    return {
      version: PROTOCOL_VERSION,
      requestID: value.request_id,
      method: value.method,
      params: { library, name, objectType },
    };
  }
  if (value.method === "catalog.resolve_candidates.v1") {
    if (!isRecord(value.params) || (!hasExactKeys(value.params, ["item"]) && !hasExactKeys(value.params, ["item", "productionLibrary"])) || typeof value.params.item !== "string") return null;
    if ("productionLibrary" in value.params && typeof value.params.productionLibrary !== "string") return null;
    return { version: PROTOCOL_VERSION, requestID: value.request_id, method: value.method, params: "productionLibrary" in value.params ? { item: value.params.item, productionLibrary: value.params.productionLibrary as string } : { item: value.params.item } };
  }
  if (value.method === "source_artifact.dispose.v1") {
    if (!hasExactKeys(value.params, ["cursor"]) || !isCursor(value.params.cursor)) return null;
    return { version: PROTOCOL_VERSION, requestID: value.request_id, method: value.method, params: { cursor: value.params.cursor } };
  }
  if (value.method === "source_artifact.page.v1") {
    if (!isRecord(value.params) || !isPageRange(value.params)) return null;
    const { start_line, max_lines } = value.params;
    const candidate = value.params.candidate;
    const cursor = value.params.cursor;
    if (hasExactKeys(value.params, ["candidate", "start_line", "max_lines"]) && isCatalogCandidate(candidate)) {
      return { version: PROTOCOL_VERSION, requestID: value.request_id, method: value.method, params: { candidate, start_line, max_lines } };
    }
    if (hasExactKeys(value.params, ["cursor", "start_line", "max_lines"]) && isCursor(cursor)) {
      return { version: PROTOCOL_VERSION, requestID: value.request_id, method: value.method, params: { cursor, start_line, max_lines } };
    }
    return null;
  }

  if (!hasExactKeys(value.params, ["sql"]) || typeof value.params.sql !== "string") {
    return null;
  }
  return {
    version: PROTOCOL_VERSION,
    requestID: value.request_id,
    method: value.method,
    params: { sql: value.params.sql },
  };
}

export function encodeResponse(request: RpcRequest, result: BrokerResult): Uint8Array {
  const safeResult = sanitizeResult(request.method, result);
  const body = encoder.encode(
    JSON.stringify({
      version: PROTOCOL_VERSION,
      request_id: request.requestID,
      result: safeResult,
    }),
  );
  const maximum = request.method === "catalog.resolve_candidates.v1" && safeResult.state === "ok" && "candidates" in safeResult
    ? MAX_CATALOG_RESPONSE_BYTES
    : request.method === "source_artifact.page.v1" && safeResult.state === "ok" && "page" in safeResult
      ? MAX_SOURCE_RESPONSE_BYTES
    : MAX_RESPONSE_BYTES;
  return body.byteLength <= maximum ? body : unavailableResponse(request);
}

export function unavailableResponse(request?: Pick<RpcRequest, "version" | "requestID">): Uint8Array {
  return encoder.encode(JSON.stringify(request
    ? { version: request.version, request_id: request.requestID, result: { state: "unavailable" } }
    : { result: { state: "unavailable" } }));
}

function sanitizeResult(method: BrokerMethod, result: BrokerResult): BrokerResult {
  if (method === "session.status") {
    return isSessionResult(result) ? { state: result.state } : { state: "companion_unavailable" };
  }
  if (method === "catalog.resolve_candidates.v1") {
    return isCatalogResult(result) ? result : { state: "failed" };
  }
  if (method === "source_artifact.page.v1" || method === "source_artifact.dispose.v1") {
    return isSourceArtifactResult(result) ? result : { state: "unavailable" };
  }
  if (method !== "sql.query") return result;
  if (!isQueryResult(result)) {
    return { state: "failed" };
  }
  if (result.state !== "ok") {
    return { state: result.state };
  }
  return { state: "ok", rows: [{ value: result.rows[0]!.value }] };
}

function isCatalogResult(result: BrokerResult): result is CatalogResolveResult {
  const catalogStates = new Set(["ok", "invalid_request", "candidate_limit_exceeded", "unavailable", "failed"]);
  const value = result as unknown as Record<string, unknown>;
  if (!catalogStates.has(result.state) || !isRecord(value)) return false;
  if (result.state !== "ok") return Object.keys(value).length === 1;
  return Object.keys(value).length === 2 && Array.isArray(value.candidates) && value.candidates.length <= 50 && value.candidates.every(isCatalogCandidate);
}

function isCatalogCandidate(value: unknown): value is CatalogCandidate {
  return hasExactKeys(value, ["item", "sourceLibrary", "sourceFileBase", "objectType", "sourceType", "application", "version", "productionLibrary", "description"])
    && Object.values(value).every(isBoundedUTF8String);
}

function isSourceArtifactResult(result: BrokerResult): result is SourceArtifactResult {
  const value = result as Record<string, unknown>;
  const states = new Set(["ok", "disposed", "invalid_request", "not_found", "ambiguous", "unavailable", "expired", "invalid_source_encoding", "response_too_large", "cleanup_failed"]);
  if (!isRecord(value) || !states.has(result.state)) return false;
  if (result.state !== "ok") return Object.keys(value).length === 1;
  return hasExactKeys(value, ["state", "cursor", "page"]) && isCursor(value.cursor) && hasExactKeys(value.page, ["content", "start_line", "line_count", "eof"]) && typeof value.page.content === "string" && Number.isSafeInteger(value.page.start_line) && Number.isSafeInteger(value.page.line_count) && typeof value.page.eof === "boolean";
}

function isPageRange(value: Record<string, unknown>): value is Record<string, unknown> & { start_line: number; max_lines: number } {
  const { start_line, max_lines } = value;
  return typeof start_line === "number" && Number.isSafeInteger(start_line) && start_line >= 1 && typeof max_lines === "number" && Number.isSafeInteger(max_lines) && maxLinesInBounds(max_lines);
}

function maxLinesInBounds(value: number): boolean { return value >= 1 && value <= 200; }

function isCursor(value: unknown): value is string {
  return typeof value === "string" && /^[A-Za-z0-9_-]{43}$/.test(value);
}

function isSessionResult(result: BrokerResult): result is { state: SessionState } {
  return Object.keys(result).length === 1 && sessionStates.has(result.state as SessionState);
}

function isQueryResult(result: BrokerResult): result is ValidQueryResult {
  const query = result as ValidQueryResult;
  if (!queryStates.has(query.state as QueryState)) {
    return false;
  }
  if (query.state !== "ok") {
    return Object.keys(query).length === 1;
  }
  const rows = query.rows;
  return (
    Object.keys(query).length === 2 &&
    Array.isArray(rows) &&
    rows.length === 1 &&
    hasExactKeys(rows[0], ["value"]) &&
    isBoundedUTF8String(rows[0].value)
  );
}

function hasExactKeys(value: unknown, expected: string[]): value is Record<string, unknown> {
  if (!isRecord(value)) {
    return false;
  }
  const keys = Object.keys(value);
  return keys.length === expected.length && expected.every((key) => keys.includes(key));
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isBoundedASCIIString(value: unknown): value is string {
  return (
    typeof value === "string" &&
    value.length > 0 &&
    encoder.encode(value).byteLength <= 128 &&
    [...value].every((character) => character.codePointAt(0)! <= 0x7f)
  );
}

function isBoundedUTF8String(value: unknown): value is string {
  if (typeof value !== "string" || encoder.encode(value).byteLength > 256) {
    return false;
  }
  try {
    return decoder.decode(encoder.encode(value)) === value;
  } catch {
    return false;
  }
}

function parseStrictJSON(source: string): unknown {
  const parsed = JSON.parse(source) as unknown;
  const cursor = new JSONCursor(source);
  cursor.readValue();
  cursor.skipWhitespace();
  if (!cursor.atEnd()) {
    throw new SyntaxError("trailing JSON");
  }
  return parsed;
}

class JSONCursor {
  private position = 0;

  constructor(private readonly source: string) {}

  atEnd(): boolean {
    return this.position === this.source.length;
  }

  skipWhitespace(): void {
    while (/\s/.test(this.source[this.position] ?? "")) {
      this.position += 1;
    }
  }

  readValue(): void {
    this.skipWhitespace();
    const character = this.source[this.position];
    if (character === "{") {
      this.readObject();
      return;
    }
    if (character === "[") {
      this.readArray();
      return;
    }
    if (character === '"') {
      this.readString();
      return;
    }
    this.readPrimitive();
  }

  private readObject(): void {
    const keys = new Set<string>();
    this.position += 1;
    this.skipWhitespace();
    if (this.source[this.position] === "}") {
      this.position += 1;
      return;
    }
    while (true) {
      const key = this.readString();
      if (keys.has(key)) {
        throw new SyntaxError("duplicate JSON key");
      }
      keys.add(key);
      this.skipWhitespace();
      this.expect(":");
      this.readValue();
      this.skipWhitespace();
      if (this.source[this.position] === "}") {
        this.position += 1;
        return;
      }
      this.expect(",");
      this.skipWhitespace();
    }
  }

  private readArray(): void {
    this.position += 1;
    this.skipWhitespace();
    if (this.source[this.position] === "]") {
      this.position += 1;
      return;
    }
    while (true) {
      this.readValue();
      this.skipWhitespace();
      if (this.source[this.position] === "]") {
        this.position += 1;
        return;
      }
      this.expect(",");
      this.skipWhitespace();
    }
  }

  private readString(): string {
    const start = this.position;
    this.expect('"');
    while (true) {
      const character = this.source[this.position];
      if (character === undefined) {
        throw new SyntaxError("unterminated JSON string");
      }
      this.position += 1;
      if (character === "\\") {
        this.position += 1;
      } else if (character === '"') {
        return JSON.parse(this.source.slice(start, this.position)) as string;
      }
    }
  }

  private readPrimitive(): void {
    const start = this.position;
    while (!/[\s,}\]]/.test(this.source[this.position] ?? "")) {
      this.position += 1;
    }
    if (start === this.position) {
      throw new SyntaxError("expected JSON value");
    }
    JSON.parse(this.source.slice(start, this.position));
  }

  private expect(character: string): void {
    if (this.source[this.position] !== character) {
      throw new SyntaxError(`expected ${character}`);
    }
    this.position += 1;
  }
}

export { CANONICAL_PROOF_QUERY };
import type { FindProgramSourceResult, ResolveProgramResult } from "./programInspection.js";
