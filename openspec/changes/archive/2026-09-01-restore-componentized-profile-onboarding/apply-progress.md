# Apply Progress: Restore Componentized Profile Onboarding

## Scope

- Completed work units: `phase1-historical-recovery` (PR 1), `phase2-four-step-controller` (PR 2), `phase3a-lease-port-ordering` (corrected PR 3), `phase3b-lease-safety-compensation` (PR 4), and `phase4-runtime-evidence-docs` (PR 5, stacked-to-main).
- Completed tasks: 1.1–1.3, 2.1–2.3, 3.1–3.8, and 4.1–4.3.
- Mode: Strict TDD.
- Delivery: `auto-chain`, `stacked-to-main`.
- Phase 2 includes only the minimal localized Step 3 prerequisite. PR 5 completes the runtime-matrix, parity, and documentation slice.

## Completed Tasks

- [x] 1.1 Added runtime `Update`/`View` regression coverage for metadata Edit validation focus and narrow overflow, while existing runtime tests retain Save/Cancel and result/discard coverage.
- [x] 1.2 Restored selected exact `d027210` feedback, overflow, and progress primitives; moved `wizardOverflowIndicator` from `model.go` to `wizard_viewport.go` without a path checkout.
- [x] 1.3 Reconciled the recovered screen files symbol-by-symbol by retaining the direct Create, Edit validation, and profile-screen implementations unchanged while relocating only the shared indicator ownership; the full Go suite remains green.
- [x] 2.1 Added RED-first controller regressions for exactly four steps, blocked name action focus, safe-value Back preservation, and review reachability.
- [x] 2.2 Replaced the direct two-field Create route with Name → Connection → Credentials → Review, including historical field focus/cursor conventions, case-insensitive duplicate checks, host→username→port validation, default port 22, and stale-result rejection.
- [x] 2.3 Kept password input inside fixed in-process `tea.Exec`, added terminal-handle/EOF/cancellation checks and a localized explicit terminal prompt, and added only the required paired ES/EN Step 3 catalog IDs plus registry entries.
- [x] 3.1 Completed RED-first lease coverage for 1–1024 capture bounds, opaque identities/statuses, atomic once-only consumption, and TUI Back revocation.
- [x] 3.2 Completed the application-owned `Capture`/`Revoke`/`StartCaptured` seam; TUI state and `tea.Exec` result messages contain only identity/generation/status and no credential bytes or process invocation.
- [x] 3.3 Proved selected port 2222 through request, inspection, proof, result, prepared commit, and persisted JSON profile, with `inspect → proof → sanitized audit → commit` ordering.
- [x] 3.4 Ran focused, full, and targeted race evidence and recorded the PR 3 rollback boundary and line accounting.
- [x] 3.5 Added RED-first fake-clock lifecycle coverage for replacement, expiry, stale revoke, failed capture, and shutdown worker-stop zeroization.
- [x] 3.6 Ensured rejected/expired leases zero before dependency rejection and added canary-negative coverage for audit, profile, recovery, result, serialization, reflection, formatting/debug, and error surfaces.
- [x] 3.7 Compensated potentially partial credential writes before profile persistence; required-audit failure now proves no commit, while existing transaction seams prove reverse compensation and retained recovery journals.
- [x] 3.8 Ran focused configuration/profile/composition, targeted race, full, vet, build, diff, rollback, and changed-line evidence.
- [x] 4.1 Added RED-first `Update`/`View` matrix evidence for 48 cases: four steps × three frames × two locales × color/`NO_COLOR`; it proves focus, overflow, bounds, navigation/Back preservation, blocked first-invalid focus, feedback precedence, and a secret-free review.
- [x] 4.2 Added paired four-step catalog/localization coverage for Spanish and English labels/descriptions without language leakage; existing catalog validation and runtime frames prove complete registry and `NO_COLOR` behavior without changing Step 3 keys owned by 2.3.
- [x] 4.3 Updated the onboarding guide, design system, and canonical configuration specification for the four-step terminal-only secret-lease architecture; ran formatting, focused, full, race, vet, build, and diff checks.

## TDD Cycle Evidence

| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|---|
| 1.1 | `internal/tui/profile_screen_test.go` | Runtime `Update`/`View` | `go test -count=1 ./internal/tui` — PASS | Approval/runtime regression written before primitive production changes | Focused runtime test PASS | Validation-focus and narrow-overflow frames | None needed |
| 1.2 | `internal/tui/wizard_progress_test.go` | Unit | `go test -count=1 ./internal/tui` — PASS | `go test -count=1 ./internal/tui -run '^TestActivateWizardProgressAllowsReadyAndBlocksInvalidContinuation$'` — FAIL: undefined wizard progress symbols | Focused test PASS after selective primitive restoration | Ready and blocked paths | Relocated duplicate overflow ownership without behavior change |
| 1.3 | `internal/tui/profile_screen_test.go` | Runtime `Update`/`View` | `go test -count=1 ./internal/tui` — PASS | Existing preservation approval tests exercised before reconciliation; no new production behavior was required | `go test -count=1 ./...` — PASS | Save/Cancel, validation, result/discard, and overflow coverage | None needed |
| 2.1 | `internal/tui/onboarding_test.go` | Controller/runtime | `go test -count=1 ./internal/tui` — PASS | `go test -count=1 ./internal/tui -run '^(TestFourStepOnboardingGuardsPreserveValuesAndKeepBlockedActionsFocusable|TestSecurePasswordActionUsesLocalizedPromptAndReturnsToReview)$'` — FAIL: missing four-step state symbols | Same focused command — PASS | Existing locale, viewport, Edit, and stale-result tests — PASS | Shared focus-order helper kept state-specific |
| 2.2 | `internal/tui/onboarding_test.go` | Controller | `go test -count=1 ./internal/tui` — PASS | Four-step guard test added before controller state and renderer | Package suite — PASS | 120x40/80x24/40x16 existing runtime frames and NO_COLOR paths — PASS | Kept the recovered shell/viewport primitives intact |
| 2.3 | `internal/tui/onboarding_test.go`, `internal/remote/ssh.go` | Terminal boundary | `go test -count=1 ./internal/tui ./internal/remote` — PASS | Prompt-status and localized-prompt tests added before terminal capture output change | Focused terminal packages — PASS | Mismatched handles, EOF, cancellation, no operation start, and no secret in result — PASS | Prompt remains a localized input to fixed `tea.Exec`; no shell/process added |
| 3.1 | `internal/configuration/onboarding_test.go`, `internal/tui/onboarding_test.go` | Unit / Bubble Tea command | `go test -count=1 ./internal/configuration ./internal/tui` — PASS | Original partial RED failed for absent lease/name/port APIs; the completed boundary matrix adds 0, 1, 1024, and 1025-byte cases before acceptance | `go test -count=1 ./internal/configuration -run '^(TestOnboardingCaptureLeaseIsBoundedSingleUseAndExpires|TestOnboardingCaptureAcceptsOnlyOneTo1024SecretBytes)$'` — PASS | Replacement, once-only consume, expiry, and both accepted limits | No behavior refactor needed |
| 3.2 | `internal/configuration/onboarding_test.go`, `internal/tui/onboarding_test.go` | Unit / fixed in-process command | Existing target packages — PASS | Original partial RED failed for the application-owned capture interface and TUI capture seam | `go test -count=1 ./internal/tui -run '^(TestOnboardingBackRevokesCapturedLease|TestOnboardingExecCommandDelegatesCaptureWithoutSecretArgument|TestOnboardingExecCommandCapturesOnlyAtTheFixedBoundary|TestOnboardingExecCommandReturnsRetryableSecretFreeCaptureStatuses)$'` — PASS | Capture success/failure, no secret argument/result, and Back revoke | Fixed `tea.Exec` remains shell-free |
| 3.3 | `internal/configuration/onboarding_test.go` | In-process integration | Existing configuration suite — PASS | Original partial RED exposed the prior inspect/audit/proof ordering and missing selected-port propagation | `go test -count=1 ./internal/configuration -run '^TestOnboardingUsesSelectedPortAndAuditsAfterProofBeforeCommit$'` — PASS | Selected port in inspect, proof, result, commit, persisted JSON load, and ordered audit | Reused the prepared commit seam |
| 3.4 | Existing focused suites | Unit / in-process integration | Focused suites — PASS | N/A: evidence-only task | `go test -count=1 -race ./internal/configuration ./internal/tui` — PASS | Normal and race execution | None needed |
| 3.5 | `internal/configuration/onboarding_test.go` | Unit / fake-clock lifecycle | `go test -count=1 ./internal/configuration ./internal/profile ./cmd/nexus` — PASS | Focused lifecycle tests written before `Shutdown` and expiry cleanup changes; RED failed: `Shutdown undefined` and expired lease remained non-zero | Focused lifecycle command — PASS | Replacement, stale revoke, expiry, failed capture, unconsumed shutdown, and worker-stop cancellation | `StartCaptured` consumes and zeroes a lease before dependency rejection; `Shutdown` snapshots workers outside the mutex |
| 3.6 | `internal/configuration/onboarding_test.go` | Unit / canary-negative | Configuration baseline — PASS | Canary-negative test added before zeroization cleanup change | Focused canary command — PASS | Audit/profile/recovery/result JSON, formatted debug, error, and reflection type surfaces contain no canary | No reflection/stringer/marshaler hook was introduced |
| 3.7 | `internal/profile/onboarding_commit_test.go`, `internal/configuration/onboarding_test.go` | Unit / in-process transaction | Profile/configuration baseline — PASS | Partial-keyring test written before compensation change; RED failed with missing rollback/journal-clear events | Focused transaction command — PASS | Required bootstrap-audit failure stops commit; committed-audit reverse compensation and journal retention retain coverage | Reused `profile.OnboardingCommit`; no new persistence abstraction or transport |
| 3.8 | Focused configuration/profile/composition suites | Unit / in-process integration | Focused suites — PASS | N/A: evidence-only task | Full/race/static/build/diff commands — PASS | Normal/race and full-suite execution | None needed |
| 4.1 | `internal/tui/render_matrix_test.go` | Runtime `Update`/`View` integration | `go test -count=1 ./internal/tui` — PASS | New matrix initially FAILED: back-focus labels were not mapped for viewport reveal | Focused matrix command — PASS after mapping Back focus to the localized action | 48 matrix cases plus navigation/Back, blocked first-invalid focus, feedback precedence, overflow, bounds, and secret-free review | Kept existing viewport/feedback behavior; added only Back focus mapping |
| 4.2 | `internal/localization/localization_test.go` | Unit / catalog integration | `go test -count=1 ./internal/localization` — PASS | Added paired-label/descriptions parity test before accepting the existing complete catalog surface | Focused localization command — PASS | Spanish and English each cover four labels and four descriptions; TUI matrix covers color and `NO_COLOR` | No catalog change: the required Step 4 keys were already paired and registered; Step 3 keys remain task 2.3 ownership |
| 4.3 | Documentation and final checks | Documentation / static | Focused suites — PASS | N/A: documentation-only behavior update | Full/race/static/build/diff commands — PASS | Canonical spec, guide, and design system state the same four-step/lease contract | None needed |

