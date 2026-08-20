# Design: v1 MCP Foundation

## Technical Approach

Add `cmd/nexus` over application, MCP, security, audit, catalog, and source seams. Pin official `github.com/modelcontextprotocol/go-sdk` v1.2.0 and use typed `mcp.AddTool` with stdio `Server.Run`. Expose only `resolve_catalog_candidates` and `read_selected_source`.

## Architecture Decisions

| Option | Tradeoff | Decision |
|---|---|---|
| Stateless copies / durable cache / immutable memory | Repeated copies mix revisions; disk persists sensitive data; memory needs quotas | Immutable process-memory leases: 4 MiB/member, 16 MiB resident aggregate, 10-minute idle TTL |
| Caller ranges / opaque cursor | Ranges lack revision identity | 32-byte random capability plus random process epoch, base64url; bind canonical selection and policy identifier |
| Stdio metadata / parent verification / OS-user boundary | Metadata is same-principal spoofable; parent checks are brittle | Current local OS principal is v1's trust boundary on Windows, macOS, and Linux. `clientInfo` and profile names are advisory capability selectors and accidental-exposure controls, never Copilot/OpenCode authentication. Keep stdio; add no parent verification |
| Existing vault / native store | Vault needs interactive master input; native mechanisms vary by platform | Consumer-owned `CredentialStore` behind one approved cross-platform native-keyring adapter: Windows Credential Manager, macOS Keychain, and Linux Secret Service; unavailable, locked, missing, ambiguous, or policy-denied stores fail closed before remote work, with no v1 fallback |
| Shared `/tmp` scan / durable ownership ledger | SFTP v1.13.9 cannot bound remote enumeration | One SQLite row per pending Nexus temporary behind consumer-owned `OwnershipLedger`; 64-record hard limit and bounded `LIMIT 65` reads |
| Shared `/tmp` / private remote directory | Shared names cannot prove ownership | `<validated-home>/.bac-nexus/tmp` (`0700`) with exclusive random files (`0600`) and exact-path cleanup only |
| Profile name / pinned target binding | Profiles can be retargeted | Record selector plus digest of canonical host, port, username, and pinned host-key fingerprint; never log values |
| Per-OS ledger / portable SQLite adapter | Per-OS backends fragment guarantees; WAL adds WAL/SHM and reset concerns for tiny, infrequent writes | One SQLite adapter for Windows/macOS/Linux using rollback journal (`journal_mode=DELETE`) and `synchronous=EXTRA`; OS-specific code is limited to unavoidable application-data path, permission, and verified sync hardening |

## Data Flow and Lifecycle

`same-local-principal stdio process → MCP adapter → advisory policy selector → app → fresh catalog query → exact selection → lease store`

First page starts at line 1. The QSYS source member is immutable and is never modified or deleted. Acquisition validates the authenticated user's absolute home and rejects traversal, symlink components, or escape; creates/verifies the private directory; generates 128 random bits; and durably commits an ownership record **before** remote creation or CPYTOSTMF. It exclusively reserves the exact IFS path, verifies a regular non-symlink file, copies, streams once, validates UTF-8/builds line offsets, then confirms exact-path removal before cursor publication. The caller supplies no path; Nexus exposes no listing or generic delete operation. Later pages re-authorize, re-query the coordinate, and read the immutable lease. Coordinate changes return `stale_coordinate`; byte changes continue the snapshot.

The SQLite database lives only in the validated absolute local-user application-data directory, never a network/shared filesystem. Nexus rejects symlink/reparse escape and wrong ownership or restrictive platform permissions before opening it; tests inject the root. One connection per process preserves simple `database/sql` behavior while SQLite coordinates multiple Nexus processes. Admission uses a short `BEGIN IMMEDIATE`, validates the database, treats the same stable token plus exact row as an idempotent retry, and checks `count < 64` plus insert atomically. Remote reserve/copy is forbidden until COMMIT succeeds and reopening/readback confirms the exact row. A 250 ms busy timeout and context-cancellable 25/50/100 ms retries are bounded; retries reuse the token. After ambiguous COMMIT, exact readback means committed, absence may retry, and mismatch or corruption fails closed.

Page packing returns the largest contiguous prefix within 200 lines and 131,072 marshaled bytes. It never splits lines/runes; no fitting line is `line_too_large`, and an oversized envelope is `response_too_large`.

