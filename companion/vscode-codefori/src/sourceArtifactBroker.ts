import { randomBytes } from "node:crypto";

import type { CodeForIAdapter } from "./codeforiAdapter.js";
import { MAX_SOURCE_RESPONSE_BYTES, type CatalogCandidate, type RpcRequest, type SourceArtifactResult, type SourcePageParams } from "./protocol.js";
import { type SourceArtifact, type SourcePageResult } from "./sourceAcquisition.js";

interface Record {
  artifact: SourceArtifact;
  candidate: CatalogCandidate;
  generation: number;
}
type SourcePage = { content: string; start_line: number; line_count: number; eof: boolean };

export interface SourceArtifactBroker {
  page(request: RpcRequest): Promise<SourceArtifactResult>;
  dispose(cursor: string): Promise<SourceArtifactResult>;
  deactivate(): Promise<void>;
}

export function createSourceArtifactBroker(adapter: CodeForIAdapter): SourceArtifactBroker {
  const records = new Map<string, Record>();
  let active = true;
  const clear = (): void => { void disposeAll().catch(() => undefined); };
  const unsubscribe = adapter.onSessionChange(clear);

  return {
    async page(request): Promise<SourceArtifactResult> {
      if (!active) return { state: "unavailable" };
      const params = request.params as SourcePageParams;
      if ("cursor" in params) {
        const record = records.get(params.cursor);
        return record ? pageRecord(request, params.cursor, record, params.start_line, params.max_lines) : { state: "expired" };
      }
      const generation = adapter.sourceSessionGeneration();
      if (generation === undefined) return { state: "unavailable" };
      const resolution = await adapter.resolveCatalogCandidates({ item: params.candidate.item, productionLibrary: params.candidate.productionLibrary });
      if (!adapter.isSourceSessionCurrent(generation)) return { state: "unavailable" };
      if (resolution.state !== "ok") return { state: resolution.state === "invalid_request" ? "invalid_request" : "unavailable" };
      const matches = resolution.candidates.filter((candidate) => exactCandidate(candidate, params.candidate));
      if (matches.length === 0) return { state: "not_found" };
      if (matches.length !== 1) return { state: "ambiguous" };
      const acquired = await adapter.acquireCatalogSource(matches[0]!, generation);
      if (acquired.state !== "ok") return { state: acquired.state === "missing" ? "not_found" : acquired.state === "invalid_coordinate" ? "invalid_request" : acquired.state === "stale_session" ? "unavailable" : acquired.state };
      if (!active || !adapter.isSourceSessionCurrent(generation)) {
        await acquired.artifact.dispose().catch(() => undefined);
        return { state: "unavailable" };
      }
      const cursor = randomBytes(32).toString("base64url");
      const record = { artifact: acquired.artifact, candidate: matches[0]!, generation };
      records.set(cursor, record);
      return pageRecord(request, cursor, record, params.start_line, params.max_lines);
    },
    async dispose(cursor): Promise<SourceArtifactResult> {
      const record = records.get(cursor);
      if (!record) return { state: "expired" };
      records.delete(cursor);
      return (await record.artifact.dispose()).state === "disposed" ? { state: "disposed" } : { state: "cleanup_failed" };
    },
    async deactivate(): Promise<void> {
      active = false;
      unsubscribe();
      await disposeAll();
    },
  };

  async function pageRecord(request: RpcRequest, cursor: string, record: Record, startLine: number, maxLines: number): Promise<SourceArtifactResult> {
    if (!adapter.isSourceSessionCurrent(record.generation)) return discard(cursor, record);
    const overhead = responseBytes(request, cursor, { content: "", start_line: startLine, line_count: 0, eof: false });
    let page: SourcePageResult;
    try {
      page = await record.artifact.page(startLine, maxLines, MAX_SOURCE_RESPONSE_BYTES - overhead);
    } catch {
      return discard(cursor, record);
    }
    if (!adapter.isSourceSessionCurrent(record.generation)) return discard(cursor, record);
    const result = toResult(cursor, page);
    if (result.state !== "ok") {
      records.delete(cursor);
      return result;
    }
    if (responseBytes(request, cursor, result.page) > MAX_SOURCE_RESPONSE_BYTES) {
      records.delete(cursor);
      await record.artifact.dispose().catch(() => undefined);
      return { state: "response_too_large" };
    }
    return result;
  }

  async function disposeAll(): Promise<void> {
    const current = [...records.entries()];
    records.clear();
    await Promise.all(current.map(([, record]) => record.artifact.dispose().catch(() => undefined)));
  }

  async function discard(cursor: string, record: Record): Promise<SourceArtifactResult> {
    records.delete(cursor);
    await record.artifact.dispose().catch(() => undefined);
    return { state: "unavailable" };
  }
}

function exactCandidate(left: CatalogCandidate, right: CatalogCandidate): boolean {
  return left.item === right.item && left.sourceLibrary === right.sourceLibrary && left.sourceFileBase === right.sourceFileBase && left.objectType === right.objectType && left.sourceType === right.sourceType;
}

function toResult(cursor: string, page: SourcePageResult): SourceArtifactResult {
  if (page.state !== "ok") return { state: page.state };
  return { state: "ok", cursor, page: { content: page.content, start_line: page.startLine, line_count: page.lineCount, eof: page.eof } };
}

function responseBytes(request: RpcRequest, cursor: string, page: SourcePage): number {
  return new TextEncoder().encode(JSON.stringify({ version: request.version, request_id: request.requestID, result: { state: "ok", cursor, page } })).byteLength;
}