## Work Unit Evidence

| Evidence | Result |
|---|---|
| Focused test command and exact result | `go test -count=1 ./internal/tui -run '^(TestActivateWizardProgressAllowsReadyAndBlocksInvalidContinuation|TestProfileScreenEditRuntimeFramesPreserveActionsValidationAndOverflow)$'` — PASS (`ok`, 0.020s) |
| Runtime harness command/scenario and exact result | Same focused command directly drives Bubble Tea `Update` then `View` for metadata Edit validation focus at 80x24 and narrow overflow/Save reachability at 40x16 — PASS |
| Full suite | `go test -count=1 ./...` — PASS |
| Static/build/diff checks | `go vet ./...` — PASS; `go build ./...` — PASS; `git diff --check` — PASS |
| Rollback boundary | Revert `internal/tui/wizard_{render,viewport,progress}.go`, `internal/tui/wizard_progress_test.go`, the `wizardOverflowIndicator` relocation in `internal/tui/model.go`, and the Phase 1 test block in `internal/tui/profile_screen_test.go`; recovered direct onboarding and Edit behavior remain intact. |

## Phase 2 Work Unit Evidence

| Evidence | Result |
|---|---|
| Focused test command and exact result | `go test -count=1 ./internal/tui -run '^(TestFourStepOnboardingGuardsPreserveValuesAndKeepBlockedActionsFocusable|TestSecurePasswordActionUsesLocalizedPromptAndReturnsToReview)$'` — PASS (`ok`, 0.014s); `go test -count=1 ./internal/tui ./internal/remote` — PASS. |
| Runtime harness command/scenario and exact result | `go test -count=1 ./internal/tui` — PASS (`ok`, 0.458s): Bubble Tea `Update`/`View` frames cover controller navigation, blocked focus, narrow viewport reachability, locale, `NO_COLOR`, cancellation, and stale result rejection. |
| Full suite and static checks | `go test -count=1 ./...` — PASS; `go vet ./...` — PASS; `go build ./...` — PASS; `git diff --check` — PASS. |
| Rollback boundary | Revert the Phase 2 changes in `internal/tui/{model,onboarding}.go`, `internal/tui/*_test.go`, `internal/remote/ssh.go`, and the paired Step 3 catalog/registry entries; Phase 1 primitives and metadata Edit remain independent. |

