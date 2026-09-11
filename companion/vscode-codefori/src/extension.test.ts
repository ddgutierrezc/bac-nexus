import { describe, expect, it, vi } from "vitest";

import type { BrokerRequest, BrokerResponse, FixedLoopbackServer } from "./broker.js";
import type { CodeForIExports, CodeForIInstance } from "./codeforiAdapter.js";
import { activate, deactivate, installedCompanionVersion } from "./extension.js";
import { tokenAuthenticator, type TokenPublisher } from "./tokenState.js";

const testToken = "test-token-not-a-production-secret";
const tokenPublisher: TokenPublisher = { publish: async () => ({ authenticate: tokenAuthenticator(testToken), update: async () => undefined, remove: async () => undefined }) };

class FakeServer implements FixedLoopbackServer {
  bindCalls: Array<{ host: string; port: number }> = [];
  closeCalls = 0;
  handler: ((request: BrokerRequest) => Promise<BrokerResponse>) | undefined;

  async listen(host: string, port: number): Promise<string> {
    this.bindCalls.push({ host, port });
    return `${host}:${port || 64140}`;
  }

  async close(): Promise<void> {
    this.closeCalls += 1;
  }
}

describe("Companion extension activation", () => {
  it("uses installed extension metadata for the displayed Companion version", () => {
    const extensions = { getExtension: (identifier: string) => identifier === "ddgutierrezc.nexus-codefori-companion" ? { activate: async () => undefined, exports: undefined, packageJSON: { version: "0.2.8" } } : undefined };
    expect(installedCompanionVersion(extensions)).toBe("0.2.8");
  });

  it("maintains the registration lease and clears the heartbeat on shutdown", async () => {
    vi.useFakeTimers();
    const server = new FakeServer();
    const update = vi.fn(async () => undefined);
    const publisher: TokenPublisher = { publish: async () => ({ authenticate: tokenAuthenticator(testToken), update, remove: async () => undefined }) };
    try {
      await activate({}, { extensionHost: { getExtension: () => undefined }, serverFactory: () => server, tokenPublisher: publisher });
      await vi.advanceTimersByTimeAsync(10_000);
      expect(update).toHaveBeenCalledTimes(1);
      await deactivate();
      await vi.advanceTimersByTimeAsync(30_000);
      expect(update).toHaveBeenCalledTimes(1);
    } finally {
      vi.useRealTimers();
    }
  });

  it("activates the Code for IBM i export through the host boundary and tears down owned resources", async () => {
    const runSQL = vi.fn().mockResolvedValue([{ CURRENT_USER: "QUSER" }]);
    const instance: CodeForIInstance = {
      getConnection: () => ({ runSQL }),
      subscribe: () => undefined,
    };
    const exports = { instance } satisfies CodeForIExports;
    const activateCodeForI = vi.fn().mockResolvedValue(exports);
    const getExtension = vi.fn().mockReturnValue({ activate: activateCodeForI, exports: undefined });
    const server = new FakeServer();

    await activate({}, { extensionHost: { getExtension }, serverFactory: (handler, authenticate) => {
      server.handler = handler;
      expect(authenticate({ "x-nexus-companion-token": testToken })).toBe(false);
      return server;
    }, tokenPublisher });

    expect(getExtension).toHaveBeenCalledWith("halcyontechltd.code-for-ibmi");
    expect(activateCodeForI).toHaveBeenCalledTimes(1);
    expect(server.bindCalls).toEqual([{ host: "127.0.0.1", port: 0 }]);

    const response = await server.handler!({
      method: "POST",
      path: "/v1/rpc",
      headers: { "x-nexus-companion-token": testToken },
      body: new TextEncoder().encode(
        '{"version":1,"request_id":"request","method":"sql.query","params":{"sql":"SELECT CURRENT_USER FROM SYSIBM.SYSDUMMY1"}}',
      ),
    });
    expect(response.status).toBe(200);
    expect(JSON.parse(new TextDecoder().decode(response.body))).toEqual({
      version: 1,
      request_id: "request",
      result: { state: "ok", rows: [{ value: "QUSER" }] },
    });
    expect(runSQL).toHaveBeenCalledWith("SELECT CURRENT_USER FROM SYSIBM.SYSDUMMY1", { rows: 1 });

    await deactivate();
    await deactivate();
    expect(server.closeCalls).toBe(1);
  });
});
