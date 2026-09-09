import { describe, expect, it } from "vitest";

import {
  decodeRequest,
  encodeResponse,
  MAX_CATALOG_RESPONSE_BYTES,
  MAX_RESPONSE_BYTES,
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

  it("accepts only the exact catalog request shape", () => {
    expect(decodeRequest(encoder.encode('{"version":1,"request_id":"catalog","method":"catalog.resolve_candidates.v1","params":{"item":" pisa061 ","productionLibrary":" prodlib "}}'))).toEqual({
      version: PROTOCOL_VERSION,
      requestID: "catalog",
      method: "catalog.resolve_candidates.v1",
      params: { item: " pisa061 ", productionLibrary: " prodlib " },
    });
    expect(decodeRequest(encoder.encode('{"version":1,"request_id":"catalog","method":"catalog.resolve_candidates.v1","params":{"item":"PISA061","sql":"SELECT 1"}}'))).toBeNull();
  });

  it("serializes catalog successes with an array and removes candidates from errors", () => {
    const request = decodeRequest(encoder.encode('{"version":1,"request_id":"catalog","method":"catalog.resolve_candidates.v1","params":{"item":"PISA061"}}'))!;
    expect(JSON.parse(decoder.decode(encodeResponse(request, { state: "ok", candidates: [] })))).toEqual({
      version: 1, request_id: "catalog", result: { state: "ok", candidates: [] },
    });
    expect(JSON.parse(decoder.decode(encodeResponse(request, { state: "failed", candidates: [] } as never)))).toEqual({
      version: 1, request_id: "catalog", result: { state: "failed" },
    });
  });

  it("encodes fifty complete catalog candidates above the standard response cap", () => {
    const request = decodeRequest(encoder.encode('{"version":1,"request_id":"catalog-many","method":"catalog.resolve_candidates.v1","params":{"item":"PISA061"}}'))!;
    const candidates = Array.from({ length: 50 }, (_, index) => ({
      item: `ITEM${index}`, sourceLibrary: `SRCLIB${index}`, sourceFileBase: `Q${index}`, objectType: "RPGLE", sourceType: "RPGLE",
      application: `Application ${index} `.repeat(4), version: `Version ${index}`, productionLibrary: `PROD${index}`, description: `Catalog description ${index} `.repeat(6),
    }));
    const encoded = encodeResponse(request, { state: "ok", candidates });

    expect(JSON.stringify(candidates).length).toBeGreaterThan(MAX_RESPONSE_BYTES);
    expect(encoded.byteLength).toBeLessThanOrEqual(MAX_CATALOG_RESPONSE_BYTES);
    expect(JSON.parse(decoder.decode(encoded))).toMatchObject({ version: 1, request_id: "catalog-many", result: { state: "ok", candidates } });
    expect(Array.isArray(JSON.parse(decoder.decode(encoded)).result.candidates)).toBe(true);
  });

  it("fails closed with a correlated unavailable catalog response above its cap", () => {
    const request = decodeRequest(encoder.encode('{"version":1,"request_id":"catalog-over-cap","method":"catalog.resolve_candidates.v1","params":{"item":"PISA061"}}'))!;
    const escaped = '"'.repeat(256);
    const candidates = Array.from({ length: 50 }, () => ({ item: escaped, sourceLibrary: escaped, sourceFileBase: escaped, objectType: escaped, sourceType: escaped, application: escaped, version: escaped, productionLibrary: escaped, description: escaped }));

    expect(JSON.parse(decoder.decode(encodeResponse(request, { state: "ok", candidates })))).toEqual({ version: 1, request_id: "catalog-over-cap", result: { state: "unavailable" } });
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
    expect(JSON.parse(decoder.decode(oversized))).toEqual({ version: 1, request_id: "request", result: { state: "unavailable" } });
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
