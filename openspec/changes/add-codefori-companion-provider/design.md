---
schema: gentle-ai.sdd-design/v1
revision: 5
change: add-codefori-companion-provider
artifact_store: openspec
skill_resolution: paths-injected
status: ready_for_task_revision
---

# Design: zero-configuration Code for IBM i Companion

## Technical approach

Replace the unfinished descriptor-authenticated path with one fixed, unauthenticated machine-local HTTP/JSON provider. Code for IBM i exclusively owns its connection and credentials. Companion exposes only bounded status and the canonical proof query; Nexus owns MCP adaptation, not IBM i connectivity. This is an accepted v1 local-process trust boundary, not user, session, process, or extension identity.

## Architecture decisions

| Option | Tradeoff | Decision |
|---|---|---|
| Descriptor, bearer token, generation, ACL | Attempts peer authentication but adds configuration and fragile Windows process/filesystem work | Remove completely; defer strong peer authentication and endpoint hardening. |
| Fixed `127.0.0.1:64139` | A local process can call it and a collision prevents service | Accept for v1; never scan, bind `localhost`/wildcard, forward, retry another port, or fall back. |
| Versioned strict JSON with request ID | Correlates responses but authenticates nobody | Retain protocol version, random request ID, strict shapes, and response/body bounds; remove token and generation fields/checks. |

## Activation and data flow

```text
VS Code activates Companion
  -> resolve/activate `halcyontechltd.code-for-ibmi`
  -> read exported `CodeForIBMi.instance`
  -> adapter subscribes to connected/disconnected
  -> broker binds 127.0.0.1:64139

MCP client -> `nexus serve` -> `codefori.Client` (`provider.Provider`) -> POST /v1/rpc
  -> Origin gate -> strict decode/correlation -> admission
  -> adapter -> instance.getConnection().runSQL(canonical, {rows: 1})
  -> normalized bounded result -> MCP tool
```

The Companion extension owns listener creation/closure, adapter references, admission slots, and deactivation. Code for IBM i owns all IBM i credentials and connection lifecycle. Nexus owns the fixed client, request deadlines, result validation, and exactly `session.status`/`sql.query`. Deactivation closes intake/listener and invalidates the adapter. Timeout, cancellation, disconnect, or deactivation suppresses late output; an admitted query retains capacity until its `runSQL` promise settles.

## Interfaces and failure order

`POST /v1/rpc` requests contain exactly `version`, `request_id`, `method`, and `params`; responses echo `version` and `request_id` plus `result`. Correlation detects mismatches but conveys no identity. Any case-insensitive `Origin` header, including empty, is rejected first with `403` and exactly `{"state":"browser_origin_rejected"}`, before decoding, normalization, admission, or execution. Then enforce route/method, 512-byte body, strict JSON, allowlist, canonical 41-byte statement, 16 immediate operations/active queries, five-second wait, one row/column, 256-byte UTF-8 value, and 1024-byte response.

Listener collision/start failure leaves Companion unavailable. `nexus serve` still starts Companion MCP and returns bounded unavailable states. Profile absence always selects Companion; `-profile <name>` always selects the unchanged Native composition. Neither availability nor failure switches providers.

## File changes

| Action | Files |
|---|---|
| Modify | `companion/vscode-codefori/src/{extension,codeforiAdapter,broker,protocol,admission}.ts` and colocated tests; add concrete `httpServer.ts`/test. |
| Modify | `companion/vscode-codefori/{package.json,package-lock.json,tsconfig.json}`; add VS Code dependency/activation/package metadata and `.vscodeignore`. |
| Modify | `internal/connectors/ibmi/codefori/{client,protocol}.go` and tests: direct fixed client, no auth/generation, retained request correlation/bounds. |
| Modify | `cmd/nexus/{main,codefori}.go`, `cmd/nexus/main_test.go`, `internal/mcp/codefori_test.go`: remove `-provider`; use profile-only selection and isolated surfaces. |
| Delete | `internal/connectors/ibmi/codefori/{descriptor.go,descriptor_windows.go,descriptor_other.go,descriptor_windows_test.go}`. |
| Delete | `companion/vscode-codefori/src/windows/{tokenStore.ts,tokenStore.test.ts,tokenStore.windows.test.ts,publish.ps1,cleanup.ps1}`. |
| Modify/Create | Replace Windows harness in `.github/workflows/release-companion.yml` with offline Go/Node lint, typecheck, test, build/package checks; create `docs/CODEFORI_COMPANION.md`. |

## Testing strategy

Fake-backed Go/TypeScript RED tests cover strict protocol, no auth/generation fields or descriptor access, Origin-first rejection, fixed binding/collision, exact tools, profile isolation, bounds, ten concurrent reverse completions, capacity retention, and late-result suppression. CI performs no live IBM i work. One separately recorded Extension Development Host proof uses Code for IBM i 3.0.12 and an already active session to verify activation plus `session.status`; executing `sql.query` against IBM i requires separate authorization.

## Threat matrix

| Boundary | Adversarial cases | Applicability | Safe/failure behavior | Planned RED tests |
|---|---|---|---|---|
| Documentation-like paths | requirements, CMake, executable MDX, `README.sh` | N/A — no classification/execution | None | None |
| Git repository selection | `git -C`, relative/absolute paths | N/A — no Git invocation | None | None |
| Commit state | staged, `commit -a`, empty index | N/A — no commit automation | None | None |
| Push state | tracking, first push, refspec | N/A — no push automation | None | None |
| PR commands | `--head`, environment prefix, composed commands | N/A — no PR automation | None | None |
| HTTP routing/origin | Origin plus valid/malformed/oversized body | Applicable | Reject before decode/handler | Each causes zero handler calls. |
| Listener lifecycle | collision, repeated start/deactivate | Applicable | One fixed bind; close resources, no alternate route | Prove no second bind/fallback. |
| CLI provider routing | profile absent/present; provider unavailable | Applicable | Select one composition | Prove isolation/no fallback with construction counters. |

## Migration / rollback

No credential or data migration is required; obsolete descriptors are ignored and may be removed as residue. Rollback disables/uninstalls Companion and uses `nexus serve -profile <name>` for unchanged Native mode.

## Open questions

None.
