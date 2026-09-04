# Apply Progress: Restore Profile UI Validation

## Scope

- Completed work units: `pr1-gate-correction`, `pr2-edit-validation-presentation`, `pr2-gate-correction`
- Delivery: auto-chain, stacked-to-main, PR 1 + PR 2
- Mode: Strict TDD
- PR 1 final scoped delta: 543 changed lines (483 additions + 60 deletions), including untracked source/test files and excluding `.atl` and `openspec` SDD artifacts; within the 800-line slice budget.
- PR 2 final incremental delta: 592 changed lines after C1/C2 correction, excluding `.atl` and `openspec` artifacts; within the 800-line PR 2 budget.

## Final-State Reconciliation

The earlier PR 2 snapshot below records the RED/GREEN history at the time it was written. The final state includes the subsequent gate correction and is authoritative for closure:

- C1: authoritative semantic validation preflight now precedes port, trust, and credential parsing; simultaneous invalid name and nonnumeric port focuses name and performs no persistence.
- C2: endpoint validation clears when host **or** port changes, while unrelated field edits retain endpoint feedback.
- Full checks PASS: focused suites, `go test -count=1 ./...`, `go vet ./...`, `go build ./...`, and `git diff --check`.
- Final verification: 5/5 requirements, 11/11 scenarios, 12/12 tasks; zero CRITICAL findings; informational TUI coverage 69.4%.

## Completed Tasks

- [x] 1.1 Runtime `View()` coverage for direct Create lifecycle and responsive frames.
- [x] 1.2 Shared BAC profile shell, centered panel, semantic feedback, footer, and focused viewport reveal.
- [x] 1.3 Authoritative ordered direct Create validation with field-specific feedback and precedence/clearing coverage.
- [x] 1.4 Direct Create renderer and update flow wired to the shared presentation while preserving the `tea.Exec` secret boundary and operation lifecycle.
- [x] 1.5 English/Spanish direct Create validation copy and verification.

## TDD Cycle Evidence

| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|---|
| 1.1 | `internal/tui/profile_screen_test.go` | Integration | `go test -count=1 ./internal/tui` — PASS (`ok ... 0.346s`) | `go test -count=1 ./internal/tui -run TestProfileScreenRendersEveryDirectOnboardingLifecycleAtRequiredFrames` — FAIL: all lifecycle status content was absent at 40x16. | Same command — PASS (`ok ... 0.126s`) after lifecycle viewport initialization and bounded traversal. | Entry/running/failed/cleanup/saved at 120x40, 80x24 NO_COLOR, 40x16; narrow frames traverse runtime `View()` output. | Removed per-`View()` focus reset so user scrolling remains persistent; focused test remains PASS. |
| 1.2 | `internal/tui/profile_screen_test.go` | Integration | Existing package baseline above | `go test -count=1 ./internal/tui -run TestProfileScreenPrimitivesRenderFieldsActionsAndSemanticFeedback` — FAIL (missing `profileField`, `profileAction`, `profileFeedback`). | Same command — PASS (`ok ... 0.005s`). | Primitive test asserts field, action, and semantic error output; direct Create composes all three. | Reused one compact primitive set; no artificial component layer. |
| 1.3 | `internal/tui/onboarding_test.go` | Unit/integration | Existing package baseline above | Added own-field clearing test before production changes; its first package execution was blocked by the Task 1.2 missing-symbol RED. | `go test -count=1 ./internal/tui -run 'TestDirectOnboardingEditingInvalidFieldClearsItsOwnValidation|TestDirectOnboardingValidationClearsOnlyTheEditedFieldAndDefersToOperationFeedback'` — PASS (`ok ... 0.008s`). | Own host edit clears host validation; username edit retains host validation and operation error precedence. | Existing field-scoped clearing was correct; retained it and added explicit behavior proof. |
| 1.4 | `internal/tui/onboarding.go`, `internal/tui/model.go` | Integration | `go test -count=1 ./internal/tui` — PASS (`ok ... 0.346s`) | No corrective RED required: secret, cancellation, stale-result, and lifecycle boundaries were preserved and not expanded. | `go test -count=1 ./internal/tui` — PASS (`ok ... 0.363s`). | Running/completion viewport navigation preserves cancel/finalize lifecycle actions without changing `tea.Exec` or operations. | Extracted shared lifecycle viewport refresh and completion message selection; package tests remain PASS. |
| 1.5 | `internal/tui/localization_test.go` | Integration | Existing package baseline above | `go test -count=1 ./internal/tui -run TestDirectOnboardingLocalizedFieldValidationRendersAtRuntime` — FAIL: Spanish/English host and username validation were not visible in the runtime frame. | Same command — PASS (`ok ... 0.027s`) after refreshing the persisted viewport after validation. | Spanish/English × host/username at 80x24 NO_COLOR; each test triggers Enter on invalid input and asserts rendered field-specific text/no ANSI. | Paired catalogs and IDs stay unchanged; correction only renders their existing field-specific messages. |

