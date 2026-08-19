# Tasks: v1 MCP Foundation

## Review Workload Forecast

| Field | Value |
|---|---|
| Estimated changed lines | 2,100–2,800 authored; ≤400/unit |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | 1→2→3A→3B.1→3B.2→3B.3→5→6→7→8 to `main` |
| Delivery strategy | ask-on-risk (resolved: approved split) |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

### Suggested Work Units

| Unit | Goal / dependency | Focused test command | Runtime harness | Rollback boundary |
|---|---|---|---|---|
| 1 | Lines;— | `go test -count=1 ./internal/source` | memory | snapshot |
| 2 | Leases;1 | `go test -count=1 ./internal/source` | fake clock | store |
| 3A | Acquire;1–2 | `go test -count=1 ./internal/source` | loopback | acquire |
| 3B.1 | Ledger;3A | `go test -count=1 ./internal/source` | FS/Win32 fakes | ownership |
| 3B.2 | Private acquire;3B.1 | `go test -count=1 ./internal/source` | loopback | acquire |
| 3B.3 | Recovery;3B.2 | `go test -count=1 ./internal/source` | recovery fake | recovery/docs |
| 5 | Security;— | `go test -count=1 ./internal/credential ./internal/security ./internal/audit` | Win32 fakes | packages |
| 6 | Freshness;2,3B.3,5 | `go test -count=1 ./internal/app` | app fakes | service |
| 7 | MCP;5–6 | `go test -count=1 ./internal/mcp ./cmd/nexus` | stdio | MCP/command/docs |
| 8 | Accept;1–7 | `go test -count=1 ./...` | approved IBM i | evidence |

Rollback: revert 3B.3 first; stop acquisition before retaining/reverting ledger semantics; retain records unless remote absence was confirmed, then revert 3B.2 and 3B.1.

## Phase 1: Source Foundation

- [x] 1.1 **RED (PR 1)**: Test `internal/source/snapshot.go` lines.
- [x] 1.2 **GREEN (PR 1)**: Implement `internal/source/snapshot.go` pages.
- [x] 1.3 **REFACTOR (PR 1)**: Simplify `snapshot_test.go` fixtures.
- [x] 1.4 **RED (PR 2)**: Test `internal/source/store.go` leases.
- [x] 1.5 **GREEN (PR 2)**: Implement `internal/source/store.go` bounds.
- [x] 1.6 **REFACTOR (PR 2)**: Isolate `store_test.go` seams.

## Phase 2: Remote Snapshot Safety

- [x] 2.1 **RED (PR 3A)**: Test `internal/source/acquire_test.go` copy/UTF-8/cancel/cleanup publication failures.
- [x] 2.2 **GREEN (PR 3A)**: Implement `acquire.go`/`retrieve.go`/`remote/ssh.go` download and independent confirmed cleanup.
- [x] 2.3 **REFACTOR (PR 3A)**: Share `acquire_test.go` fakes; scope cleanup.
- [ ] 2.4 **RED (PR 3B.1)**: `internal/source/ownership*_test.go`: strict schema/forbidden/unknown, `ReadDir(65)` 64-limit, duplicate/malformed/oversized/nonregular/reparse/owner-ACL, Win32 lock/durable create/remove contracts.
- [ ] 2.5 **GREEN (PR 3B.1)**: `internal/source/ownership*.go`: generic ledger plus build-tagged Windows/non-Windows backends passing those tests.
- [ ] 2.6 **REFACTOR (PR 3B.1)**: Simplify `internal/source/ownership*_test.go` fixtures/seams; rerun source checks.
- [ ] 2.7 **RED (PR 3B.2)**: `acquire_test.go`: absolute-home/0700/traversal-symlink-reparse-escape/128-bit-0600/binding/ledger-first.
- [ ] 2.8 **GREEN (PR 3B.2)**: `acquire.go`/`retrieve.go`/`remote/ssh.go`: private tmp; record-before-copy; Remove+Stat-before-record removal.
- [ ] 2.9 **REFACTOR (PR 3B.2)**: `acquire_test.go` fakes; retain record/no snapshot; no recovery/generic API.
- [ ] 2.10 **RED (PR 3B.3)**: `ownership_test.go`: lock/load/profile-credential-digest-pin/crash/exact-path/absent/corruption-overflow-contention-ambiguity blocks.
- [ ] 2.11 **GREEN (PR 3B.3)**: `ownership.go`: locked startup/pre-acquire fresh-pin recovery; remove record after absence.
- [ ] 2.12 **REFACTOR (PR 3B.3)**: `docs/SECURITY.md`: no `/tmp` discovery; operator/privileged-admin risk; no MCP operation.

## Phase 3: Local Security and Freshness

- [ ] 3.1 **RED (PR 5)**: `internal/security/*_test.go`, `internal/credential/*_test.go`, `internal/audit/*_test.go`: selector/clientInfo/credential/TOFU/redaction/no-remote.
- [ ] 3.2 **GREEN (PR 5)**: Create `security/policy.go`, `audit/audit.go`, `credential/wincred_*`.
- [ ] 3.3 **REFACTOR (PR 5)**: Table-drive package tests.
- [ ] 3.4 **RED (PR 6)**: `internal/app/service_test.go`: 50/re-query/stale/bytes/pre-remote-denial.
- [ ] 3.5 **GREEN (PR 6)**: Create narrow deterministic `internal/app/service.go`.
- [ ] 3.6 **REFACTOR (PR 6)**: Consolidate `service_test.go` fakes.

## Phase 4: MCP, Rollout, and Acceptance

- [ ] 4.1 **RED (PR 7)**: `mcp/server_test.go`, `cmd/nexus/main_test.go`: typed tools/no paths/errors/cancel/shutdown.
- [ ] 4.2 **GREEN (PR 7)**: Pin SDK; create `mcp/server.go`, `cmd/nexus/main.go`; no generic remote.
- [ ] 4.3 **REFACTOR (PR 7)**: Update `README.md`/`docs/SECURITY.md`; preserve spike/full tests.
- [ ] 4.4 **Acceptance (PR 8)**: `docs/SECURITY.md`: IBM i line-1/EOF/newline/cleanup evidence; no source.
