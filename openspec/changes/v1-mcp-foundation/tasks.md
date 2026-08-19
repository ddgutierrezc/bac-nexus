# Tasks: v1 MCP Foundation

## Review Workload Forecast

| Field | Value |
|---|---|
| Estimated changed lines | 1,700–2,300 |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 → 2 → 3A → 3B → 5 → 6 → 7 → 8; each targets `main` |
| Delivery strategy | ask-on-risk (resolved) |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

Target: `main`; merge in order; under 400 lines each.

### Suggested Work Units

| Unit | Goal / depends | PR | Focused test command | Runtime harness | Rollback boundary |
|---|---|---|---|---|---|
| 1 | Lines; none | 1 | `go test -count=1 ./internal/source` | memory model | `snapshot*` |
| 2 | Leases; 1 | 2 | `go test -count=1 ./internal/source` | fake clock/store | `store*` |
| 3A | Acquire/cleanup; 1–2 | 3A | `go test -count=1 ./internal/source` | fake/loopback copy | acquisition/remote seam |
| 3B | Recovery; 3A | 3B | `go test -count=1 ./internal/source` | fake/loopback listing | recovery/listing seam |
| 5 | Credentials/security/audit; none | 5 | `go test -count=1 ./internal/credential ./internal/security ./internal/audit` | Win32 fakes | package files |
| 6 | Freshness; 2, 3A, 5 | 6 | `go test -count=1 ./internal/app` | resolver/acquirer fakes | `service*` |
| 7 | MCP/lifecycle/docs; 5–6 | 7 | `go test -count=1 ./internal/mcp ./cmd/nexus` | stdio loopback | MCP, `cmd/nexus`, docs |
| 8 | Approved acceptance; 1–7 | 8 | `go test -count=1 ./...` | read-only IBM i | acceptance evidence |

## Phase 1: Source Foundation

- [x] 1.1 **RED**: Add `internal/source/snapshot_test.go` line-contract coverage.
- [x] 1.2 **GREEN**: Create `internal/source/snapshot.go` immutable page contracts.
- [x] 1.3 **REFACTOR**: Simplify snapshot fixtures; rerun the source suite.
- [x] 1.4 **RED**: Add `internal/source/store_test.go` lease-lifecycle coverage.
- [x] 1.5 **GREEN**: Create `internal/source/store.go` bounded opaque leases.
- [x] 1.6 **REFACTOR**: Isolate seams; rerun the source suite.

## Phase 2: Remote Snapshot Safety

- [x] 2.1 **RED (PR 3A)**: In `internal/source/acquire_test.go`, prove one fixed copy, regular-file Stat, 4 MiB cap, exact download/Stat length, complete UTF-8, post-copy ownership, and no snapshot on cancel/deadline/download/read/close/cleanup/joined errors.
- [x] 2.2 **GREEN (PR 3A)**: Create `internal/source/acquire.go`; update `retrieve.go` and `remote/ssh.go` for one download, remove/not-found confirmation before publication, and a cleanup-owned independent connection/lifecycle.
- [x] 2.3 **REFACTOR (PR 3A)**: Share fake/loopback acquisition helpers; keep cleanup scoped to the Nexus temporary and rerun the focused source suite.
- [ ] 2.4 **RED (PR 3B)**: Add recovery tests for exact `/tmp/bac-nexus-catalog-<32 lowercase hex>.utf8`, 256-entry/32-delete/one-hour bounds, preserved generic/recent/malformed/nonregular entries, and fail-closed listing, confirmation, truncation, ambiguity, or >32 stale files.
- [ ] 2.5 **GREEN (PR 3B)**: Add bounded recovery behind the cleanup-owned connection in `acquire.go`, `retrieve.go`, and `remote/ssh.go`; expose no generic listing/deletion outside the connector.
- [ ] 2.6 **REFACTOR (PR 3B)**: Isolate recovery listing fakes and rerun `go test -count=1 ./internal/source`.

## Phase 3: Local Security and Freshness

- [ ] 3.1 **RED**: Add `internal/{security,credential,audit}/*_test.go`: invalid selectors/differing `clientInfo` before remote work, unavailable credentials, TOFU change, redaction; no product-authentication or parent checks.
- [ ] 3.2 **GREEN**: Create `internal/security/policy.go`, `internal/audit/audit.go`, and build-tagged `internal/credential/wincred_*` stores.
- [ ] 3.3 **REFACTOR**: Table-drive those tests; run `go test -count=1 ./internal/credential ./internal/security ./internal/audit`.
- [ ] 3.4 **RED**: Add `internal/app/service_test.go` for 50 candidates, re-query, stale coordinate, byte changes, and denial before remote work.
- [ ] 3.5 **GREEN**: Create `internal/app/service.go` with narrow resolver/acquirer/store interfaces and deterministic errors.
- [ ] 3.6 **REFACTOR**: Consolidate `internal/app` fakes; run its suite.

## Phase 4: MCP, Rollout, and Acceptance

- [ ] 4.1 **RED**: Add `internal/mcp/server_test.go` and `cmd/nexus/main_test.go` for schemas, two read tools, errors, cancellation, and shutdown.
- [ ] 4.2 **GREEN**: Pin SDK in `go.mod`; create `internal/mcp/server.go` and `cmd/nexus/main.go`.
- [ ] 4.3 **REFACTOR**: Update `README.md` and `docs/SECURITY.md`; preserve `cmd/catalogspike`; run `go test -count=1 ./...`.
- [ ] 4.4 Perform the approved manual IBM i traversal from line 1 to EOF; record sanitized newline, EOF, success/cancellation cleanup evidence without retaining source.
