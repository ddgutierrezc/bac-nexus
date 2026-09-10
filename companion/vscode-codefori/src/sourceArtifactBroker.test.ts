import { describe, expect, it, vi } from "vitest";

import type { CodeForIAdapter } from "./codeforiAdapter.js";
import { encodeResponse, MAX_SOURCE_RESPONSE_BYTES, type CatalogCandidate, type RpcRequest } from "./protocol.js";
import { createSourceArtifactBroker } from "./sourceArtifactBroker.js";
import type { SourceArtifact } from "./sourceAcquisition.js";

const candidate: CatalogCandidate = { item: "PISA061", sourceLibrary: "SRCLIB", sourceFileBase: "Q", objectType: "RPGLE", sourceType: "RPGLE", application: "APP", version: "V1", productionLibrary: "PROD", description: "source" };
const request = (params: RpcRequest["params"]): RpcRequest => ({ version: 1, requestID: "source-page", method: "source_artifact.page.v1", params });

function setup(matches: CatalogCandidate[] = [candidate]) {
  let sessionChange: (() => void) | undefined;
  let generation = 1;
  const artifactPage = vi.fn(async () => ({ state: "ok" as const, content: "one\ntwo", startLine: 1, lineCount: 2, eof: true, nextLine: null }));
  const artifactDispose = vi.fn(async () => ({ state: "disposed" as const }));
  const artifact = { page: artifactPage, dispose: artifactDispose } as unknown as SourceArtifact;
  const acquireCatalogSource = vi.fn(async () => ({ state: "ok" as const, artifact }));
  const adapter = {
    resolveCatalogCandidates: vi.fn(async () => ({ state: "ok" as const, candidates: matches })),
    sourceSessionGeneration: vi.fn(() => generation),
    isSourceSessionCurrent: vi.fn((value: number) => value === generation),
    acquireCatalogSource,
    onSessionChange: vi.fn((callback: () => void) => { sessionChange = callback; return () => undefined; }),
  } as unknown as CodeForIAdapter;
  return { adapter, artifact, artifactPage, artifactDispose, acquireCatalogSource, sessionChange: () => { generation += 1; sessionChange?.(); } };
}

describe("source artifact broker", () => {
  it("re-resolves and acquires only one exact selected candidate, then pages by opaque cursor", async () => {
    const { adapter, artifact } = setup([{ ...candidate, description: "fresh" }]);
    const broker = createSourceArtifactBroker(adapter);

    const first = await broker.page(request({ candidate, start_line: 1, max_lines: 2 }));
    expect(first).toMatchObject({ state: "ok", page: { content: "one\ntwo", start_line: 1, line_count: 2, eof: true } });
    if (first.state !== "ok") throw new Error("expected source page");
    expect(first.cursor).toMatch(/^[A-Za-z0-9_-]{43}$/);
    expect(adapter.resolveCatalogCandidates).toHaveBeenCalledWith({ item: "PISA061", productionLibrary: "PROD" });
    expect(adapter.acquireCatalogSource).toHaveBeenCalledWith({ ...candidate, description: "fresh" }, 1);

    await broker.page(request({ cursor: first.cursor, start_line: 2, max_lines: 1 }));
    expect(artifact.page).toHaveBeenLastCalledWith(2, 1, expect.any(Number));
    await expect(broker.dispose(first.cursor)).resolves.toEqual({ state: "disposed" });
    await expect(broker.page(request({ cursor: first.cursor, start_line: 1, max_lines: 1 }))).resolves.toEqual({ state: "expired" });
  });

  it("fails closed for missing or ambiguous current catalog coordinates and disposes records on session change", async () => {
    const missing = setup([]);
    await expect(createSourceArtifactBroker(missing.adapter).page(request({ candidate, start_line: 1, max_lines: 1 }))).resolves.toEqual({ state: "not_found" });
    expect(missing.adapter.acquireCatalogSource).not.toHaveBeenCalled();

    const active = setup([candidate, { ...candidate, description: "duplicate" }]);
    await expect(createSourceArtifactBroker(active.adapter).page(request({ candidate, start_line: 1, max_lines: 1 }))).resolves.toEqual({ state: "ambiguous" });
    const current = setup();
    const broker = createSourceArtifactBroker(current.adapter);
    const result = await broker.page(request({ candidate, start_line: 1, max_lines: 1 }));
    current.sessionChange();
    await Promise.resolve();
    expect(current.artifact.dispose).toHaveBeenCalledOnce();
    if (result.state === "ok") await expect(broker.page(request({ cursor: result.cursor, start_line: 1, max_lines: 1 }))).resolves.toEqual({ state: "expired" });
  });

  it("accounts for the RPC envelope before returning source content", async () => {
    const { adapter, artifactPage } = setup();
    artifactPage.mockResolvedValueOnce({ state: "ok", content: "x".repeat(MAX_SOURCE_RESPONSE_BYTES), startLine: 1, lineCount: 1, eof: true, nextLine: null });
    const result = await createSourceArtifactBroker(adapter).page(request({ candidate, start_line: 1, max_lines: 1 }));
    expect(result).toEqual({ state: "response_too_large" });
    expect(encodeResponse(request({ candidate, start_line: 1, max_lines: 1 }), result).byteLength).toBeLessThanOrEqual(MAX_SOURCE_RESPONSE_BYTES);
  });

  it("does not register or return a first-page cursor after its captured session changes", async () => {
    const { adapter, artifact, acquireCatalogSource, sessionChange } = setup();
    let finish!: () => void;
    acquireCatalogSource.mockImplementation(() => new Promise((resolve) => { finish = () => resolve({ state: "ok", artifact }); }));
    const broker = createSourceArtifactBroker(adapter);
    const pending = broker.page(request({ candidate, start_line: 1, max_lines: 1 }));
    for (let attempt = 0; attempt < 10 && !finish; attempt += 1) await Promise.resolve();
    sessionChange();
    finish();

    await expect(pending).resolves.toEqual({ state: "unavailable" });
    expect(artifact.dispose).toHaveBeenCalledOnce();
    expect(artifact.page).not.toHaveBeenCalled();
  });

  it("sanitizes rejected artifact pages and immediately disposes the record", async () => {
    const { adapter, artifact, artifactPage } = setup();
    artifactPage.mockRejectedValueOnce(new Error("provider path secret"));

    await expect(createSourceArtifactBroker(adapter).page(request({ candidate, start_line: 1, max_lines: 1 }))).resolves.toEqual({ state: "unavailable" });
    expect(artifact.dispose).toHaveBeenCalledOnce();
  });
});