One mutex protects lookup/admission/refcounts; bytes are read outside it. Valid access refreshes monotonic TTL. Eviction hides leases immediately; zeroization waits for readers, and retired bytes count toward quota. Expiry returns `snapshot_expired`; reacquisition starts at line 1. Shutdown retires leases and best-effort zeroizes buffers; Go copies cannot be guaranteed erased.

Cancellation propagates through remote work. Immediate cleanup retains `context.WithTimeout(context.WithoutCancel(requestCtx), 15s)` and an independent connection. Startup/pre-acquisition recovery uses SQLite's single-writer transaction boundary, re-resolves the current profile and credential, compares the target binding, verifies the recorded pin on a fresh connection, and revalidates the private directory, token, and exact path. `Remove` plus `Stat`-not-found precedes row deletion in a transaction. An ambiguous or failed local delete retains or may resurrect a stale row; recovery safely repeats exact not-found because tokens and paths are never reused. Durable row deletion is liveness/capacity hardening, not the pre-copy safety invariant. Crash before copy removes the empty reservation or accepts not-found; crash during copy/download removes the partial/full file; crash after remote removal accepts not-found then removes the record. Overflow, corruption, contention, or ambiguity blocks recovery and new acquisition.

## File Changes

| File | Action | Description |
|---|---|---|
| `cmd/nexus/main.go`, `internal/app/service.go`, `internal/mcp/server.go` | Create | Lifecycle, orchestration, tools |
| `internal/source/{snapshot,store,acquire,ownership}.go`, `internal/ownership/sqlite/{ledger,path}.go` | Create | Lines, leases, quotas, consumer-owned ledger contract, portable SQLite adapter, and recovery |
| `internal/security/policy.go`, `internal/audit/audit.go` | Create | Advisory capabilities and allowlisted audit |
| `internal/credential/{store,keyring}.go`, matching tests | Create | Consumer-owned credential contract and one narrow cross-platform native-keyring adapter; avoid custom per-OS backends unless unavoidable |
| `internal/remote/ssh.go`, `internal/source/retrieve.go` | Modify | Private-directory fixed operations and cleanup; preserve spike prefix behavior |
| `go.mod`, `README.md` | Modify | Pin SDK; document operation and success boundary |
| `docs/SECURITY.md` | Create | Trust model, threat model, redaction, recovery, migration, rollout |
| Matching package-local `*_test.go` | Create/Modify | Strict RED-first tests |

## Interfaces / Contracts

Use narrow `CatalogResolver`, `SnapshotAcquirer`, `LeaseStore`, `Authorizer`, consumer-owned `CredentialStore`, and `Auditor` interfaces. Add consumer-owned `OwnershipLedger`, `TargetResolver`, and remote `PreparePrivateDirectory/CreateExclusive/Lstat/CopyToUTF8/Download/Remove/Stat` seams; place the SQLite adapter behind that contract. `CredentialStore` remains separately platform-adapted, and SQLite stores neither credentials nor source. Unknown/malformed selectors fail `unauthorized` before remote work; allowed selectors do not authenticate products. Credentials never enter argv/env/MCP/logs. Configurable pinned TOFU rejects changes and never silently enrolls or rotates.

`CredentialStore` exposes only exact `Get`, `Set`, and `Delete`; no `DeleteAll`, enumeration, prompt, plaintext, SQLite-secret, or encrypted-vault fallback exists in v1. The service is fixed to `BAC Nexus`; accounts are `ibmi/<profile>`, where profile is 1–64 ASCII characters matching `[A-Za-z0-9][A-Za-z0-9._-]{0,63}`. Secrets are 1–4096 bytes. Validate before adapter calls, zero temporary byte buffers where possible, and exclude secrets from argv, environment, logs, audit, MCP, SQLite, fixtures, and errors. Missing, locked, unavailable, policy-denied, malformed, or ambiguous native-store results map deterministically to `credentials_unavailable`; no remote operation begins before successful credential retrieval.

