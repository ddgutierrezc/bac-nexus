```yaml
schema: gentle-ai.verify-result/v1
evidence_revision: sha256:34150b0df3ba4972ff4830654be86731e2fafccb8542db04e078a8a0e2924e4e
verdict: fail
blockers: 3
critical_findings: 3
requirements: 10/15
scenarios: 29/37
test_command: go test -count=1 ./...
test_exit_code: 0
test_output_hash: sha256:a32485f2daff27d3a4ae374445ca9ecd48c2e32eb3af71b5f554ce2bf95da110
build_command: go build -o C:\\Users\\David\\AppData\\Local\\Temp\\opencode\\catalogspike-verify.exe ./cmd/catalogspike
build_exit_code: 0
build_output_hash: sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
```

## Verification Report

**Change**: `mapepire-dual-transport-onboarding`  
**Evidence range**: `a9eba0d..9147261` (six committed slices)  
**Mode**: Strict TDD, hybrid OpenSpec + Engram, offline-only

### Completeness

| Metric | Value |
|---|---:|
| Planning artifacts read | proposal, design, tasks, cumulative apply-progress, 7 specs |
| Tasks total / complete / incomplete | 13 / 13 / 0 |
| Requirements / scenarios retrieved | 15 / 37 |
| Requirements / scenarios compliant | 10 / 15; 29 / 37 |

### Fresh Command Evidence

| Command | Exit | Result |
|---|---:|---|
| `go test -count=1 ./internal/profile ./internal/security` | 0 | 2 packages; hash `a9643401f4ce04ca9bdd35e1d24f1d02df4dba0519db9738538cac901b743293` |
| `go test -count=1 ./internal/mapepire` | 0 | 1 package; hash `b6a08a2117e53c4608faf9b8fdbb752e9d5a12453b1a1c208b46533ef21c8a53` |
| `go test -count=1 ./internal/mapepire/wss` | 0 | 1 package; hash `bdc1a69104ab1209790d7849aba6989180d323b56cfd0c6d966d74c953e17f6b` |
| `go test -count=1 ./internal/mapepire/sshstdio ./internal/mapepire ./internal/connectors/ibmi/mapepirestdio ./internal/remote` | 0 | 4 packages; hash `d551f318450a17238a9a4c6c25777a458fbd0cbf0dcaa3234847bd5e2579db2e` |
| `go test -count=1 ./internal/configuration ./internal/audit ./internal/security` | 0 | 3 packages; hash `7859f8fdcab8d00ff19bb528fc8f329b78fdf55382964573b145cdd4ecfaab19` |
| `go test -count=1 ./internal/tui` | 0 | 1 package; hash `41849d2944e16d0e66f6184a0e815f41cfcb8405ca89c6be9c37568622c4a27a` |
| `go test -count=1 ./...` | 0 | 23 tested packages, 3 `[no test files]`; hash in envelope |
| `go vet ./...` | 0 | no output; SHA-256 empty output |
| `gofmt -d` over all 26 changed Go files | 0 | no output; SHA-256 empty output |
| `go build -o C:\\Users\\David\\AppData\\Local\\Temp\\opencode\\catalogspike-verify.exe ./cmd/catalogspike` | 0 | no output; binary removed |
| `git diff --check a9eba0d..HEAD` | 0 | no output; SHA-256 empty output |

All tests used deterministic in-process fakes or `httptest` loopback. No IBM i, corporate network, credentials, Java, artifact transfer, or remote SSH process was invoked.

### Requirements Traceability

| Spec requirement | Scenarios (runtime evidence) | Status |
|---|---|---|
| Configuration / Honest readiness | local refresh; cancellation; configured is not checked (`readiness_test.go`, `resolver_test.go`) | COMPLIANT |
| Configuration / Ephemeral observations | restart recomputes (`profile_test.go`) | COMPLIANT |
| WSS / Trusted bounded WSS | CA text framing; identity failure (`wss_test.go`) | COMPLIANT |
| WSS / Supported daemon probe | supported `/version`; unsupported fallback; no authorization | UNTESTED |
| Onboarding / Managed resolver policy | unavailable fallback; credential terminality; missing SSH trust (`resolver_test.go`) | PARTIAL |
| Onboarding / Step 3/4 truthful readiness | no overclaim; Step 8 proof; no Step-3 credentials (`mapepire_onboarding_step_test.go`, identity tests) | PARTIAL |
| Onboarding / Secret-free persistence and audit | conservative legacy migration (`profile_test.go`) | COMPLIANT |
| SSH single / Independent trust and framing | trusted availability fallback; unsafe identity (`sshstdio_test.go`, host identity tests) | COMPLIANT |
| SSH single / Verified protocol detection | matching success; reject missing/mismatched ID (`sshstdio_test.go`) | COMPLIANT |
| Protocol / Pinned typed subset | bounded valid; malformed/unknown rejected (`typed_protocol_test.go`) | COMPLIANT |
| Protocol / Correlated bounded session | out-of-order; violations; paging limits; official fixture framing (`typed_protocol_test.go`) | PARTIAL |
| Protocol / Cancellation semantics | cancellation closes session (`typed_protocol_test.go`) | COMPLIANT |
| SSH runtime / Verified artifact boundary | artifact safety; consented runtime; daemon independence (artifact/policy tests) | COMPLIANT |
| Security / Pinned host trust policy | TLS enrollment; SSH TOFU; TLS mismatch; SSH mismatch; independent unchanged identity (`profile`, `remote`, `wss` tests) | COMPLIANT |
| Security / Bounded audit surface | sanitized audit; no live IBM i (`transport_test.go`, offline suite) | PARTIAL |

