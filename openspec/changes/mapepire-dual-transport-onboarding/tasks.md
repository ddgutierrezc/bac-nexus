# Tasks: Mapepire Dual-Transport Onboarding

## Review Workload Forecast
**4,200 lines**; High, auto-chain feature-branch-chain; five GREEN 7.3 units, tests/progress under 400.

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: feature-branch-chain
400-line budget risk: High

| Unit/dependency | Branch; estimate; focused test | Harness; acceptance; rollback; non-goals |
|---|---|---|
| 7.3.1 / 7.2 | `feature/step8-orchestrator-ssh`→`feature/step8-compose-marker-audit`; 280–330(≥70); `go test -count=1 ./internal/profile ./internal/audit` | temp stores; redaction/invalidation; rollback `internal/profile/step8_marker.go`,`internal/audit/step8.go`,tests,progress; no credentials/transport/TUI/root. |
| 7.3.2 / 7.3.1 | `feature/step8-compose-marker-audit`→`feature/step8-compose-security`; 300–350(≥50); `go test -count=1 ./internal/credential ./internal/security` | prompt+temp SSH trust; fail-closed isolation; rollback `internal/credential/prompt_provider.go`,`internal/security/ssh_trust.go`,tests,progress; no factories/TUI/root. |
| 7.3.3 / 7.3.1–7.3.2 | `feature/step8-compose-security`→`feature/step8-compose-factories`; 300–350(≥50); `go test -count=1 ./internal/configuration ./internal/mapepire/wss ./internal/remote` | TLS/WSS loopback+SSH/artifact counters; WSS/no-downgrade/cleanup; zero SSH/artifact/Java/upload; rollback `internal/configuration/step8_production.go`,tests,progress; no TUI/root. |
| 7.3.4 / 7.3.3 | `feature/step8-compose-factories`→`feature/step8-compose-tui-seam`; 180–240(≥160); `go test -count=1 ./internal/tui` | constructor; only `Step8Runner`, Steps 3–4 pre-auth; rollback seam/tests/progress; no action/loading/cancel/retry/stale/view/adapters/root. |
| 7.3.5 / 7.3.3–7.3.4 | `feature/step8-compose-tui-seam`→`feature/step8-compose`; 250–310(≥90); `go test -count=1 ./cmd/nexus ./internal/tui` | `runConfigure→runConfigureTUI`, local-profile/counting-runner; main wiring/presence-not-invocation; rollback root/tests/progress; no Phase-8 UI/live IBM i. |
| 8.1–8.3 / 7.3.5 | `feature/step8-compose`→`feature/step8-tui`; ≤360(≥40); `go test -count=1 ./internal/tui` | `Update/View` 120x40,80x24,40x16,NO_COLOR; lifecycle safety; rollback `internal/tui/step8*`,task/progress; no adapters. |
| 9.1–9.3 / 8.1–8.3 | `feature/step8-tui`→`feature/step8-audit`; ≤300(≥100); `go test -count=1 ./internal/audit ./internal/configuration ./internal/tui` | counting matrix; redaction/docs/`not_validated_on_ibmi`; rollback audit/docs/task/progress; no archive. |

- [x] 1.1
- [x] 1.2
- [x] 1.3
- [x] 1.4
- [x] 2.1
- [x] 2.2
- [x] 2.3
- [x] 2.4
- [x] 4.1
- [x] 4.2
- [x] 4.3
- [x] 5.1
- [x] 5.2
- [x] 5.3
- [x] 6.1
- [x] 6.2
- [x] 6.3
- [x] 6.4
- [x] 6.5
- [x] 6.6
- [x] 6.7
- [x] 6.8
- [x] 7.1
- [x] 7.2

## Phase 7 — Production composition
Eligible: `daemon_refused|daemon_unavailable|daemon_availability_timeout|daemon_policy_disabled|unsupported_version`; other/unknown terminal; helpers/catalogspike never prove composition.
- [x] 7.3.1 `internal/profile/step8_marker.go` + `internal/audit/step8.go`: add adapters; assert prohibited-field redaction and endpoint/policy/trust invalidation; retain bounded marker/allowlisted audit.
- [x] 7.3.2 `internal/credential/prompt_provider.go` + `internal/security/ssh_trust.go`: add adapters; assert denial=`credentials_unavailable`, mismatch blocks; no downgrade/remote/TLS-trust reuse.
- [x] 7.3.3 `internal/configuration/step8_production.go`: compose factories; assert `daemon_refused|daemon_unavailable|daemon_availability_timeout|daemon_policy_disabled|unsupported_version`→SSH, WSS proof zero SSH/artifact/Java/upload; unknown/terminal fail closed/no downgrade.
- [ ] 7.3.4 `internal/tui/model.go`: inject only `configuration.Step8Runner`; assert Steps 3–4 pre-auth/zero runner-credential-runtime calls; retain credential-free runtime-free state.
- [ ] 7.3.5 `cmd/nexus/main.go`: wire `runConfigure→runConfigureTUI`; assert constructor receives/no Step-8 invocation; retain Phase-8 action ownership/no live IBM i.

## Phase 8 — Bubble Tea lifecycle
- [ ] 8.1 `internal/tui/step8_action_test.go`: RED-test cancel/retry/stale IDs; cancellation terminal/cleaned, retry new ID, stale result cannot mutate state.
- [ ] 8.2 `internal/tui/step8_action.go`: implement `idle→loading→cancelled|failed|succeeded`; assert cancel closes once/post-navigation stale; retain sanitized secret-free state.
- [ ] 8.3 `internal/tui/step8_view.go`: render lifecycle 120x40,80x24,40x16,NO_COLOR; assert focus/retry/back reachable; retain responsive no-I/O/no-stale `View`.

## Phase 9 — Evidence
- [ ] 9.1 `internal/audit/step8_test.go`: test marker/audit prohibited fields+cleanup failure; retain bounded/allowlisted evidence, no failure readiness marker.
- [ ] 9.2 `docs/IBM_I_PROFILE_WIZARD.md`: document loopback/fakes; assert no live IBM i; report `not_validated_on_ibmi`.
- [ ] 9.3 `sdd-verify`: run acceptance matrix; assert offline `not_validated_on_ibmi`; no archive/live-IBM-i claim.

Exact checkbox totals: **35 total = 27 completed + 8 pending**. Tasks through 7.3.3 source/tests and hybrid task/progress evidence are complete; no native/VCS/.atl/tmp edits occurred.
