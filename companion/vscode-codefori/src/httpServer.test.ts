import { request as nodeRequest } from "node:http";
import { createConnection } from "node:net";

import { describe, expect, it } from "vitest";

import { createBroker } from "./broker.js";
import { createHTTPServer, hasOriginHeader } from "./httpServer.js";
import { tokenAuthenticator, type TokenPublisher } from "./tokenState.js";

const testToken = "test-token-not-a-production-secret";
const tokenPublisher: TokenPublisher = { publish: async () => ({ authenticate: tokenAuthenticator(testToken), update: async () => undefined, remove: async () => undefined }) };

function post(port: number, body: string, headers: Record<string, string> = {}): Promise<{ status: number; body: string }> {
  return new Promise((resolve, reject) => {
    const request = nodeRequest(
      { host: "127.0.0.1", port, path: "/v1/rpc", method: "POST", headers },
      (response) => {
        const chunks: Buffer[] = [];
        response.on("data", (chunk: Buffer) => chunks.push(chunk));
        response.on("end", () => resolve({ status: response.statusCode ?? 0, body: Buffer.concat(chunks).toString() }));
      },
    );
    request.on("error", reject);
    request.end(body);
  });
}

function postIncomplete(port: number): Promise<string> {
  return new Promise((resolve, reject) => {
    const socket = createConnection({ host: "127.0.0.1", port });
    let response = "";
    socket.once("connect", () => socket.write("POST /v1/rpc HTTP/1.1\r\nHost: 127.0.0.1\r\nContent-Length: 100\r\n\r\npartial"));
    socket.on("data", (chunk: Buffer) => { response += chunk.toString(); });
    socket.once("end", () => resolve(response));
    socket.once("error", reject);
  });
}

describe("fixed-loopback HTTP server", () => {
  it("rejects every Origin header before decoding or handling valid, malformed, and oversized bodies", async () => {
    let calls = 0;
    const broker = createBroker({
      serverFactory: createHTTPServer,
      handler: async () => {
        calls += 1;
        return { state: "connected" };
      },
      tokenPublisher,
    });
    await expect(broker.start()).resolves.toBe(true);
    const port = Number(broker.endpoint()?.split(":")[1]);

    try {
      for (const body of [
        '{"version":1,"request_id":"request","method":"session.status","params":{}}',
        "not JSON",
        "x".repeat(513),
      ]) {
        await expect(post(port, body, { Origin: "" })).resolves.toEqual({
          status: 403,
          body: '{"state":"browser_origin_rejected"}',
        });
      }
      expect(hasOriginHeader({ oRiGiN: "https://example.test" })).toBe(true);
      expect(hasOriginHeader({ ORIGIN: "" })).toBe(true);
      expect(calls).toBe(0);
      await expect(post(port, '{"version":1,"request_id":"request","method":"session.status","params":{}}', { "x-nexus-companion-token": testToken })).resolves.toEqual({
        status: 200,
        body: '{"version":1,"request_id":"request","result":{"state":"connected"}}',
      });
    } finally {
      await broker.stop();
    }
  });

  it("uses distinct OS-assigned loopback ports and safely repeats lifecycle calls", async () => {
    const first = createBroker({ serverFactory: createHTTPServer, handler: async () => ({ state: "connected" }), tokenPublisher });
    const second = createBroker({ serverFactory: createHTTPServer, handler: async () => ({ state: "connected" }), tokenPublisher });

    await expect(first.start()).resolves.toBe(true);
    await expect(first.start()).resolves.toBe(false);
    await expect(second.start()).resolves.toBe(true);
    expect(first.endpoint()).not.toBe(second.endpoint());
    await expect(second.stop()).resolves.toBeUndefined();
    await expect(first.stop()).resolves.toBeUndefined();
    await expect(first.stop()).resolves.toBeUndefined();

    const replacement = createBroker({
      serverFactory: createHTTPServer,
      handler: async () => ({ state: "connected" }),
      tokenPublisher,
    });
    await expect(replacement.start()).resolves.toBe(true);
    await replacement.stop();
  });

  it("completes an unauthorized response without waiting for an incomplete body", async () => {
    let calls = 0;
    const broker = createBroker({
      serverFactory: createHTTPServer,
      handler: async () => { calls += 1; return { state: "connected" }; },
      tokenPublisher,
    });
    await expect(broker.start()).resolves.toBe(true);
    try {
      const response = postIncomplete(Number(broker.endpoint()?.split(":")[1]));
      await expect(response).resolves.toContain("HTTP/1.1 401");
      await expect(response).resolves.toContain('{"state":"unauthorized"}');
      expect(calls).toBe(0);
    } finally {
      await broker.stop();
    }
  });

});
