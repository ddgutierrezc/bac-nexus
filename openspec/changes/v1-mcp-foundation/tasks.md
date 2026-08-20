# Tasks: v1 MCP Foundation

## Review Workload Forecast

| Field | Value |
|---|---|
| Estimated changed lines | 2,800–3,600 authored; ≤400 per unit |
| 400-line budget risk | High |
| Suggested split | 1→2→3A→3B.1a→3B.1b→3B.2→3B.3→5A→5B→6→7→8 to `main` |
| Delivery strategy | ask-on-risk (resolved: stacked-to-main) |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

PoC exceptions are approved; production/corporate SQLite/keyring rollout approval remains unresolved and mandatory.

### Suggested Work Units

| Unit | Goal / deps | Focused test command | Harness | Rollback boundary |
|---|---|---|---|---|
| 1 | Lines | `go test -count=1 ./internal/source` | memory | snapshot |
| 2 | Leases; 1 | `go test -count=1 ./internal/source` | fake clock | store |
| 3A | Acquire; 1–2 | `go test -count=1 ./internal/source` | loopback | acquire |
| 3B.1a | Isolated ledger foundation; 3A; target `main` | `go test -count=1 ./internal/ownership/sqlite ./internal/source` | draft #26 RED/temp DB | ledger foundation |
| 3B.1b | Ledger hardening; 3B.1a; target `main` | `go test -count=1 ./internal/ownership/sqlite` | temp roots/processes | hardening |
| 3B.2 | Private acquire; 3B.1b; target `main` | `go test -count=1 ./internal/source ./internal/remote` | loopback | acquire |
| 3B.3 | Recovery; 3B.2; target `main` | `go test -count=1 ./internal/source ./internal/ownership/sqlite` | crash/contention fake | recovery/docs |
| 5A | Credentials | `go test -count=1 ./internal/credential` | available OS only | credential |
| 5B | Policy/audit; 5A | `go test -count=1 ./internal/security ./internal/audit` | fakes | policy/audit |
| 6 | Freshness; 2,3B.3,5B | `go test -count=1 ./internal/app` | app fakes | service |
| 7 | MCP; 5B,6 | `go test -count=1 ./internal/mcp ./cmd/nexus` | stdio | MCP/docs |
| 8 | Acceptance; all | `go test -count=1 ./...` | approved IBM i | evidence |

OS matrix: use available-runner CI evidence for windows/darwin/linux amd64+arm64; locally use `$env:GOOS='windows'; $env:GOARCH='amd64'; go build ./...` (substitute each target). Do not claim unavailable runners.

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
- [x] 2.4 **GATE (PR 3B.1a)**: Verify Go 1.25 minimum (evaluated Go 1.27.0 windows/amd64); PoC-exception `modernc.org/sqlite` v1.54.0 (module `h1:JCxR4qwkJvOaqAoYcgDoO25Nc+ROg6EJ2LfBVzdrgog=`; go.mod `h1:4ntCLuNmnH8+GNqjka1wNg7KJd5/Hi5FYp8K+XQ7GZw=`; SQLite 3.53.3; pure-Go/no-CGO; windows/darwin/linux amd64+arm64); explicitly resolve `golang.org/x/mod` v0.40.0+ fixed Go-1.25-compatible (evaluated `x/mod` v0.40.0, `x/sys` v0.47.0); inventory full graph/SBOM/checksums/licenses, separating runtime/shipped from tooling/test-only without omitting either; runtime admission: direct OSV + `govulncheck` (use pinned temporary `go run golang.org/x/vuln/cmd/govulncheck@v1.7.0 ./...` when no global binary exists), no unresolved runtime vulnerability, DLL/runtime download, service/listener, or admin dependency; six CGO-disabled compile targets plus consumer open/schema/roundtrip/pragmas tests on available runners only—never upstream recursive race/CGO suite; endpoint policy; production rollout approval remains unresolved and mandatory. Failure blocks 3B.1.
- [x] 2.5 **RED (PR 3B.1a)**: Test `internal/ownership/sqlite/*_test.go` BACN `application_id=1111573326`/`user_version=1`, exact schema/DELETE-EXTRA pragmas, no sensitive fields, consumer contract, and basic cancellable `BEGIN IMMEDIATE` idempotence/capacity; draft PR #26 RED suite is the evidence target.
- [x] 2.6 **GREEN (PR 3B.1a)**: Pin Go 1.25, `modernc.org/sqlite` v1.54.0, and resolved dependencies; add `internal/source/ownership.go` consumer contract and isolated basic SQLite adapter, unused by remote code.
- [x] 2.7 **REFACTOR (PR 3B.1a)**: Compact fixtures/static checks and CI green; document non-integration and independently revertible ledger-foundation rollback.
- [ ] 2.8 **RED (PR 3B.1b)**: Test complete local root/permission/symlink/reparse/network defenses, cancellable 25/50/100ms retries, ambiguous-COMMIT exact readback, bounded quick/proportional integrity checks, and cross-process contention.
- [ ] 2.9 **GREEN (PR 3B.1b)**: Implement the SQLite root, permission, symlink/reparse/network, retry, readback, integrity, and contention hardening; keep remote acquisition and recovery uninvolved.
- [ ] 2.10 **REFACTOR (PR 3B.1b)**: Focus hardening fixtures/assertions, CI, and available platform/cross-process evidence; retain independent hardening rollback.
- [ ] 2.11 **RED (PR 3B.2)**: Test `internal/source/acquire_test.go` authenticated home, private `0700`, exclusive random `0600`, immutable source, traversal/symlink escape, durable row readback before reserve/copy, and retain-on-failure.
- [ ] 2.12 **GREEN (PR 3B.2)**: Update `internal/source/{acquire,retrieve}.go` and `internal/remote/ssh.go` for exact private path; `Remove` plus `Stat`-not-found before transactional DELETE, never recovery-loop.
- [ ] 2.13 **REFACTOR (PR 3B.2)**: Consolidate acquisition fakes; no snapshot if row/cleanup confirmation fails.
- [ ] 2.14 **RED (PR 3B.3)**: Test bounded `LIMIT 65` exact rows, fresh profile/credential/pin/binding, crash idempotence, corruption/contention/retarget blocking, and no historical `/tmp` discovery.
- [ ] 2.15 **GREEN (PR 3B.3)**: Implement exact-path startup/pre-acquire recovery in `internal/source/ownership.go`; delete only after confirmed absence.
- [ ] 2.16 **REFACTOR (PR 3B.3)**: Add `docs/SECURITY.md` operator/privileged-risk guidance and available cross-process/platform evidence; no MCP recovery operation.

