# Tasks: Mapepire Dual-Transport Onboarding

## Review Workload Forecast
Production estimate: **2,070 lines**; overall risk **High**; every slice is strictly below 400 additions+deletions. Delivery: `auto-chain`, `feature-branch-chain`; no size exception. Planning has its own reviewable boundary.

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: feature-branch-chain
400-line budget risk: High

| Slice | Parent → child; ceiling; focused test | Deterministic harness; rollback |
|---|---|---|
| 1 Foundation | `fix/mapepire-dual-transport-verification` → `feature/step8-foundation`; 380; `go test -count=1 ./internal/profile ./internal/credential ./internal/configuration` | in-memory fakes; foundation contracts |
| 2 WSS | `feature/step8-foundation` → `feature/step8-wss`; 350; `go test -count=1 ./internal/configuration ./internal/mapepire ./internal/mapepire/wss` | loopback TLS/WSS; WSS adapter |
| 3 SSH | `feature/step8-wss` → `feature/step8-ssh`; 390; `go test -count=1 ./internal/configuration ./internal/remote ./internal/mapepire/sshstdio ./internal/connectors/ibmi/mapepirestdio` | counting SSH/process/artifact fakes; fallback |
| 4 Composition | `feature/step8-ssh` → `feature/step8-compose`; 330; `go test -count=1 ./cmd/nexus ./internal/configuration` | real configure + loopback; root wiring |
| 5 TUI | `feature/step8-compose` → `feature/step8-tui`; 380; `go test -count=1 ./internal/tui` | `Update/View` 120x40,80x24,40x16,NO_COLOR; TUI |
| 6 Final | `feature/step8-tui` → `feature/step8-audit`; 240; `go test -count=1 ./internal/audit ./internal/configuration ./internal/tui` | counting-fake matrix; audit/docs |

## Completed lower-layer history
- [x] 1.1 RED: schema-v2 persistence, conservative migration, secret-free fields, and independent TLS/SSH trust tests.
- [x] 1.2 GREEN: schema-v2 validation/migration and Nexus-owned resolver limits in `internal/profile` and `internal/configuration`.
- [x] 1.3 RED: typed operations, bounds, IDs, correlation, cursors, limits, and cancellation tests in `internal/mapepire`.
- [x] 1.4 GREEN: typed envelopes/session, one reader, bounded pending state, safe errors, cancellation, and `Execute` compatibility.
- [x] 2.1 RED: WSS dependency evidence and loopback text/TLS/TOFU/bounds/compression/identity tests.
- [x] 2.2 GREEN: bounded trusted WSS adapter using the approved `github.com/coder/websocket` dependency.
- [x] 2.3 RED: SSH LF framing, ID/success detection, EOF/cancel, host-key mismatch, and arbitrary-command rejection tests.
- [x] 2.4 GREEN: bounded SSH adapter and consent-gated fixed `--single` runtime seams.

Original 3.1–3.5 remain historical superseded/helper-only evidence; they are not production acceptance and must not claim Step 8 composition. `apply-progress.md` remains evidence; `verify-report.md` remains immutable FAIL history. Current remediation maps to Slice 2 (`configuration/daemon.go`, `mapepire/wss/http.go`), Slice 1 historical resolver evidence, Slice 5 (`tui/model.go`, `mapepire_onboarding_step.go`, `profile_identity_step.go`), and Slice 6 (`audit/{audit.go,transport_test.go}`, `mapepire/{testdata,typed_protocol_test.go}`). `.atl/` and `tmp/` stay outside every boundary.

## Production Step 8 (all pending; RED → GREEN → REFACTOR/VERIFY)

### Phase 4 — Foundation (non-goal: transport/runtime/TUI)
- [ ] 4.1 RED: exhaustively test `Decision`, reason, `ResultClass`, fail-closed unknowns, saved-profile gate, credential failures, marker invalidation, key `ibmi/<name>`, proof metadata/bounds (security/config/protocol scenarios).
- [ ] 4.2 GREEN: add application-owned Step8 contracts/service, exact mappings, v3 `prompt|keyring` migration, marker schema, credential derivation, fixed proof constants.
- [ ] 4.3 REFACTOR/VERIFY: narrow interfaces; in-memory acquire/settle fakes with unique IDs; prove no transport/TUI/secret exposure and stop before 400.

### Phase 5 — Authenticated WSS (non-goal: SSH fallback)
- [ ] 5.1 RED: test distinct `/version` pre-auth, credential-only `connect`, `VALUES 1`, TLS mismatch, cancellation/close, zero SSH/artifact calls (WSS scenarios).
- [ ] 5.2 GREEN: compose credential-aware typed WSS factory/session reusing existing trust/framing and fixed proof lifecycle.
- [ ] 5.3 REFACTOR/VERIFY: loopback TLS/WSS acquire/settle; test protocol/limit terminality and no SSH imports/calls; stop before 400.

### Phase 6 — Managed SSH Fallback (non-goal: generic primitives)
- [ ] 6.1 RED: test eligible-only policy/trust/consent-before-credential, unsafe artifact, Java/upload/launch failures, LIFO cleanup/cancel, arbitrary command rejection (SSH/fallback threat scenarios).
- [ ] 6.2 GREEN: invoke `remote.Dial`, verified artifact/Java/upload/fixed `--single`, authenticated proof, rollback, and typed terminal results.
- [ ] 6.3 REFACTOR/VERIFY: counting fakes acquire/settle every resource with unique IDs; prove no silent retry/downgrade and stop before 400.

### Phase 7 — Production Composition (non-goal: helper/catalogspike evidence)
- [ ] 7.1 RED: actual `runConfigure → runConfigureTUI → production constructor → Step8 runner → result`; prove saved profile and daemon zero SSH (composition scenarios).
- [ ] 7.2 GREEN: compose stores, policy, trust, credential, resolver, WSS/SSH factories, audit, marker, and service in `cmd/nexus`.
- [ ] 7.3 REFACTOR/VERIFY: real configure counting/loopback tests; reject helper/fake-only proof; rollback root wiring; stop before 400.

### Phase 8 — Bubble Tea Step 8 (non-goal: connector/security ownership)
- [ ] 8.1 RED: test saved-profile action, loading/cancel/failure/retry/success/back, child context, request IDs/stale results, exact sanitized copy, responsive/no-color views; Steps 3/4 zero runtime (UI scenarios).
- [ ] 8.2 GREEN: add presentation-only Step 8 lifecycle/action/model/view; keep secrets out of state/messages/view and preserve shared wizard primitives.
- [ ] 8.3 REFACTOR/VERIFY: runtime `Update/View` matrix at 120x40,80x24,40x16 and `NO_COLOR`; prove focus/reachability/feedback and stop before 400.

### Phase 9 — Audit, Docs, Integrated Evidence (non-goal: archive)
- [ ] 9.1 RED: reject sensitive audit/marker fields; test marker staleness, full terminal matrix, cleanup, cancellation, and `not_validated_on_ibmi` (security/offline scenarios).
- [ ] 9.2 GREEN: implement allowlisted audit, marker write/clear behavior, docs distinguishing current/proposed behavior, and complete deterministic evidence.
- [ ] 9.3 VERIFY: run `gofmt`, `go test -count=1 ./...`, `go vet ./...`, `go build ./...`, `git diff --check`, forbidden-path checks, and fresh independent `sdd-verify`; archive only after zero CRITICAL.

Actual checkbox totals: **26 total = 8 completed + 18 pending**. Every runtime-bearing slice uses acquire/settle and unique IDs. No staging, commit, push, PR, or automatic apply is authorized; first decision is planning review/native reset to Slice 1.
