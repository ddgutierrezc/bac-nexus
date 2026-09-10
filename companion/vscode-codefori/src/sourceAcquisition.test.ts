import { readFile, rm, stat, symlink, writeFile } from "node:fs/promises";
import { dirname } from "node:path";

import { describe, expect, it, vi } from "vitest";

import { MAX_SOURCE_PAGE_BYTES, MAX_SOURCE_PAGE_LINES, SOURCE_ARTIFACT_TTL_MS, acquireCatalogSource, type MemberContentProvider } from "./sourceAcquisition.js";

const candidate = {
  item: "PISA061", sourceLibrary: "SRCLIB", sourceFileBase: "Q", objectType: "RPGLE", sourceType: "RPGLE",
  application: "", version: "", productionLibrary: "", description: "",
};

function provider(content: string, returned = content): MemberContentProvider {
  return { downloadMemberContent: async (_library, _file, _member, localPath) => {
    await writeFile(localPath!, content, "utf8");
    return returned;
  } };
}

describe("Catalog source acquisition seam", () => {
  it("passes only the exact Catalogados coordinate and a private Nexus-owned local path", async () => {
    const calls: Array<[string, string, string, string | undefined]> = [];
    const source = provider("one\ntwo", "provider-owned whole member");
    const original = source.downloadMemberContent;
    source.downloadMemberContent = async (...args) => { calls.push([args[0], args[1], args[2], args[3]]); return original(...args); };

    const result = await acquireCatalogSource(candidate, source, () => true);

    expect(calls).toHaveLength(1);
    expect(calls[0]?.slice(0, 3)).toEqual(["SRCLIB", "QRPGLE", "PISA061"]);
    expect(calls[0]?.[3]).toContain("bac-nexus-source-");
    expect(result.state).toBe("ok");
    if (result.state === "ok") {
      expect(JSON.stringify(result)).not.toContain("provider-owned whole member");
      expect(JSON.stringify(result)).not.toContain(calls[0]![3]!);
      expect((await stat(calls[0]![3]!)).mode & 0o777).toBe(0o600);
      expect((await stat(dirname(calls[0]![3]!))).mode & 0o777).toBe(0o700);
      await expect(result.artifact.page(1)).resolves.toEqual({ state: "ok", content: "one\ntwo", startLine: 1, lineCount: 2, eof: true, nextLine: null });
      await result.artifact.dispose();
    }
  });

  it("does not reject an estimated or actual large member solely for size", async () => {
    const large = "x".repeat(MAX_SOURCE_PAGE_BYTES + 1);
    let estimates = 0;
    const source = Object.assign(provider(large), { getMemberList: async () => { estimates += 1; return [{ lines: 999999 }]; } });
    const result = await acquireCatalogSource(candidate, source, () => true);

    expect(result.state).toBe("ok");
    expect(estimates).toBe(0);
    if (result.state === "ok") {
      await expect(result.artifact.page(1)).resolves.toEqual({ state: "response_too_large" });
      await result.artifact.dispose();
    }
  });

  it("pages at most 200 complete UTF-8 lines from the private artifact", async () => {
    const content = Array.from({ length: MAX_SOURCE_PAGE_LINES + 1 }, (_value, index) => `línea-${index + 1}`).join("\n");
    const result = await acquireCatalogSource(candidate, provider(content), () => true);

    expect(result.state).toBe("ok");
    if (result.state === "ok") {
      await expect(result.artifact.page(2)).resolves.toEqual({ state: "ok", content: content.split("\n").slice(1, 201).join("\n"), startLine: 2, lineCount: 200, eof: true, nextLine: null });
      await result.artifact.dispose();
    }
  });

  it("rejects invalid UTF-8 without returning a partial page", async () => {
    const invalid: MemberContentProvider = { downloadMemberContent: async (_library, _file, _member, localPath) => {
      await writeFile(localPath!, Buffer.from([0xc3, 0x28]));
      return "provider-owned whole member";
    } };
    const result = await acquireCatalogSource(candidate, invalid, () => true);

    expect(result.state).toBe("ok");
    if (result.state === "ok") {
      await expect(result.artifact.page(1)).resolves.toEqual({ state: "invalid_source_encoding" });
      await result.artifact.dispose();
    }
  });

  it("bounds the complete serialized response and recognizes empty and final unterminated lines", async () => {
    const overhead = Buffer.byteLength(JSON.stringify({ state: "ok", content: "", startLine: 1, lineCount: 1, eof: false, nextLine: 2 }));
    const exact = "x".repeat(MAX_SOURCE_PAGE_BYTES - overhead);
    const accepted = await acquireCatalogSource(candidate, provider(`${exact}\nnext`), () => true);
    const empty = await acquireCatalogSource(candidate, provider(""), () => true);
    const oversized = await acquireCatalogSource(candidate, provider(`${exact}x\nnext`), () => true);

    if (accepted.state === "ok") {
      const page = await accepted.artifact.page(1, 1);
      expect(page).toEqual({ state: "ok", content: exact, startLine: 1, lineCount: 1, eof: false, nextLine: 2 });
      expect(Buffer.byteLength(JSON.stringify(page))).toBeLessThanOrEqual(MAX_SOURCE_PAGE_BYTES);
      await accepted.artifact.dispose();
    } else throw new Error("expected artifact");
    if (empty.state === "ok") {
      await expect(empty.artifact.page(1)).resolves.toEqual({ state: "ok", content: "", startLine: 1, lineCount: 0, eof: true, nextLine: null });
      await empty.artifact.dispose();
    } else throw new Error("expected empty artifact");
    if (oversized.state === "ok") {
      await expect(oversized.artifact.page(1, 1)).resolves.toEqual({ state: "response_too_large" });
    } else throw new Error("expected oversized artifact");
  });

  it("cleans private files and suppresses content for stale or failed providers", async () => {
    let stalePath = "";
    let current = true;
    const stale: MemberContentProvider = { downloadMemberContent: async (_library, _file, _member, localPath) => {
      stalePath = localPath!;
      await writeFile(stalePath, "must not escape", "utf8");
      current = false;
      return "must not escape";
    } };
    const staleResult = await acquireCatalogSource(candidate, stale, () => current);
    expect(staleResult).toEqual({ state: "stale_session" });

    let failedPath = "";
    const failed: MemberContentProvider = { downloadMemberContent: async (_library, _file, _member, localPath) => {
      failedPath = localPath!;
      await writeFile(failedPath, "must not escape", "utf8");
      throw new Error("host.example secret");
    } };
    const failedResult = await acquireCatalogSource(candidate, failed, () => true);
    expect(failedResult).toEqual({ state: "unavailable" });
    await expect(readFile(stalePath)).rejects.toMatchObject({ code: "ENOENT" });
    await expect(readFile(failedPath)).rejects.toMatchObject({ code: "ENOENT" });
    expect(JSON.stringify([staleResult, failedResult])).not.toContain("must not escape");
  });

  it("rejects a provider-created symbolic link without exposing its target", async () => {
    const linked: MemberContentProvider = { downloadMemberContent: async (_library, _file, _member, localPath) => {
      await symlink("/etc/passwd", localPath!);
      return "provider-owned whole member";
    } };

    await expect(acquireCatalogSource(candidate, linked, () => true)).resolves.toEqual({ state: "missing" });
  });

  it("expires and removes artifacts without an unhandled cleanup rejection", async () => {
    vi.useFakeTimers();
    let localPath = "";
    const source: MemberContentProvider = { downloadMemberContent: async (_library, _file, _member, path) => {
      localPath = path!;
      await writeFile(localPath, "line");
      return "provider-owned whole member";
    } };
    const result = await acquireCatalogSource(candidate, source, () => true);
    if (result.state !== "ok") throw new Error("expected artifact");

    await vi.advanceTimersByTimeAsync(SOURCE_ARTIFACT_TTL_MS);

    await expect(result.artifact.page(1)).resolves.toEqual({ state: "expired" });
    await expect(readFile(localPath)).rejects.toMatchObject({ code: "ENOENT" });
    vi.useRealTimers();
  });

  it("reports cleanup failures without leaking provider details", async () => {
    let localPath = "";
    const source: MemberContentProvider = { downloadMemberContent: async (_library, _file, _member, path) => {
      localPath = path!;
      await writeFile(localPath, "line");
      return "provider-owned whole member";
    } };
    const result = await acquireCatalogSource(candidate, source, () => true, async () => false);
    if (result.state !== "ok") throw new Error("expected artifact");

    await expect(result.artifact.dispose()).resolves.toEqual({ state: "cleanup_failed" });
    await expect(result.artifact.page(1)).resolves.toEqual({ state: "cleanup_failed" });
    expect(JSON.stringify(result)).not.toContain(localPath);
    await rm(dirname(localPath), { force: true, recursive: true });
  });

  it("reports a provider-failure cleanup error without exposing provider details", async () => {
    let localPath = "";
    const failed: MemberContentProvider = { downloadMemberContent: async (_library, _file, _member, path) => {
      localPath = path!;
      await writeFile(localPath, "must not escape");
      throw new Error("host.example secret");
    } };

    const result = await acquireCatalogSource(candidate, failed, () => true, async () => false);

    expect(result).toEqual({ state: "cleanup_failed" });
    expect(JSON.stringify(result)).not.toMatch(/host\.example|secret|must not escape/);
    await rm(dirname(localPath), { force: true, recursive: true });
  });

  it("contains rejecting TTL cleanup without an unhandled rejection", async () => {
    vi.useFakeTimers();
    const unhandled: unknown[] = [];
    const observe = (reason: unknown) => { unhandled.push(reason); };
    process.once("unhandledRejection", observe);
    const result = await acquireCatalogSource(candidate, provider("line"), () => true, async () => { throw new Error("filesystem path secret"); });
    if (result.state !== "ok") throw new Error("expected artifact");

    await vi.advanceTimersByTimeAsync(SOURCE_ARTIFACT_TTL_MS);
    await Promise.resolve();

    await expect(result.artifact.page(1)).resolves.toEqual({ state: "cleanup_failed" });
    expect(unhandled).toEqual([]);
    process.off("unhandledRejection", observe);
    vi.useRealTimers();
  });
});