## Changed-Line Accounting

- Phase 1 authored implementation and test delta: approximately 224 lines (selected primitive files, relocation, and regression tests).
- Phase 1 OpenSpec evidence/checklist delta: approximately 74 lines.
- Total attributable Phase 1 slice: approximately 298 changed lines, below the 800-line limit.
- Pre-existing dirty recovery code and protected `.atl`/OpenSpec artifacts are excluded and were not reset, stashed, checked out, or overwritten.
- Phase 2 authored delta is approximately 670 lines (controller, terminal boundary, focused tests, and minimal localization/task evidence), below the 800-line work-unit cap. Repository-wide `git diff` also includes preserved dirty Phase 1 recovery and is not Phase 2 attribution.
- Corrected PR 3 started with 427 charged changed lines. This completion adds 41 changed lines in `internal/configuration/onboarding_test.go`, for 468 cumulative changed lines, below the authorized 800-line maximum.
- PR 4 adds 223 net code/test lines across `internal/configuration/{onboarding.go,onboarding_test.go}` and `internal/profile/{onboarding_commit.go,onboarding_commit_test.go}`. Its task/progress updates add 33 changed artifact lines, including four checkbox replacements; the 256-line attributable slice is below the 800-line authorization. Protected PR 1–3 dirty work is excluded because no reset/stash/checkout was permitted.

## Deviations and Risks

- No design deviation. The historical recovery is intentionally selective: only reusable feedback, overflow, and progress primitives were restored; obsolete eight-step state/viewport semantics remain absent.
- Phase 2 is required before these primitives become a four-step onboarding controller.
- The localization dependency was corrected: task 2.3 owns only the Step 3 action, hidden terminal prompt, and secret-free status keys; task 4.2 remains pending for the complete parity/matrix work.

## Corrected PR 3 Work Unit Evidence

| Evidence | Result |
|---|---|
| Focused test command and exact result | `go test -count=1 ./internal/configuration -run '^(TestOnboardingCaptureLeaseIsBoundedSingleUseAndExpires|TestOnboardingCaptureAcceptsOnlyOneTo1024SecretBytes|TestOnboardingUsesSelectedPortAndAuditsAfterProofBeforeCommit)$'` — PASS (`ok`, 0.005s); `go test -count=1 ./internal/tui -run '^(TestOnboardingBackRevokesCapturedLease|TestOnboardingExecCommandDelegatesCaptureWithoutSecretArgument|TestOnboardingExecCommandCapturesOnlyAtTheFixedBoundary|TestOnboardingExecCommandReturnsRetryableSecretFreeCaptureStatuses)$'` — PASS (`ok`, 0.006s). |
| Runtime harness command/scenario and exact result | In-process Bubble Tea command/model and fake-clock/ordered-spy seams: capture returns opaque metadata only, Back revokes the lease, and selected port 2222 traverses inspect, proof, audit, prepared commit, and JSON persistence. The focused commands above passed; live IBM i is N/A because the approved test boundary is local-only. |
| Full/static/build/diff results | `go test -count=1 ./...` — PASS; `go test -count=1 -race ./internal/configuration ./internal/tui` — PASS; `go vet ./...` — PASS; `go build ./...` — PASS; `git diff --check` — PASS. |
| Rollback boundary | Revert only corrected PR 3 changes in `internal/configuration/{onboarding.go,onboarding_test.go}` and `internal/tui/{model.go,onboarding.go,onboarding_test.go}`. This removes the lease/selected-port seam without reverting protected Phase 1–2 recovery/controller work. |

