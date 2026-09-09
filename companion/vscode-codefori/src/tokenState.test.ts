import { mkdtemp, mkdir, readFile, readdir, rm, symlink, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { afterEach, describe, expect, it } from "vitest";

import { createTokenPublisher, tokenStatePath } from "./tokenState.js";

const roots: string[] = [];

afterEach(async () => {
  await Promise.all(roots.splice(0).map((root) => rm(root, { force: true, recursive: true })));
});

describe("Companion private token state", () => {
  it("atomically publishes a fresh v1 token and invalidates the prior generation", async () => {
    const root = await temporaryDirectory();
    const publisher = createTokenPublisher({ configDirectory: () => root });

    const first = await publisher.publish();
    const path = tokenStatePath(root);
    const before = await readState(path);
    expect(before.version).toBe(1);
    expect(before.token).toMatch(/^[A-Za-z0-9_-]{43}$/);
    expect(first({ "x-nexus-companion-token": before.token })).toBe(true);

    const second = await publisher.publish();
    const after = await readState(path);
    expect(after.token).not.toBe(before.token);
    expect(first({ "x-nexus-companion-token": after.token })).toBe(false);
    expect(second({ "x-nexus-companion-token": before.token })).toBe(false);
    expect(second({ "x-nexus-companion-token": after.token })).toBe(true);
    expect((await readdir(join(root, "bac-nexus"))).every((name) => !name.endsWith(".tmp"))).toBe(true);
  });

  it("fails closed when publication cannot create a private state directory", async () => {
    const root = join(await temporaryDirectory(), "not-a-directory");
    await writeFile(root, "x");
    await expect(createTokenPublisher({ configDirectory: () => root }).publish()).rejects.toThrow("private token state unavailable");
  });

  it("fails closed for a symlinked prior state", async () => {
    const root = await temporaryDirectory();
    const directory = join(root, "bac-nexus");
    await mkdir(directory, { mode: 0o700 });
    const target = join(root, "target");
    await writeFile(target, "x");
    try {
      await symlink(target, tokenStatePath(root));
    } catch {
      return;
    }
    await expect(createTokenPublisher({ configDirectory: () => root }).publish()).rejects.toThrow("private token state unavailable");
  });
});

async function temporaryDirectory(): Promise<string> {
  const root = await mkdtemp(join(tmpdir(), "nexus-companion-token-"));
  roots.push(root);
  return root;
}

async function readState(path: string): Promise<{ version: number; token: string }> {
  return JSON.parse(await readFile(path, "utf8")) as { version: number; token: string };
}
