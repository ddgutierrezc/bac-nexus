import { constants } from "node:fs";
import { chmod, mkdtemp, open, rm, type FileHandle } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

import type { CatalogCandidate } from "./protocol.js";

export const MAX_SOURCE_PAGE_LINES = 200;
export const MAX_SOURCE_PAGE_BYTES = 128 * 1024;
export const SOURCE_ARTIFACT_TTL_MS = 10 * 60 * 1000;

const encoder = new TextEncoder();
const SYSTEM_NAME = /^[A-Z$#@][A-Z0-9_$#@]{0,9}$/;

export interface MemberContentProvider {
  downloadMemberContent(library: string, file: string, member: string, localPath?: string): Promise<string | undefined>;
}

export type SourceAcquisitionResult =
  | { state: "ok"; artifact: SourceArtifact }
  | { state: "invalid_coordinate" | "unavailable" | "missing" | "stale_session" | "cleanup_failed" };
export type SourcePageResult =
  | { state: "ok"; content: string; startLine: number; lineCount: number; eof: boolean; nextLine: number | null }
  | { state: "invalid_request" | "unavailable" | "expired" | "invalid_source_encoding" | "response_too_large" | "cleanup_failed" };
export type SourceCleanupResult = { state: "disposed" | "cleanup_failed" };

// The only provider contract used here is Code for IBM i's documented
// downloadMemberContent. Its returned whole-member string is deliberately
// ignored: the provider owns that transient allocation, while Nexus pages its
// own private local artifact. The API cannot preflight transfer size or cancel.
export async function acquireCatalogSource(
  candidate: CatalogCandidate,
  provider: MemberContentProvider | undefined,
  stillCurrent: () => boolean,
  cleanup = removePrivateDirectory,
): Promise<SourceAcquisitionResult> {
  const coordinate = coordinateFromCatalog(candidate);
  if (!coordinate) return { state: "invalid_coordinate" };
  if (!provider || !stillCurrent()) return { state: "unavailable" };

  let directory: string | undefined;
  let accepted = false;
  let result: SourceAcquisitionResult;
  try {
    directory = await mkdtemp(join(tmpdir(), "bac-nexus-source-"));
    await chmod(directory, 0o700);
    const localPath = join(directory, "member");
    await provider.downloadMemberContent(coordinate.library, coordinate.file, coordinate.member, localPath);
    if (!stillCurrent()) result = { state: "stale_session" };
    else {
      const opened = await openProducedArtifact(localPath);
      if (opened.cleanupFailed) result = { state: "cleanup_failed" };
      else if (!opened.handle) result = { state: "missing" };
      else {
        const artifact = new SourceArtifact(directory, opened.handle, cleanup);
        accepted = true;
        result = { state: "ok", artifact };
      }
    }
  } catch {
    result = stillCurrent() ? { state: "unavailable" } : { state: "stale_session" };
  }
  if (directory && !accepted && !(await cleanupSafely(cleanup, directory))) return { state: "cleanup_failed" };
  return result;
}

// SourceArtifact keeps its local path private and self-cleans on expiry. It is
// an internal seam, not a cursor or a public MCP contract.
export class SourceArtifact {
  #disposed = false;
  #cleanupFailed = false;
  #expiry: ReturnType<typeof setTimeout>;
  #expiryCleanup: Promise<SourceCleanupResult> | undefined;
  #directory: string;
  #handle: FileHandle;
  #cleanup: (directory: string) => Promise<boolean>;

  constructor(directory: string, handle: FileHandle, cleanup = removePrivateDirectory) {
    this.#directory = directory;
    this.#handle = handle;
    this.#cleanup = cleanup;
    this.#expiry = setTimeout(() => { this.#expiryCleanup = this.dispose(); }, SOURCE_ARTIFACT_TTL_MS);
    this.#expiry.unref?.();
  }

  async page(startLine: number, maximumLines = MAX_SOURCE_PAGE_LINES): Promise<SourcePageResult> {
    if (this.#expiryCleanup) return (await this.#expiryCleanup).state === "disposed" ? { state: "expired" } : { state: "cleanup_failed" };
    if (this.#cleanupFailed) return { state: "cleanup_failed" };
    if (this.#disposed) return { state: "expired" };
    if (!Number.isSafeInteger(startLine) || startLine < 1 || !Number.isSafeInteger(maximumLines) || maximumLines < 1 || maximumLines > MAX_SOURCE_PAGE_LINES) return { state: "invalid_request" };
    const result = await readPage(this.#handle, startLine, maximumLines);
    if (result.state === "ok") return result;
    return (await this.dispose()).state === "disposed" ? result : { state: "cleanup_failed" };
  }

  async dispose(): Promise<SourceCleanupResult> {
    if (this.#cleanupFailed) return { state: "cleanup_failed" };
    if (this.#disposed) return { state: "disposed" };
    this.#disposed = true;
    clearTimeout(this.#expiry);
    const closed = await this.#handle.close().then(() => true, () => false);
    const removed = await cleanupSafely(this.#cleanup, this.#directory);
    this.#cleanupFailed = !closed || !removed;
    return { state: this.#cleanupFailed ? "cleanup_failed" : "disposed" };
  }
}

async function readPage(handle: FileHandle, startLine: number, maximumLines: number): Promise<SourcePageResult> {
  const decoder = new TextDecoder("utf-8", { fatal: true });
  const lines: string[] = [];
  let currentLine = 1;
  let pending = "";
  try {
    for await (const chunk of handle.createReadStream({ autoClose: false, start: 0 })) {
      try {
        pending += decoder.decode(chunk, { stream: true });
      } catch {
        return { state: "invalid_source_encoding" };
      }
      const result = consumeLines();
      if (result) return result;
      if (lines.length === maximumLines && pending !== "") return pageResult(lines, startLine, false);
      if (encoder.encode(pending).byteLength > MAX_SOURCE_PAGE_BYTES) return { state: "response_too_large" };
    }
    try {
      pending += decoder.decode();
    } catch {
      return { state: "invalid_source_encoding" };
    }
    const result = consumeLines(true);
    return result ?? pageResult(lines, startLine, true);
  } catch {
    return { state: "unavailable" };
  }

  function consumeLines(final = false): SourcePageResult | undefined {
    for (;;) {
      const newline = pending.indexOf("\n");
      if (newline < 0) break;
      if (lines.length === maximumLines) return pageResult(lines, startLine, false);
      const line = pending.slice(0, newline).replace(/\r$/, "");
      pending = pending.slice(newline + 1);
      const result = addLine(line);
      if (result) return result;
    }
    if (final && pending !== "") return lines.length === maximumLines ? pageResult(lines, startLine, false) : addLine(pending.replace(/\r$/, ""));
    return undefined;
  }

  function addLine(line: string): SourcePageResult | undefined {
    if (currentLine++ < startLine) return undefined;
    const content = [...lines, line].join("\n");
    if (marshaledBytes({ state: "ok", content, startLine, lineCount: lines.length + 1, eof: false, nextLine: startLine + lines.length + 1 }) > MAX_SOURCE_PAGE_BYTES) return { state: "response_too_large" };
    lines.push(line);
    return undefined;
  }
}

function pageResult(lines: string[], startLine: number, eof: boolean): SourcePageResult {
  const page = { state: "ok" as const, content: lines.join("\n"), startLine, lineCount: lines.length, eof, nextLine: eof ? null : startLine + lines.length };
  return marshaledBytes(page) <= MAX_SOURCE_PAGE_BYTES ? page : { state: "response_too_large" };
}

function marshaledBytes(value: unknown): number {
  return encoder.encode(JSON.stringify(value)).byteLength;
}

async function openProducedArtifact(localPath: string): Promise<{ handle?: FileHandle; cleanupFailed: boolean }> {
  let handle: FileHandle | undefined;
  try {
    // O_NOFOLLOW is unavailable on Windows; the private random directory is the
    // fallback there, not a claim of equivalent same-identity race resistance.
    handle = await open(localPath, constants.O_RDONLY | (process.platform === "win32" ? 0 : constants.O_NOFOLLOW));
    if (!(await handle.stat()).isFile()) throw new Error("not a regular file");
    await handle.chmod(0o600);
    return { handle, cleanupFailed: false };
  } catch {
    const closed = handle ? await handle.close().then(() => true, () => false) : true;
    return { cleanupFailed: !closed };
  }
}

async function removePrivateDirectory(directory: string): Promise<boolean> {
  try {
    await rm(directory, { force: true, recursive: true });
    return true;
  } catch {
    return false;
  }
}

async function cleanupSafely(cleanup: (directory: string) => Promise<boolean>, directory: string): Promise<boolean> {
  try {
    return await cleanup(directory);
  } catch {
    return false;
  }
}

function coordinateFromCatalog(candidate: CatalogCandidate): { library: string; file: string; member: string } | undefined {
  const library = exactSystemName(candidate.sourceLibrary);
  const sourceFileBase = exactSystemName(candidate.sourceFileBase);
  const objectType = exactSystemName(candidate.objectType);
  const member = exactSystemName(candidate.item);
  if (!library || !sourceFileBase || !objectType || !member || !exactSystemName(candidate.sourceType)) return undefined;
  const file = `${sourceFileBase}${objectType}`;
  return SYSTEM_NAME.test(file) ? { library, file, member } : undefined;
}

function exactSystemName(value: string): string | undefined {
  return SYSTEM_NAME.test(value) ? value : undefined;
}