## Remaining Risks

- No live IBM i validation was run; it remains explicitly opt-in and outside the approved local test boundary.

## PR 4 Work Unit Evidence

| Evidence | Result |
|---|---|
| Focused test command and exact result | `go test -count=1 ./internal/configuration ./internal/profile -run '^(TestOnboardingLeaseRevocationZeroesReplacementExpiryStaleAndFailedCapture|TestOnboardingShutdownWaitsForWorkerAndZeroesSecret|TestOnboardingCanaryNeverEscapesSecretOwningBoundary|TestOnboardingRequiredAuditFailurePreventsCommitAndZeroesWorkerSecret|TestOnboardingCommitCompensatesPartialCredentialFailureBeforeProfilePersistence|TestOnboardingCommitCompensatesInReverseOrderAfterCommittedAuditFailure|TestOnboardingCommitRetainsJournalWhenCompensationFails)$'` — PASS (`ok` configuration 0.004s; profile 0.002s). |
| Runtime harness command/scenario and exact result | In-process fake-clock, terminal-prompt, ordered transaction, and worker-cancellation harnesses — PASS. They prove local-only capture/revoke/expiry/shutdown, audit-before-commit failure, partial keyring compensation, and retained journal behavior. Live IBM i is N/A: this work unit has no approved external runtime boundary. |
| Focused configuration/profile/composition | `go test -count=1 ./internal/configuration ./internal/profile ./cmd/nexus` — PASS (0.017s, 0.008s, 0.005s). |
| Targeted race / full / static / build / diff | `go test -race -count=1 ./internal/configuration ./internal/profile ./cmd/nexus` — PASS; `go test -count=1 ./...` — PASS; `go vet ./...` — PASS; `go build ./...` — PASS; `git diff --check` — PASS. |
| Rollback boundary | Revert PR 4 changes in `internal/configuration/{onboarding.go,onboarding_test.go}` and `internal/profile/{onboarding_commit.go,onboarding_commit_test.go}` plus the PR 4 task/progress entries. Phase 1–3 UI, port propagation, prepared-journal infrastructure, and all PR 5 work remain independent. |

## PR 5 Work Unit Evidence

| Evidence | Result |
|---|---|
| Focused test command and exact result | `go test -count=1 ./internal/tui -run '^(TestFourStepOnboardingRuntimeMatrix|TestFourStepRuntimeNavigationPreservesValuesAndFeedbackPrecedence)$'` — PASS (`ok`, 0.152s); `go test -count=1 ./internal/localization -run '^TestFourStepOnboardingCatalogsRemainPairedWithoutLanguageLeakage$'` — PASS (`ok`, 0.012s). |
| Runtime harness command/scenario and exact result | The 48-case Bubble Tea `Update`/`View` harness (4 steps × 120x40/80x24/40x16 × Spanish/English × color/`NO_COLOR`) — PASS. It proves localized focus/reachability, overflow disclosure, bounds, no ANSI in `NO_COLOR`, secret-free review, Back preservation, blocked first-invalid state, and feedback precedence. Live IBM i is N/A because no approved external runtime boundary exists. |
| Full/race/static/build/diff results | `go test -count=1 ./...` — PASS; `go test -race -count=1 ./...` — PASS; `go vet ./...` — PASS; `go build ./...` — PASS; `git diff --check` — PASS. |
| Rollback boundary | Revert PR 5 changes in `internal/tui/{onboarding.go,render_matrix_test.go}`, `internal/localization/localization_test.go`, `docs/IBM_I_PROFILE_WIZARD.md`, `DESIGN.md`, `openspec/specs/nexus-configuration/spec.md`, and the PR 5 task/progress entries. All PR 1–4 behavior remains independent. |

## PR 5 Changed-Line Accounting

- PR 5 attributable delta: approximately 230 changed lines across the runtime matrix/focus mapping, localization parity test, documentation/specification, and task/progress evidence; below the authorized 800-line maximum. This excludes the cumulative dirty diff retained from PR 1–4.
- Preserved dirty PR 1–4 recovery, implementation, `.atl`, and artifact changes were neither reset, stashed, checked out, nor overwritten.
