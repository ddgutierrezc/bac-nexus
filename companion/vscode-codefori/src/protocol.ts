import { CANONICAL_PROOF_QUERY } from "./query.js";

export const PROTOCOL_VERSION = 1;
export const MAX_REQUEST_BYTES = 512;
export const MAX_RESPONSE_BYTES = 1024;

export type BrokerMethod = "session.status" | "sql.query";
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
  | { state: SessionState };

type ValidQueryResult =
  | { state: "ok"; rows: [{ value: string }] }
  | { state: Exclude<QueryState, "ok"> };

export interface RpcRequest {
  version: typeof PROTOCOL_VERSION;
  generation: string;
  requestID: string;
  method: BrokerMethod;
  params: Record<string, never> | { sql: string };
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

  if (!hasExactKeys(value, ["version", "generation", "request_id", "method", "params"])) {
    return null;
  }
  if (
    value.version !== PROTOCOL_VERSION ||
    !isBoundedASCIIString(value.generation) ||
    !isBoundedASCIIString(value.request_id) ||
    (value.method !== "session.status" && value.method !== "sql.query")
  ) {
    return null;
  }

  if (value.method === "session.status") {
    if (!hasExactKeys(value.params, [])) {
      return null;
    }
    return {
      version: PROTOCOL_VERSION,
      generation: value.generation,
      requestID: value.request_id,
      method: value.method,
      params: {},
    };
  }

  if (!hasExactKeys(value.params, ["sql"]) || typeof value.params.sql !== "string") {
    return null;
  }
  return {
    version: PROTOCOL_VERSION,
    generation: value.generation,
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
      generation: request.generation,
      request_id: request.requestID,
      result: safeResult,
    }),
  );
  return body.byteLength <= MAX_RESPONSE_BYTES ? body : unavailableResponse();
}

export function unavailableResponse(): Uint8Array {
  return encoder.encode(JSON.stringify({ result: { state: "unavailable" } }));
}

function sanitizeResult(method: BrokerMethod, result: BrokerResult): BrokerResult {
  if (method === "session.status") {
    return isSessionResult(result) ? { state: result.state } : { state: "companion_unavailable" };
  }
  if (!isQueryResult(result)) {
    return { state: "failed" };
  }
  if (result.state !== "ok") {
    return { state: result.state };
  }
  return { state: "ok", rows: [{ value: result.rows[0]!.value }] };
}

function isSessionResult(result: BrokerResult): result is { state: SessionState } {
  return Object.keys(result).length === 1 && sessionStates.has(result.state as SessionState);
}

function isQueryResult(result: BrokerResult): result is ValidQueryResult {
  if (!queryStates.has(result.state as QueryState)) {
    return false;
  }
  if (result.state !== "ok") {
    return Object.keys(result).length === 1;
  }
  const rows = result.rows;
  return (
    Object.keys(result).length === 2 &&
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