Each record contains only version, token, validated path, profile selector, target-binding digest, and creation time—never source, credentials, command text, cursor, or model content. Table `ownership` has one active row: `token BLOB PRIMARY KEY CHECK(length(token)=16)`, `remote_path TEXT UNIQUE CHECK(length(CAST(remote_path AS BLOB)) BETWEEN 1 AND 1024)`, `version INTEGER CHECK(version=1)`, `profile TEXT CHECK(length(profile) BETWEEN 1 AND 64)`, `target_digest BLOB CHECK(length(target_digest)=32)`, and non-null canonical UTC `created_at TEXT` (`YYYY-MM-DDTHH:MM:SSZ`, 20 bytes). The absolute per-user database root requires restrictive platform ownership and permissions. Reject symlink/reparse points, unknown schema, malformed tokens, duplicate paths, wrong permissions/owner, oversized values, and row 65; never truncate and proceed.

Ordering is `BEGIN IMMEDIATE` → validate/configure → count/idempotent insert → COMMIT → reopen/exact readback → remote work; cleanup is confirmed remote absence → transactional row DELETE. Every open verifies effective `journal_mode=DELETE`, `synchronous=EXTRA`, busy timeout, `application_id`, `user_version`, exact schema, permissions, and bounded `quick_check`; unknown/mismatched pragmas or corruption fail closed. Use proportional `integrity_check` diagnostics, preserve evidence without logging row values, and never rebuild destructively. macOS `fullfsync` is optional hardening only when official applicability and its effective setting are verifiable.

## Dependencies

Project approval covers official MCP Go SDK v1.2.0. Windows APIs and a separately supplied, checksum-verified Mapepire JAR are runtime dependencies. Corporate deployment approval, IBM i access, source/audit policy, and a manual window are rollout prerequisites, not design blockers.

The maintainer-approved PoC-only exception pins pure-Go, no-CGO `modernc.org/sqlite` v1.38.2, which declares Go 1.23, embeds SQLite 3.50.1, and supports windows/darwin/linux amd64+arm64; v1.39+ requires Go 1.24 and is excluded from this PoC. SQLite remains embedded in the signed Nexus executable with no runtime DLL/download, service/listener, or admin rights. PR 3B.1 may begin technical evaluation under the exception, but its first fail-closed task must verify the exact module graph/SBOM, license inventory, checksums, `govulncheck`/known vulnerabilities, no DLL/runtime download, platform compile/tests, and endpoint policy. Any failure stops implementation/delivery. Rollback-journal `DELETE` plus `synchronous=EXTRA` remains mandatory with no WAL reliance. Production/corporate approval, signed/approved Nexus, and an approved database directory remain mandatory rollout gates; SQLite bypasses no policy, and denial is deterministic.

The same PoC-only exception pins `github.com/zalando/go-keyring` v0.2.8, which declares Go 1.18. PR 5A may begin technical evaluation, but its first fail-closed task must verify the exact module graph/SBOM, transitive license inventory, checksums, `govulncheck`/known vulnerabilities, no DLL/runtime download, platform compile/tests, and endpoint policy; any failure stops implementation/delivery. Windows uses `danieljoos/wincred` and native Credential Manager APIs. macOS invokes only fixed `/usr/bin/security`, never a generic shell: Set uses `-i` with encoded command/secret on stdin, never argv/environment; Get uses the fixed find command and captures output. Linux uses `godbus/dbus` and Secret Service. Signed/approved Nexus remains required for production; fixed `/usr/bin/security` execution and D-Bus/native-store access remain subject to enterprise policy and fail deterministically when denied. Production/corporate dependency approval remains unresolved and mandatory, separately from SQLite rollout approval.

## Testing Strategy

Strict TDD writes RED tests before each slice: MCP schemas/errors/cancellation; selector outcomes and spoofed `clientInfo` equivalence; no parent-identity branch; 128 KiB UTF-8 packing; cursor binding/replay; TTL/readers/quota/restart; single copy, cleanup/redial/recovery; freshness; SQLite fakes/temporary databases, redaction, and TOFU changes. Recovery RED tests cover each crash point, 64/65 rows, corruption/forgery, collisions, profile retargeting, contention, cancellation, ambiguous COMMIT/readback, schema/pragma/permission mismatch, reparse/symlink/traversal, remote replacement, ordering, and redaction. Use consumer fakes, temporary databases/directories, and loopback SSH; proportional real cross-process/concurrency/crash harnesses plus Windows/macOS/Linux CI prove platform behavior. Unit tests do not prove power-loss durability; require documented SQLite guarantees and platform evidence. No live IBM i is required for normal tests; manual acceptance retains no source and validates newline, EOF, cancellation, and cleanup.

