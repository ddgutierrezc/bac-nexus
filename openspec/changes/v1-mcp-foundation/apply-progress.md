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
- [x] 2.17 — bounded stale-temporary recovery is complete through Slice C: Slice A merged in PR #40 (`LIMIT 65` listing, no partial results, and `guardRecoveryRecord` RED/GREEN); Slice B merged in PR #41 (fresh identity guards); Slice C merged in PR #42 as `eecd803783eeb176b5866313babf3886a76d47d5` (exact cleanup and exact-record deletion).
- [x] 2.18 — source-owned pre-acquire recovery gate is complete; real Nexus startup composition remains deferred to task 3.9.
- [x] 2.19 — operator/security documentation and evidence refactor is complete; no MCP recovery operation exists.

Task count: 28/42 canonical parent tasks complete.

## Strict TDD Cycle Evidence
| Task / microcycle | Test file / layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|
| 2.11–2.13c | SQLite transaction/integrity package tests | Prior GHA baselines recorded in earlier apply-progress revisions; WDAC blocks local test execution. | Independent transaction/integrity RED runs recorded through CI 32419671217. | GHA package/full-suite and vet green through CI 32419671217. | Distinct retry, cancellation, mapping, ordering, corruption, size, and bounded-query paths. | Completed in task 2.13/2.13c only. |
| 2.14 private acquisition | `internal/source/acquire_test.go`, GHA Ubuntu source fakes | CI 32419671217 green before acquisition work; local Go test execution blocked by WDAC. | CI 32421871674, 32422071150, 32423035573, 32423204499, and 32423417915 independently failed the missing home, directory, reservation, escape, and admission behaviors. | CI 32423941299 passed `go test -count=1 ./...` and `go vet ./...`. | Home, directory, exclusive-file/Lstat, traversal/symlink, admission ordering, and immutable-source cases exercise distinct paths. | Not assigned; task 2.16 remains pending. |
| 2.15 confirmed cleanup deletion | `internal/source/acquire_test.go`, `internal/ownership/sqlite/ledger_test.go`, GHA temporary SQLite/source fakes | CI 32423941299 was green before task 2.15; WDAC blocked local test execution. | CI 32424557263 failed only `TestAcquirerDeletesExactOwnershipOnlyAfterConfirmedPrivateCleanup`: `cleanup/delete/events = 1/0/[admit reserve copy stat download remove stat]`. | CI 32424770121 passed `go test -count=1 ./...` and `go vet ./...`. | Success proves exact record deletion after `Remove`/not-found `Stat`; remove failure, stat error, and still-present path each retain the row; SQLite proves exact-record transactional deletion/mismatch rejection. | Not assigned; task 2.16 remains pending. |
| 2.16 acquisition fixture refactor | `internal/source/acquire_test.go`, package-local approval tests | GHA CI 32425042252 passed before the refactor; WDAC blocked local test execution. | Existing 2.14/2.15 approval tests were retained unchanged; no behavior change was specified or introduced. | GHA CI 32425625908 passed `go test -count=1 ./...` and `go vet ./...` on the refactor commit. | Existing independent acquisition, ordering, cleanup, and uncertain-cleanup cases retain distinct request/cleanup fixture state. | Replaced repeated default request/cleanup/ledger construction with one explicit fixture constructor; CI remained green. |
| 2.17a bounded recovery list RED | `internal/ownership/sqlite/ledger_recovery_test.go`, package-private SQLite boundary | New test file; prior GHA CI 32425813926 was green on the preceding 3B.2 head; WDAC blocks local test execution. | GHA CI 32428048429 and the evidence-commit CI 32428236175 compiled the suite and failed only both new subtests because `Ledger does not implement bounded recovery listing`. | Not assigned — task 2.17b is the paired GREEN. | Exact valid rows in creation order and row 65 overflow are independent RED scenarios. | Not assigned — RED-only task; no production code changed. |
| 2.17b bounded recovery list GREEN | `internal/ownership/sqlite/ledger_recovery_test.go`, package-private SQLite/temporary-DB boundary | The 2.17a RED evidence provides the pre-production baseline: GHA CI 32428355060 failed only the absent-boundary subtests; WDAC blocks local Go test execution. | 2.17a supplied the original exact-order and row-65 RED; a malformed canonical-time row was added before production code and compile-checked locally without running a test binary. | GHA CI 32429026447 passed `go test -count=1 ./...` and `go vet ./...` on `464bfac67666e9d5ff6abb571baf356e123143df`. | Exact ordered rows, row 65 overflow, and malformed-row no-partial-result cases exercise distinct success and fail-closed paths. | None needed — the minimum package-private query/row mapping is clear and independently bounded. |
| 2.17c.1 initial source recovery seam compile RED | `internal/source/ownership_recovery_test.go`, package-local compiler boundary | N/A (new test file); WDAC blocks local Go test execution. | GHA CI 32430525136 ran the test first on `1ed0b167a05e7321808a46fa9983acc99c9eab80` and failed only `internal/source/ownership_recovery_test.go:18:12: undefined: guardRecoveryRecord`. | N/A — authorized RED-only work unit; 2.17c.2 owns the minimal production seam. | N/A — one canonical exact record establishes the first absent seam; malformed/unavailable behavior belongs to 2.17c.2. | N/A — no production code exists to refactor. |
| 2.17c.2 source recovery-record guard GREEN | `internal/source/ownership_recovery_test.go`, package-private source validation | 2.17c.1 authorized absent-symbol RED; WDAC blocks local Go test execution. | GHA CI 32431142613 on clean head `033409867f399c2434ff032ab2a0223327384024` failed only the absent `guardRecoveryRecord` symbol. The malformed/unavailable table was added before production code and compile-checked without executing a local test binary. | GHA CI 32431657989 passed `go test -count=1 ./...` and `go vet ./...` on `a9c17d629752bcf2b7a60851022bf6f2e1779896`. | One exact valid immutable record passes; malformed token, relative/traversal/historical paths, malformed profile, unavailable digest, and unavailable/non-canonical times return `ErrOwnershipInvalid`. The guard accepts only an `OwnershipRecord`, so it has no ledger mutation or remote dependency. | None needed — the source-only record guard remains package-private and has no recovery coordination. |
| 2.17 Slice B fresh identity guards | `internal/source/ownership_recovery_guards_test.go`, package-private source coordinator | Slice A final GHA CI 32431834329 passed on merged PR #40; WDAC blocked local Go test runtime. | GHA CI 32433749715 ran the compiling suite on `43a92b1f34ddac2a868e098015bfd5326dc5d447`; only `TestRecoverOwnershipRecordRunsFreshGuardsBeforeExactPathCallback` failed because the temporary fail-closed seam returned `ErrOwnershipInvalid`. | GHA CI 32433893468 passed `go test -count=1 ./...` and `go vet ./...` on `a54871d76a8c99d55a53f4b4b2c5381f29659f4d`. | Success proves fresh profile → credential → constrained pinned opener → exact-path callback and secret clearing. Separate cancelled, resolver, credential, target-binding, invalid pin, and opener failures stop at their exact stage without cleanup-ready. | None needed — package-private consumer-owned callback types keep Slice C remote removal/deletion unavailable. |
| 2.17 Slice C exact cleanup recovery | `internal/source/ownership_recovery_coordinator_test.go`, `internal/source/ownership_recovery_guards_test.go`, `internal/ownership/sqlite/ledger_recovery_test.go`; source/SQLite fake and temporary-database layer | Slice B final GHA CI 32434676786 passed on merged PR #41; WDAC blocked local Go test runtime. | GHA CI 32435522491 ran `go test -count=1 ./...` on `9c82eac`; existing packages passed and only the new Slice C expected exact-list/deletion and coordinator behavior failed from fail-closed declarations. | Code-head GREEN GHA CI 32435646241 passed `go test -count=1 ./...` and `go vet ./...` on `46577df33f78aecb314a5aebd948989522c40513`. | Two exact records cover pre/during-copy removal and already-absent retry; a repeat finds no rows. List failure, corrupt historical row, retargeting, uncertain remove, present Stat, and Delete failure each retain the row and stop without discovery. | Extracted the shared `removalRemote` capability so acquisition and recovery reuse the exact `Remove` → `Stat` confirmation without broadening `AcquisitionRemote` or `OwnershipLedger`. |
| 2.18 pre-acquire recovery gate | `internal/source/acquire_test.go`; source fake unit layer | Prior Slice C GHA CI 32435646241 passed `go test -count=1 ./...` and `go vet ./...`; WDAC blocks local Go test runtime. | Commit `36e59f4` is behavior-first RED candidate authoring evidence: tests were committed before the recovery-gate production change. No commit-specific GitHub Actions run or runtime RED result is available. | Commit `200166e` is the GREEN implementation candidate commit. No commit-specific GitHub Actions run exists. Runtime proof is final GHA CI 32439957422 on `e927bf6`, which passed repository tests and vet. | Missing dependency, recovery error, cancelled context, and success cover fail-closed/no-open/no-copy/no-admit and one successful existing acquisition path. | The named `successfulAcquisitionRecovery` no-op is test-only; production composition must supply recovery explicitly. |
| 2.19 security documentation | `docs/SECURITY.md`; documentation layer | The final runtime proof for the Slice D delivery head is GHA CI 32439957422 on `e927bf6`; WDAC blocks local Go test runtime. | N/A — documentation-only refactor with no production behavior. | No documentation-specific runner is configured; GHA CI 32439957422 passed repository tests and vet for the delivery head. | The document separately covers exact ownership cleanup, pre-acquire blocking, privileged/same-principal risk, evidence limits, rollout gates, and rollback. | No fixture refactor was needed beyond the explicit test-only recovery fixture required by task 2.18. |

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

