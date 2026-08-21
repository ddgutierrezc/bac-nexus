# Tasks: v1 MCP Foundation

## Review Workload Forecast

| Field | Value |
|---|---|
| Estimated changed lines | 2,970–3,750 authored; active remaining slices are independently bounded to ≤1000 lines |
| 400-line budget risk | High |
| Maintainer-selected active PR review ceiling | 1000 authored additions + deletions; a ceiling, not permission for unrelated scope |
| Suggested split | 1→2→3A→3B.1a→3B.1b→3B.1c-T→3B.1c-I→3B.2→3B.3→5A→5B→6→7→8 to `main` |
| Delivery strategy | ask-on-risk (resolved: stacked-to-main) |
| Chain strategy | stacked-to-main |
| Execution mode | Automatic coherent slices |

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
| 3B.1b | Filesystem policy; 3B.1a; target `main` | `go test -count=1 ./internal/ownership/sqlite` | injected OS queries/temp roots | policy only |
| 3B.1c-T | Transaction retry/readback; 3B.1b; PR #32 → `main`; 330–380 lines | `go test -count=1 ./internal/ownership/sqlite` | GHA Linux real child-process SQLite lock: 25/50/100ms, cancel, ambiguous COMMIT, deadline contention | `ledger.go` transaction retry/readback plus `ledger_transaction_red_test.go` transaction cases |
| 3B.1c-I | Integrity microcycles; 3B.1c-T merged; draft PR #36 → `main`; 330–380 lines | `go test -count=1 ./internal/ownership/sqlite` | GHA real temporary SQLite ordering/corruption/cancellation/bound cases plus injected `Open` mapping and bounded-row/query edges; no shared gate | `ledger.go` verifier/Open boundary and `ledger_integrity_red_test.go` only; independently revertible |
| 3B.2 | Private acquisition microcycles; 3B.1c-I merged; PR 3B.2 → `main`; 340–390 lines | `go test -count=1 ./internal/source ./internal/remote` | GHA Ubuntu loopback SSH + temporary remote root; no live IBM i | revert only private acquire/retrieve/SSH boundaries and package tests |
| 3B.3 | Recovery microcycles; 3B.2; target `main`; ≤800 authored lines | `go test -count=1 ./internal/source ./internal/ownership/sqlite` | GHA available-runner temp SQLite plus credential/target/pin/remote fakes and cross-process contention; WDAC blocks local Go runtime evidence | `internal/source/ownership.go`, SQLite recovery-list boundary, recovery tests, and `docs/SECURITY.md` only |
| 5A | Credentials | `go test -count=1 ./internal/credential` | available OS only | credential |
| 5B | Policy/audit; 5A | `go test -count=1 ./internal/security ./internal/audit` | fakes | policy/audit |
| 6 | Freshness; 2,3B.3,5B | `go test -count=1 ./internal/app` | app fakes | service |
| 7 | MCP; 5B,6 | `go test -count=1 ./internal/mcp ./cmd/nexus` | stdio | MCP/docs |
| 8 | Acceptance; all | `go test -count=1 ./...` | approved IBM i | evidence |

For each 3B.1c PR, GHA must record the focused command, `gofmt -d internal/ownership/sqlite/*.go`, `go vet ./...`, and matrix `CGO_ENABLED=0 GOOS=$GOOS GOARCH=$GOARCH go build ./...` for windows/darwin/linux × amd64/arm64; runtime claims use only available runners. Local WDAC blocks Go test executables, so it supplies no runtime result. PR #36 task 2.13c has maintainer-authorized `size:exception` with a native 500-line ceiling: coherent fixture completion avoids artificial compaction, technical debt, and wasted iterations; it does not authorize unrelated growth.

