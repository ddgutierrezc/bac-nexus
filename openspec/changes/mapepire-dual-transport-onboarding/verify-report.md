```yaml
schema: gentle-ai.verify-result/v1
evidence_revision: sha256:f9638f3954bb49015b173df9ddfb45e236b8e31f3f95d87f8bed822ebf3d2498
verdict: pass_with_warnings
blockers: 0
critical_findings: 0
requirements: 18/18
scenarios: 48/48
test_command: go test -count=1 ./...
test_exit_code: 0
test_output_hash: sha256:41f6fafa0c7f73e49ac4aa1e7825e28976e4a74f051667bf23b53a02dea9a58a
build_command: go build ./...
build_exit_code: 0
build_output_hash: sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
```

## Verification Report

**Change**: `mapepire-dual-transport-onboarding`
**Mode**: Strict TDD; hybrid OpenSpec + Engram; offline-only
**Native attempt**: `canonical-final-verification` under token `sha256:bc0d0fec8a26c75eb660e37fb7f9e58192c0d1519314150eb96b01053b3a8605`
**Verdict**: **PASS WITH WARNINGS**

### Canonical Verification Evidence

The envelope `evidence_revision` is the SHA-256 identifier of these exact LF-terminated evidence-preimage bytes, retained here for native settlement:

```yaml
schema: gentle-ai.verification-evidence/v1
attempt_token: sha256:bc0d0fec8a26c75eb660e37fb7f9e58192c0d1519314150eb96b01053b3a8605
request_id: canonical-final-verification-rerun-20260828
candidate_identity: sha256:f7bdbbdb2545c2ab98d70aee1e09d30df1d9fef04deb4a3d16d097d1517d907a
candidate_tree: d0799f7774c8969e3fb713076136d850ac62de54
requirements: 18/18
scenarios: 48/48
test_command: go test -count=1 ./...
test_exit_code: 0
test_output_hash: sha256:41f6fafa0c7f73e49ac4aa1e7825e28976e4a74f051667bf23b53a02dea9a58a
build_command: go build ./...
build_exit_code: 0
build_output_hash: sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
focused_output_hash: sha256:237af5d4e244a12b7c5d93ab40fcfa8e65100eaa67d4bea07185a1e31e57bfe2
coverage_output_hash: sha256:bdef6438d6873b691500e8c59dc764addfd2b1b6be8e24846a7e848885b1d77f
race_output_hash: sha256:efbaaa186877615a34aa378f6626efcda5e4a58b300d3b7a5ce2d569dcae8cb0
whole_worktree_gofmt_output_hash: sha256:c0ca0c807a52aa33ec2f357d7acd1840c6a21ae52b6c8bb5a7b36a42d721d8dc
diff_check_output_hash: sha256:4f174106ba6b8831957610c34cac11c6cb6b7ffa999f6fa5d2e0184dc1f8401f
validator_command: gentle-ai sdd-verify-validate --input <exact-candidate-bytes> --requirements 18 --scenarios 48
```

### Completeness

| Metric | Result |
|---|---:|
| Planning artifacts inspected | proposal, 7 delta specs, design, tasks, apply-progress, prior verify report |
| Implementation tasks | 48 / 48 complete; 0 pending |
| Requirements retrieved and compliant | 18 / 18 |
| Scenarios retrieved and runtime-compliant | 48 / 48 |
| Candidate implementation delta | unchanged from post-remediation candidate |

### Build, Tests, and Coverage

