# Apply Progress: v1 MCP Foundation

## Cumulative Task State

- [x] 1.1–1.6 — source foundation.
- [x] 2.1–2.4 — remote snapshot safety and SQLite dependency gate.
- [x] 2.5–2.7 — PR #26 SQLite ledger foundation.
- [x] 2.8–2.10 — PR #28 filesystem-policy hardening.
- [x] 2.11 — four independent transaction RED cases.
- [x] 2.12 — context-cancellable admission retries and exact-token readback.
- [x] 2.13 — transaction child-process fixture refactor and evidence refresh.
- [ ] 2.13a and later — pending; integrity, remote, and recovery scope untouched.

Task count: 19/42 complete.

## Strict TDD Cycle Evidence

| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|---|
| 2.11 | `ledger_transaction_red_test.go` | GHA real child-process SQLite | WDAC blocks local test execution; CI baseline 32409460869 failed transaction behavior. | CI 32410283697 failed only the four independent transaction cases. | N/A — RED task. | Retry, cancellation, ambiguous COMMIT, and deadline contention are independent. | Not assigned. |
| 2.12 | Existing 2.11 contract | GHA real child-process SQLite | RED baseline above. | Existing four RED cases. | CI 32410783761 passed all packages. | Four unchanged cases passed. | Not assigned. |
| 2.13 | Same four approval tests | GHA real child-process SQLite | Local execution blocked by WDAC; CI 32410783761 was green before fixture-only refactor. | N/A — fixture-only refactor; existing independent behavior tests are approval coverage. | CI 32412640014 passed all packages; final CI 32412756552 passed. | Existing four independent cases remain unchanged. | Consolidated ledger setup and child-process cleanup with LIFO `t.Cleanup`; no production code changed. |

## Work Unit Evidence: 3B.1c-T

| Evidence | Result |
|---|---|
| Focused test command and exact result | GHA run 32412640014: `go test -count=1 ./...` exit 0; `internal/ownership/sqlite` passed in 3.692s and all package results passed. Final run 32412756552 at `e4372ee` repeated it: SQLite 3.655s, all packages passed. |
| Runtime harness command/scenario and exact result | GHA Ubuntu runner executed the four real child-process cases against the same SQLite database: 25/50/100ms retry, cancellation, ambiguous-COMMIT exact-token readback, and deadline-bounded contention; both runs passed. |
| Static and build evidence | `gofmt -d internal/ownership/sqlite/ledger_transaction_red_test.go`, `go vet ./...`, compile-only `go test -c -o %TEMP%/bac-nexus-ledger-2.13.test.exe ./internal/ownership/sqlite`, and six `CGO_ENABLED=0 GOOS={windows,darwin,linux} GOARCH={amd64,arm64} go build ./...` commands all exited 0. No local test binary executed. |
| Rollback boundary | Revert `a0ce8d3` and `e4372ee` to remove only fixture cleanup and task evidence; `Ledger.Admit`, integrity, filesystem policy, remote acquisition, and recovery remain unchanged. |

## Boundary and Delivery

- Draft PR #32 remains stacked-to-main targeting `main`.
- `origin/main...HEAD`: 266 additions + 14 deletions = 280 authored changed lines, within the hard 400-line cap.
- Commits: `a0ce8d3 refactor(ownership): consolidate transaction lock fixtures`; `e4372ee docs(sdd): complete transaction fixture refactor`.
- Harness disposition: reused; no test behaviors were weakened and no test process or compiled binary remains locally.
- No integrity verifier, filesystem policy, remote, recovery, or later task was implemented.

## Settlement Handoff

- Authorized attempt token: `sha256:0116a3676bd11dbb5c7fb6eeb364a022ac07c5d7e4b3ba3b599e3a2119fa42a2`.
- Proposed evidence revision: `sha256:c698e0bcd422803837bdea4b63b7a557ba72fc29a6b403506e7d615659aa7a9b` (BOM-less UTF-8 canonical evidence JSON).
- The parent orchestrator performs settlement with its persisted lineage and generation.