> Historical (superseded): the former Draft PR #28 / issue #27 narrowing instruction is retained only as prior planning context and is not an active instruction.

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
- [x] 2.8 **RED (PR 3B.1b)**: Test `internal/ownership/sqlite/*_test.go` deterministic injected OS-query policy for app-data confinement, local-only/network/shared classification, ownership/restrictive permissions, symlink, and Windows reparse points; no interface-presence gate or mandatory mounted-network dependency.
- [x] 2.9 **GREEN (PR 3B.1b)**: Implement internal platform-query seams and fail-closed filesystem policy only; no transaction, integrity, or remote changes.
- [x] 2.10 **REFACTOR (PR 3B.1b)**: Compact per-platform fixtures/static/available-runner evidence; retain independently revertible policy rollback.
- [x] 2.11 **RED (PR 3B.1c-T)**: Retain only the four independent real-child-process RED cases in `internal/ownership/sqlite/ledger_transaction_red_test.go`: 25/50/100ms retry, cancellation, ambiguous-COMMIT exact-token readback, and deadline contention; remove the two masked integrity fixtures.
- [x] 2.12 **GREEN (PR 3B.1c-T)**: In `internal/ownership/sqlite/ledger.go`, add context-cancellable retries and post-COMMIT exact-token readback/retry semantics; end green in CI with no filesystem, integrity, remote, or recovery change.
- [x] 2.13 **REFACTOR (PR 3B.1c-T)**: Compact transaction/child-process fixtures, rerun focused CI and six-target build/static evidence, and keep PR #32 independently green below 400 lines.
- [x] 2.13a **RED→GREEN (PR 3B.1c-I)**: In `ledger_integrity_red_test.go`/`ledger.go`, complete isolated `Open` microcycles: invoke verification for new/existing ledgers; retain injected `passed` as approval-safety-net success; independently RED then GREEN-map not-run, corrupt, inconclusive, and bound-exceeded to `source.ErrOwnershipInvalid`, with no shared placeholder/gate.
- [x] 2.13b **RED→GREEN (PR 3B.1c-I)**: In `ledger_integrity_red_test.go`/`ledger.go`, complete isolated verifier microcycles: prove/implement ordered real `quick_check(1)` then eligible `integrity_check(1)`, existing-before-metadata ordering, one-second context for every query, real corruption/cancellation/>4 MiB; inject bounded rows/queries only for malformed/multiple/absent output, deterministic failure/blocking, and arithmetic overflow.
- [x] 2.13c **REFACTOR (PR 3B.1c-I)**: Compact verifier fixtures; record focused CI, runtime-harness, static, and six-target evidence; retain the independently revertible `ledger.go`/integrity-test boundary under the maintainer-authorized `size:exception` native 500-line ceiling.
- [x] 2.14 **RED→GREEN (PR 3B.2 — private acquisition boundaries)**: In `internal/source/acquire_test.go` and minimal production boundaries, independently RED then GREEN authenticated absolute home; private `0700`; exclusive random `0600` + `Lstat`; immutable-source approval/safety-net success; traversal/symlink rejection; durable `Admit`/readback before reserve/copy—no shared compile gate or synthetic test-only implementation.
- [x] 2.15 **RED→GREEN (PR 3B.2 — cleanup integration)**: In `internal/source/{acquire,retrieve}.go` and `internal/remote/ssh.go`, independently RED then GREEN exact private-path wiring and `Remove` + `Stat`-not-found before transactional DELETE; retain on failure and never add recovery-loop behavior.
- [x] 2.16 **REFACTOR (PR 3B.2)**: Consolidate acquisition fixtures; record focused CI/runtime/static evidence, independent rollback, and the maintainer-selected 800-line review ceiling; no snapshot when row or cleanup confirmation fails.
- [x] 2.17 **Recovery (3B.3)**: Delivered bounded stale-temporary recovery through strict RED→GREEN Slices A–C; Slice D owns later startup/pre-acquire invocation.
  - **Slice A — MERGED PR #40:** `LIMIT 65` SQLite listing returns exact validated rows only and fails without partial results on overflow/malformed data; `guardRecoveryRecord` accepts only valid source records. RED/GREEN GHA evidence and isolated rollback remain in `apply-progress.md`.
   - **Slice B — MERGED PR #41:** Completed RED→GREEN fresh recorded-profile resolution → credential retrieval → canonical target-binding comparison → pin/trust validation → constrained cleanup-ready exact-path callback, with GREEN implementation evidence GHA 32433893468 on `a54871d`. Each failure retains ownership and makes zero cleanup-ready/`Remove` calls; no Phase 3 native credential storage.
   - **Slice C — MERGED PR #42 (`eecd803783eeb176b5866313babf3886a76d47d5`):** Completed RED→GREEN bounded `RecoveryLedger` listing → Slice B guards → exact recorded-path `Remove` → `Stat`-not-found → transactional exact-row `Delete`. Code-head GREEN GHA 32435646241 proves crash idempotence and fail-closed list/corruption/retargeting/remove/stat/delete paths. It accepts only bounded ledger records and never discovers, lists, globs, or deletes historical `/tmp/bac-nexus-catalog-*` paths.
- [x] 2.18 **Recovery integration (3B.3)**: RED→GREEN wire `source.RecoveryCoordinator.Recover` before every `source.Acquirer.Acquire`; a recovery failure or retained unsafe row blocks opening new acquisition remote work. Do not add startup, app, or MCP wiring.
   - **Slice D — complete:** Pre-acquire gate plus task 2.19 operator/security documentation. `36e59f4` is behavior-first RED candidate authoring evidence and `200166e` is the GREEN implementation candidate; neither has a commit-specific GHA run. Final delivery-head runtime proof is GHA CI 32439957422 on `e927bf6`; WDAC supplies no local runtime result. No startup, app, or MCP scope. The coherent slice remains within the 800-line ceiling.
