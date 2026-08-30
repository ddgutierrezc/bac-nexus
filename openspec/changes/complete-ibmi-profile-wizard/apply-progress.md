# Apply Progress: Complete IBM i Profile Wizard

**Mode:** Strict TDD. **Delivery:** `size:exception`; Units 1–6 complete. Unit 6 authority remains parent-owned `proceed`; no authority mutation occurred.

**Native attempt:** `sha256:dacf4e89db83679909f5dad912b8954037317f998f04e1b7f1f216054a7a1d79`; settlement remains owned by the parent orchestrator.

**Unit 4 native authority:** `sha256:cea6806647690c21200e2c0347b74dad97905caad699ea3ed65526680be06700`; parent-owned state is `proceed` and settlement remains untouched.

**Unit 5 native authority:** `sha256:ad7f676964ae9de12cbb807898778f577cf8eb3bdffb95452ae288a5ed734cba`; parent-owned state is `proceed` and settlement remains untouched.

## Completed Tasks
- [x] 1.1–1.6 Prepared creation and idempotent exact-profile creation.
- [x] 2.1–2.3 Ticketed WSS/SSH proof consent.
- [x] 3.1–3.3 Secret-free credential selection and save-once exact-profile handoff.
- [x] 3.4–3.6 Ticketed optional proof and truthful completion frames.
- [x] 4.1–4.3 Deterministic compatibility tests, canonical eight-step indicators, and full verification.

