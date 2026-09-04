# Tasks: Restore Profile UI Validation

## Review Workload Forecast

| Field | Value |
|---|---|
| Estimated changed lines | PR 1: 620–760; PR 2: 540–700 |
| 400-line budget risk | High |
| 800-line slice budget risk | Low |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 primitives/Create → PR 2 Edit/localization |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|---|---|---|---|---|---|
| 1 | Shared profile UI and direct Create recovery | PR 1 → main | `go test -count=1 ./internal/tui` | `go test -count=1 ./internal/tui -run 'DirectOnboarding|RuntimeFrames'` | `profile_screen.go`, Create wiring/tests/catalog entries |
| 2 | Validated metadata Edit and locale matrix | PR 2 → main (after PR 1) | `go test -count=1 ./internal/tui` | `go test -count=1 ./internal/tui -run 'Edit|LocaleAndViewport'` | Edit adapter/model/tests and paired Edit catalog entries |

## Phase 1: PR 1 — Shared Presentation and Direct Create

- [x] 1.1 **RED**: Extend `internal/tui/profile_screen_test.go`, `onboarding_test.go`, and `render_matrix_test.go` with runtime `View()` frames for Create entry/running/failed/cleanup/saved at 120x40, 80x24 NO_COLOR, and 40x16; assert shell/panel, cursor, complete wrapping, overflow/reachability, and secret/wizard/proof absence.
- [x] 1.2 **GREEN**: Create `internal/tui/profile_screen.go`; compose `homeTheme`, `shellLayout`, and `wrapWizardText` into reusable profile shell, fields, semantic feedback, footer, and persistent viewport/reveal primitives.
- [x] 1.3 **RED**: Add table-driven `internal/tui/onboarding_test.go` cases for `ValidateHost` then `ValidateUsername`, first-invalid focus, blocked-but-focusable action, feedback precedence/clearing, no prompt/operation on invalid input, prompt failure/cancel fail-closed, stale-result rejection, and Cancel lifecycle.
- [x] 1.4 **GREEN**: Modify `internal/tui/onboarding.go` and `internal/tui/model.go` direct-onboarding state/update/render paths to use profile validation and renderer while retaining `tea.Exec`, `OnboardingOperations`, transient password capture, cancellation, and saved/failed/cleanup result mapping.
- [x] 1.5 **REFACTOR/verify**: Add paired Create feedback IDs to `internal/localization/catalogs/{es,en}_direct_onboarding.json` and `{es,en}_extra.json`; run `go test -count=1 ./internal/tui` then `go test -count=1 ./...`. Preserve `.atl` and `stash@{0}`; do not alter profile/domain/operation lifecycle behavior.

## Phase 2: PR 2 — Validated Metadata Edit

- [x] 2.1 **RED**: Add table-driven `internal/tui/profile_screen_test.go`/`render_matrix_test.go` Edit cases mapping `Profile.Validate` order (name, endpoint, username, host-key pair, Java, Mapepire, credential mode) to semantic field IDs; assert invalid Save causes no `Update`/`Save`, focuses first invalid field, and retains focusable Save.
- [x] 2.2 **GREEN**: Create `internal/tui/profile_validation.go` with Create/Edit adapters; call exported validators directly and isolate draft groups through `Profile.Validate`, retaining `Cause` without parsing/rendering error prose.
- [x] 2.3 **RED**: Extend `internal/tui/onboarding_test.go` and `localization_test.go` for Edit Save failure/success, Cancel discard, validation clearing/operation precedence, and proof/auth/connectivity non-regression across English/Spanish frames and NO_COLOR.
- [x] 2.4 **GREEN**: Modify `internal/tui/model.go` form/update/render orchestration to use authoritative mapping, persistent viewport reveal, scoped sanitized errors, simple Save/Cancel, and no connectivity/proof execution.
- [x] 2.5 **REFACTOR/verify**: Add paired Edit feedback catalog IDs; remove duplicate presentation logic only after GREEN tests. Run `go test -count=1 ./internal/tui`, `go test -count=1 ./...`, `go vet ./...`, and `git diff --check`; preserve `.atl` and `stash@{0}`.

## Corrective Rerun: PR 2 Gate Correction

- [x] C1 **RED/GREEN**: Preflight raw Edit form conversion with authoritative `Profile.Validate` order so earlier semantic errors win over later parsing errors and invalid Save never persists.
- [x] C2 **RED/GREEN**: Treat host and port as the endpoint validation group when clearing local feedback; unrelated edits retain endpoint feedback.