## Phase 3: Credentials, Policy, and Freshness

- [ ] 3.1 **GATE (PR 5A)**: Verify PoC-exception `github.com/zalando/go-keyring` v0.2.8 (declares Go 1.18): exact module graph/SBOM, checksums, licenses/transitives, `govulncheck`/known vulnerabilities, no DLL/runtime download, platform compile/tests, endpoint policy. Failure blocks 5A.
- [ ] 3.2 **RED (PR 5A)**: Test `internal/credential/*_test.go` only exact Get/Set/Delete, grammar/1–4096 bounds, redaction/zeroing, unavailable-before-remote, fixed macOS stdin/no argv-env, and Windows/Linux deterministic failures.
- [ ] 3.3 **GREEN (PR 5A)**: Add `CredentialStore`/keyring adapter: Credential Manager, fixed `/usr/bin/security`, Secret Service D-Bus; explicit vault write/readback/zero/delete migration, retaining vault when native storage fails.
- [ ] 3.4 **REFACTOR (PR 5A)**: Isolate consumer and per-platform tests; preserve available-runner-only evidence.
- [ ] 3.5 **RED (PR 5B)**: Test `internal/{security,audit}/*_test.go` selector and spoofed `clientInfo`, TOFU changes, allowlisted audit redaction, and no remote/generic path operations.
- [ ] 3.6 **GREEN (PR 5B)**: Add `internal/security/policy.go` and `internal/audit/audit.go` pinned trust and sanitized outcomes, depending on 5A credentials.
- [ ] 3.7 **REFACTOR (PR 5B)**: Table-drive policy/audit fakes and rerun both package commands.
- [ ] 3.8 **RED (PR 6)**: Test `internal/app/service_test.go` 50-result bound, re-query/stale, page bytes, and credential/policy denial before remote work.
- [ ] 3.9 **GREEN (PR 6)**: Create deterministic `internal/app/service.go` over 2, 3B.3, and 5B.
- [ ] 3.10 **REFACTOR (PR 6)**: Consolidate service fakes and freshness cases.

## Phase 4: MCP and Acceptance

- [ ] 4.1 **RED (PR 7)**: Test `internal/mcp/server_test.go` and `cmd/nexus/main_test.go` typed tools, no paths, deterministic errors, cancellation, and shutdown.
- [ ] 4.2 **GREEN (PR 7)**: Create `internal/mcp/server.go` and `cmd/nexus/main.go`; pin SDK, wire 5B/6, and expose no generic remote tool.
- [ ] 4.3 **REFACTOR (PR 7)**: Update `README.md` and `docs/SECURITY.md`; preserve `catalogspike` and full tests.
- [ ] 4.4 **Acceptance (PR 8)**: Run documented approved IBM i line-1/EOF/newline/success-cancel cleanup evidence without retained source; verify `go test -count=1 ./...`.

Rollback: stop acquisition; revert 3B.3→3B.2→3B.1b→3B.1a while retaining unresolved rows. Revert 5B, then 5A only after preserving accessible secrets and migration state.