## Strict TDD Cycle Evidence
| Unit / tasks | Concrete RED test file and layer | Safety net | RED / pre-GREEN failure | GREEN execution | Triangulation cases | REFACTOR |
|---|---|---|---|---|---|---|
| 1 / 1.1–1.3 | Declared: `internal/configuration/profile_creation_test.go`, `internal/profile/recovery_test.go`; package-local Go test layer (execution-layer capture: not captured). | `TestPrepared` baseline recorded; exact command, outcome, exit code, and count not captured. | Production API absent; RED-before-GREEN execution order not captured. | `go test -count=1 ./internal/profile ./internal/configuration -run TestPrepared` — exited 0 / PASS; test count/summary not captured. | Declared: pre-existing credential, journal phases, crash/uncertain preparation, ownership mismatch, no unsafe delete, cleanup-required, operator recovery; execution confirmation not captured. | Clean; post-refactor execution detail not captured. |
| 2 / 1.4–1.6 | Declared: `internal/configuration/profile_creation_test.go`; package-local Go test layer (execution-layer capture: not captured). | `TestCreateProfile` baseline recorded; exact command, outcome, exit code, and count not captured. | Production API absent; RED-before-GREEN execution order not captured. | `go test -count=1 ./internal/configuration -run TestCreateProfile` — exited 0 / PASS; test count/summary not captured. | Declared: generation/request/digest binding, pending join, terminal replay/cache, current identity acceptance, mismatch rejection, new-identity retry; fake-keyring failure with no profile/proof; execution confirmation not captured. | Clean; post-refactor execution detail not captured. |
| 3 / 2.1–2.3 | `internal/configuration/step8_test.go`, `step8_service_test.go`, `step8_production_test.go`; Go unit plus in-process fake SSH integration layer. | `go test -count=1 ./internal/configuration -run TestStep8` — exit 0, `ok bac-nexus/internal/configuration 0.009s`; Go output provides no test count. | First RED was written before production changes; `go test -count=1 ./internal/configuration -run TestStep8` exited 1 at compile time because `Generation`, `WSSConsent`, `SSHConsent`, ticket store, ticket result, and `RunSSH` did not exist. A second production-composition RED (`-run TestNewStep8Production`) exited 1 because its new ticket store was not wired. | After minimal ticket store/service changes: `go test -count=1 ./internal/configuration -run TestStep8` — exit 0, `ok bac-nexus/internal/configuration 0.009s`; composition GREEN: `go test -count=1 ./internal/configuration -run TestNewStep8Production` — exit 0, `ok bac-nexus/internal/configuration 0.010s`. | Five eligible classes; 192-bit encoding; profile/request/generation/class mismatch; forged, replayed, expired, cancelled, and superseded tickets; WSS consent absent before observation/credential/network; separate SSH consent; every rejected ticket admission has zero gate/factory/proof calls. | Added a per-profile latest-generation watermark during refactor so a late stale WSS result cannot reissue a lower-generation ticket; `gofmt` then focused command exited 0 (`ok ... 0.009s`). |
| 4 / 3.1–3.3 | `internal/tui/profile_credentials_step_test.go`, `internal/tui/profile_review_step_test.go`; direct Bubble Tea `Update` unit layer. | `go test -count=1 ./internal/tui -run TestProfile` — exit 0, `ok bac-nexus/internal/tui 1.063s` before existing-file edits. | Tests were written first; the focused command exited 1 at compile time because credential/review screens, state, and update handlers did not exist. | `go test -count=1 ./internal/tui -run TestProfile` — exit 0, `ok bac-nexus/internal/tui 1.214s`. | Keyring unavailable stays blocked; prompt advances without a command; pending save does not emit twice; stale save result is ignored; exact returned profile is handed to the next-step request. | Renamed create-state fields to avoid unrelated `gofmt` alignment churn; focused tests remained green. |
| 5 / 3.4–3.6 | `internal/tui/profile_proof_step_test.go`, `internal/tui/profile_completion_step_test.go`; direct Bubble Tea `Update` and runtime `View()` unit layer. | `go test -count=1 ./internal/tui -run TestProfile` — exit 0, `ok bac-nexus/internal/tui 2.404s` before existing-file edits. | Tests were written first; `go test -count=1 ./internal/tui -run TestProfile` exited 1 at compile time because proof/completion state and messages did not exist. | `go test -count=1 ./internal/tui -run TestProfile` — exit 0, `ok bac-nexus/internal/tui 2.562s`. | Cancel-before-command and omit; retry supersedes generation 1 and rejects its late result; timeout maps failed; explicit SSH fallback consent retains only the ticket; success requires cleanup; failed cleanup maps failed; every completion state renders without color; 120x40, 80x24 NO_COLOR, and 40x16 frames remain bounded and completion is keyboard-reachable. | Added explicit proof/completion panel functions so viewport refresh uses panel content instead of nesting a rendered shell; preserved the legacy Step 8 action when Unit 5 is not enabled. |
| 6 / 4.1–4.3 | `internal/credential/keyring_{darwin_red,native_linux,store_red}_test.go`, `internal/tui/render_matrix_test.go`, and `cmd/nexus/configure_test.go`; Go unit/runtime-View layer. | Baseline exposed a real native credential and obsolete absent-WSS-consent expectation; `internal/tui` passed. | Canonical-frame RED exposed `of/de 9` drift and a phantom-step risk. | Focused credential, CLI, and canonical-frame tests passed; final full-suite command passed. | Native failure mapping, exact-profile rotation, macOS stdin opacity, explicit WSS consent, eight numbered frames, no Java or Step 9 documentation indicators. | Kept catalogs stable; used the localizer boundary and deterministic native factory seam. |
| 6 remediation / documentation and evidence | Documentation structural guard; no Go test was added because remediation scope prohibits test and production-code changes. | Guard exited 1 before edit with `Steps 5–9`, `Steps 4–9`, and `Steps 4–9`. | The failing structural guard was executed before the documentation correction. | Corrected guard exited 0 with eight headings and no prohibited references. | N/A — three independently located stale references were the bounded cases. | No refactor needed. |

## Test Summary
- **New behavioral tests**: 15 test functions across Units 3–5.
- **Total focused tests passing**: Unit 5 `go test -count=1 -v ./internal/tui -run 'TestProfile(Proof|Completion)'` — exit 0, six top-level tests and five subtests passed in `0.139s`.
- **Layers used**: Unit and in-process fake SSH integration.
- **Approval tests**: None — behavior change, not a pure refactor.
- **Pure functions created**: 1 (`validFallbackTicketClaim`).

