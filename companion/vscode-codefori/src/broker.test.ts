import { describe, expect, it, vi } from "vitest";

import { ImmediateAdmission } from "./admission.js";
import {
  createBroker,
  createCodeForIBrokerHandler,
  FIXED_LOOPBACK_HOST,
  FIXED_LOOPBACK_PORT,
  type BrokerRequest,
  type BrokerResponse,
  type FixedLoopbackServer,
} from "./broker.js";
import {
  createCodeForIAdapter,
  type CodeForIAdapter,
  type CodeForIConnection,
  type CodeForIExports,
  type CodeForIInstance,
} from "./codeforiAdapter.js";
import type { BrokerResult } from "./protocol.js";
import { tokenAuthenticator, type TokenPublisher } from "./tokenState.js";

const encoder = new TextEncoder();
const decoder = new TextDecoder();
const testToken = "test-token-not-a-production-secret";

function testTokenPublisher(): TokenPublisher {
  return { publish: async () => tokenAuthenticator(testToken) };
}

class FakeServer implements FixedLoopbackServer {
  bindCalls: Array<{ host: string; port: number }> = [];
  requestHandler: ((request: BrokerRequest) => Promise<BrokerResponse>) | undefined;
  closeCalls = 0;

  constructor(private readonly bindError?: Error) {}

  async listen(host: string, port: number): Promise<void> {
    this.bindCalls.push({ host, port });
    if (this.bindError) {
      throw this.bindError;
    }
  }

  async close(): Promise<void> { this.closeCalls += 1; }

  async dispatch(request: BrokerRequest): Promise<BrokerResponse> {
    if (!this.requestHandler) {
      throw new Error("broker did not register a request handler");
    }
    return this.requestHandler(request);
  }
}

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

function request(body: string): BrokerRequest {
  return {
    method: "POST",
    path: "/v1/rpc",
    headers: { "x-nexus-companion-token": testToken },
    body: encoder.encode(body),
  };
}