| Command | Exit | Output SHA-256 | Result |
|---|---:|---|---|
| `go test -count=1 ./...` | 0 | `41f6fafa0c7f73e49ac4aa1e7825e28976e4a74f051667bf23b53a02dea9a58a` | 22 test-bearing packages passed; 5 have no test files |
| `go test -count=1 ./internal/tui -run '^(TestInjectedStep8RunnerIsReachableFromWizardAfterPreAuth|TestPreAuthContinuationRequiresSavedProfile|TestStepThreeAndFourRemainPreAuthWithInjectedStep8Runner|TestStep8Action)'` | 0 | `237af5d4e244a12b7c5d93ab40fcfa8e65100eaa67d4bea07185a1e31e57bfe2` | production reachability, pre-auth boundary, and lifecycle coverage passed |
| `go vet ./...` | 0 | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` | passed |
| `go build ./...` | 0 | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` | passed |
| `go test -count=1 -cover ./internal/tui` | 0 | `bdef6438d6873b691500e8c59dc764addfd2b1b6be8e24846a7e848885b1d77f` | 82.0% statement coverage |
| `go test -race ./...` | 2 | `efbaaa186877615a34aa378f6626efcda5e4a58b300d3b7a5ce2d569dcae8cb0` | unavailable because `CGO_ENABLED=0` |
| `gofmt -d internal/tui/mapepire_onboarding_step.go internal/tui/model_step8_runner_test.go` | 0 | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` | remediation files formatted |
| whole-worktree check-only `gofmt -d` | 1 | `c0ca0c807a52aa33ec2f357d7acd1840c6a21ae52b6c8bb5a7b36a42d721d8dc` | unrelated CRLF-only formatting deltas |
| `git diff --check` | 0 | `4f174106ba6b8831957610c34cac11c6cb6b7ffa999f6fa5d2e0184dc1f8401f` | passed; CRLF warnings only |

All runtime evidence used deterministic in-process fakes or local loopback. No IBM i, enterprise network, credentials/keyring, SSH/Java process, artifact transfer, shell, unrestricted SQL, or external effect was used.

### Requirements and Scenario Compliance

| Delta requirement | Scenarios | Runtime evidence | Status |
|---|---:|---|---|
| Nexus configuration: readiness and Step 8 orchestration | 4 | configuration Step 8, pre-auth, and marker tests | ✅ COMPLIANT |
| Nexus configuration: offline status | 1 | readiness tests and truthful documentation | ✅ COMPLIANT |
| WSS: trusted authenticated session | 5 | WSS loopback and Step 8 service/factory tests | ✅ COMPLIANT |
| WSS: text framing and bounds | 1 | WSS and typed protocol tests | ✅ COMPLIANT |
| Onboarding: Step 8 proof and ownership | 9 | configuration/TUI lifecycle and focused navigation tests | ✅ COMPLIANT |
| Onboarding: truthful readiness and marker | 2 | marker, configuration, and TUI tests | ✅ COMPLIANT |
| Onboarding: production configure composition | 3 | `cmd/nexus` composition and real wizard reachability test | ✅ COMPLIANT |
| SSH single: trust, consent, framing | 4 | gate, SSH trust, SSH stdio, and runtime tests | ✅ COMPLIANT |
| SSH single: verified detection | 1 | SSH stdio tests | ✅ COMPLIANT |
| Application protocol: fixed proof | 4 | typed protocol, WSS, and SSH proof tests | ✅ COMPLIANT |
| Application protocol: correlated session | 1 | typed protocol bounds tests | ✅ COMPLIANT |
| Application protocol: cancellation | 1 | typed protocol and TUI tests | ✅ COMPLIANT |
| SSH runtime: managed boundary | 4 | runtime/proof cleanup tests | ✅ COMPLIANT |
| SSH runtime: artifact/upload bounds | 1 | runtime artifact tests | ✅ COMPLIANT |
| Security: native secret boundary | 4 | credential/provider and Step 8 tests | ✅ COMPLIANT |
| Security: independent trust/audit | 2 | SSH trust and auditor tests | ✅ COMPLIANT |
| Security: controlled mutation surface | 1 | Step 3/4 and runtime boundary tests | ✅ COMPLIANT |
| Security: marker privacy | 1 | marker and audit tests | ✅ COMPLIANT |

**Compliance summary**: 48 / 48 scenarios compliant at runtime.

The real `cmd/nexus configure` composition supplies the production runner to the TUI. A matching saved-profile Step 4 pre-auth result reaches `screenProfileStep8Action` without runner invocation; explicit Step 8 Enter invokes the injected runner once. An unmatched profile remains at Step 4 with zero invocations.

### Correctness

| Dimension | Result | Notes |
|---|---|---|
| Spec compliance | ✅ | All 18 requirements and 48 scenarios have passing runtime coverage. |
| Task completion | ✅ | Native status reports 48 / 48 complete. |
| Production reachability remediation | ✅ | Focused current TUI test passed through the real saved-profile action path. |
| Offline constraint | ✅ | No prohibited external boundary was executed. |

### Design Coherence

| Design decision | Result | Notes |
|---|---|---|
| `tui -> configuration contract <- cmd/nexus` | ✅ | TUI owns only the runner seam; command root composes it. |
| WSS first; five eligible SSH fallback classes | ✅ | Terminal and unknown cases remain fail-closed. |
| Independent TLS/SSH trust and ordered gates | ✅ | Policy, trust, consent, credential ordering is covered. |
| Fixed bounded `VALUES 1` proof | ✅ | No generic SQL or result surface. |
| Cleanup, zeroization, bounded audit, non-readiness marker | ✅ | Runtime and audit tests cover the boundary. |
| Explicit operator-reachable Step 8 action | ✅ | Current focused reachability test passed. |

### TDD Compliance

| Check | Result | Details |
|---|---|---|
| TDD evidence reported | ✅ | apply-progress contains RED/GREEN/triangulation evidence for the 48 completed implementation tasks. |
| RED confirmed | ✅ | Reported test-bearing task evidence exists in the current test suite. |
| GREEN confirmed | ✅ | Focused reachability matrix and full test suite passed on this attempt. |
| Triangulation adequate | ✅ | Boundary, terminal, lifecycle, and action-path variants are represented. |
| Safety net | ✅ | Package and whole-suite execution passed; documentation-only evidence is structurally read back. |

**TDD compliance**: 5 / 5 checks passed.

### Test Layer Distribution

| Layer | Result | Evidence |
|---|---|---|
| Unit | ✅ | deterministic package tests for policy, protocol, configuration, profile, credential, audit, and TUI state transitions |
| Integration | ✅ | local TLS/WSS loopback and Bubble Tea `Update`/`View` composition tests |
| E2E | ➖ | no live IBM i or external environment is permitted |

### Changed File Coverage

| File scope | Line coverage | Branch coverage | Rating |
|---|---:|---:|---|
| `internal/tui` package containing remediation files | 82.0% | unavailable in Go command output | ⚠️ Acceptable |

### Assertion Quality

**Assertion quality**: ✅ Current reachability tests assert saved-profile gating, navigation, zero pre-action calls, one explicit invocation, and unmatched rejection; no trivial assertion finding was identified.

### Issues

**CRITICAL (0)** None.

**WARNING (2)**
1. Race verification is unavailable: `go test -race ./...` exits 2 because `CGO_ENABLED=0`. No race-clean claim is made.
2. Whole-worktree check-only `gofmt -d` detects unrelated CRLF-only deltas. The remediation files are formatted and no file was modified.

**SUGGESTION (0)** None.

### Pre-write Admission

`gentle-ai sdd-verify-validate --input C:\Users\David\AppData\Local\Temp\opencode\mapepire-dual-transport-onboarding-verify-candidate.md --requirements 18 --scenarios 48` must admit these exact candidate bytes before this report is persisted to OpenSpec or Engram.

### Verdict

**PASS WITH WARNINGS** — all 18 requirements and 48 scenarios have passing offline runtime coverage. Archive is recommended after native settlement admits this exact report; `not_validated_on_ibmi` remains required.