## Work Unit Evidence
| Unit | Focused test command and exact recorded result | Runtime harness / rollback boundary |
|---|---|---|
| 1 | `go test -count=1 ./internal/profile ./internal/configuration -run TestPrepared` — exited 0 / PASS; test count/summary not captured. | N/A: `t.TempDir()` journal, no external runtime boundary; revert `prepared-recovery` hunks while retaining journals. |
| 2 | `go test -count=1 ./internal/configuration -run TestCreateProfile` — exited 0 / PASS; test count/summary not captured. | N/A: in-process fake credential-store request, no external runtime boundary; revert `create-singleflight` hunks. |
| 3 | `go test -count=1 ./internal/configuration -run TestStep8` — exit 0, `ok bac-nexus/internal/configuration 0.007s`; Go output provides no test count. Package safety net: `go test -count=1 ./internal/configuration` — exit 0, `ok bac-nexus/internal/configuration 0.036s`. | `go test -count=1 ./internal/configuration -run TestStep8ServiceRejectedTicketAdmissionsHaveZeroSSHEffects` — exit 0, `ok bac-nexus/internal/configuration 0.007s`; in-process fake SSH proves forged/mismatched/replayed/cancelled/superseded admissions make zero gate, runtime-factory, and proof calls. No real host contacted. Roll back only `fallback-ticket` hunks in `internal/configuration/step8.go`, `step8_service.go`, `step8_production.go`, and their Unit 3 tests. |
| 4 | `go test -count=1 ./internal/tui -run TestProfile` — exit 0, `ok bac-nexus/internal/tui 1.214s`; package safety net `go test -count=1 ./internal/tui` — exit 0, `ok bac-nexus/internal/tui 8.581s`. | `go test -count=1 -v ./internal/tui -run 'TestProfile(CredentialsBlocksUnavailableKeyring|CredentialsPromptAdvancesWithoutCredentialMaterial|ReviewSavesOnceAndHandsOffExactProfile|ReviewIgnoresStaleSaveResult)$'` — exit 0, four direct `Update` scenarios passed in `0.010s`; no remote call or IBM i host is involved. Roll back only Unit 4 files/hunks: `profile_credentials_step{,_test}.go`, `profile_review_step{,_test}.go`, and Unit 4 hunks in `model.go` and `mapepire_onboarding_step.go`. |
| 5 | `go test -count=1 -v ./internal/tui -run 'TestProfile(Proof|Completion)'` — exit 0, six top-level tests and five subtests passed in `0.139s`. Package safety net: `go test -count=1 ./internal/tui` — exit 0, `ok bac-nexus/internal/tui 18.391s`. | Runtime harness is the same direct Bubble Tea `Update`/`View()` path: proof and completion frames at 120x40, 80x24 with `NO_COLOR`, and 40x16; it proves focus marker, keyboard reachability, lossless wrapping/bounds, semantic feedback, and no ANSI in the no-color frame. No real host or IBM i call occurs. Roll back only `proof-wiring` unit-owned files (`profile_{proof,completion}_step{,_test}.go`) and Unit 5 hunks in `model.go`/`wizard_viewport.go`. |
| 6 | `go test -count=1 ./...` — exit 0; all tested packages passed (four packages have no test files). | N/A — no approved IBM i runner exists and no host, real credential, home keyring, or remote system was accessed. Roll back only Unit 6 hunks and this cumulative progress entry. |

## Safety and Diff Evidence
- Unit 3 diff check: `git diff --check` — exit 0; no output.
- Unit 3 changed-line authority: `git diff --numstat -- internal/configuration/step8.go internal/configuration/step8_service.go internal/configuration/step8_production.go internal/configuration/step8_test.go internal/configuration/step8_service_test.go internal/configuration/step8_production_test.go` reports **359 additions + 19 deletions = 378 changed lines**, within this batch's 400-line maximum.
- Existing Unit 1–2 native evidence is preserved verbatim above; it remains separately recorded as 241 changed lines within its acquired 650-line bound.
- Unit 4 checks: `go vet ./...`, `go build ./...`, and `git diff --check` — each exit 0 with no output. Its six unit-owned files/hunks report **309 additions + 8 deletions = 317 changed lines**, within the 400-line maximum.
- Unit 5 checks: `go vet ./...`, `go build ./...`, and `git diff --check` — each exit 0 with no output. Its unit-owned files/hunks report **362 additions + 5 deletions = 367 changed lines**, within the 400-line maximum.
- Unit 6 checks: `go test -count=1 ./...`, `go vet ./...`, `go build ./...`, and `git diff --check` — each exit 0. Unit 6 authored changes are 157 lines before this corrective documentation update and remain within the 400-line limit.

