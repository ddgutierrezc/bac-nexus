import { request as nodeRequest } from "node:http";

import { describe, expect, it } from "vitest";

import { createBroker } from "./broker.js";
import { createHTTPServer, hasOriginHeader } from "./httpServer.js";

function post(body: string, headers: Record<string, string> = {}): Promise<{ status: number; body: string }> {
  return new Promise((resolve, reject) => {
    const request = nodeRequest(
      { host: "127.0.0.1", port: 64139, path: "/v1/rpc", method: "POST", headers },
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

describe("fixed-loopback HTTP server", () => {
  it("rejects every Origin header before decoding or handling valid, malformed, and oversized bodies", async () => {
    let calls = 0;
    const broker = createBroker({
      serverFactory: createHTTPServer,
      handler: async () => {
        calls += 1;
        return { state: "connected" };
      },
    });
    await expect(broker.start()).resolves.toBe(true);

    try {
      for (const body of [
        '{"version":1,"request_id":"request","method":"session.status","params":{}}',
        "not JSON",
        "x".repeat(513),
      ]) {
        await expect(post(body, { Origin: "" })).resolves.toEqual({
          status: 403,
          body: '{"state":"browser_origin_rejected"}',
        });
      }
      expect(hasOriginHeader({ oRiGiN: "https://example.test" })).toBe(true);
      expect(hasOriginHeader({ ORIGIN: "" })).toBe(true);
      expect(calls).toBe(0);
    } finally {
      await broker.stop();
    }
  });

  it("uses only the fixed loopback port, fails collisions, and safely repeats lifecycle calls", async () => {
    const first = createBroker({ serverFactory: createHTTPServer, handler: async () => ({ state: "connected" }) });
    const second = createBroker({ serverFactory: createHTTPServer, handler: async () => ({ state: "connected" }) });

    await expect(first.start()).resolves.toBe(true);
    await expect(first.start()).resolves.toBe(false);
    await expect(second.start()).resolves.toBe(false);
    await expect(second.stop()).resolves.toBeUndefined();
    await expect(first.stop()).resolves.toBeUndefined();
    await expect(first.stop()).resolves.toBeUndefined();

    const replacement = createBroker({
      serverFactory: createHTTPServer,
      handler: async () => ({ state: "connected" }),
    });
    await expect(replacement.start()).resolves.toBe(true);
    await replacement.stop();
  });
});
