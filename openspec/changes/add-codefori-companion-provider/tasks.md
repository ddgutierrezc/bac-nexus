# Tasks: deployable Code for IBM i Companion v1

Revision 6 preserves Slice 1–7 history in `apply-progress.md`; only reusable work stays checked. Obsolete security work is retired.

## Review Workload Forecast

| Field | Value |
|---|---|
| Estimated changed lines | 1,600–2,100 additions + deletions |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | three units; PR 3 cohesive, approved `size:exception`: measured 1,500–1,650; hard cap 1,650 additions+deletions |
| Delivery strategy | ask-on-risk |
| Chain strategy | feature-branch-chain |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: feature-branch-chain
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|---|---|---|---|---|---|
| 1 | Direct Go client | PR 1 base = feature/tracker branch | `go test -count=1 ./internal/connectors/ibmi/codefori ./cmd/nexus` | N/A: fake transport | client/protocol/CLI edits |
| 2 | Activated HTTP Companion | PR 2 base = PR 1 branch | `npm test -- --run src/{httpServer,extension,broker}.test.ts` | offline loopback + fakes | extension/server/broker |
| 3 | Retire security/docs (approved exception) | PR 3 base = PR 2 branch | `npm ci && npm test && go test -count=1 ./internal/connectors/ibmi/codefori` | N/A: cleanup only | descriptor, Windows, docs |

## Phase 0: Reconciled reusable completion

- [x] 0.1 Slice 1: retain `internal/provider/provider.go` query/result validation and tests.
- [x] 0.2 Slice 2: retain `internal/connectors/ibmi/codefori/protocol.go` strict bounds; replace auth fields.
- [x] 0.3 Slice 3: retain MCP surface/Native isolation in `internal/mcp/codefori.go` and `cmd/nexus/main_test.go`.
- [x] 0.4 Slices 4–5: retain recognizer, adapter, broker normalization, bounds, and correlation in `companion/vscode-codefori/src/`.
- [x] 0.5 Slice 5: retain 16-slot admission, settlement, cancellation, and reverse-completion tests in `companion/vscode-codefori/src/{admission.ts,broker.test.ts}`.

## Phase 1: RED contracts

- [x] 1.1 RED: add `companion/vscode-codefori/src/httpServer.test.ts`: any-case `Origin` (including empty), valid/malformed/oversized bodies reject before parse/handler.
- [x] 1.2 RED: add `companion/vscode-codefori/src/httpServer.test.ts` lifecycle cases: only `127.0.0.1:64139`, collision, repeated start/deactivate, no alternate bind/fallback.
- [x] 1.3 RED: update `cmd/nexus/main_test.go` construction counters: no profile remains Companion, `-profile` remains Native, unavailable never switches.
- [x] 1.4 RED: revise `internal/connectors/ibmi/codefori/{client_test.go,protocol_test.go}` for direct POST without descriptor/bearer/token/generation; retain correlation/bounds.
- [x] 1.5 RED: add `companion/vscode-codefori/src/extension.test.ts` activation cases for exported Code for IBM i instance and no-auth fields.

## Phase 2: Green implementation

- [x] 2.1 Replace `internal/connectors/ibmi/codefori/{client.go,protocol.go}` with a fixed unauthenticated client; remove `-provider` from `cmd/nexus/{main,codefori}.go`; keep Native `-profile`.
- [x] 2.2 Wire `companion/vscode-codefori/src/{extension.ts,httpServer.ts,broker.ts,protocol.ts}` to activate Code for IBM i, bind loopback, Origin-gate first, and preserve bounds/admission.

## Phase 3: Retire and document

- [x] 3.1 Delete `internal/connectors/ibmi/codefori/{descriptor.go,descriptor_windows.go,descriptor_other.go,descriptor_windows_test.go}`; prove no descriptor references remain.
- [x] 3.2 Delete `companion/vscode-codefori/src/windows/{tokenStore.ts,tokenStore.test.ts,tokenStore.windows.test.ts,publish.ps1,cleanup.ps1}` and Windows ACL workflow steps; reduce `.github/workflows/release-companion.yml` to offline Go/Node checks; pin `companion/vscode-codefori/{package.json,package-lock.json}` to exact `vitest@3.2.7` only.
- [x] 3.3 Add `docs/CODEFORI_COMPANION.md`: local-process risk, deferred hardening, fixed-port failure, rollback, no fallback.

## Phase 4: Verification

- [x] 4.1 Run offline Go/Node lint, typecheck, tests, build/package checks; record fake/loopback evidence as not IBM i proof.
- [ ] 4.2 Separately authorize and run Extension Development Host from `companion/vscode-codefori/` (read-only) with Code for IBM i 3.0.12 and an active session; prove only `session.status`. Live `sql.query` requires separate authorization.