**Scenario summary**: 29 COMPLIANT, 4 PARTIAL, 4 UNTESTED, 0 runtime failures.

### Correctness and Design Coherence

| Decision / requirement | Result | Evidence |
|---|---|---|
| Schema v2, conservative v1 migration, independent trust, ephemeral fields | Yes | `internal/profile/profile.go` and passing profile/security tests |
| Typed seven-operation client, CSPRNG IDs, correlation, release limits and cancellation | Yes | `protocol.go`, `typed_session.go`, passing `internal/mapepire` tests |
| WSS text framing, TLS/pin bounds and coder/websocket admission | Yes | `wss.go`, `wss_test.go`, `go.mod` v1.8.15; apply-progress records checksum/license provenance |
| Bounded SSH LF adapter and consent-gated fixed `--single` command | Yes | `sshstdio`, `mapepirestdio/policy.go`, focused tests |
| Concrete WSS-first `/version` probe and production resolver composition | No | `Resolver` accepts only an abstract `DaemonProbe`; no `/version` implementation or composition exists. `cmd/nexus/runConfigure` supplies only a host-identity inspector, while `NewModel` receives no Mapepire probe. |
| Step 3 daemon-TLS-first and Step 4 actual pre-auth ownership | No | Current Step 3 remains SSH host-key inspection; Step-4 tests inject a probe, but production leaves it nil. |
| Audit carries policy identity, trust outcome, fallback reason, protocol revision/version | No | `TransportEvent` contains only transport/reason/outcome/protocol; no policy identity or trust outcome is represented. |
| Official pinned protocol fixture at `2ef44166fcb515744fb922b49ed3673b2dac6b26` | No | Revision is planning-only; no implementation fixture or runtime covering test exists. |

### TDD Compliance

| Check | Result | Details |
|---|---|---|
| TDD evidence reported | PASS | cumulative `apply-progress.md` has RED/GREEN/triangulation evidence for all 13 tasks |
| Test files exist and pass | PASS | 13/13 task items map to present test files and the focused/full commands passed |
| RED historical outcome | PASS | reported failures cannot be independently replayed without source reversal; current files and green execution corroborate the evidence |
| Triangulation | WARNING | 12/13 adequate; official protocol-fixture scenario lacks a fixture test |
| Safety nets | PASS | task evidence records focused safety nets; new packages are new in the commit range |
| Assertion quality | PASS | inspected changed tests assert behavior, errors, framing, state, or side effects; no tautology, ghost loop, or production-free assertion found |

**TDD compliance**: 5/6 checks passed.

### Test Layer Distribution and Coverage

| Layer | Tests / files | Evidence |
|---|---|---|
| Unit | changed Go test files across profile, protocol, adapters, resolver, audit, TUI | deterministic fakes |
| Integration | WSS loopback tests in `internal/mapepire/wss/wss_test.go` | `httptest` TLS/WebSocket only |
| E2E | 0 | no browser or live IBM i test required or run |

Coverage analysis was not run: no cached coverage threshold/capability was supplied. `go vet` passed; Go compilation is the type check.

### Boundary, Secret, and Path Inspection

- `git diff --name-only a9eba0d..HEAD` contains zero paths under `.atl/`, `tmp/`, or `openspec/changes/mapepire-artifact-acquisition/`.
- `git diff --check` passed. Changed-source inspection found no new `ssh_exec`, `run_any_command`, `execute_sql`, or `exec.Command` surface.
- The current unrelated worktree changes remain only `.atl/` and `tmp/`; they were not read or edited.
- The committed diff contains no credential fixture or secret persistence evidence. Audit validation allowlists metadata and rejects sensitive reason text.

### Issues Found

**CRITICAL**
1. **WSS probe/resolver is not implemented or composed** — maps to `mapepire-wss-transport: Supported Daemon Probe` (3 scenarios) and `mapepire-transport-onboarding: Managed Resolver`. The passing resolver tests use an injected string-returning fake. There is no bounded TLS-trusted HTTP `/version` client, endpoint policy construction, or production call path; therefore the system cannot meet the daemon-first requirement.
2. **Wizard ownership is only test scaffolding** — maps to all `Step 3/4 Truthful Readiness` scenarios. `cmd/nexus/runConfigure` wires `remote.HostIdentityInspector` only; `NewModel` has no Mapepire probe, Step 3 remains SSH-only, and Step 8 is an uncomposed helper. Passing injected-fake TUI tests do not prove the required product flow.
3. **Pinned protocol-fixture and complete audit contracts are missing** — maps to `Fixture framing is conformant` and `Bounded Dual-Transport Audit Surface`. No fixture is pinned to revision `2ef44166...`; audit events omit required policy identity and trust outcome.

**WARNING**
- The tests are strong at adapter and state-machine seams but cannot compensate for missing production composition.
- Live IBM i validation remains intentionally not performed; status must remain `not_validated_on_ibmi`.

**SUGGESTION**
- Add changed-file coverage reporting once the project defines a threshold; this is informational only.

### Verdict

**FAIL** — all 13 tasks are checked and all mandated commands pass, but three substantive implementation gaps leave 5/15 requirements and 8/37 scenarios not fully compliant. The native verification attempt should record `fail` and must not settle/archive.

**next_recommended**: `resolve-blockers`
