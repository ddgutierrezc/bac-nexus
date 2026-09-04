```yaml
schema: gentle-ai.verify-result/v1
evidence_revision: sha256:ea43b20d8383fb4deba4612dc4466b97ad94a04e357a56e5059ea6a6e75248f8
verdict: pass_with_warnings
blockers: 0
critical_findings: 0
requirements: 5/5
scenarios: 14/14
test_command: go test -count=1 ./...
test_exit_code: 0
test_output_hash: sha256:462f829f4ec7ccf3859302b763679388925d8bca4971fb82026e7c2c1bc0bcd0
build_command: go build ./...
build_exit_code: 0
build_output_hash: sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
```

## Verification Report

**Change**: restore-componentized-profile-onboarding  
**Mode**: Hybrid OpenSpec+Engram; Strict TDD  
**Evidence revision**: `d7f1da5a88c3e069782516998ad911c12479dcb6` plus preserved candidate diff `sha256:ea43b20d8383fb4deba4612dc4466b97ad94a04e357a56e5059ea6a6e75248f8`.

### Completeness

| Metric | Result |
|---|---:|
| Tasks completed | 17/17 |
| Tasks incomplete | 0 |
| Delta requirements | 5/5 |
| Delta scenarios | 14/14 |

All task checkboxes are complete in OpenSpec and match the cumulative Engram apply-progress record.

### Build and Test Evidence

| Command | Exit | Output SHA-256 | Result |
|---|---:|---|---|
| Focused TUI, lifecycle, port, compensation, and locale suites | 0 | Runtime output observed | PASS |
| `go test -count=1 ./...` | 0 | `462f829f4ec7ccf3859302b763679388925d8bca4971fb82026e7c2c1bc0bcd0` | PASS |
| `go test -race -count=1 ./...` | 0 | `a9ea1ee180f0295be565ef6361d39ecc70153c1e6eafe78255f508d62bdfa2e0` | PASS |
| `go vet ./...` | 0 | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` | PASS |
| `go build ./...` | 0 | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` | PASS |
| `git diff --check` | 0 | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` | PASS |

No live IBM i command ran. Normal tests use deterministic local fakes; live validation remains explicit opt-in external follow-up.

### Spec Compliance Matrix

| Requirement | Scenarios | Passing runtime evidence | Result |
|---|---:|---|---|
| Componentized Four-Step Onboarding | 3/3 | `TestFourStepOnboardingRuntimeMatrix`; `TestFourStepRuntimeNavigationPreservesValuesAndFeedbackPrecedence`; `TestFourStepOnboardingGuardsPreserveValuesAndKeepBlockedActionsFocusable` | COMPLIANT |
| Shared Responsive Profile Screens | 2/2 | TUI runtime matrix and `TestProfileScreenEditRuntimeFramesPreserveActionsValidationAndOverflow` | COMPLIANT |
| Field-Specific Create Validation and Feedback | 2/2 | `TestFourStepRuntimeNavigationPreservesValuesAndFeedbackPrecedence`; guard runtime test | COMPLIANT |
| Direct Secure Onboarding | 4/4 | terminal capture/retry tests, matrix secret-free review, stale and cancellation TUI tests | COMPLIANT |
| Bounded Backend Connection and Persistence | 3/3 | `TestOnboardingUsesSelectedPortAndAuditsAfterProofBeforeCommit`; lifecycle and compensation tests | COMPLIANT |

The matrix executes 48 `Update`/`View` frames: four steps × 120x40, 80x24, 40x16 × English/Spanish × color/NO_COLOR. It covers focus and readiness, blocked first-invalid reveal, feedback precedence, overflow, wrapping/bounds, no ANSI under NO_COLOR, and secret-free review.

### Correctness

| Check | Result | Evidence |
|---|---|---|
| Exactly four Create steps; Edit preservation | PASS | Controller/runtime tests and Edit frames prove Name, Connection, Credentials, Review; metadata Edit retains Save/Cancel. |
| Name/duplicate then host, username, port validation | PASS | Guard tests prove ordered first-invalid focus and default editable port 22. |
| Selected non-default port | PASS | Port 2222 crosses request, inspection, proof, identity, commit, and persisted JSON. |
| Secret boundary and lease lifecycle | PASS | Capture is fixed in-process `tea.Exec`; messages contain identity/generation/status only; bounded single-use lease tests cover revoke, expiry, replacement, cancellation, stale result, shutdown, and zeroization. |
| Ordered transaction and compensation | PASS | Ordered-spy and commit tests prove inspect, proof, audit, credential, commit sequence, fail-closed audit behavior, reverse compensation, CleanupRequired, and retained journal. |
| Retired semantics absent | PASS | Source/test inspection and active docs/specs retain no active eight-step, proof-choice, Mapepire onboarding, draft, or proof-rerun route. Historical archived artifacts are excluded. |

### Design Coherence

| Decision | Result |
|---|---|
| Historical component recovery with responsive viewport and wrapping | PASS |
| TUI opaque identity only; application-owned secret lease | PASS |
| Inspection/proof before audit, credential handling, and commit | PASS |
| Metadata Edit remains separate from connection/proof behavior | PASS |

### TDD Compliance

| Check | Result | Details |
|---|---|---|
| TDD evidence reported | PASS | Complete cycle table is present for all 17 tasks. |
| Test files exist | PASS | Referenced TUI, configuration, profile, remote, and localization tests exist. |
| GREEN evidence reconfirmed | PASS | Focused suites plus clean full and race runs passed. |
| Triangulation | PASS | Runtime frames, fake-clock lease tests, terminal boundary tests, ordered spies, and transaction compensation cover distinct paths. |
| Assertion quality | PASS | Inspected scenario tests exercise production calls and behavioral output; no tautology, ghost-loop, or smoke-only assertion was found. |

**TDD compliance**: 5/5 checks passed.

### Test Layer Distribution

| Layer | Files | Coverage role |
|---|---:|---|
| Unit | 5 | lease, validation, localization, transaction/compensation |
| In-process integration/runtime | 4 | Bubble Tea Update/View, terminal seam, port/order flow |
| E2E/live IBM i | 0 | Explicitly opt-in and not run |

### Issues

**CRITICAL**: None.  
**WARNING**: No live IBM i validation was run; this is an approved external opt-in follow-up, not a local verification failure.  
**SUGGESTION**: Run the approved read-only IBM i integration suite when an authorized environment, identity, authority, endpoint, and trust policy are available.

## Final Verdict: PASS WITH WARNINGS