### Unit 6 failed-evidence remediation reconciliation
- **Failed evidence revision:** `sha256:5db3f4d98fae68ebd20cafd58bc4862e442e9cd7ed25916dacfe2b9b4bae6461`.
- **Validator-assigned tracked Unit 6 slice:** 135 additions + 113 deletions = 248 changed lines.
- **Actor broader tracked/document slice:** 145 additions + 123 deletions = 268 changed lines.
- **Native runtime accounting (authoritative for the failed objective attempt):** 284 changed lines. The native 400-line ceiling therefore leaves 116 lines for this focused remediation; the differing prior measurements are retained because their scopes differ.
- This remediation changes only the three stale canonical-journey references and this evidence reconciliation. It makes no isolated line-count claim beyond the native runtime accounting.
- **SQLite reproduction:** `go test -count=1 ./internal/ownership/sqlite -run TestLedgerAdmissionUsesExactRetrySchedule` — exit 0, `ok bac-nexus/internal/ownership/sqlite 1.131s`; classified as intermittent/non-reproduced this run and unrelated to wizard behavior.
- **Final commands:** `go test -count=1 ./...` — exit 1 only at `TestLedgerAdmissionUsesExactRetrySchedule` with `database is locked (5) (SQLITE_BUSY); want retry after exactly 25ms, 50ms, and 100ms`; `go vet ./...`, `go build ./...`, and `git diff --check` — each exit 0 with no output.
- **Structural verification:** exactly eight numbered `### Step N —` headings; no `Java`, `Step 9`, `nine-step`, `de 9`, `of 9`, or stale `Steps 4–9`/`Steps 5–9` reference remains.
- **Remote boundary:** no real credential, home keyring, IBM i host, remote system, or network access occurred.

## Deferred Scope
- No real IBM i, host, credential, or native home keyring was accessed. Live validation remains explicitly unproven.

### Final-verification SQLite remediation
- **Authority:** parent-acquired token `sha256:9f1be812ba2b6c14e55a716f1a34ce3c85e48c832b60965f484e76f11a4d21a9`; parent settlement remains untouched.
- **Failed evidence revision remediated:** `sha256:4d0c1e3b222ac9a32f44fa92370ef234a584da7ba47b8c606cd69a35542984ec`.
- **Root classification:** repository defect, not an environmental/harness-only failure. `Ledger.Admit` returned the final SQLite `BUSY` result at the competing lock's release boundary after the three required waits, rather than making a final immediate admission probe. This made the 1.1-second cooperating-process contention test intermittent.
- **Correction:** preserve the exact `25ms`, `50ms`, and `100ms` retry delays, then make one bounded immediate probe before surfacing `BUSY`. The strengthened regression records every delay so an added sleep or altered schedule fails.

| Task | Test file / layer | Safety net | RED | GREEN | Triangulate | Refactor |
|---|---|---|---|---|---|---|
| Final verification SQLite remediation | `internal/ownership/sqlite/ledger_transaction_red_test.go`; process-backed SQLite integration | Existing failure reproduced: `go test -count=20 ./internal/ownership/sqlite -run '^TestLedgerAdmissionUsesExactRetrySchedule$'` — exit 1, seven failures in `22.261s`. | After strengthening the test, `go test -count=10 ./internal/ownership/sqlite -run '^TestLedgerAdmissionUsesExactRetrySchedule$'` — exit 1, five failures in `11.225s`. | `go test -count=20 ./internal/ownership/sqlite -run '^TestLedgerAdmissionUsesExactRetrySchedule$'` — exit 0, `ok bac-nexus/internal/ownership/sqlite 22.358s`. | Repeated independent process-lock cases retain the exact three-delay assertion and prove the release-boundary case. | No further refactor needed; minimal bounded branch added. |

| Work unit evidence | Exact result |
|---|---|
| Focused/stress regression | `go test -count=20 ./internal/ownership/sqlite -run '^TestLedgerAdmissionUsesExactRetrySchedule$'` — exit 0, `ok bac-nexus/internal/ownership/sqlite 22.358s`. |
| Package regression | `go test -count=1 ./internal/ownership/sqlite` — exit 0, `ok bac-nexus/internal/ownership/sqlite 3.481s`. |
| Runtime harness | The focused test starts a bounded local helper process holding an SQLite exclusive lock for `1100ms`; no network, credential, keyring, IBM i host, or remote system is used. Result: 20/20 test-process runs passed. |
| Final verification | `go test -count=1 ./...` — exit 0; `go vet ./...` — exit 0; `go build ./...` — exit 0; `git diff --check` — exit 0. |
| Rollback boundary | Revert only `internal/ownership/sqlite/ledger.go` and `internal/ownership/sqlite/ledger_transaction_red_test.go`; this restores the former final-`BUSY` behavior and removes its exact-schedule regression assertion. |

- **Changed lines:** `4 additions + 1 deletion` in `ledger.go`; `14 additions` in `ledger_transaction_red_test.go`; **19 changed lines total**, within the fresh 400-line remediation budget.
