# Tasks: Simplify Connection Onboarding

## Review Workload Forecast

| Field | Value |
|---|---|
| Estimated changed lines | 1,250–1,650 |
| 400-line budget risk | High |
| 800-line authorized budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | WU1 → WU2 → WU3 → WU4 |
| Delivery strategy | single-pr |
| Chain strategy | pending |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: pending
400-line budget risk: High
800-line authorized budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|---|---|---|---|---|---|
| 1 | Secret/keyring contracts | PR 1 | `go test -count=1 ./internal/remote ./internal/credential` | Fake prompt/keyring cases | New prompt/keyring files |
| 2 | Policy transaction | PR 2 | `go test -count=1 ./internal/configuration ./internal/profile` | Fake audit/proof/commit matrix | Onboarding transaction files |
| 3 | Direct TUI flow | PR 3 | `go test -count=1 ./internal/tui` | Update/View at 120x40, 80x24, 40x16 | Replacement TUI routes |
| 4 | Migration and docs | PR 4 | `go test -count=1 ./...` | `nexus configure` fake composition | Legacy deletion and docs |

## Phase 1: Secure Capture and Credential Seams

- [x] 1.1 RED: add `internal/remote/secret_prompt_test.go` for non-file/non-terminal/EOF/interrupt failure, zeroing, and no operation side effects (Direct Secure Onboarding).
- [x] 1.2 GREEN: implement injected `SecretPrompt.Read` in `internal/remote/secret_prompt.go`; compose it from `cmd/nexus/main.go`, never global terminal state.
- [x] 1.3 RED/GREEN/REFACTOR: extend `internal/credential/keyring_store_test.go` and `keyring_store.go` with independent Capability and Presence; supported probe/read/write errors fail closed, unavailable uses prompt-only.

## Phase 2: Policy-Owned Transaction

- [x] 2.1 RED: add `internal/configuration/onboarding_test.go` matrices for capture ownership/zeroing, timeout/cancel, both secret-free audits, proof-before-save, and no live IBM i.
- [x] 2.2 GREEN/REFACTOR: create `internal/configuration/{onboarding,identity_policy}.go` for automatic first-use pin/bootstrap audit, existing-pin mismatch audit, and bounded operation identities.
- [x] 2.3 RED/GREEN: add `internal/configuration/step8_onboarding_test.go` and adapter for five eligible fallback reasons only; bind grant to ID/generation/reason, consume ticket immediately, reject security downgrades.
- [x] 2.4 RED/GREEN/REFACTOR: add `internal/profile/onboarding_commit_test.go` and `onboarding_commit.go` for profile/pin/keyring/audit order, reverse compensation, journal retention, and cleanup-required results.

## Phase 3: Replacement TUI

- [x] 3.1 RED: add `internal/tui/onboarding_test.go` for shell-free `tea.Exec` capture, secret-free messages, Escape cancellation, operation/generation stale-result rejection, and scoped feedback.
- [x] 3.2 GREEN/REFACTOR: create `internal/tui/{onboarding,profile_management}.go` and update `model.go`/`home.go` with host/username/Connect and Save, running/completion, Finalize reload, open/delete/back/exit recovery.
- [x] 3.3 RED/GREEN: update `internal/tui/render_matrix_test.go` and localization tests for Spanish-first/English parity, NO_COLOR, focus, feedback, wrapping, and reachable controls at 120x40, 80x24, and 40x16.

## Phase 4: Route Migration and Verification

- [x] 4.1 After Phase 3 tests pass, remove eight-step route/state/render/test files and obsolete catalog keys; prove no legacy route remains from `internal/tui/model_test.go`.
- [x] 4.2 Update `docs/IBM_I_PROFILE_WIZARD.md` and `docs/SECURITY.md` with direct flow, automatic trust disclosure, prompt-on-use, recovery, and opt-in live validation.
- [x] 4.3 Run `gofmt -w`, `go test -count=1 ./...`, `go vet ./...`, `go build ./...`, and `git diff --check`; keep live IBM i tests explicitly skipped without approved configuration.
