# Tasks: Mapepire Dual-Transport Onboarding

## Review Workload Forecast
1,260 lines; 120–260/unit; auto-chain.

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: feature-branch-chain
400-line budget risk: High

### Work Units
All: no IBM i; rollback named files.
| Unit | Branch/base; files; RED/test | Harness; rollback |
|---|---|---|
| S done | `feature/step8-compose`←`feature/step8-compose-tui-seam`; `internal/tui/model.go`,`model_step8_runner_test.go`; zero invocation; `go test -count=1 ./internal/tui` | counting TUI |
| P 160 | `feature/step8-preauth`←`feature/step8-compose`; `internal/configuration/step8_pre_auth{,_test}.go`; exact mapping; `go test -count=1 ./internal/configuration` | TLS `/version` loopback |
| W 190 | `feature/step8-wss-adapter`←P; `internal/configuration/step8_wss{,_test}.go`; profile binding; `go test -count=1 ./internal/configuration ./internal/mapepire/wss` | TLS-WSS loopback |
| C 170 | `feature/step8-credentials`←W; `internal/credential/step8_provider{,_test}.go`; mode/no-leak; `go test -count=1 ./internal/credential` | in-process fakes |
| A 140 | `feature/step8-ssh-policy`←C; `internal/security/step8_ssh_policy{,_test}.go`; deny; `go test -count=1 ./internal/security` | policy fake |
| T 170 | `feature/step8-ssh-trust`←A; `internal/security/step8_ssh_trust_adapter{,_test}.go`; observed≠enrolled; `go test -count=1 ./internal/security` | candidate fake |
| D 150 | `feature/step8-audit-adapter`←T; `internal/audit/step8_auditor{,_test}.go`; redaction; `go test -count=1 ./internal/audit` | recorder |
| R 230 | `feature/step8-compose-live`←D; `cmd/nexus/main.go`,`cmd/nexus/configure_test.go`; real compose/zero startup; `go test -count=1 ./cmd/nexus ./internal/configuration` | stores+loopback/counters |

## Completed foundation and composition
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
- [x] 7.3.1 Marker/audit completed.
- [x] 7.3.2 Prompt/trust completed.
- [x] 7.3.3 Factory/routing completed.
- [x] 7.3.4 TUI seam completed.
- [x] 7.3.5 Startup seam injects runner; zero invocation, empty dependencies, final wiring pending.
- [x] 7.3.6a RED: `daemon_refused|unavailable|availability_timeout|policy_disabled|unsupported` only; other/unknown terminal/no downgrade.
- [x] 7.3.6b GREEN: add credential-free, SSH-free `configuration/step8_pre_auth.go`.
- [x] 7.3.7a RED: profile endpoint/TLS binds WSS; auth failure terminal.
- [x] 7.3.7b GREEN: add profile-aware WSS/session adapter; fixed `VALUES 1` only.
- [x] 7.3.8a RED: prompt/keyring only; denial/unavailable/invalid/empty → unavailable, no leak.
- [x] 7.3.8b GREEN: add `credential/step8_provider.go` dispatcher.
- [x] 7.3.9a RED: `AllowSSH` admits approved policy only; otherwise fail closed.
- [x] 7.3.9b GREEN: add `security/step8_ssh_policy.go`.
- [ ] 7.3.10a RED: observed SSH fingerprint differs from enrollment; block and never reuse TLS evidence.
- [ ] 7.3.10b GREEN: add observed-fingerprint `security/step8_ssh_trust_adapter.go`.
- [ ] 7.3.11a RED: reject endpoint/host/user/path/error/SQL/rows/secrets from Step8 audit.
- [ ] 7.3.11b GREEN: add bounded `audit/step8_auditor.go` over Recorder.
- [ ] 7.3.12a RED: real configure needs all adapters; startup zero calls, WSS success zero SSH/artifact/Java/upload.
- [ ] 7.3.12b GREEN: fill `Step8ProductionDependencies` in `cmd/nexus/main.go`; wiring-only, action Phase 8.

## Phase 8 — Bubble Tea lifecycle
- [ ] 8.1 `internal/tui/step8_action_test.go`: RED cancel/retry/stale.
- [ ] 8.2 `internal/tui/step8_action.go`: lifecycle; sanitized cancellation.
- [ ] 8.3 `internal/tui/step8_view.go`: responsive no-I/O View.

## Phase 9 — Evidence
- [ ] 9.1 `internal/audit/step8_test.go`: prohibited fields/cleanup/no marker.
- [ ] 9.2 `docs/IBM_I_PROFILE_WIZARD.md`: loopback/fakes, no-live status.
- [ ] 9.3 `sdd-verify`: offline matrix only.

Exact checkbox totals: **49 total = 37 completed + 12 pending**. No generic SQL/shell/download/retry or protected-artifact changes.
