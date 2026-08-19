# Design: v1 MCP Foundation

## Technical Approach

Add `cmd/nexus` over application, MCP, security, audit, catalog, and source seams. Pin official `github.com/modelcontextprotocol/go-sdk` v1.2.0 and use typed `mcp.AddTool` with stdio `Server.Run`. Expose only `resolve_catalog_candidates` and `read_selected_source`.

## Architecture Decisions

| Option | Tradeoff | Decision |
|---|---|---|
| Stateless copies / durable cache / immutable memory | Repeated copies mix revisions; disk persists sensitive data; memory needs quotas | Immutable process-memory leases: 4 MiB/member, 16 MiB resident aggregate, 10-minute idle TTL |
| Caller ranges / opaque cursor | Ranges lack revision identity | 32-byte random capability plus random process epoch, base64url; bind canonical selection and policy identifier |
| Stdio metadata / parent verification / OS-user boundary | Metadata is same-user spoofable; parent checks are brittle | Current Windows user is v1's trust boundary. `clientInfo` and profile names are advisory capability selectors and accidental-exposure controls, never Copilot/OpenCode authentication. Keep stdio; add no parent verification |
| Existing vault / native store | Vault needs interactive master input; native store is platform-specific | Direct Win32 Credential Manager API; build-tagged non-Windows implementation fails closed |

## Data Flow and Lifecycle

`same-user stdio process → MCP adapter → advisory policy selector → app → fresh catalog query → exact selection → lease store`

First page starts at line 1. Acquisition creates `/tmp/bac-nexus-catalog-<32 lowercase hex>.utf8`, runs CPYTOSTMF, streams once, validates UTF-8/builds line offsets, then confirms removal before cursor publication. Later pages re-authorize, re-query the coordinate, and read the immutable lease. Coordinate changes return `stale_coordinate`; byte changes continue the snapshot.

Page packing returns the largest contiguous prefix within 200 lines and 131,072 marshaled bytes. It never splits lines/runes; no fitting line is `line_too_large`, and an oversized envelope is `response_too_large`.

One mutex protects lookup/admission/refcounts; bytes are read outside it. Valid access refreshes monotonic TTL. Eviction hides leases immediately; zeroization waits for readers, and retired bytes count toward quota. Expiry returns `snapshot_expired`; reacquisition starts at line 1. Shutdown retires leases and best-effort zeroizes buffers; Go copies cannot be guaranteed erased.

Cancellation propagates through remote work. Cleanup uses `context.WithTimeout(context.WithoutCancel(requestCtx), 15s)` and its own connection. Recovery scans 256 `/tmp` entries and deletes at most 32 exact-pattern regular files older than one hour; truncation/failure blocks acquisition. Generic files are untouched.

## File Changes

| File | Action | Description |
|---|---|---|
| `cmd/nexus/main.go`, `internal/app/service.go`, `internal/mcp/server.go` | Create | Lifecycle, orchestration, tools |
| `internal/source/{snapshot,store,acquire}.go` | Create | Lines, leases, quotas, cleanup/recovery |
| `internal/security/policy.go`, `internal/audit/audit.go` | Create | Advisory capabilities and allowlisted audit |
| `internal/credential/wincred_windows.go`, `wincred_unsupported.go` | Create | Credential Manager; fail-closed portability |
| `internal/remote/ssh.go`, `internal/source/retrieve.go` | Modify | Cleanup-owned operations; preserve spike prefix behavior |
| `go.mod`, `README.md` | Modify | Pin SDK; document operation and success boundary |
| `docs/SECURITY.md` | Create | Trust model, threat model, redaction, rollout |
| Matching package-local `*_test.go` | Create/Modify | Strict RED-first tests |

## Interfaces / Contracts

Use narrow `CatalogResolver`, `SnapshotAcquirer`, `LeaseStore`, `Authorizer`, `CredentialStore`, and `Auditor` interfaces. Unknown/malformed selectors fail `unauthorized` before remote work; allowed selectors do not authenticate products. Credentials never enter argv/env/MCP/logs. Configurable pinned TOFU rejects changes and never silently enrolls or rotates.

## Dependencies

Project approval covers official MCP Go SDK v1.2.0. Windows APIs and a separately supplied, checksum-verified Mapepire JAR are runtime dependencies. Corporate deployment approval, IBM i access, source/audit policy, and a manual window are rollout prerequisites, not design blockers.

## Testing Strategy

Strict TDD writes RED tests before each slice: MCP schemas/errors/cancellation; selector outcomes and spoofed `clientInfo` equivalence; no parent-identity branch; 128 KiB UTF-8 packing; cursor binding/replay; TTL/readers/quota/restart; single copy, cleanup/redial/recovery; freshness; Win32 fakes, redaction, and TOFU changes. Use fakes and loopback SSH; live IBM i acceptance is manual and retains no source.

## Threat Matrix

| Boundary | Applicability | Safe/failure behavior and planned RED test |
|---|---|---|
| Local stdio process identity | Applicable | Same-user callers receive selected capabilities; invalid selectors fail before remote work. RED: selector outcomes and differing `clientInfo` equivalence; never claim product authentication |
| Documentation-like paths | N/A: no classification/execution | None |
| Git repository selection | N/A: no VCS automation | None |
| Commit state | N/A: no commits | None |
| Push state | N/A: no pushes | None |
| PR commands | N/A: no PR automation | None |

## Migration / Rollout

Keep `catalogspike` prefix behavior and vault unchanged. `enroll` writes terminal input to Credential Manager. Explicit `migrate` prompts for the old master, decrypts once, writes native storage, zeroizes buffers, and deletes the vault only after confirmation. No automatic migration or JAR redistribution.

Deliver revertible strict-TDD slices under 400 changed lines: lines; leases; acquisition/cleanup/recovery; credentials/security/audit; freshness; MCP/lifecycle/docs; manual acceptance. Success proves bounded coherent traversal and cleanup under OS-user authorization—not cryptographic Copilot/OpenCode authentication. Corporate approval gates deployment only.

## Open Questions

None. No design blocker remains.
