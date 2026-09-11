import { mkdtemp, readFile, readdir, rm, symlink, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { afterEach, describe, expect, it } from "vitest";

import { createTokenPublisher, tokenStatePath } from "./tokenState.js";

const roots: string[] = [];
afterEach(async () => { await Promise.all(roots.splice(0).map((root) => rm(root, { force: true, recursive: true }))); });

describe("Companion private instance registry", () => {
  it("publishes independent dynamic-instance records and removes only its own record", async () => {
    const root = await temporaryDirectory();
    const first = await createTokenPublisher({ configDirectory: () => root }).publish("127.0.0.1:41001", { connected: true, focused: true, generation: 1 });
    const second = await createTokenPublisher({ configDirectory: () => root }).publish("127.0.0.1:41002", { connected: true, focused: false, generation: 2 });
    const directory = tokenStatePath(root);
    const records = await readdir(directory);
    expect(records).toHaveLength(2);
    const states = await Promise.all(records.map(async (record) => JSON.parse(await readFile(join(directory, record), "utf8")) as { version: number; endpoint: string; token: string }));
    expect(states.map((state) => state.endpoint).sort()).toEqual(["127.0.0.1:41001", "127.0.0.1:41002"]);
    expect(states[0]?.token).not.toBe(states[1]?.token);
    await first.remove();
    expect(await readdir(directory)).toHaveLength(1);
    await second.update({ connected: false, focused: false, generation: 3 });
    expect(second.authenticate({ "x-nexus-companion-token": "wrong" })).toBe(false);
  });

  it("refuses unsafe registry paths without altering another instance", async () => {
    const root = await temporaryDirectory();
    const valid = await createTokenPublisher({ configDirectory: () => root }).publish("127.0.0.1:41001", { connected: true, focused: true, generation: 1 });
    const directory = tokenStatePath(root);
    const before = await readdir(directory);
    try { await symlink(join(root, "missing"), join(directory, "unsafe.json")); } catch { return; }
    await expect(createTokenPublisher({ configDirectory: () => root }).publish("127.0.0.1:41002", { connected: true, focused: true, generation: 1 })).resolves.toBeDefined();
    expect(await readdir(directory)).toContain(before[0]!);
    await valid.remove();
  });

  it("quarantines a replacement at the removal boundary instead of deleting it", async () => {
    const root = await temporaryDirectory();
    const replacement = JSON.stringify({ version: 2, instance: "replacement", token: "replacement-token" });
    const publisher = createTokenPublisher({
      configDirectory: () => root,
      beforeQuarantine: async (path) => { await writeFile(path, replacement, { mode: 0o600 }); },
    });
    const registration = await publisher.publish("127.0.0.1:41001", { connected: true, focused: true, generation: 1 });
    await registration.remove();
    const directory = tokenStatePath(root);
    const entries = await readdir(directory);
    expect(entries).toHaveLength(1);
    expect(entries[0]).toContain(".removing");
    expect(await readFile(join(directory, entries[0]!), "utf8")).toBe(replacement);
  });

  it("removes the owned quarantined record without leaving an artifact", async () => {
    const root = await temporaryDirectory();
    const registration = await createTokenPublisher({ configDirectory: () => root }).publish("127.0.0.1:41001", { connected: true, focused: true, generation: 1 });
    await registration.remove();
    expect(await readdir(tokenStatePath(root))).toEqual([]);
  });
});

async function temporaryDirectory(): Promise<string> { const root = await mkdtemp(join(tmpdir(), "nexus-companion-token-")); roots.push(root); return root; }