## Work Unit Evidence: 3B.3 task 2.17c.1 RED
| Evidence | Result |
|---|---|
| Focused test command | GHA CI 32430525136 ran `go test -count=1 ./...` on `1ed0b167a05e7321808a46fa9983acc99c9eab80` with exit 1. Its only compiler error was `internal/source/ownership_recovery_test.go:18:12: undefined: guardRecoveryRecord`; `internal/ownership/sqlite` passed and the unrelated `cmd/catalogspike`, `internal/catalog`, `internal/credential`, `internal/mapepire`, `internal/profile`, `internal/remote`, and `internal/strictjson` packages compiled/passed. |
| Runtime harness | GHA Ubuntu exercised the repository Go compiler/package harness. The intentional missing symbol prevents an `internal/source` test binary by design; no runtime boundary exists before 2.17c.2. No live IBM i boundary exists, and WDAC blocked local Go test binaries without bypass. |
| Static / builds | Local `gofmt -d internal/source/ownership_recovery_test.go` and `git diff --check` exited 0. Compile-only `go test -c -o NUL ./internal/source` exited 1 only with `undefined: guardRecoveryRecord`; no local Go test binary ran. |
| Rollback boundary | Revert `1ed0b16` and this evidence commit to remove only `internal/source/ownership_recovery_test.go`, the 2.17c microcycle planning correction, and this progress evidence. No source guard, profile/credential/binding/pin/remote logic, cleanup, or production behavior exists. |

