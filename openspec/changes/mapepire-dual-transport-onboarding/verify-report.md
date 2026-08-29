```yaml
schema: gentle-ai.verify-result/v1
evidence_revision: sha256:3efb00ca3bcf7eec52bbd741c0298fb1cec9e65a74968a63c046e8bcc5e0515e
verdict: pass_with_warnings
blockers: 0
critical_findings: 0
requirements: 18/18
scenarios: 48/48
test_command: go test -count=1 ./...
test_exit_code: 0
test_output_hash: sha256:d38268fa3d5b0c6770c2eac5fcf4ff5db13a8b0e525aae4447dc648d95b88e02
build_command: go build ./...
build_exit_code: 0
build_output_hash: sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
```

## Verification Report

**Change**: `mapepire-dual-transport-onboarding`
**Mode**: Strict TDD; hybrid OpenSpec + Engram; offline-only
**Native attempt**: `sdd-verify-binding-correction` — `sha256:39d8f251612e012730dca15e11def1f5d4acfe471a7165d685fa95387e98c0d7`
**Verdict**: **PASS WITH WARNINGS**

### Canonical Verification Evidence

```yaml
schema: gentle-ai.verification-evidence/v1
attempt_token: sha256:39d8f251612e012730dca15e11def1f5d4acfe471a7165d685fa95387e98c0d7
request_id: sdd-verify-binding-correction-20260829
candidate_identity: sha256:ed18242f6d19acec67a0aeb48960106ee85b8057fd6249ec75514f48d2447c10
candidate_tree: d8029afddaf9ec444a47b85c8590bc37555475a2
requirements: 18/18
scenarios: 48/48
test_command: go test -count=1 ./...
test_exit_code: 0
test_output_hash: sha256:d38268fa3d5b0c6770c2eac5fcf4ff5db13a8b0e525aae4447dc648d95b88e02
build_command: go build ./...
build_exit_code: 0
build_output_hash: sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
focused_output_hash: sha256:a04ff32f55ce2760c686e061ca86f2d3f1e5a98061bd0c19017aa6a5f4cfddee
coverage_output_hash: sha256:80211de91b651dc5a88a0c5e20cb68b3e7015757fd21a52cadc5318d08467a4e
race_output_hash: sha256:efbaaa186877615a34aa378f6626efcda5e4a58b300d3b7a5ce2d569dcae8cb0
diff_check_output_hash: sha256:fba534980de1f0570ddd0852c9ce84d050e72ded4c6f78539e6a05a137beb39d
validator_command: gentle-ai sdd-verify-validate --input <exact-candidate-bytes> --requirements 18 --scenarios 48
```

### Completeness

| Metric | Result |
|---|---:|
| Planning artifacts inspected | proposal, 7 specs, design, tasks, apply-progress, prior report |
| Implementation tasks | 48 / 48 complete; 0 pending |
| Requirements and scenarios | 18 / 18; 48 / 48 |

### Build, Tests, and Coverage

| Command | Exit | Output SHA-256 | Result |
|---|---:|---|---|
| `go test -count=1 ./...` | 0 | `d38268fa3d5b0c6770c2eac5fcf4ff5db13a8b0e525aae4447dc648d95b88e02` | 22 packages passed; 5 no-test packages |
| `go build ./...`; `go vet ./...` | 0; 0 | empty | passed |
| focused Step 8 TUI matrix | 0 | `a04ff32f55ce2760c686e061ca86f2d3f1e5a98061bd0c19017aa6a5f4cfddee` | reachability and lifecycle passed |
| `go test -count=1 -cover ./internal/tui` | 0 | `80211de91b651dc5a88a0c5e20cb68b3e7015757fd21a52cadc5318d08467a4e` | 82.0% statements |
| `go test -race ./...` | 1 | `efbaaa186877615a34aa378f6626efcda5e4a58b300d3b7a5ce2d569dcae8cb0` | CGO disabled |
| `gofmt -d` remediation; `git diff --check` | 0; 0 | empty; `fba534980de1f0570ddd0852c9ce84d050e72ded4c6f78539e6a05a137beb39d` | formatted; CRLF warnings only |

No IBM i, enterprise network, credentials/keyring, SSH/Java process, artifact transfer, arbitrary shell, unrestricted SQL, or external effect was used.

### Requirements and Scenario Compliance

| Requirement group | Scenarios | Status |
|---|---:|---|
| Nexus configuration | 5 | ✅ COMPLIANT |
| Trusted WSS and framing | 6 | ✅ COMPLIANT |
| Step 8 ownership, marker, and composition | 14 | ✅ COMPLIANT |
| SSH trust, consent, and detection | 5 | ✅ COMPLIANT |
| Typed proof, session, and cancellation | 6 | ✅ COMPLIANT |
| SSH runtime lifecycle and artifact bounds | 5 | ✅ COMPLIANT |
| Secret, audit, mutation, and marker security | 7 | ✅ COMPLIANT |

**Compliance summary**: 48 / 48 scenarios compliant at runtime.

### Correctness

| Dimension | Result |
|---|---|
| Spec compliance, task completion, offline constraint | ✅ |
| Production reachability | ✅ saved-profile Step 4 reaches Step 8; Enter invokes the runner |

### Design Coherence

| Decision | Result |
|---|---|
| TUI contract and command composition | ✅ |
| WSS first, typed fallback, fixed proof, bounded cleanup | ✅ |
| Responsive request lifecycle and no-I/O View | ✅ |

### TDD Compliance

| Check | Result | Details |
|---|---|---|
| Evidence, RED, GREEN, triangulation, safety net | ✅ | apply-progress covers 48/48 tasks; focused and full suites pass |

**TDD compliance**: 5 / 5 checks passed.

### Test Layer Distribution

| Layer | Result |
|---|---|
| Unit | deterministic Go tests |
| Integration | local loopback and Bubble Tea `Update`/`View` |
| E2E | ➖ not permitted |

### Changed File Coverage

`internal/tui` remediation package: 82.0% statements; branch coverage unavailable; ⚠️ Acceptable.

### Assertion Quality

**Assertion quality**: ✅ Reachability and lifecycle assertions exercise production behavior; no tautology, ghost loop, or assertion without behavior was found.

### Quality Metrics

**Linter**: ✅ `go vet ./...` passed. **Type Checker**: ✅ `go build ./...` passed.

### Issues

**CRITICAL (0)** None.

**WARNING (2)**
1. Race verification is unavailable: `go test -race ./...` exits 1 because `CGO_ENABLED=0`. No race-clean claim is made.
2. Unrelated CRLF-only formatting deltas remain in `.atl/` and `internal/credential/keyring_store.go`; `git diff --check` passes and no unrelated path was modified.

**SUGGESTION (0)** None.

### Pre-write Admission

`gentle-ai sdd-verify-validate --input - --requirements 18 --scenarios 48` admitted these exact candidate bytes before persistence.

### Verdict

**PASS WITH WARNINGS** — all 18 requirements and 48 scenarios have passing offline runtime coverage. `not_validated_on_ibmi` remains required.