| 2.1 | `internal/tui/profile_screen_test.go` | Unit/integration | `go test -count=1 ./internal/tui` — PASS (`ok ... 0.430s`) | `go test -count=1 ./internal/tui -run TestValidateEditProfileMapsAuthoritativeValidationOrderToSemanticFields` — FAIL: missing semantic field constants and `validateEditProfile`. | Same command — PASS (`ok ... 0.006s`) after the adapter called ordered authoritative validators. | Seven independent invalid candidates map name, endpoint, username, host key, Java, Mapepire, and credential mode to semantic field IDs. | Final `Profile.Validate` remains the save gate after isolated draft-group checks. |
| 2.2 | `internal/tui/profile_validation.go` | Unit | New production file | RED above. | Adapter GREEN command — PASS (`ok ... 0.006s`). | Direct exported validators cover first groups; draft-group `Profile.Validate` covers Java, Mapepire, and credential mode without error-prose classification. | Removed an unused adapter parameter and simplified nil-cause handling; focused tests remained green. |
| 2.3 | `internal/tui/localization_test.go`, `internal/tui/profile_screen_test.go` | Integration | Existing package baseline PASS | `go test -count=1 ./internal/tui -run TestEditOperationFeedbackStaysSanitizedAndPrecedesLocalValidation` — FAIL: missing scoped `formOperationFeedback`. | Same focused group — PASS (`ok ... 0.021s`) after scoped sanitized feedback was retained across field edits. | Save success, Cancel discard, local-clearing/operation precedence, and English/Spanish validation rendering are exercised. | Operation feedback is model-scoped and outranks local validation; no remote operation path was added. |
| 2.4 | `internal/tui/model.go`, `internal/tui/render_matrix_test.go` | Integration | Existing package baseline PASS | `go test -count=1 ./internal/tui -run TestEditLocalizedValidationFramesRemainReachable` — FAIL: 40x16 validation feedback remained hidden at the stale Save offset. | Same command — PASS (`ok ... 0.038s`) after focus reveal recognized wrapped semantic feedback. | 120x40, 80x24 NO_COLOR, and 40x16 Spanish/English runtime frames prove bounds, feedback, and narrow overflow disclosure. | Form uses the existing profile shell, viewport, fields, actions, and feedback primitives. |
| 2.5 | `internal/localization/catalogs/{en,es}_direct_onboarding.json`, `internal/localization/localization.go` | Integration | Existing package baseline PASS | Catalog/runtime RED was included in the localized 40x16 frame failure above. | Focused Edit suite — PASS (`ok bac-nexus/internal/tui 0.056s`). | Paired Save/Cancel and all Edit validation message IDs render in English and Spanish; 80x24 NO_COLOR contains no ANSI. | Reused complete-catalog validation rather than creating a separate localization path. |

## Work Unit Evidence

| Evidence | Result |
|---|---|
| Focused test | `go test -count=1 ./internal/tui -run 'TestProfileScreen|TestDirectOnboarding(EditingInvalidFieldClearsItsOwnValidation|ValidationClearsOnlyTheEditedFieldAndDefersToOperationFeedback|LocalizedFieldValidationRendersAtRuntime)'` — PASS (`ok bac-nexus/internal/tui 0.150s`). |
| Runtime harness | `go test -count=1 ./internal/tui -run TestProfileScreenRendersEveryDirectOnboardingLifecycleAtRequiredFrames` — PASS (`ok bac-nexus/internal/tui 0.126s`); executes `Model.Update`/`View()` for entry, running, failed, cleanup, saved at 120x40, 80x24 NO_COLOR, and 40x16, with narrow viewport traversal. |
| Package gate | `go test -count=1 ./internal/tui` — PASS (`ok bac-nexus/internal/tui 0.363s`). |
| Full required suite | `go test -count=1 ./...` — PASS; all packages completed, including `internal/tui`. |
| Formatting | `gofmt -w internal/tui/profile_screen.go internal/tui/profile_screen_test.go internal/tui/onboarding.go internal/tui/onboarding_test.go internal/tui/localization_test.go internal/tui/model.go` — PASS. |
| Diff validation | `git diff --check` — PASS. |
| Scoped count | Reproducible Python command over `git diff --numstat` plus `git diff --no-index --numstat /dev/null` for `git ls-files --others --exclude-standard`; excludes `:(exclude).atl/**` and `:(exclude)openspec/**`: 483 additions + 60 deletions = 543. |
| Rollback boundary | Revert direct Create renderer/viewport changes in `internal/tui/profile_screen.go`, `internal/tui/onboarding.go`, `internal/tui/model.go`, and their TUI tests; localization and all domain/configuration/operation behavior remain isolated. |