## Work Unit Evidence: 3B.3 task 2.17c.2 GREEN
| Evidence | Result |
|---|---|
| Focused test command | GHA CI 32431657989 executed `go test -count=1 ./...` with exit 0 on `a9c17d629752bcf2b7a60851022bf6f2e1779896`; it includes `internal/source/ownership_recovery_test.go`. The same run executed `go vet ./...` with exit 0. |
| Runtime harness | GHA Ubuntu exercised the package-local source validation through the real Go test binary. No remote-runtime boundary exists for this pure value guard: it receives only `OwnershipRecord` and has no ledger or remote callback, so invalid records retain ownership by returning before later recovery stages and make zero remote calls. WDAC was not bypassed. |
| Static / builds | Local `gofmt -w internal/source/ownership.go internal/source/ownership_recovery_test.go`, `git diff --check`, `go vet ./...`, and compile-only `go test -c -o NUL ./internal/source` exited 0. No local Go test binary ran. |
| Rollback boundary | Revert `a9c17d6` to remove only the package-private source guard and its direct tests. It does not remove the prior 2.17c.1 RED, SQLite list boundary, acquisition behavior, or any later profile/credential/binding/pin/remote cleanup scope. |

## Work Unit Evidence: 3B.3 Slice B fresh identity guards
| Evidence | Result |
|---|---|
| Focused test command | GREEN implementation GHA CI 32433893468: `go test -count=1 ./...` exited 0, including `internal/source` fresh-guard coverage; `go vet ./...` exited 0 on `a54871d76a8c99d55a53f4b4b2c5381f29659f4d`. |
| Runtime harness | GHA Ubuntu executed package-local fake profile, credential, constrained remote, and cleanup-ready callbacks. It proved exact-stage failure stopping and zero cleanup-ready calls; no `Remove`, `Stat`, or `Delete` contract exists in Slice B. WDAC blocked local test binaries and was not bypassed. |
| Rollback boundary | Revert `43a92b1` and `a54871d` plus the Slice B artifact-evidence correction commit to remove only Slice B package-private guard seams/tests and its progress wording; Slice A listing/record validation remains and unresolved ownership rows are retained. |

