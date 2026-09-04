# Tasks: Enable Production Nexus Serve

## Review Workload Forecast

| Field | Value |
|---|---|
| Remaining Phase 6.1 implementation units | Maximum 3, each ≤800 lines |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | 6.1c → 6.1d → 6.1e; then 6.2, 7.1, and 7.2 |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

No size exception: every remaining Phase 6.1 implementation unit is bounded to ≤800 lines.

### Suggested Work Units

| Unit | Goal | PR | Focused test command | Runtime harness | Rollback boundary |
|---|---|---|---|---|---|
| 1 | Protect onboarding | PR 1 | `go test -count=1 ./internal/tui/...` | matrix | baseline tests |
| 2 | Eligibility | PR 2 | `go test -count=1 ./internal/profile/...` | fake journal/store | eligibility files |
| 3 | Path evidence | PR 3 | `go test -count=1 ./internal/audit/... ./internal/ownership/...` | platform fakes | platform adapters |
| 4–5 | Audit write/recovery | PR 4–5 | `go test -count=1 ./internal/audit/...` | temp faults | audit sink |
| 6–8 | Adapters/composition | PR 6–8 | `go test -count=1 ./cmd/nexus ./internal/{remote,source,mcp}/...` | loopback | current slice |
| 9–10 | Diagnostic/release | PR 9–10 | `go test -count=1 ./...` | gate compile; no live | diagnostic/docs |

## Phase 1: Protected Baseline (PR 1, ≤500 lines)
- [x] 1.1 RED: lock four-step Create and metadata-only Edit in `internal/tui/{onboarding,render_matrix}_test.go`.
- [x] 1.2 GREEN: prove Serve does not alter TUI visuals; preserve `.atl`, stash, screenshot branch, archives, dirty candidate.

## Phase 2: Eligibility (PR 2, ≤780 lines)
- [x] 2.1 RED: cover owner-only schema/store, bindings, stale/missing/keyring rejection, and legacy-ineligible migration in `internal/profile/eligibility_test.go`.
- [x] 2.2 GREEN: create `internal/profile/eligibility.go` V3 target/policy/pin/artifact/proof loader/store.
- [x] 2.3 RED→GREEN→REFACTOR: extend `internal/profile/onboarding_commit{,_test}.go` for journal ordering, compensation, revocation, recovery, rollback.

## Phase 3: Local State (PR 3, ≤760 lines)
- [x] 3.1 RED: cover symlink/reparse, owner, mode/DACL, device/volume, race, and unsupported evidence in `internal/{audit,ownership/sqlite}/platform_*_test.go`.
- [x] 3.2 GREEN: implement handle-walking `SecurePathPlatform`: Unix `0700/0600` local FS; Windows local-volume/current-user ACL.

## Phase 4: Durable Audit (PR 4–5, ≤800 each)
- [x] 4.1 RED→GREEN: create `internal/audit/file{,_test}.go`: seven fields, retention, limits, lock/write/sync, poison on short/zero/write/sync/rotation failure.
- [x] 4.2 RED→GREEN→REFACTOR: repair only newest torn tail with sync; poison old tail/corruption/unknown file; test UTC rotation/retention.

## Phase 5: Fixed Remote Adapters (PR 6–7, ≤800 each)
- [x] 5.1 RED→GREEN: issue only an immutable Mapepire 2.3.6 artifact receipt after activation from the fixed official URL plus remote SHA verification; bind it to the authenticated files/client capability, host, safe path, SHA, and policy revision; through the receipt-owned admission boundary rehash immediately before its fixed-start seam/session creation; render only private approved IBM i environment constants, fixed Java, `-jar`, receipt path, and `--single`; reject all receipt mismatches, resource leaks, and cancellation leaks.
- [x] 5.2 RED→GREEN: adapt request-scoped SSH/SFTP fixed `CPYTOSTMF` in `internal/remote/ssh{,_test}.go`; enforce 4 MiB, cancellation, cleanup, sanitized errors, no generic capability.

## Phase 6: MCP Lifecycle (PR 8, ≤800 lines)
- [x] 6.1a RED→GREEN: admit strict operator retention and independently derived current V3/keyring eligibility before any local-state, recovery, server-construction, or remote boundary.
- [x] 6.1b RED→GREEN: open durable audit then ownership local state, run injected recovery before server construction, and close ownership then audit on every later outcome.
- [x] 6.1c RED→GREEN: make the default ownership opener return a fully configured bounded exact-record recovery coordinator using fresh profile loading, native-keyring credentials, and a narrow SSH cleanup remote.
- [x] 6.1d RED→GREEN: implement an authenticated bounded Mapepire Catalogados executor and production resolver factory with request-scoped remote cleanup and deterministic sanitized failures.
- [x] 6.1e RED→GREEN: compose the production source acquirer, ownership ledger, process lease store, resolver, recovery coordinator, durable auditor, and MCP server in `nexus serve`; prove the complete factory graph with fakes and zero remote contact on rejection.
- [x] 6.2 RED→GREEN→REFACTOR: add `internal/mcp/*_test.go` for both tools, paging/EOF/stale cursor, no partial source, stdout-only/stderr, and shutdown races/order. Completed under maintainer-approved `size:exception`; the bounded local subprocess harness uses one fixed test-helper child with fixed argv/env, pipes, timeout, and kill+Wait cleanup.

### Phase 6.1 Slice Progress

- Independent serve eligibility consumer admission is complete: `runWithDeps` admits strict operator retention first, then loads the V3 profile, derives its expected binding with `profile.DeriveEligibilityBinding`, and invokes the configured `EligibilityStore.Check` seam before recovery, server construction, or run.
- The binding is derived from current controlled inputs; persisted eligibility is not used to derive the expected value. Missing, malformed, expired, and six binding-dimension mismatches fail before the next boundary.
- Phase 6 is complete: 6.1a–6.1e and 6.2 are complete; 7.1 and 7.2 remain pending in that order.
- Historical 6.1 completion note: durable local-state opening and recovery-before-server use the admitted V3 profile, recovery precedes factory construction, and ownership/audit close in reverse order. 6.2 adds all-lease eviction, one lifecycle-completion audit record before audit close, real-service MCP cursor/paging coverage, and a bounded local subprocess stdio proof. Final `sdd-verify` remains blocked until Phase 7 completes.
- Canonical evidence preimage (UTF-8, LF, final LF; hash excludes fences):
  ```text
  change=enable-production-nexus-serve
  task=6.1-durable-local-state-recovery-before-server
  focused=go test -count=1 ./cmd/nexus -run ^(TestRunWithDepsDurableLocalStateAndRecoveryBeforeServer|TestRunWithDepsCancellationAfterAuditOpenClosesWithoutRemoteWork)$: exit 0
  race=go test -race -count=1 ./...: exit 0
  full=go test -count=1 ./...: exit 0
  vet=go vet ./...: exit 0
  build=go build ./...: exit 0
  diff=git diff --check: exit 0
  ```
  Evidence hash: `sha256:7ed77d84c9aafaaaec1582d3e2da5a2bbeb40420c44f49eb5a4796609a4d7d99`.

## Phase 7: Diagnostics/Release (PR 9–10, ≤700 lines)
- [x] 7.1 RED→GREEN: add persistent sanitized diagnostic audit, append-failure override, timeout/cancel, no external-client mutation/live claim in `internal/configuration/readiness{,_test}.go`.
- [x] 7.2 Add build-tagged `integration/ibmi/serve_live_test.go`, `internal/release/*`, docs/workflow/manifest/runbook; retain `not_validated_on_ibmi`, rollback/handoff, and do not run the gate.
