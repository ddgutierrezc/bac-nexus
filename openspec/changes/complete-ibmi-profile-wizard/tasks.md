# Tasks: Complete IBM i Profile Wizard

## Review Workload Forecast

| Field | Value |
|---|---|
| Estimated changed lines | 1,450–1,850 |
| 800-line budget risk | High |
| Delivery strategy | exception-ok |
| Approved exception | `size:exception` |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: size-exception
400-line budget risk: High
800-line budget risk: High
Delivery strategy: exception-ok

| Unit | Scope/test | Runtime / rollback |
|---|---|---|
| 1 (180) | `go test -count=1 ./internal/profile ./internal/configuration -run TestPrepared` | N/A: `t.TempDir()` journal; `prepared-recovery` hunks. |
| 2 (250) | Create; `go test -count=1 ./internal/configuration -run TestCreateProfile` | N/A: in-process fake-store request; `create-singleflight` hunks. |
| 3 (260) | Tickets; `go test -count=1 ./internal/configuration -run TestStep8` | N/A: in-process fake SSH zero-call; `fallback-ticket` hunks. |
| 4 (210) | `go test -count=1 ./internal/tui -run TestProfile` | `Update` save; unit-owned files. |
| 5 (260) | `go test -count=1 ./internal/tui -run TestProfile` | `View()` proof; `proof-wiring` hunks. |
| 6 (180) | Compatibility; `go test -count=1 ./...` | N/A: runners only; unit-owned tests. |

## Phase 1: Prepared Creation
- [x] 1.1 **RED** Create `internal/configuration/profile_creation_test.go`; modify `internal/profile/recovery_test.go`: pre-existing credential, journal phases, crash/uncertain preparation, ownership mismatch, no unsafe delete, cleanup-required, operator recovery.
- [x] 1.2 **GREEN** Create `internal/configuration/profile_creation.go`; modify `internal/profile/profile.go`, `internal/profile/recovery.go`, `internal/configuration/security.go` for locked journals and delete-only-if-owned.
- [x] 1.3 **REFACTOR/evidence** Run Unit 1; `prepared-recovery` rollback retains journals.
- [x] 1.4 **RED** `internal/configuration/profile_creation_test.go`: generation/request/digest binding, pending join, terminal replay/cache, current identity acceptance, mismatch rejection, new-identity retry.
- [x] 1.5 **GREEN** Add single-flight `CreateProfileRequest/Result` to `internal/configuration/profile_creation.go`; return exact profile, never secret/ticket.
- [x] 1.6 **REFACTOR/evidence** Run Unit 2 with fake keyring failure: no profile/proof; revert `create-singleflight` hunks.

## Phase 2: Ticketed Proof
- [x] 2.1 **RED** Modify `internal/configuration/step8_test.go`, `step8_service_test.go`: profile/request/generation/class binding; five eligible classes; forgery, mismatch, replay, supersession, five-minute expiry, zero SSH effects.
- [x] 2.2 **GREEN** Modify `internal/configuration/step8.go`, `step8_service.go`: opaque 192-bit issue/atomic consume; separate WSS/SSH consent.
- [x] 2.3 **REFACTOR/evidence** Run Unit 3 command; rollback only `fallback-ticket` hunks.

## Phase 3: Eight-Step TUI
- [x] 3.1 **RED** Create `internal/tui/profile_credentials_step_test.go`, `internal/tui/profile_review_step_test.go`: child context/loading/request identity, blocked keyring, save-once, exact handoff.
- [x] 3.2 **GREEN** Create `internal/tui/profile_credentials_step.go`, `internal/tui/profile_review_step.go`; modify `internal/tui/model.go`, `internal/tui/mapepire_onboarding_step.go` for secret-free Steps 5–6.
- [x] 3.3 **REFACTOR/evidence** Run Unit 4 `Update` scenario; rollback those unit-owned files.
- [x] 3.4 **RED** Create `internal/tui/profile_proof_step_test.go`, `internal/tui/profile_completion_step_test.go`: cancel, timeout, retry, supersession, stale results, omitted/cancelled/failed/successful.
- [x] 3.5 **GREEN** Create `internal/tui/profile_proof_step.go`, `internal/tui/profile_completion_step.go`; modify `internal/tui/model.go` for bounded contexts and current-generation-only transitions.
- [x] 3.6 **REFACTOR/evidence** Run Unit 5 `View()` frames at 120x40, 80x24, 40x16 and `NO_COLOR`; rollback `proof-wiring` hunks.

## Phase 4: Compatibility and Verification
- [x] 4.1 **RED** Modify `internal/credential/keyring_darwin_red_test.go`, `internal/credential/keyring_native_linux_test.go`, `internal/credential/keyring_store_red_test.go`, `internal/tui/render_matrix_test.go`, `cmd/nexus/configure_test.go`: TOFU rotation, `nexus serve`, prompt opacity, status classes, macOS stdin, Windows/Linux failures, no Java/phantom step, and runner evidence.
- [x] 4.2 **GREEN** Modify `internal/credential/keyring_store.go`, `cmd/nexus/main.go`, `internal/localization/localization.go`, and `docs/IBM_I_PROFILE_WIZARD.md` without migration or readiness claims.
- [x] 4.3 **REFACTOR/evidence** Run `go test -count=1 ./...`, `go vet ./...`, `go build ./...`, and `git diff --check`; runtime N/A—no approved IBM i runner; rollback Unit 6 files.