## Work Unit Evidence: 3B.3 Slice C exact cleanup recovery
| Evidence | Result |
|---|---|
| Focused test command and exact result | Code-head GREEN GHA CI 32435646241: `go test -count=1 ./...` exit 0, including `internal/source` coordinator/guard fakes and `internal/ownership/sqlite` bounded-list tests; `go vet ./...` exit 0. |
| Runtime harness command/scenario and exact result | GHA Ubuntu ran the source fake ledger/remote and temporary SQLite harness. It proved one bounded list, sequential exact `Remove` then `Stat`-not-found then exact `Delete`, already-absent idempotence, no-op rerun, and row retention on uncertainty/corruption/retargeting/delete failure. No live IBM i runtime boundary exists; WDAC was not bypassed. |
| Rollback boundary | Revert `9c82eac` and `46577df` plus this evidence/artifact commit to remove only Slice C `RecoveryLedger`/`RecoveryRemote` contracts, exported SQLite list adapter, exact cleanup coordinator, shared confirmation capability, and their tests. Slice A/B remain; unresolved rows are retained. |

## Work Unit Evidence: 3B.3 Slice D pre-acquire gate and security documentation
| Evidence | Result |
|---|---|
| Focused test command and exact result | No local command result is admissible under WDAC. `36e59f4` is behavior-first RED candidate authoring evidence because the focused recovery tests were committed before production; no runtime RED result or commit-specific GHA run exists. `200166e` is the GREEN implementation candidate commit; no commit-specific GHA run exists. GHA CI 32439957422 on `e927bf6` ran `go test -count=1 ./...` and `go vet ./...` successfully. |
| Runtime harness command/scenario and exact result | Runtime proof is exclusively GitHub Actions. GHA CI 32439957422 on `e927bf6` passed the repository test and vet steps for the Slice D delivery head. Its package fakes exercise recovery failure/cancellation before acquisition open/copy/admit and success through the existing acquisition flow; no live IBM i or process-startup boundary exists. The correction-head GHA triggered by this evidence-only commit is recorded in the delivery result; no subsequent commit is made to embed its run identifier. |
| Rollback boundary | Revert `36e59f4` and `200166e` plus the Slice D documentation/artifact commit to remove only `AcquisitionRecovery`, the pre-acquire gate/tests, `docs/SECURITY.md`, and Slice D status/evidence. Earlier exact recovery slices remain; unresolved ownership rows must be retained. |