describe("fixed-loopback broker", () => {
  it("binds the one fixed address and serves an authenticated proof query", async () => {
    const server = new FakeServer();
    const broker = createBroker({
      serverFactory: (handler) => {
        server.requestHandler = handler;
        return server;
      },
      handler: async () => ({ state: "ok", rows: [{ value: "QUSER" }] }),
      tokenPublisher: testTokenPublisher(),
    });

    await expect(broker.start()).resolves.toBe(true);
    expect(server.bindCalls).toEqual([
      { host: FIXED_LOOPBACK_HOST, port: FIXED_LOOPBACK_PORT },
    ]);

    const response = await server.dispatch(
      request(
        '{"version":1,"request_id":"request","method":"sql.query","params":{"sql":"select current_user from sysibm.sysdummy1"}}',
      ),
    );

    expect(response.status).toBe(200);
    expect(JSON.parse(decoder.decode(response.body))).toEqual({
      version: 1,
      request_id: "request",
      result: { state: "ok", rows: [{ value: "QUSER" }] },
    });
  });

  it("fails unavailable on a fixed-port collision without another bind attempt", async () => {
    const server = new FakeServer(new Error("address in use"));
    const publish = vi.fn(async () => tokenAuthenticator(testToken));
    const broker = createBroker({
      serverFactory: () => server,
      handler: async () => ({ state: "connected" }),
      tokenPublisher: { publish },
    });

    await expect(broker.start()).resolves.toBe(false);
    expect(server.bindCalls).toEqual([
      { host: FIXED_LOOPBACK_HOST, port: FIXED_LOOPBACK_PORT },
    ]);
    expect(publish).not.toHaveBeenCalled();
  });

  it("closes a bound server when token publication fails", async () => {
    const server = new FakeServer();
    const broker = createBroker({
      serverFactory: (handler) => { server.requestHandler = handler; return server; },
      handler: async () => ({ state: "connected" }),
      tokenPublisher: { publish: async () => { throw new Error("state unavailable"); } },
    });
    await expect(broker.start()).resolves.toBe(false);
    expect(server.closeCalls).toBe(1);
  });

  it("rejects unauthenticated requests before broker dispatch", async () => {
    const server = new FakeServer();
    const broker = createBroker({
      serverFactory: (handler) => {
        server.requestHandler = handler;
        return server;
      },
      handler: async () => ({ state: "connected" }),
      tokenPublisher: testTokenPublisher(),
    });
    await broker.start();

    for (const headers of [{}, { "x-nexus-companion-token": "invalid" }, { "x-nexus-companion-token": "stale" }]) {
      const response = await server.dispatch({ ...request('{"version":1,"request_id":"request","method":"session.status","params":{}}'), headers });
      expect(response).toEqual({ status: 401, headers: { "content-type": "application/json" }, body: encoder.encode('{"state":"unauthorized"}') });
    }
  });

  it("authenticates and correlates only the catalog allowlisted method", async () => {
    const server = new FakeServer();
    const handler = vi.fn(async () => ({ state: "ok", candidates: [] } as BrokerResult));
    const broker = createBroker({ serverFactory: (requestHandler) => { server.requestHandler = requestHandler; return server; }, handler, tokenPublisher: testTokenPublisher() });
    await broker.start();

    const response = await server.dispatch(request('{"version":1,"request_id":"catalog","method":"catalog.resolve_candidates.v1","params":{"item":"PISA061"}}'));
    expect(JSON.parse(decoder.decode(response.body))).toEqual({ version: 1, request_id: "catalog", result: { state: "ok", candidates: [] } });
    expect(handler).toHaveBeenCalledWith(expect.objectContaining({ params: { item: "PISA061" } }));
  });

  it.each([
    [
      "resolve success",
      '{"version":1,"request_id":"resolve-success","method":"program_inspection.v1.resolve","params":{"name":"PISA061"}}',
      { state: "resolved", searchStrategy: "code_for_i_configured_context", librariesSearched: ["LIBA"], matches: [{ library: "LIBA", name: "PISA061", objectType: "*PGM", provenance: "code_for_i_configured_library_list", matchPosition: 1 }], completeness: "complete", truncated: false, runtimeLiblVerified: false },
    ],
    [
      "resolve not found",
      '{"version":1,"request_id":"resolve-not-found","method":"program_inspection.v1.resolve","params":{"name":"PISA061"}}',
      { state: "not_found", searchStrategy: "code_for_i_configured_context", librariesSearched: ["LIBA"], matches: [], completeness: "complete", truncated: false, runtimeLiblVerified: false },
    ],
    [
      "resolve unavailable",
      '{"version":1,"request_id":"resolve-unavailable","method":"program_inspection.v1.resolve","params":{"name":"PISA061"}}',
      { state: "unavailable", searchStrategy: "code_for_i_configured_context", librariesSearched: [], matches: [], completeness: "complete", truncated: false, runtimeLiblVerified: false, reason: "codefori_extension_unavailable" },
    ],
    [
      "find source",
      '{"version":1,"request_id":"find-source","method":"program_inspection.v1.find_source","params":{"library":"LIBA","name":"PISA061","objectType":"*PGM"}}',
      { state: "unavailable", reason: "compiled_object_source_metadata_unsupported", nextStep: "configure_documented_compile_provenance_api", certainty: "unavailable", completeness: "complete", runtimeLiblVerified: false },
    ],
  ] as Array<[string, string, BrokerResult]>)("echoes the correlation ID for %s", async (_name, body, result) => {
    const server = new FakeServer();
    const broker = createBroker({
      serverFactory: (handler) => {
        server.requestHandler = handler;
        return server;
      },
      handler: async () => result,
      tokenPublisher: testTokenPublisher(),
    });
    await broker.start();

    const response = await server.dispatch(request(body));

    expect(JSON.parse(decoder.decode(response.body))).toMatchObject({
      version: 1,
      request_id: JSON.parse(body).request_id,
      result,
    });
  });

  it("rejects malformed, stale, and unsupported input before the fake handler", async () => {
    const server = new FakeServer();
    let calls = 0;
    const broker = createBroker({
      serverFactory: (handler) => {
        server.requestHandler = handler;
        return server;
      },
      handler: async () => {
        calls += 1;
        return { state: "connected" };
      },
      tokenPublisher: testTokenPublisher(),
    });
    await broker.start();

    const invalidRequest = await server.dispatch(
      request(
        '{"version":1,"request_id":"request","method":"sql.query","params":{"sql":"SELECT CURRENT_USER FROM SYSIBM.SYSDUMMY1;"}}',
      ),
    );
    expect(invalidRequest.status).toBe(200);
    expect(JSON.parse(decoder.decode(invalidRequest.body))).toEqual({
      version: 1,
      request_id: "request",
      result: { state: "invalid_query" },
    });

    for (const malformed of [
      request("{}"),
      request(
        '{"version":2,"request_id":"request","method":"session.status","params":{}}',
      ),
      request(
        '{"version":1,"request_id":"request","method":"session.status","params":{},"generation":"stale"}',
      ),
      { ...request("{}"), method: "GET" },
      { ...request("{}"), path: "/other" },
    ]) {
      const response = await server.dispatch(malformed);
      expect(JSON.parse(decoder.decode(response.body))).toEqual({
        result: { state: "unavailable" },
      });
    }
    expect(calls).toBe(0);
  });

  it("sanitizes handler failures without exposing request details", async () => {
    const server = new FakeServer();
    const broker = createBroker({
      serverFactory: (handler) => {
        server.requestHandler = handler;
        return server;
      },
      handler: async () => {
        throw new Error("raw backend failure for QUSER");
      },
      tokenPublisher: testTokenPublisher(),
    });
    await broker.start();

    const response = await server.dispatch(
      request(
        '{"version":1,"request_id":"request","method":"sql.query","params":{"sql":"SELECT CURRENT_USER FROM SYSIBM.SYSDUMMY1"}}',
      ),
    );

    expect(JSON.parse(decoder.decode(response.body))).toEqual({
      version: 1,
      request_id: "request",
      result: { state: "failed" },
    });
  });

  it("records a program resolve handler failure without changing its unavailable response", async () => {
    const server = new FakeServer();
    const recordOperationFailure = vi.fn();
    const adapter: CodeForIAdapter = {
      sessionStatus: () => ({ state: "connected" }),
      query: async () => ({ state: "failed" }),
      resolveCatalogCandidates: async () => ({ state: "unavailable" }),
      acquireCatalogSource: async () => ({ state: "unavailable" }),
      resolveProgram: async () => { throw new Error("host.example QUSER secret PISA061"); },
      findProgramSource: async () => ({ state: "unavailable", reason: "compiled_object_source_metadata_unsupported", nextStep: "configure_documented_compile_provenance_api", certainty: "unavailable", completeness: "complete", runtimeLiblVerified: false }),
      diagnostics: () => ({ instance: "available", subscriptions: "registered", getConnection: "available", sqlCapability: "unknown", operationFailure: undefined }),
      recordOperationFailure,
      onSessionChange: () => () => undefined,
      deactivate: () => undefined,
    };
    const broker = createBroker({
      serverFactory: (handler) => {
        server.requestHandler = handler;
        return server;
      },
      handler: createCodeForIBrokerHandler(adapter),
      tokenPublisher: testTokenPublisher(),
    });
    await broker.start();

    const response = await server.dispatch(
      request('{"version":1,"request_id":"request","method":"program_inspection.v1.resolve","params":{"name":"PISA061"}}'),
    );

    expect(JSON.parse(decoder.decode(response.body))).toEqual({
      version: 1,
      request_id: "request",
      result: { state: "unavailable" },
    });
    expect(recordOperationFailure).toHaveBeenCalledWith("program.resolve", "handler");
    expect(decoder.decode(response.body)).not.toContain("host.example");
    expect(decoder.decode(response.body)).not.toContain("QUSER");
    expect(decoder.decode(response.body)).not.toContain("secret");
  });

  it("correlates ten immediately admitted public-adapter results completed in reverse order", async () => {
    const server = new FakeServer();
    const gates = Array.from({ length: 10 }, () => new Deferred<Array<Record<string, string>>>());
    const started: string[] = [];
    const connection: CodeForIConnection = {
      runSQL: async (sql) => {
        started.push(sql as string);
        return gates[started.length - 1]!.promise;
      },
    };
    const instance: CodeForIInstance = {
      getConnection: () => connection,
      subscribe: () => undefined,
    };
    const adapter = createCodeForIAdapter({ instance } satisfies CodeForIExports, {});
    const broker = createBroker({
      serverFactory: (handler) => {
        server.requestHandler = handler;
        return server;
      },
      handler: createCodeForIBrokerHandler(adapter, new ImmediateAdmission()),
      tokenPublisher: testTokenPublisher(),
    });
    await broker.start();

    const responses = Array.from({ length: 10 }, (_, index) =>
      server.dispatch(
        request(
          `{"version":1,"request_id":"request-${index}","method":"sql.query","params":{"sql":"SELECT CURRENT_USER FROM SYSIBM.SYSDUMMY1"}}`,
        ),
      ),
    );

    for (let attempt = 0; attempt < 20 && started.length !== 10; attempt += 1) {
      await Promise.resolve();
    }
    expect(started).toHaveLength(10);

    for (let index = gates.length - 1; index >= 0; index -= 1) {
      gates[index]!.complete([{ ARBITRARY_RAW_LABEL: `value-${index}` }]);
    }

    await expect(Promise.all(responses)).resolves.toEqual(
      Array.from({ length: 10 }, (_, index) => ({
        status: 200,
        headers: { "content-type": "application/json" },
        body: encoder.encode(
          `{"version":1,"request_id":"request-${index}","result":{"state":"ok","rows":[{"value":"value-${index}"}]}}`,
        ),
      })),
    );
  });
});
