import { describe, expect, it } from "vitest";

import {
  decodeRequest,
  encodeResponse,
  PROTOCOL_VERSION,
} from "./protocol.js";

const encoder = new TextEncoder();
const decoder = new TextDecoder();

describe("Companion protocol", () => {
  it("accepts only the canonical SQL request envelope", () => {
    const request = decodeRequest(
      encoder.encode(
        '{"version":1,"request_id":"request","method":"sql.query","params":{"sql":"SELECT CURRENT_USER FROM SYSIBM.SYSDUMMY1"}}',
      ),
    );

    expect(request).toEqual({
      version: PROTOCOL_VERSION,
      requestID: "request",
      method: "sql.query",
      params: { sql: "SELECT CURRENT_USER FROM SYSIBM.SYSDUMMY1" },
    });
  });

  it.each([
    [
      '{"version":1,"request_id":"resolve-without-library","method":"program_inspection.v1.resolve","params":{"name":"PISA061"}}',
      { version: PROTOCOL_VERSION, requestID: "resolve-without-library", method: "program_inspection.v1.resolve", params: { name: "PISA061" } },
    ],
    [
      '{"version":1,"request_id":"resolve-with-library","method":"program_inspection.v1.resolve","params":{"name":"PISA061","library":"LIBA"}}',
      { version: PROTOCOL_VERSION, requestID: "resolve-with-library", method: "program_inspection.v1.resolve", params: { name: "PISA061", library: "LIBA" } },
    ],
    [
      '{"version":1,"request_id":"find-source","method":"program_inspection.v1.find_source","params":{"library":"LIBA","name":"PISA061","objectType":"*PGM"}}',
      { version: PROTOCOL_VERSION, requestID: "find-source", method: "program_inspection.v1.find_source", params: { library: "LIBA", name: "PISA061", objectType: "*PGM" } },
    ],
  ])("normalizes program request correlation", (body, expected) => {
    expect(decodeRequest(encoder.encode(body))).toEqual(expected);
  });

  it("encodes a correlated normalized success result", () => {
    const body = encodeResponse(
      {
        version: PROTOCOL_VERSION,
        requestID: "request",
        method: "sql.query",
        params: { sql: "SELECT CURRENT_USER FROM SYSIBM.SYSDUMMY1" },
      },
      { state: "ok", rows: [{ value: "QUSER" }] },
    );

    expect(JSON.parse(decoder.decode(body))).toEqual({
      version: PROTOCOL_VERSION,
      request_id: "request",
      result: { state: "ok", rows: [{ value: "QUSER" }] },
    });
  });

  it("keeps worst-case metadata bounded and replaces oversized metadata deterministically", () => {
    const request = decodeRequest(encoder.encode('{"version":1,"request_id":"request","method":"program_inspection.v1.resolve","params":{"name":"PISA061"}}'))!;
    const base = { state: "ambiguous", searchStrategy: "code_for_i_configured_context", librariesSearched: Array(16).fill("ABCDEFGHIJ"), matches: Array(8).fill({ library: "ABCDEFGHIJ", name: "ABCDEFGHIJ", objectType: "*PGM", provenance: "code_for_i_configured_library_list", matchPosition: 16 }), completeness: "complete", truncated: false, runtimeLiblVerified: false } as const;
    expect(encodeResponse(request, base).byteLength).toBeLessThanOrEqual(4096);
    const oversized = encodeResponse(request, { ...base, reason: "x".repeat(5000) });
    expect(decoder.decode(oversized)).toBe('{"result":{"state":"unavailable"}}');
  });

  it.each([
    '{"version":1,"version":1,"request_id":"request","method":"session.status","params":{}}',
    '{"version":1,"request_id":"request","method":"session.status","params":{},"unknown":true}',
    '{"version":1,"request_id":"request","method":"session.status","params":{}} trailing',
    '{"version":1,"method":"session.status","params":{}}',
  ])("rejects duplicate, unknown, trailing, and incomplete request bodies", (body) => {
    expect(decodeRequest(encoder.encode(body))).toBeNull();
  });

  it("rejects over-limit bodies and strips invalid non-success result data", () => {
    expect(decodeRequest(encoder.encode("x".repeat(513)))).toBeNull();

    const response = encodeResponse(
      {
        version: PROTOCOL_VERSION,
        requestID: "request",
        method: "sql.query",
        params: { sql: "SELECT CURRENT_USER FROM SYSIBM.SYSDUMMY1" },
      },
      { state: "failed", rows: [{ value: "must not leave the broker" }] },
    );

    expect(JSON.parse(decoder.decode(response))).toEqual({
      version: PROTOCOL_VERSION,
      request_id: "request",
      result: { state: "failed" },
    });
  });
});