## Current Delivery and Scope
- PR #40 is MERGED as `11e1c46c18ca7d8092cbe0babad65aa82486f205`, PR #41 is MERGED as `7d57617c6a5270d6949c25b4baa0943ddcbf3468`, and PR #42 is MERGED as `eecd803783eeb176b5866313babf3886a76d47d5`; issue #39 is OPEN and approved.
- Slice A is settled complete and merged. Its final GHA evidence is 32431834329. Delivery remains `ask-on-risk`, resolved to the `stacked-to-main` chain strategy; the 800-line ceiling remains a boundary, not permission for unrelated scope.
- PR #42 completed Slice C at code-head GREEN commit `46577df33f78aecb314a5aebd948989522c40513`; GHA CI 32435646241 passed full test and vet commands. `RecoveryLedger` is consumer-owned and has only bounded `ListRecovery` plus exact `Delete`; SQLite's exported adapter delegates to its tested package-private `LIMIT 65` list. `RecoveryRemote` has only `Close`, exact `Remove`, and exact `Stat`. The coordinator lists once, runs every record through Slice B guards, confirms absence before deletion, and stops on any uncertain result.
- Slice D completes tasks 2.18/2.19 with a required source-owned recovery dependency invoked before every acquisition remote open. It excludes real Nexus startup/app composition (task 3.9), Phase 3 native credential storage, MCP, remote discovery/list/glob, historical `/tmp` handling, and any `cmd/catalogspike` behavior change. Task 3.1 is next. `36e59f4` and `200166e` have no commit-specific GHA checks; final delivery-head runtime proof is GHA CI 32439957422 on `e927bf6`. The correction-head run is carried by the delivery result and PR metadata without a later commit.

## Attempt 5A: Task 3.1 Keyring Dependency Gate — BLOCKED

Task 3.1 remains unchecked. No `CredentialStore`, keyring adapter, credential test, migration, remote credential path, or task 3.2–3.4 behavior was created.

### Candidate Runtime Graph and Integrity

The candidate is explicitly pinned only for gate evaluation at `6c878d88e465c325fffe39144abf26aeef6589b8`; no application package imports it yet, so this is a resolved candidate runtime graph rather than a claim that a credential feature ships.

| Module | Role | Module checksum | `go.mod` checksum |
|---|---|---|---|
| `github.com/zalando/go-keyring v0.2.8` | Candidate direct module; upstream tag commit `a8cdfe320cc8bc0534c895a648dc9214715a9da5`; declares Go 1.18 | `h1:6sD/Ucpl7jNq10rM2pgqTs0sZ9V3qMrqfIIy5YPccHs=` | `h1:tsMo+VpRq5NGyKfxoBVjCuMrG47yj8cmakZDO5QGii0=` |
| `github.com/danieljoos/wincred v1.2.3` | Windows Credential Manager backend | `h1:v7dZC2x32Ut3nEfRH+vhoZGvN72+dQ/snVXo/vMFLdQ=` | `h1:6qqX0WNrS4RzPZ1tnroDzq9kY3fu1KwE7MRLQK4X0bs=` |
| `github.com/godbus/dbus/v5 v5.2.2` | Linux Secret Service D-Bus backend | `h1:TUR3TgtSVDmjiXOgAAyaZbYmIeP3DPkld3jgKGV8mXQ=` | `h1:3AAv2+hPq5rdnr5txxxRwiGjPXamgoIHgz9FPBfOp3c=` |
| `golang.org/x/sys v0.47.0` | Existing project-selected Windows API support; satisfies keyring's `v0.27.0` minimum under MVS | `h1:o7XGOvZQCADBQQ4Y7VNq2dRWQR7JmOUW8Kxx4ZsNgWs=` | `h1:4GL1E5IUh+htKOUEOaiffhrAeqysfVGipDYzABqnCmw=` |

`go mod verify` exited 0 locally without executing a generated Go binary. GitHub Actions ran `go mod download all`, `go mod verify`, and `go list -m -json all` successfully on Windows, macOS, and Ubuntu in run `32441478559`.

### License and Source Review

| Module | License evidence | Runtime behavior evidence |
|---|---|---|
| `go-keyring` | `LICENSE`: MIT | Platform-selected source has no HTTP/download API. Windows calls `wincred`; macOS executes only fixed `/usr/bin/security`; Linux opens the local D-Bus Secret Service. |
| `wincred` | `LICENSE`: MIT | `sys.go` uses `windows.NewLazySystemDLL("advapi32.dll")`, a preinstalled Windows system DLL load. No DLL download is present. |
| `godbus/dbus/v5` | `LICENSE`: BSD-3-Clause | Used for local D-Bus IPC; the reviewed keyring graph contains no HTTP client/download call. |
| `golang.org/x/sys` | `LICENSE`: BSD-3-Clause | Provides platform syscall bindings; no downloaded runtime binary is introduced. |