### PR 2 — `pr2-edit-validation-presentation`

| Evidence | Result |
|---|---|
| Focused test | `go test -count=1 ./internal/tui -run 'Test(ValidateEditProfileMapsAuthoritativeValidationOrderToSemanticFields|EditInvalidSaveIsFocusableBlockedAndRevealsFirstInvalidField|EditSaveUpdatesMetadataAndCancelDiscardsDraft|EditOperationFeedbackStaysSanitizedAndPrecedesLocalValidation|EditLocalizedValidationFramesRemainReachable)'` — PASS (`ok bac-nexus/internal/tui 0.056s`). |
| Runtime harness | `go test -count=1 ./internal/tui -run TestEditLocalizedValidationFramesRemainReachable` — PASS (`ok bac-nexus/internal/tui 0.038s`); real `Model.Update`/`View()` frames at 120x40, 80x24 NO_COLOR, and 40x16 for English and Spanish invalid Edit Save. |
| Package gate | `go test -count=1 ./internal/tui` — PASS (`ok bac-nexus/internal/tui 0.424s`). |
| Full required suite | `go test -count=1 ./...` — PASS; all packages completed, including `internal/tui` (`0.475s`). |
| Static validation | `go vet ./...` — PASS. |
| Formatting | `gofmt -w internal/localization/localization.go internal/tui/model.go internal/tui/profile_screen.go internal/tui/profile_validation.go internal/tui/profile_screen_test.go internal/tui/localization_test.go internal/tui/render_matrix_test.go` — PASS. |
| Diff validation | `git diff --check` — PASS. |
| Scoped count | Current non-`.atl`, non-OpenSpec source/test/localization delta is 1,007 changed lines; subtracting the documented PR 1 543-line baseline yields 464 PR 2 incremental changed lines, within the 800-line budget. |
| Rollback boundary | Revert `internal/tui/profile_validation.go`, PR 2 portions of `internal/tui/model.go`, `internal/tui/profile_screen.go`, Edit tests, and paired Edit catalog IDs; PR 1 Create behavior, domain validators, store implementation, authentication, connectivity, and proof behavior remain intact. |

### Corrective Rerun — `pr2-gate-correction`

| Evidence | Result |
|---|---|
| C1 semantic preflight | PASS; authoritative `Profile.Validate` ordering is evaluated before later port/trust/credential conversion, so an earlier invalid name wins and invalid Save never persists. |
| C2 endpoint clearing | PASS; editing either host or port clears endpoint validation, while unrelated edits retain it. |
| Final incremental delta | 592 changed lines after correction, within the 800-line PR 2 budget. |
| Final checks | PASS: focused suites, `go test -count=1 ./...`, `go vet ./...`, `go build ./...`, and `git diff --check`. |

## Requirements Covered

- Every direct Create lifecycle state now has runtime `View()` proof at each required viewport, including narrow overflow traversal.
- Direct Create composes reusable field, action, and semantic feedback primitives rather than formatting all controls inline.
- Local validation clearing is proven both for its edited field and an unrelated field.
- English and Spanish host/username validation is triggered and rendered in real 80x24 NO_COLOR frames without ANSI.
- Terminal-only transient password capture, `tea.Exec`, cancellation, stale-result rejection, and secret-free contracts remain unchanged.
- Edit validates fields in `Profile.Validate` order through semantic IDs, blocks invalid Save without `Save`/`Update`, and focuses the first invalid field while retaining focusable Save.
- Edit uses the shared BAC shell/panel/field/action/feedback/footer/viewport primitives with paired English/Spanish feedback and 80x24 NO_COLOR proof.
- Edit Save remains metadata-only; Cancel discards the draft, and no authentication, connectivity, or proof path is invoked.

## Remaining

- None. Phase 1 tasks 1.1–1.5, Phase 2 tasks 2.1–2.5, and corrective tasks C1–C2 are checked complete: 12/12 total.
