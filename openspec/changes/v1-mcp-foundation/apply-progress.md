# Apply Progress: v1 MCP Foundation

## Cumulative Task State
- [x] 1.1–1.6 — source foundation.
- [x] 2.1–2.4 — remote snapshot safety and SQLite dependency gate.
- [x] 2.5–2.7 — SQLite ledger foundation.
- [x] 2.8–2.10 — filesystem policy.
- [x] 2.11–2.13 — transaction retry/readback and refactor.
- [x] 2.13a–2.13c — integrity verifier microcycles and evidence refactor.
- [x] 2.14 — private acquisition boundaries.
- [x] 2.15 — exact private cleanup and transactional ownership deletion.
- [x] 2.16 — acquisition fixture refactor and final evidence.
- [x] 2.17a — bounded recovery-list RED contract and row-65 overflow evidence.
- [x] 2.17b — bounded SQLite recovery-list GREEN boundary and malformed-row fail-closed evidence.
- [ ] 2.17c and later — recovery coordination and remaining product scope.

Task count: 27/42 complete.

## Strict TDD Cycle Evidence
| Task / microcycle | Test file / layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|
| 2.11–2.13c | SQLite transaction/integrity package tests | Prior GHA baselines recorded in earlier apply-progress revisions; WDAC blocks local test execution. | Independent transaction/integrity RED runs recorded through CI 32419671217. | GHA package/full-suite and vet green through CI 32419671217. | Distinct retry, cancellation, mapping, ordering, corruption, size, and bounded-query paths. | Completed in task 2.13/2.13c only. |
| 2.14 private acquisition | `internal/source/acquire_test.go`, GHA Ubuntu source fakes | CI 32419671217 green before acquisition work; local Go test execution blocked by WDAC. | CI 32421871674, 32422071150, 32423035573, 32423204499, and 32423417915 independently failed the missing home, directory, reservation, escape, and admission behaviors. | CI 32423941299 passed `go test -count=1 ./...` and `go vet ./...`. | Home, directory, exclusive-file/Lstat, traversal/symlink, admission ordering, and immutable-source cases exercise distinct paths. | Not assigned; task 2.16 remains pending. |
| 2.15 confirmed cleanup deletion | `internal/source/acquire_test.go`, `internal/ownership/sqlite/ledger_test.go`, GHA temporary SQLite/source fakes | CI 32423941299 was green before task 2.15; WDAC blocked local test execution. | CI 32424557263 failed only `TestAcquirerDeletesExactOwnershipOnlyAfterConfirmedPrivateCleanup`: `cleanup/delete/events = 1/0/[admit reserve copy stat download remove stat]`. | CI 32424770121 passed `go test -count=1 ./...` and `go vet ./...`. | Success proves exact record deletion after `Remove`/not-found `Stat`; remove failure, stat error, and still-present path each retain the row; SQLite proves exact-record transactional deletion/mismatch rejection. | Not assigned; task 2.16 remains pending. |
| 2.16 acquisition fixture refactor | `internal/source/acquire_test.go`, package-local approval tests | GHA CI 32425042252 passed before the refactor; WDAC blocked local test execution. | Existing 2.14/2.15 approval tests were retained unchanged; no behavior change was specified or introduced. | GHA CI 32425625908 passed `go test -count=1 ./...` and `go vet ./...` on the refactor commit. | Existing independent acquisition, ordering, cleanup, and uncertain-cleanup cases retain distinct request/cleanup fixture state. | Replaced repeated default request/cleanup/ledger construction with one explicit fixture constructor; CI remained green. |
| 2.17a bounded recovery list RED | `internal/ownership/sqlite/ledger_recovery_test.go`, package-private SQLite boundary | New test file; prior GHA CI 32425813926 was green on the preceding 3B.2 head; WDAC blocks local test execution. | GHA CI 32428048429 and the evidence-commit CI 32428236175 compiled the suite and failed only both new subtests because `Ledger does not implement bounded recovery listing`. | Not assigned — task 2.17b is the paired GREEN. | Exact valid rows in creation order and row 65 overflow are independent RED scenarios. | Not assigned — RED-only task; no production code changed. |
| 2.17b bounded recovery list GREEN | `internal/ownership/sqlite/ledger_recovery_test.go`, package-private SQLite/temporary-DB boundary | The 2.17a RED evidence provides the pre-production baseline: GHA CI 32428355060 failed only the absent-boundary subtests; WDAC blocks local Go test execution. | 2.17a supplied the original exact-order and row-65 RED; a malformed canonical-time row was added before production code and compile-checked locally without running a test binary. | GHA CI 32429026447 passed `go test -count=1 ./...` and `go vet ./...` on `464bfac67666e9d5ff6abb571baf356e123143df`. | Exact ordered rows, row 65 overflow, and malformed-row no-partial-result cases exercise distinct success and fail-closed paths. | None needed — the minimum package-private query/row mapping is clear and independently bounded. |

## Work Unit Evidence: 3B.2 task 2.14
| Evidence | Result |
|---|---|
| Focused test command | GHA CI 32423941299: `go test -count=1 ./...` and `go vet ./...` exited 0. |
| Runtime harness | GHA Ubuntu source fakes exercised private acquisition behaviors. No live IBM i boundary exists; WDAC blocked local test binaries and was not bypassed. |
| Rollback boundary | Revert commits `6ba0d1f` through `7790eed` to remove only private acquisition/reservation tests and boundaries. |