The code review proves no runtime binary or DLL download behavior in the evaluated sources. It does not claim that an enterprise endpoint permits Windows system-DLL loading, macOS `/usr/bin/security`, or Linux D-Bus access.

### Vulnerability Evidence and Limits

- Direct OSV queries for `go-keyring v0.2.8`, `wincred v1.2.3`, and `godbus/dbus/v5 v5.2.2` returned no vulnerability records.
- The direct OSV query for resolved `golang.org/x/sys v0.47.0` returned historical advisories whose fixed ranges end before `v0.47.0`; none applies to the resolved version.
- GHA macOS and Windows ran `go run golang.org/x/vuln/cmd/govulncheck@v1.7.0 github.com/zalando/go-keyring`: both reported `No vulnerabilities found` and `Your code is affected by 0 vulnerabilities`. The tool is tooling-only; it downloaded `golang.org/x/vuln v1.7.0` and `golang.org/x/mod v0.39.0`, which are not candidate runtime modules.
- The scanner also reported uncalled package/module findings for the pre-existing full project graph (macOS: 3 package/43 module; Windows: 4 package/42 module). They are not evidence that the keyring candidate is clean or affected; they require separate project-level review if shipment proceeds.
- Ubuntu did not execute `govulncheck` because its required preceding native test failed; this must not be represented as Ubuntu vulnerability proof.

### TDD Cycle Evidence

| Task | Test file / layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|
| 3.1 dependency gate | GHA three-runner upstream-module runtime layer | Local runtime blocked by WDAC. GHA `go test -count=1 ./...` run `32441478626` failed an unrelated existing SQLite contention test (`TestLedgerAdmissionUsesExactRetrySchedule`), so it is not a valid safety-net pass. | N/A — this is a supply-chain gate; creating task 3.2 credential tests or production behavior is prohibited. | **FAILED** — required Ubuntu native Secret Service test failed. | Windows, macOS, and Ubuntu ran the same six compile targets; native runtime passed on Windows/macOS and actually failed on Ubuntu. | None — fail closed at the first required gate failure. |

### Work Unit Evidence: 5A-keyring-dependency-gate

| Evidence | Result |
|---|---|
| Focused test command and exact result | GHA `32441478559`, head `6c878d88e465c325fffe39144abf26aeef6589b8`: `go test -count=1 github.com/zalando/go-keyring` passed on macOS (`ok`, `0.413s`) and Windows (`ok`, `0.095s`), but Ubuntu exited 1 because `org.freedesktop.secrets` was not provided by any `.service` file. |
| Runtime harness command/scenario and exact result | The unmodified upstream native tests exercised each runner's actual native backend, not a mock harness. Windows Credential Manager and macOS Keychain tests passed. Ubuntu attempted the real D-Bus Secret Service and failed because the service was unavailable; this is evidence of unavailable runner capability, not proof of Linux admission. |
| Static / compile evidence | Local `go mod verify`, six `GOOS/GOARCH` `CGO_ENABLED=0 go test -c -o NUL github.com/zalando/go-keyring`, and `git diff --check` exited 0; no local generated Go binary ran. The GHA matrix repeated all six compile targets successfully on each runner. |
| Endpoint-policy admission | **NOT PROVEN.** The available GitHub runner result cannot establish corporate endpoint policy for fixed macOS executable access, Windows `advapi32.dll`, or Linux D-Bus/Secret Service. The actual Ubuntu service absence also blocks technical Linux runtime admission. |
| Rollback boundary | Revert `6c878d8` to remove only the candidate `go-keyring` pin/checksums and `.github/workflows/keyring-dependency-gate.yml`. No credential behavior, remote path, migration, or tasks 3.2–3.4 implementation exists. |

### Delivery Status

- Issue: #44 (open, `status:approved`)
- Branch: `chore/keyring-dependency-gate`
- Evidence-only draft PR: #45, targeting `main`, intentionally `Part of #44` and not closing the issue
- Gate decision: **BLOCKED** — actual Ubuntu Secret Service absence and unproven corporate endpoint-policy admission prevent task completion.
- Required next action: do not start tasks 3.2–3.4. A maintainer must supply an approved Linux Secret Service/endpoint-policy runner or change the approved dependency/design through a new decision before re-evaluation.
