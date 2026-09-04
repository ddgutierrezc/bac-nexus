# Tasks: Restore Componentized Profile Onboarding

## Review Workload Forecast

| Field | Value |
|---|---|
| Estimated changed lines | 2,490–2,820 total; every slice ≤800 |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 → PR 2 → PR 3 → PR 4 → PR 5 |
| Delivery strategy / chain | auto-chain / stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|---|---|---|---|---|---|
| 1 | Historical recovery (complete) | PR 1, ~298 | `go test -count=1 ./internal/tui` | Edit `Update/View`, 40x16 | Phase 1 primitives/tests |
| 2 | Controller/capture seam (complete) | PR 2, ~670 | `go test -count=1 ./internal/tui ./internal/remote` | four-step `Update/View` | Phase 2 TUI/keys |
| 3 | Finish partial lease, port, ordering | PR 3, 560–650 | `go test -count=1 ./internal/configuration ./internal/profile ./internal/tui ./cmd/nexus` | fake clock + ordered spies | five partial-attempt files |
| 4 | Lease safety and compensation | PR 4, 400–500 | `go test -count=1 ./internal/configuration ./internal/profile ./cmd/nexus` | fakes; N/A external | lifecycle/canary/compensation |
| 5 | Matrix, locale parity, documentation | PR 5, 560–700 | `go test -count=1 ./internal/tui` | 4×3×2×2 `Update/View` matrix | catalogs, frames, docs/spec |

## Phase 1: Historical Recovery Guard (PR 1)

- [x] 1.1 RED: `internal/tui/profile_screen_test.go` runtime Edit Save/Cancel, validation, result/discard, and overflow.
- [x] 1.2 GREEN: restore exact `d027210` wizard primitives; relocate `wizardOverflowIndicator` selectively.
- [x] 1.3 GREEN: reconcile affected TUI symbols; run `go test -count=1 ./...`.

## Phase 2: Four-Step Controller (PR 2)

- [x] 2.1 RED: TUI guards for four steps, Back, focus order, blocked actions, and feedback precedence.
- [x] 2.2 GREEN: four-step `internal/tui/{model,onboarding}.go` controller; default port, no retired states, stale rejection.
- [x] 2.3 RED then GREEN: fixed `tea.Exec` terminal rejection/cancel/EOF/retry and only the paired Step 3 ES/EN prompt/status keys.

## Phase 3: Split Secure Backend Lifecycle (PR 3 → PR 4)

- [x] 3.1 PR 3 RED: extend partial `internal/configuration/onboarding_test.go` for 1–1024 capture, opaque status, once-only consume, and Back revoke.
- [x] 3.2 PR 3 GREEN: finish `Capture/Revoke/StartCaptured` in `internal/configuration/onboarding.go` and `internal/tui/{onboarding,model}.go`; no secret in TUI/messages/process.
- [x] 3.3 PR 3 RED then GREEN: use configuration/profile/composition tests to prove non-default port through inspect/proof/identity/result/commit/JSON and `inspect → proof → audit → commit`.
- [x] 3.4 PR 3 evidence: focused suite plus `go test -race -count=1 ./internal/configuration ./internal/tui`; use Unit 3 rollback boundary.
- [x] 3.5 PR 4 RED: `internal/configuration/onboarding_test.go` fake-clock retry/replacement, expiry, edit/cancel/stale/shutdown revoke, worker-stop zeroization, and failed capture.
- [x] 3.6 PR 4 GREEN: complete `internal/configuration/onboarding.go` zeroization; canary audit/log/file/error/persistence/TUI/serialization/reflection exposure.
- [x] 3.7 PR 4 RED then GREEN: `internal/configuration/onboarding_test.go` and `cmd/nexus/main.go` cover audit failure, `CleanupRequired`, reverse compensation, retained journal.
- [x] 3.8 PR 4 evidence: focused configuration/profile/composition and race tests; use Unit 4 rollback boundary.

## Phase 4: Runtime Evidence and Docs (PR 5)

- [x] 4.1 RED then GREEN: extend `internal/tui/render_matrix_test.go` runtime `Update/View` coverage for 4 steps × 120x40/80x24/40x16 × ES/EN × color/NO_COLOR, navigation/focus, overflow, and error precedence.
- [x] 4.2 GREEN: complete all remaining locale-parity/catalog matrix work, localization tests, lossless wrapping, and `NO_COLOR` evidence; it excludes the minimal Step 3 prerequisite keys owned by task 2.3.
- [x] 4.3 GREEN: update `docs/IBM_I_PROFILE_WIZARD.md`, `DESIGN.md`, and `openspec/specs/nexus-configuration/spec.md`; run `go test -count=1 ./...`, `go test -race ./...`, `go vet ./...`, `go build ./...`, and `git diff --check`.
