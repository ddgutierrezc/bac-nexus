```yaml
schema: gentle-ai.verify-result/v1
evidence_revision: sha256:f1bd1c2f9ae2de20b429e887391a83fece9ae8d919b308653f6abe30eb8fde03
verdict: pass
blockers: 0
critical_findings: 0
requirements: 5/5
scenarios: 11/11
test_command: go test -count=1 ./...
test_exit_code: 0
test_output_hash: sha256:baf4f0be711269656cd50b1026a6cbafd710d3af24fe694f234a6ec480015e4a
build_command: go build ./...
build_exit_code: 0
build_output_hash: sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
```

## Verification Report

**Change**: restore-profile-ui-validation  
**Mode**: Strict TDD  
**Evidence scope**: current non-`.atl`, non-OpenSpec source/test/localization diff; no live IBM i used.

### Completeness

| Metric | Value |
|---|---:|
| Tasks total | 12 |
| Tasks complete | 12 |
| Tasks incomplete | 0 |
| Requirements | 5/5 |
| Scenarios | 11/11 |

### Build & Tests Execution

| Command | Exit | Output SHA-256 | Result |
|---|---:|---|---|
| `go test -count=1 ./internal/tui -run TestProfileScreenRendersEveryDirectOnboardingLifecycleAtRequiredFrames` | 0 | `c6b1838ffeba54f11a68143a3f91192d8effca56058514b76b832b4745cea453` | PASS |
| `go test -count=1 ./internal/tui -run 'TestEdit(AuthoritativeValidationOrderWinsOverSyntacticParsing|EndpointValidationClearsWhenEitherEndpointFieldChanges|InvalidSaveIsFocusableBlockedAndRevealsFirstInvalidField|SaveUpdatesMetadataAndCancelDiscardsDraft)'` | 0 | `c864211cd8f617d34cc1266f57adc4c3c49d8f4e96deb8369b8ab11c18437d18` | PASS |
| `go test -count=1 ./internal/tui -run 'Test(EditLocalizedValidationFramesRemainReachable|DirectOnboardingLocalizedFieldValidationRendersAtRuntime)'` | 0 | `0a4c410380b47b2344b6ab3ee6d9ab9e034f1aec244e32fff8e5dbf4ba80d915` | PASS |
| `go test -count=1 ./internal/tui` | 0 | `eec2e996933dd33b7a95444547a1463f4d54cdb294fe4d34dbfa98cada156b2f` | PASS |
| `go test -count=1 ./...` | 0 | `baf4f0be711269656cd50b1026a6cbafd710d3af24fe694f234a6ec480015e4a` | PASS |
| `go vet ./...` | 0 | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` | PASS |
| `go build ./...` | 0 | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` | PASS |
| `git diff --check` | 0 | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` | PASS |

No live IBM i command was executed. `go test -cover ./internal/tui` passed with 69.4% package statement coverage; coverage is informational.

### Spec Compliance Matrix

| Requirement | Scenario | Runtime covering test(s) | Result |
|---|---|---|---|
| Shared Responsive Profile Screens | Direct create lifecycle | `TestProfileScreenRendersEveryDirectOnboardingLifecycleAtRequiredFrames`; `TestDirectOnboardingEscapeCancelsAndRejectsStaleResult`; `TestDirectOnboardingCompletionFinalizesToReloadedProfileList` | ✅ COMPLIANT |
| Shared Responsive Profile Screens | Edit lifecycle | `TestEditSaveUpdatesMetadataAndCancelDiscardsDraft` | ✅ COMPLIANT |
| Field-Specific Create Validation and Feedback | Invalid create submission | `TestDirectOnboardingValidationBlocksTheFirstInvalidField` | ✅ COMPLIANT |
| Field-Specific Create Validation and Feedback | Feedback precedence and clearing | `TestDirectOnboardingValidationClearsOnlyTheEditedFieldAndDefersToOperationFeedback`; `TestDirectOnboardingEditingInvalidFieldClearsItsOwnValidation` | ✅ COMPLIANT |
| Validated Metadata Edit | Invalid edit save | `TestValidateEditProfileMapsAuthoritativeValidationOrderToSemanticFields`; `TestEditInvalidSaveIsFocusableBlockedAndRevealsFirstInvalidField`; `TestEditAuthoritativeValidationOrderWinsOverSyntacticParsing` | ✅ COMPLIANT |
| Validated Metadata Edit | Cancel edit | `TestEditSaveUpdatesMetadataAndCancelDiscardsDraft` | ✅ COMPLIANT |
| Accessible Terminal Rendering and Copy Parity | Narrow runtime frames | `TestProfileScreenRendersEveryDirectOnboardingLifecycleAtRequiredFrames`; `TestEditLocalizedValidationFramesRemainReachable`; `TestProfileScreenKeepsNarrowDirectOnboardingReachable` | ✅ COMPLIANT |
| Accessible Terminal Rendering and Copy Parity | No-color localized feedback | `TestDirectOnboardingLocalizedFieldValidationRendersAtRuntime`; `TestEditLocalizedValidationFramesRemainReachable`; `TestNoColorUsesInjectableEnvironmentAndPreservesLocaleParityAt80x24` | ✅ COMPLIANT |
| Direct Secure Onboarding | Connect and save | `TestOnboardingExecCommandCapturesOnlyAtTheFixedBoundary`; `TestDirectOnboardingCompletionFinalizesToReloadedProfileList` | ✅ COMPLIANT |
| Direct Secure Onboarding | Password prompt cannot proceed | `TestOnboardingExecCommandPromptFailureStartsNoOperation` | ✅ COMPLIANT |
| Direct Secure Onboarding | Retired semantics remain absent | `TestDirectOnboardingViewContainsNoSecretAndHasSpanishAction`; `TestOnboardingExecCommandCapturesOnlyAtTheFixedBoundary`; full TUI suite | ✅ COMPLIANT |

**Compliance summary**: 11/11 scenarios compliant.

### Correctness (Static Evidence)

| Requirement | Status | Notes |
|---|---|---|
| Shared screen shell and lifecycle | ✅ Implemented | `profileScreen` composes BAC header, centered panel, footer, semantic states, and persistent `viewport.Model`. |
| Create validation | ✅ Implemented | `directOnboardingValidation` delegates to `profile.ValidateHost` then `profile.ValidateUsername`; invalid action returns no command and focuses first invalid field. |
| Edit validation | ✅ Implemented | `validateEditProfilePreflight` prevents later port/trust parsing from masking name/endpoint/username errors; `validateEditProfile` preserves authoritative group order and final `Profile.Validate` gate. |
| Feedback and endpoint clearing | ✅ Implemented | Feedback is operation > explicit error > local validation; `formValidationIncludesField` treats host and port as one endpoint group. |
| Security/lifecycle boundary | ✅ Implemented | The direct `tea.Exec`/`OnboardingOperations` boundary, cancellation, generation-based stale result rejection, and terminal-only secret capture remain in place; Edit invokes only metadata store operations. |

### Coherence (Design)

| Decision | Followed? | Notes |
|---|---|---|
| Reusable profile primitives over existing shell/wrap behavior | ✅ Yes | `profile_screen.go` supplies shell/panel/field/action/feedback/viewport helpers. |
| Semantic validation adapters, not error-string parsing | ✅ Yes | `profileValidation` carries field/message IDs and cause; rendered feedback localizes message IDs. |
| Persistent viewport with focus reveal | ✅ Yes | Viewport is model-owned and refreshed/revealed on size, focus, validation, and lifecycle changes. |
| Preserve direct onboarding operation boundary | ✅ Yes | No Edit connectivity/auth/proof path was added; no password state is present in `Model`. |

### TDD Compliance

| Check | Result | Details |
|---|---|---|
| TDD evidence reported | ✅ | Engram merged `apply-progress` contains all 12 task rows, including C1/C2. |
| All tasks have tests | ✅ | 12/12 task rows name a test file or existing integration boundary. |
| RED confirmed | ✅ | Reported test files exist in the current diff/package. |
| GREEN confirmed | ✅ | Focused tests, package gate, and full suite passed in this verification. |
| Triangulation adequate | ✅ | Lifecycle/frame, ordered validation, endpoint-group clearing, locale, save/cancel, and security paths use varied cases. |
| Safety net for modified files | ✅ | Each modified-file row reports a passing package baseline; new files are identified as new. |

**TDD Compliance**: 6/6 checks passed.

### Test Layer Distribution

| Layer | Tests | Files | Tools |
|---|---:|---:|---|
| Unit | 8+ | 2 | Go testing |
| Integration/runtime `Model.Update`/`View()` | 20+ | 4 | Go testing |
| E2E | 0 | 0 | Not available/not required |

### Changed File Coverage

`go test -cover ./internal/tui` passed at **69.4%** package statement coverage. Changed profile primitives and validation adapters have direct runtime/unit coverage; the package aggregate includes unrelated legacy TUI code. Coverage is informational; no configured coverage threshold exists.

### Assertion Quality

**Assertion quality**: ✅ All inspected changed-test assertions invoke production code or runtime rendering and assert state, commands, persistence counts, content, bounds, focus, or ANSI absence. No tautologies, ghost loops, or assertion-only tests were found.

### Quality Metrics

**Linter/static analysis**: ✅ `go vet ./...` passed.  
**Type checker/build**: ✅ `go build ./...` passed.

### Issues Found

**CRITICAL**: None.

**WARNING**:
- Hybrid apply-progress artifacts are not byte-equivalent: OpenSpec records the earlier 1,007-line/464-line PR 2 state and omits the C1/C2 correction evidence, while Engram records the merged 1,135-line/592-line state and C1/C2. The runtime implementation and all 12 checked tasks are verified, but the planning stores should be reconciled before archival evidence is treated as historical source of truth.
- Package coverage is 69.4%; it is unconfigured and non-blocking, but below the Strict TDD module's 80% informational threshold.

**SUGGESTION**: Reconcile the OpenSpec `apply-progress.md` with the merged Engram correction evidence during the orchestrator-owned settlement/archive flow.

### Verdict

**PASS WITH WARNINGS** — all 5 requirements and 11 scenarios have passing runtime coverage; warnings concern evidence-store drift and informational coverage only.