## Work Unit Evidence: 3B.2 task 2.15
| Evidence | Result |
|---|---|
| Focused test command | GHA CI 32424770121: `go test -count=1 ./...` and `go vet ./...` exited 0 after the task 2.15 GREEN. |
| Runtime harness | GHA Ubuntu ran the source fake and temporary SQLite harness: exact private path cleanup performs `Remove`, then `Stat`-not-found, then a transactional exact-record delete. No live IBM i boundary exists. |
| Static / builds | Local `gofmt`, `go vet ./...`, compile-only `go test -c` for `./internal/source` (output removed), six `CGO_ENABLED=0` builds for windows/darwin/linux × amd64/arm64, and `git diff --check` exited 0. No local Go test binary ran. |
| Rollback boundary | Revert `fb60a3a` and `1d40c5a` to remove only confirmed cleanup deletion tests, the ownership `Delete` contract/SQLite transaction, and acquisition cleanup wiring; reservation, copy, and recovery remain untouched. |

## Work Unit Evidence: 3B.2 task 2.16
| Evidence | Result |
|---|---|
| Focused test command | GHA CI 32425625908: `go test -count=1 ./...` and `go vet ./...` exited 0 on `5a17a1ae89a08b4cbc3c6eeda611663b3973746b`. |
| Runtime harness | GHA Ubuntu executed the in-process source fake and temporary SQLite cleanup path. No live IBM i runtime boundary exists for this fixture-only refactor; WDAC blocked local test binaries and was not bypassed. |
| Static / builds | Local `gofmt -d internal/source/acquire_test.go`, `go vet ./...`, compile-only `go test -c` for `./internal/source` (output removed), six `CGO_ENABLED=0 go build ./...` targets for windows/darwin/linux × amd64/arm64, and `git diff --check` exited 0. No local Go test binary ran. |
| Rollback boundary | Revert `5a17a1a` to restore only repeated acquisition test fixture construction in `internal/source/acquire_test.go`; 2.14/2.15 production behavior, independent tests, cleanup lifecycle, and all recovery scope remain unchanged. |

## Work Unit Evidence: 3B.3 task 2.17a RED
| Evidence | Result |
|---|---|
| Focused test command | GHA CI 32428048429 and evidence-commit CI 32428236175 executed the repository Go Verification command `go test -count=1 ./...`; the latter ran on `80281cf084f2ce3742f0c53cf875c337bc7defba`. `internal/source` and every unrelated package passed; `internal/ownership/sqlite` compiled and failed only `TestLedgerListsBoundedValidatedRecoveryRows` because the bounded recovery-list boundary is intentionally absent. |
| Runtime harness | GHA Ubuntu created temporary SQLite ledgers and ran both RED subtests: exact valid-row ordering and row-65 overflow. No live IBM i runtime boundary exists; WDAC blocked local Go test binaries and was not bypassed. |
| Static / builds | Local `gofmt -d internal/ownership/sqlite/ledger_recovery_test.go`, `go vet ./...`, `go test -c -o NUL ./internal/ownership/sqlite`, and `git diff --check` exited 0. The compile check did not execute a local Go test binary. |
| Rollback boundary | Revert `673dca3` to remove only `ledger_recovery_test.go` and the recovery microcycle planning correction in `tasks.md`; production behavior, source recovery coordination, remote behavior, and later recovery tasks remain absent. |

## Work Unit Evidence: 3B.3 task 2.17b GREEN
| Evidence | Result |
|---|---|
| Focused test command | GHA CI 32429026447 executed `go test -count=1 ./...` with exit 0; it includes the focused `internal/ownership/sqlite` recovery-list package tests. The same run executed `go vet ./...` with exit 0. |
| Runtime harness | GHA Ubuntu used temporary SQLite ledgers to prove exact creation-order rows, row-65 overflow returning no rows, and a malformed canonical-time row returning `source.ErrOwnershipInvalid` with no partial rows. No live IBM i runtime boundary exists; WDAC blocked local Go test binaries and was not bypassed. |
| Static / builds | Local `gofmt -d internal/ownership/sqlite/ledger.go internal/ownership/sqlite/ledger_recovery_test.go`, `go vet ./...`, `go test -c -o NUL ./internal/ownership/sqlite`, and `git diff --check` exited 0. The compile check did not execute a local Go test binary. |
| Rollback boundary | Revert `464bfac` to remove only the package-private `Ledger.listRecovery` boundary and its malformed-row triangulation test; source recovery coordination, credentials, target/pin validation, remote cleanup, historical discovery, and later recovery tasks remain absent. |

## Delivery / exclusions
- Draft PR #40 is stacked-to-main → `main`, open and draft; issue #39 is approved and linked.
- Maintainer-selected review ceiling: 800 authored additions + deletions. The RED code/planning commit is 116 additions + 3 deletions = 119, under the ceiling; it remains a ceiling, not permission for unrelated scope. Delivery remains `ask-on-risk` resolved as `stacked-to-main`.
- Excluded: task 2.17c and later; source recovery coordination, credential/pin/remote behavior, historical `/tmp` behavior, acquisition/rescope/reset/settle, and merge.

## Settlement Handoff
- Authorized attempt token: `sha256:54c4e688316ff4f75bd6ec17380c343246bb2a3fcf57053866d018c11d02d3e5`.
- Passing settlement obligation retained by parent: `--remediates-evidence-revision sha256:5a12bd8fbdc02a10b6be03dd4e084704e7310e3a109dbad9002bd91ec5a09b38`.
- The distinct passing evidence revision and final-head GHA receipt are returned to the parent after the artifact-evidence commit completes; this executor does not acquire, rescope, reset, settle, or merge.
- The parent/orchestrator retains acquire, rescope, reset, settle, recovery, and merge authority.