Credential RED tests use a consumer fake plus per-OS integration/CI where runners exist: exact service/account and size bounds; Get/Set/Delete only; no fallback or remote work; deterministic missing/locked/unavailable/policy/ambiguous outcomes; temporary-buffer zeroing and redaction. macOS tests prove secrets absent from argv/environment and fixed-command output is bounded; Linux tests cover unavailable/locked Secret Service; Windows tests map native Credential Manager errors. Do not infer coverage from unavailable runners.

## Threat Model

Caller-supplied paths are impossible by contract. Validated local database paths, restrictive permissions, strict schema, and integrity checks constrain forged ledgers and other-user attackers; the same local OS principal remains trusted on every platform. A malicious same-principal process is therefore residual risk. Exclusive random names address collisions. Lstat detects ordinary replacement, but a privileged IBM i administrator can race or replace remote files; absolute protection is not claimed. Binding checks stop profile retargeting; SQLite transaction locking serializes cooperating processes. Fixed macOS subprocess output exists briefly in Nexus memory, and privileged OS users/administrators may access native stores; no absolute credential-confidentiality claim is made. Uncertainty fails closed.

## Threat Matrix

| Boundary | Applicability | Safe/failure behavior and planned RED test |
|---|---|---|
| Local stdio process identity | Applicable | Same-local-principal callers receive selected capabilities on every platform; invalid selectors fail before remote work. RED: selector outcomes and differing `clientInfo` equivalence; never claim product authentication |
| Fixed IBM i copy command | Applicable | Only validated QSYS-to-owned-path copy; malformed path/binding fails before command. RED: injection/traversal and immutable-source cases |
| Fixed macOS keyring subprocess | Applicable | Execute only `/usr/bin/security` with fixed arguments and stdin secret; policy denial fails `credentials_unavailable` before remote work. RED: no shell, argv/environment secret, fallback, or remote attempt |
| Documentation-like paths | N/A: no classification/execution | None |
| Git repository selection | N/A: no VCS automation | None |
| Commit state | N/A: no commits | None |
| Push state | N/A: no pushes | None |
| PR commands | N/A: no PR automation | None |

## Migration / Rollout

Keep `catalogspike` prefix behavior and vault unchanged. `enroll` writes terminal input to the active platform-native store. Explicit `migrate` prompts for the old master, decrypts once, writes the active native store, reads back and confirms the exact credential, zeroizes buffers, and only then deletes the old vault. Native-store unavailability makes migration and Nexus access fail closed. No automatic migration, fallback, or JAR redistribution.

PR 3A (`3a85360`) remains historical merged work. Deliver stacked-to-main strict-TDD slices, each ≤400 authored lines: 3B.1 adds the SQLite dependency/configuration/schema/concurrency/durability foundation; 3B.2 migrates acquisition from `/tmp` to the private directory and durable row lifecycle; 3B.3 adds the recovery gate, cross-process/platform evidence, and operational migration documentation. Later credentials/security/audit, freshness, MCP/lifecycle/docs, and manual acceptance phases remain.

For tasks regeneration, split the former PR 5 credential/security/audit unit into ≤400-line stacked-to-main slices: **5A** first performs the fail-closed PoC-exception dependency verification, then adds the consumer-owned credential contract, keyring adapter, and consumer/per-OS tests; **5B** adds client authorization, pinned TOFU, sanitized audit, and their tests. PR 3B.1 likewise starts with its fail-closed PoC-exception dependency verification. PRs 3B.1–3B.3 and all later non-PR-5 ordering remain unchanged.

Pre-ledger `/tmp/bac-nexus-catalog-*` files cannot be proven owned and MUST NOT be discovered or pattern-deleted. An authorized IBM i operator may inspect and remove confirmed historical paths under existing controls; Nexus adds no generic MCP operation. Rollback stops acquisition first and retains records unless exact remote absence was confirmed; each 3B slice reverts independently. Success proves bounded coherent traversal and cleanup under OS-user authorization—not cryptographic Copilot/OpenCode authentication. Corporate approval gates deployment only.

## Open Questions

None for PoC planning: the version exceptions are approved, and PR 3B.1/5A verification failures stop delivery rather than reopen product decisions. Production/corporate dependency and endpoint-policy approval remains a mandatory rollout gate. No unsupported portable filesystem or power-loss guarantee is claimed.
