# Apply Progress: v1 MCP Foundation

## Cumulative Task State

- [x] 1.1–1.6 — source foundation.
- [x] 2.1–2.4 — remote snapshot safety and SQLite dependency gate.
- [x] 2.5–2.7 — PR #26 SQLite ledger foundation.
- [x] 2.8–2.10 — PR #28 filesystem-policy hardening.
- [x] 2.11 — four independent transaction RED cases.
- [x] 2.12 — context-cancellable admission retries and exact-token readback.
- [x] 2.13 — transaction child-process fixture refactor and evidence refresh.
- [x] 2.13a — Open-boundary verifier invocation and independent non-success result mapping.
- [x] 2.13b — real bounded SQLite integrity verifier microcycles.
- [x] 2.13c — verifier fixture lifecycle/seam refactor with approval coverage and independent GHA evidence.
- [ ] 2.14 and later — remote and recovery scope remain pending.

Task count: 22/42 complete.

## Strict TDD Cycle Evidence

| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|---|
| 2.11 | `ledger_transaction_red_test.go` | GHA real child-process SQLite | WDAC blocks local test execution; CI baseline 32409460869 failed transaction behavior. | CI 32410283697 failed only the four independent transaction cases. | N/A — RED task. | Retry, cancellation, ambiguous COMMIT, and deadline contention are independent. | Not assigned. |
| 2.12 | Existing 2.11 contract | GHA real child-process SQLite | RED baseline above. | Existing four RED cases. | CI 32410783761 passed all packages. | Four unchanged cases passed. | Not assigned. |
| 2.13 | Same four approval tests | GHA real child-process SQLite | Local execution blocked by WDAC; CI 32410783761 was green before fixture-only refactor. | N/A — fixture-only refactor. | CI 32412640014, 32412756552, and 32413019667 passed. | Existing four cases remain unchanged. | Consolidated fixtures; no production behavior changed. |
| 2.13a | `ledger_integrity_red_test.go` | GHA temporary SQLite package tests | WDAC forbids local test execution; existing suite safety net was supplied by GHA. | CI 32417114349 independently failed new invocation, existing-before-metadata invocation, passed observation, and each non-success mapping. | CI 32417203981 passed `go test -count=1 ./...` and `go vet ./...`; final-head CI 32417380586 repeated green. | New/existing ledger paths plus five independently injected results exercise distinct paths. | Not assigned. |
| 2.13b | `ledger_integrity_red_test.go` | GHA temporary SQLite package tests | CI 32417380586 was the pre-change green safety net; WDAC blocked local test execution. | CI 32417940275, 32418332944, and 32418539538 each failed the missing required verifier behavior. | CI 32418077313, 32418443116, 32418691268, and final-head 32418844082 passed `go test -count=1 ./...` and `go vet ./...`. | Real SQLite proves successful ordered full verification, size refusal, and corruption; injection proves malformed output, errors, overflow, and cancellation. | Not assigned. |
| 2.13c | `ledger_integrity_red_test.go` | GHA temporary SQLite package tests | CI 32418844082 was the pre-refactor green safety net; WDAC blocked local test execution. | N/A — fixture-only refactor; 2.13a/2.13b behavior tests are approval coverage. | CI 32419431344 passed `go test -count=1 ./...` and `go vet ./...` after the refactor; final evidence-artifact CI 32419671217 repeated green. | Existing independent new/existing, mapping, ordering, corruption, cancellation, bound, and malformed-output cases remain unchanged. | Replaced repeated ledger opening and global seam restoration with intention-revealing `t.Cleanup` helpers; production semantics unchanged. |

## Work Unit Evidence: 3B.1c-I task 2.13c

| Evidence | Result |
|---|---|
| Focused test command and exact result | GHA run 32419431344: `go test -count=1 ./...` exit 0; `internal/ownership/sqlite` and all packages passed. `go vet ./...` exit 0. Final evidence-artifact CI 32419671217 repeated green. |
| Runtime harness command/scenario and exact result | GHA Ubuntu temporary SQLite exercised the approval suite: initialized/existing ledger verifier ordering, `Open` passed/non-success mapping, real quick/full integrity verification, real corruption, cancellation, size bound, and injected bounded-query edge cases. No live IBM i boundary exists. |
| Static and build evidence | Local `gofmt -d internal/ownership/sqlite/ledger_integrity_red_test.go`, `go vet ./...`, compile-only `go test -c -o %TEMP%/bac-nexus-integrity-refactor.test.exe ./internal/ownership/sqlite`, and all six `CGO_ENABLED=0 GOOS={windows,darwin,linux} GOARCH={amd64,arm64} go build ./...` commands exited 0. No local test binary executed; the compile output was removed. |
| Rollback boundary | Revert `52f7ee3` to remove only integrity test fixture lifecycle/seam helpers. Revert `7841480` and `d9c6b2f` to restore task/progress context; verifier behavior, transaction behavior, filesystem policy, remote acquisition, and recovery remain unchanged. |

## Boundary and Delivery

- Draft PR #36 remains stacked-to-main targeting `main`; it was not merged.
- Maintainer-authorized `size:exception` applies only to coherent task 2.13c completion under a native 500-line ceiling. It prevents artificial compaction, technical debt, and wasted iterations; it does not permit unrelated growth.
- The refactor is limited to `internal/ownership/sqlite/ledger_integrity_red_test.go`; the PR boundary remains `ledger.go`, the integrity test, and current change evidence artifacts.
- Harness disposition: GitHub Actions is runtime evidence; WDAC was not bypassed and no local Go test executable ran. No compiled package test executable remains locally.

## Settlement Handoff

- Authorized attempt token: `sha256:ded96cdf6d42288376fcd0466e75197b3913c6e03b880631f5dbfd4bf3161f4f`.
- Proposed evidence revision: `sha256:8aaf81895002371b8e7299732d9c76e4d7a4ad706841e3cdc4c118e498c4f398` (BOM-less UTF-8 canonical JSON for the authorized attempt, work unit, task, PR, refactor head, and CI run).
- The parent orchestrator retains acquire, rescope, reset, and settle authority.
