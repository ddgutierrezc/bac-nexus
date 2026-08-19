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
| Shared `/tmp` scan / durable ownership ledger | SFTP v1.13.9 cannot bound remote enumeration | One immutable local record per pending Nexus temporary; 64-record hard limit and bounded `os.File.ReadDir(n)` iteration |
| Shared `/tmp` / private remote directory | Shared names cannot prove ownership | `<validated-home>/.bac-nexus/tmp` (`0700`) with exclusive random files (`0600`) and exact-path cleanup only |
| Profile name / pinned target binding | Profiles can be retargeted | Record selector plus digest of canonical host, port, username, and pinned host-key fingerprint; never log values |

## Data Flow and Lifecycle

`same-user stdio process → MCP adapter → advisory policy selector → app → fresh catalog query → exact selection → lease store`

First page starts at line 1. The QSYS source member is immutable and is never modified or deleted. Acquisition validates the authenticated user's absolute home and rejects traversal, symlink components, or escape; creates/verifies the private directory; generates 128 random bits; and durably commits an ownership record **before** remote creation or CPYTOSTMF. It exclusively reserves the exact IFS path, verifies a regular non-symlink file, copies, streams once, validates UTF-8/builds line offsets, then confirms exact-path removal before cursor publication. The caller supplies no path; Nexus exposes no listing or generic delete operation. Later pages re-authorize, re-query the coordinate, and read the immutable lease. Coordinate changes return `stale_coordinate`; byte changes continue the snapshot.

Page packing returns the largest contiguous prefix within 200 lines and 131,072 marshaled bytes. It never splits lines/runes; no fitting line is `line_too_large`, and an oversized envelope is `response_too_large`.

One mutex protects lookup/admission/refcounts; bytes are read outside it. Valid access refreshes monotonic TTL. Eviction hides leases immediately; zeroization waits for readers, and retired bytes count toward quota. Expiry returns `snapshot_expired`; reacquisition starts at line 1. Shutdown retires leases and best-effort zeroizes buffers; Go copies cannot be guaranteed erased.

Cancellation propagates through remote work. Immediate cleanup retains `context.WithTimeout(context.WithoutCancel(requestCtx), 15s)` and an independent connection. Startup/pre-acquisition recovery holds a cross-process ledger lock, re-resolves the current profile and credential, compares the target binding, verifies the recorded pin on a fresh connection, and revalidates the private directory, token, and exact path. `Remove` plus `Stat`-not-found precedes record deletion. Crash before copy removes the empty reservation or accepts not-found; crash during copy/download removes the partial/full file; crash after remote removal accepts not-found then removes the record. Overflow, corruption, contention, or ambiguity blocks recovery and new acquisition.

## File Changes

| File | Action | Description |
|---|---|---|
| `cmd/nexus/main.go`, `internal/app/service.go`, `internal/mcp/server.go` | Create | Lifecycle, orchestration, tools |
| `internal/source/{snapshot,store,acquire,ownership}.go`, platform ownership files | Create | Lines, leases, quotas, durable ownership and recovery |
| `internal/security/policy.go`, `internal/audit/audit.go` | Create | Advisory capabilities and allowlisted audit |
| `internal/credential/wincred_windows.go`, `wincred_unsupported.go` | Create | Credential Manager; fail-closed portability |
| `internal/remote/ssh.go`, `internal/source/retrieve.go` | Modify | Private-directory fixed operations and cleanup; preserve spike prefix behavior |
| `go.mod`, `README.md` | Modify | Pin SDK; document operation and success boundary |
| `docs/SECURITY.md` | Create | Trust model, threat model, redaction, recovery, migration, rollout |
| Matching package-local `*_test.go` | Create/Modify | Strict RED-first tests |

## Interfaces / Contracts

Use narrow `CatalogResolver`, `SnapshotAcquirer`, `LeaseStore`, `Authorizer`, `CredentialStore`, and `Auditor` interfaces. Add `OwnershipLedger`, `LedgerLock`, `DurableRecordFS`, `TargetResolver`, and remote `PreparePrivateDirectory/CreateExclusive/Lstat/CopyToUTF8/Download/Remove/Stat` seams. Unknown/malformed selectors fail `unauthorized` before remote work; allowed selectors do not authenticate products. Credentials never enter argv/env/MCP/logs. Configurable pinned TOFU rejects changes and never silently enrolls or rotates.