- [x] 2.19 **REFACTOR (PR 3B.3)**: Add `docs/SECURITY.md` operator/privileged-risk guidance and available cross-process/platform evidence; no MCP recovery operation.

Only canonical parent tasks use checkboxes: native count is 42, with 29 complete through 3.1. Task 3.2 is next. Every Slice B–D RED must compile against its preceding GREEN and prove independent behavior. Run `go test -count=1 ./internal/source ./internal/ownership/sqlite` in GitHub Actions for every later pair; WDAC blocks local test runtime and must not be bypassed. Roll back in reverse dependency order: Slice D/2.19, Slice C, Slice B, then Slice A; retain unresolved rows and never introduce an MCP recovery operation.

## Phase 3: Credentials, Policy, and Freshness

- [x] 3.1 **GATE (PR 5A)**: Verify PoC-exception `github.com/zalando/go-keyring` v0.2.8 (declares Go 1.18): exact module graph/SBOM, checksums, licenses/transitives, `govulncheck`/known vulnerabilities, no DLL/runtime download, platform compile/tests, endpoint policy. Failure blocks 5A.
  - **Historical failed attempt:** GHA Keyring Dependency Gate run `32441478559` on `6c878d88e465c325fffe39144abf26aeef6589b8` passed module verification and six `CGO_ENABLED=0` compile targets on all three runners; native upstream tests passed on macOS and Windows but failed on Ubuntu because `org.freedesktop.secrets` was unavailable.
  - **Successful precursor after maintainer scope decision:** GHA Keyring Dependency Gate run `32442432657` on `f1e15be5db0bed150fcc9a60d6b50981fc1e502f` installed GNOME Keyring in an isolated `dbus-run-session`, activated `org.freedesktop.secrets`, and passed upstream native `TestSet`, `TestGet`, and `TestDelete` behavior on Ubuntu. The Windows/macOS matrix, six compile targets, graph/checksum verification, and `govulncheck` all passed. This final-artifact candidate requires a new exact-head gate run before settlement. Corporate endpoint-policy approval remains deferred and unproven rollout evidence, not task 3.1 completion evidence. Tasks 3.2–3.4 remain untouched.
- [x] 3.2 **RED (PR 5A)**: Test `internal/credential/*_test.go` only exact Get/Set/Delete, grammar/1–4096 bounds, redaction/zeroing, unavailable-before-remote, fixed macOS stdin/no argv-env, and Windows/Linux deterministic failures.
- [x] 3.3 **GREEN (PR 5A)**: Add `CredentialStore`/keyring adapter: Credential Manager, fixed `/usr/bin/security`, Secret Service D-Bus; explicit vault write/readback/zero/delete migration, retaining vault when native storage fails.
- [x] 3.4 **REFACTOR (PR 5A)**: Isolate consumer and per-platform tests; preserve available-runner-only evidence.
- [x] 3.5 **RED (PR 5B)**: Test `internal/{security,audit}/*_test.go` selector and spoofed `clientInfo`, TOFU changes, allowlisted audit redaction, and no remote/generic path operations.
- [x] 3.6 **GREEN (PR 5B)**: Add `internal/security/policy.go` and `internal/audit/audit.go` pinned trust and sanitized outcomes, depending on 5A credentials.
- [x] 3.7 **REFACTOR (PR 5B)**: Table-drive policy/audit fakes and rerun both package commands.
- [x] 3.8 **RED (PR 6)**: Test `internal/app/service_test.go` 50-result bound, re-query/stale, page bytes, and credential/policy denial before remote work.
- [x] 3.9 **GREEN (PR 6)**: Create deterministic `internal/app/service.go` over 2, 3B.3, and 5B; invoke `source.RecoveryCoordinator.Recover` during real Nexus process startup before service availability, retaining the 3.8 freshness/credential/policy service scope.
- [x] 3.10 **REFACTOR (PR 6)**: Consolidate service fakes and freshness cases.

## Phase 4: MCP and Acceptance

- [x] 4.1 **RED (PR 7)**: Test `internal/mcp/server_test.go` and `cmd/nexus/main_test.go` typed tools, no paths, deterministic errors, cancellation, and shutdown.
- [x] 4.2 **GREEN (PR 7)**: Create `internal/mcp/server.go` and `cmd/nexus/main.go`; pin SDK, wire 5B/6, and expose no generic remote tool.
- [x] 4.3 **REFACTOR (PR 7)**: Update `README.md` and `docs/SECURITY.md`; preserve `catalogspike` and full tests.
- [ ] 4.4 **Acceptance (PR 8)**: Run documented approved IBM i line-1/EOF/newline/success-cancel cleanup evidence without retained source; verify `go test -count=1 ./...`.

Rollback: stop acquisition; revert 3B.3→3B.2→3B.1c→3B.1b→3B.1a while retaining unresolved rows. Revert 5B, then 5A only after preserving accessible secrets and migration state.
