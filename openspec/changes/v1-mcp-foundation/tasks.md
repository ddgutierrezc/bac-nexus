# Tasks: v1 MCP Foundation

## Review Workload Forecast

| Field | Value |
|---|---|
| Estimated changed lines | 1,500–2,100 |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 → PR 7; each targets `main`, merges in order |
| Delivery strategy | ask-on-risk |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

All PRs target `main`, merge in dependency order PR 1 → PR 7, and remain under 400 lines.

### Suggested Work Units

| Unit | Goal / depends | PR / base | Focused test command | Runtime harness | Rollback boundary |
|---|---|---|---|---|---|
| 1 | Lines; none | PR 1 → `main` | `go test -count=1 ./internal/source` | N/A: pure memory model | `internal/source/snapshot*` |
| 2 | Leases; 1 | PR 2 → `main` | `go test -count=1 ./internal/source` | N/A: fake clock/store | `internal/source/store*` |
| 3 | Acquire/cleanup/recovery; 1–2 | PR 3 → `main` | `go test -count=1 ./internal/source` | loopback/fake remote | `internal/source/acquire*`, retrieval seam |
| 4 | Credentials/security/audit; none | PR 4 → `main` | `go test -count=1 ./internal/credential ./internal/security ./internal/audit` | N/A: Win32 fakes | security, audit, credential files |
| 5 | Freshness; 2–4 | PR 5 → `main` | `go test -count=1 ./internal/app` | fake resolver/acquirer | `internal/app/service*` |
| 6 | MCP/lifecycle/docs; 4–5 | PR 6 → `main` | `go test -count=1 ./internal/mcp ./cmd/nexus` | stdio loopback | `cmd/nexus`, `internal/mcp`, docs |
| 7 | Approved acceptance; 1–6 | PR 7 → `main` | `go test -count=1 ./...` | approved read-only IBM i window | acceptance evidence only |

## Phase 1: Source Foundation

- [ ] 1.1 **RED**: Add `internal/source/snapshot_test.go` cases for complete UTF-8 lines, LF/final record, trailing spaces, ranges, 200-line/128-KiB packing, and no partial content.
- [ ] 1.2 **GREEN**: Create `internal/source/snapshot.go` with immutable offsets and deterministic page/error contracts.
- [ ] 1.3 **REFACTOR**: Simplify snapshot test fixtures and run `go test -count=1 ./internal/source`.
- [ ] 1.4 **RED**: Add `internal/source/store_test.go` cases for cursor bindings, replay/order/concurrent reads, TTL, quota, reader retirement, eviction, and restart expiry.
- [ ] 1.5 **GREEN**: Create `internal/source/store.go` with opaque epoch capabilities, bounded leases, monotonic expiry, and deferred zeroization.
- [ ] 1.6 **REFACTOR**: Isolate clock/random seams and rerun the focused source suite.

## Phase 2: Remote Snapshot Safety

- [ ] 2.1 **RED**: Extend `internal/source/acquire_test.go` for one copy, size/UTF-8/cancel/deadline failures, confirmed cleanup before publication, own-prefix recovery limits, and generic-file preservation.
- [ ] 2.2 **GREEN**: Create `internal/source/acquire.go`; update `internal/source/retrieve.go` and `internal/remote/ssh.go` for owned cleanup redial/recovery.
- [ ] 2.3 **REFACTOR**: Share fake remote helpers and rerun `go test -count=1 ./internal/source`.

## Phase 3: Local Security and Freshness

- [ ] 3.1 **RED**: Add policy/audit/credential tests for invalid selectors and differing `clientInfo` equivalence before remote work, unavailable credentials, TOFU key change, and redacted audit; never assert product authentication or parent checks.
- [ ] 3.2 **GREEN**: Create `internal/security/policy.go`, `internal/audit/audit.go`, and build-tagged `internal/credential/wincred_*` fail-closed stores.
- [ ] 3.3 **REFACTOR**: Table-drive policy/redaction cases and run `go test -count=1 ./internal/credential ./internal/security ./internal/audit`.
- [ ] 3.4 **RED**: Add `internal/app/service_test.go` for authorized 50-candidate resolution, exact fresh re-query, stale coordinate, byte-change continuity, and no remote work on denial.
- [ ] 3.5 **GREEN**: Create `internal/app/service.go` with narrow resolver/acquirer/store interfaces and deterministic errors.
- [ ] 3.6 **REFACTOR**: Consolidate fakes and run `go test -count=1 ./internal/app`.

## Phase 4: MCP, Rollout, and Acceptance

- [ ] 4.1 **RED**: Add `internal/mcp/server_test.go` and `cmd/nexus/main_test.go` for typed schemas, only two read tools, errors, cancellation, and lifecycle shutdown.
- [ ] 4.2 **GREEN**: Pin SDK in `go.mod`; create `internal/mcp/server.go` and `cmd/nexus/main.go` using stdio adapters.
- [ ] 4.3 **REFACTOR**: Update `README.md` and create `docs/SECURITY.md`; preserve `cmd/catalogspike` prefix behavior and run `go test -count=1 ./...`.
- [ ] 4.4 Perform the approved manual IBM i traversal from line 1 to EOF; record sanitized newline, EOF, success/cancellation cleanup evidence without retaining source.