Each record contains only version, token, validated path, profile selector, target-binding digest, and creation time—never source, credentials, command text, cursor, or model content. The absolute per-user ledger root requires restrictive Windows ACLs. Reject reparse points/symlinks, unknown fields, malformed tokens, duplicate paths, wrong ACL/owner, oversized records, and entry 65; never truncate and proceed.

Ordering is exclusive create → write → file sync → close → remote work; cleanup is confirmed remote absence → record removal → durable metadata commit. `os.File.Sync` does not prove Windows directory-entry durability. A build-tagged Windows backend must use a documented Win32 exclusive/write-through create and durable-unlink primitive, plus `LockFileEx` or documented equivalent. Non-Windows fails closed until equivalent guarantees exist.

## Dependencies

Project approval covers official MCP Go SDK v1.2.0. Windows APIs and a separately supplied, checksum-verified Mapepire JAR are runtime dependencies. Corporate deployment approval, IBM i access, source/audit policy, and a manual window are rollout prerequisites, not design blockers.

## Testing Strategy

Strict TDD writes RED tests before each slice: MCP schemas/errors/cancellation; selector outcomes and spoofed `clientInfo` equivalence; no parent-identity branch; 128 KiB UTF-8 packing; cursor binding/replay; TTL/readers/quota/restart; single copy, cleanup/redial/recovery; freshness; Win32 fakes, redaction, and TOFU changes. Recovery RED tests cover each crash point, 64/65 records, corruption/forgery, collisions, profile retargeting, lock contention, reparse/symlink/traversal, remote replacement, ordering, and redaction. Use consumer fakes, temporary directories, and loopback SSH; Windows integration tests prove the real lock/durability adapter. No live IBM i is required for normal tests; manual acceptance retains no source and validates newline, EOF, cancellation, and cleanup.

## Threat Model

Caller-supplied paths are impossible by contract. Validation and ACLs constrain forged ledger files and other-user attackers; the same Windows user remains trusted. Exclusive random names address collisions. Lstat detects ordinary replacement, but a privileged IBM i administrator can race or replace remote files; absolute protection is not claimed. Binding checks stop profile retargeting; kernel locking serializes cooperating processes. Uncertainty fails closed.

## Threat Matrix

| Boundary | Applicability | Safe/failure behavior and planned RED test |
|---|---|---|
| Local stdio process identity | Applicable | Same-user callers receive selected capabilities; invalid selectors fail before remote work. RED: selector outcomes and differing `clientInfo` equivalence; never claim product authentication |
| Fixed IBM i copy command | Applicable | Only validated QSYS-to-owned-path copy; malformed path/binding fails before command. RED: injection/traversal and immutable-source cases |
| Documentation-like paths | N/A: no classification/execution | None |
| Git repository selection | N/A: no VCS automation | None |
| Commit state | N/A: no commits | None |
| Push state | N/A: no pushes | None |
| PR commands | N/A: no PR automation | None |

## Migration / Rollout

Keep `catalogspike` prefix behavior and vault unchanged. `enroll` writes terminal input to Credential Manager. Explicit `migrate` prompts for the old master, decrypts once, writes native storage, zeroizes buffers, and deletes the vault only after confirmation. No automatic migration or JAR redistribution.

PR 3A (`3a85360`) remains historical merged work. Deliver stacked-to-main strict-TDD slices, each ≤400 authored lines: 3B.1 adds ledger, ACL/reparse defenses, and Windows locking/durability; 3B.2 migrates acquisition from `/tmp` to the private directory and ledger lifecycle; 3B.3 adds the recovery gate and operational migration documentation. Later credentials/security/audit, freshness, MCP/lifecycle/docs, and manual acceptance phases remain.

Pre-ledger `/tmp/bac-nexus-catalog-*` files cannot be proven owned and MUST NOT be discovered or pattern-deleted. An authorized IBM i operator may inspect and remove confirmed historical paths under existing controls; Nexus adds no generic MCP operation. Rollback stops acquisition first and retains records unless exact remote absence was confirmed; each 3B slice reverts independently. Success proves bounded coherent traversal and cleanup under OS-user authorization—not cryptographic Copilot/OpenCode authentication. Corporate approval gates deployment only.

## Open Questions

None. PR 3B.1 must validate its documented Windows durability primitive; no unsupported portable filesystem guarantee is claimed.
