# Apply Progress: Nexus Configuration TUI

## Slice 1: Service Extraction and Dependency Admission

### Tasks Complete

- [x] 1.1 RED: add `cmd/catalogspike` compatibility and `internal/configuration` contract tests; extract reusable profile/credential/remote orchestration without changing setup or `serve`.
- [x] 1.2 GREEN: record exact v1 Charm versions, licenses, vulnerability scan, supported builds and Windows Terminal feasibility in `.github/workflows/`/docs before any Charm import; denial/unavailability blocks TUI imports.
- [x] 1.3 REFACTOR: wire service-only composition, document fallback and stdio isolation, then run the Slice 1 command and GHA harness.

### Slice 1 Facts

| Field | Value |
|---|---|
| Tasks complete | 1.1, 1.2, 1.3 (3 of 13 total tasks complete) |
| PR head / merge | PR #66 squash-merged as commit `c7bb0bb` on `main` |
| Issue | #65 (`status:approved` + `type:feature`) closed by PR merge |
| Branch | `feat/config-services` (deleted after squash-merge) |
| Changed lines | 4 files, 777 insertions, 222 deletions, 999 total (under the 1000-line budget) |
| Files changed | `internal/configuration/service.go`, `internal/configuration/service_test.go`, `cmd/catalogspike/main.go`, `.github/workflows/charm-dependency-gate.yml` |
| PR label | `type:feature` |
| Commits on the branch | `9019567`, `2edd76b`, `aa52243`, `3a85735`, `7b93331`, `a235f62`, `b894d27`, `4984fae` |
| Charm admission | passed on macos-latest, ubuntu-latest, windows-latest |
| Charm versions | `github.com/charmbracelet/bubbletea` v1.3.10, `github.com/charmbracelet/bubbles` v1.0.0, `github.com/charmbracelet/lipgloss` v1.1.0 |
| Charm licenses | MIT (verified by `head -n 1 LICENSE` in each module's directory) |
| Charm module integrity | `go mod verify` passed |
| Charm vulnerability scan | `govulncheck@v1.7.0` clean |
| Charm supported targets | windows/amd64, windows/arm64, darwin/amd64, darwin/arm64, linux/amd64, linux/arm64 (compiled without CGO) |
| Windows Terminal feasibility | confirmed on `windows-latest` runner |
| GHA admission gate run | `32536811502` (pass) |
| GHA go-verification run (PR head) | `32536811427` (pass) |
| GHA go-verification run (post-merge main) | `32537167712` (pass) |
| Native settlement evidence | `sha256:934b7c3cd669190a60d933d4ac4efc66364600fbb8b4c07d5db5fecd01f3ad97` |
| Post-merge GHA status | green on `main` |
| Local `main` synchronized | yes (fast-forwarded to `c7bb0bb`) |
| Fallback contract | if Charm admission is denied in a future slice, `internal/configuration` remains usable for `cmd/catalogspike` without TUI screens |
| Stdio isolation | every I/O surface (Output, Notices, ReadLine, ReadSecret) supplied through `Dependencies`; the Service never calls `os.Stdin`, `os.Stdout`, or `os.Stderr` |
| Product status | `ready_for_controlled_ibmi_validation` |
| IBM i validation | `not_validated_on_ibmi` |

### Runtime Evidence Policy

Windows Defender Application Control (WDAC) blocks local Go test
binaries on the developer host. Runtime evidence for Slice 1 is
GitHub Actions only. No local Go test execution, no local
`go test -count=1`, and no local binary build is used as
evidence. Every check recorded above was captured by the
authoritative GHA workflow on the exact PR head and on the
exact post-merge `main` head.

### Slice 1 Non-Goals Preserved

- No TUI screens, Bubble Tea imports, or `nexus configure` subcommand (Slice 4).
- No profile `List` / `Update` / `Delete` / backup recovery (Slice 2).
- No credential or trust services (Slice 3).
- No readiness diagnostics and no `internal/integrationpreview` adapters (Slice 6).
- No modification of `nexus serve` composition.
- No live IBM i validation.
- No persistent audit / sink / history.

### Slice 1 SDD Artifacts Committed by the Corrective PR

- `openspec/changes/nexus-configuration-tui/exploration.md`
- `openspec/changes/nexus-configuration-tui/proposal.md`
- `openspec/changes/nexus-configuration-tui/specs/local-mcp-security/spec.md`
- `openspec/changes/nexus-configuration-tui/specs/nexus-configuration/spec.md`
- `openspec/changes/nexus-configuration-tui/design.md`
- `openspec/changes/nexus-configuration-tui/tasks.md`
- `openspec/changes/nexus-configuration-tui/apply-progress.md` (this file)

### Next Routing

- `sdd-apply Slice 2` (profile recovery CRUD) is the next native step.
- Final `sdd-verify` and `sdd-archive` remain blocked until Slice 6 completes and a maintainer authorizes them.
- No RDD review, no IBM i contact, and no live IBM i validation in this session.

## Slice 2A: Profile Store List and Atomic Update Primitives

### Completed Delivery

- Slice 2A delivered the bounded deterministic List and atomic Update primitives; its remaining recovery/delete work was intentionally deferred to Slice 2B below.

| Field | Value |
|---|---|
| Work unit | `slice2a-profile-store-crud` |
| WIP preservation | Local branch `safety/profile-recovery-wip`, commit `55ed60b` (`chore(recovery): preserve interrupted profile work`) |
| Review issue | #70 (`status:approved` + `type:feature`) |
| Review PR | #71, `feat/profile-store-crud`, stacked to `main`, squash-merged |
| Slice commits | `c48b1d8`, `ed9839a`, `b64614d`, `1e49d66`, `4ebe123` |
| Authored changed lines | 425 (`.github/workflows/go-verification.yml`, `internal/profile/recovery.go`, `internal/profile/recovery_test.go`) |
| Scope | `Store.List` bounded to 128 and deterministic; `Store.Update` validates first, backs up, atomically replaces, and restores on failure; no configuration integration or delete coordinator |
| Exact-head | `4ebe123f5f609819e5d3dff176dc214729e1c18` |
| GHA | Run `32542384232`: Ubuntu `verify` PASS; Windows `profile-windows` PASS |
| Local runtime | Not executed; WDAC forbids local Go runtime claims |
| Native settlement | Slice 2A settled exactly once as passed under the maintainer-authorized clean 425-line accounting; no size exception |
| IBM i | No contact; `not_validated_on_ibmi` preserved |

### Work Unit Evidence

| Evidence | Result |
|---|---|
| Focused test command | GHA `go test -count=1 ./internal/profile` PASS on Windows; full `go test -count=1 ./...` PASS on Ubuntu |
| Runtime harness | GHA run `32542384232`; Windows atomic-store path and Ubuntu full verification PASS; no local runtime |
| Rollback boundary | Revert PR #71 / commits `c48b1d8..4ebe123`; preserved WIP remains independently recoverable at `55ed60b` |

### Next Routing

- Slice 2A final merge facts were reconciled from Engram and the pre-settlement working-tree metadata before Slice 2B implementation.
- Next recommended step: Slice 3 later; do not start it in this delivery.

## Slice 2B: Configuration Profile Recovery and Delete Coordination

### Tasks Complete

- [x] 2.1 RED: configuration/profile recovery/delete contract coverage.
- [x] 2.2 GREEN/REFACTOR: safe recovery/delete implementation and orchestration.

### Delivery Facts

| Field | Value |
|---|---|
| Tasks complete | 1.1, 1.2, 1.3, 2.1, 2.2 (5 of 13 total tasks complete) |
| Work unit | `slice2b-profile-recovery-delete` |
| Branch | `feat/profile-recovery-delete` |
| Approved issue | #69 (`status:approved` + `type:feature`) |
| PR | #72, stacked to `main`, exactly one `type:feature` label |
| Implementation commits | `6f82514`, `ce5b9b1` |
| PR head | `ce5b9b1b883f2c3288dc9c869b02143b7a17fc1f` |
| Changed-line budget | 476 authored changed lines on the PR snapshot; under 1000; no exception |
| Pre-merge GHA | Run `32549594146`: Ubuntu full verification and Windows profile verification PASS |
| Admission GHA | Run `32549594143`: macOS, Ubuntu, and Windows admission evidence PASS |
| Squash merge | PR #72 merged as `a29e009de62c02737996070d1429ed80c5712969`; issue #69 closed |
| Post-merge GHA | Run `32549710447`: Ubuntu full verification and Windows profile verification PASS |
| Local runtime | Not executed; WDAC requires GHA-only runtime evidence |
| Safety WIP | Preserved intact and unpushed at `safety/profile-recovery-wip@55ed60b`; no information was removed |
| IBM i | No contact; `not_validated_on_ibmi` preserved |
| Native settlement | Token `sha256:d15935433421f2901fecb794049ddaf3a61bc81582f2005d439f246b6c090d0f` settled exactly once as passed after authoritative evidence |

### Work Unit Evidence

| Evidence | Result |
|---|---|
| Focused test command and exact result | GHA run `32549594146`, Ubuntu `go test -count=1 ./...`: PASS; configuration/profile packages PASS within full suite |
| Runtime harness command/scenario and exact result | GHA run `32549594146`, Windows `go test -count=1 ./internal/profile`: PASS; no local runtime claim due WDAC |
| Rollback boundary | Revert PR #72 / squash merge `a29e009`; remove only Slice 2B profile recovery and configuration coordination while preserving Slice 2A and safety WIP |

### TDD Cycle Evidence

| Task | RED | GREEN | REFACTOR |
|---|---|---|---|
| 2.1 | GHA exact-head contract tests admitted the new recovery/delete scenarios | GHA full suite and Windows profile suite PASS | Tests use bounded temp roots, typed outcomes, cancellation, and leakage assertions |
| 2.2 | Contract failures were addressed before final implementation evidence | GHA full suite, vet, formatting, and Windows profile evidence PASS | Narrow `ProfilesStore` recovery seam; no TUI, MCP, Slice 3 lifecycle, or IBM i contact |

## Slice 3: Credential Lifecycle and Explicit Host Trust

### Tasks Complete

- [x] 3.1 RED: credential lifecycle/status, bounded transient input, migration, platform transport, deterministic failure, and secret-free clipboard contracts.
- [x] 3.2 GREEN/REFACTOR: native status/lifecycle orchestration, manual verified enrollment, warned cancellable TOFU inspection/enrollment, provenance persistence, and fail-closed host-key mismatch.

### Delivery Facts

| Field | Value |
|---|---|
| Tasks complete | 1.1, 1.2, 1.3, 2.1, 2.2, 3.1, 3.2 (7 of 13 total tasks complete) |
| Work unit | `slice3-credential-trust-services` |
| Approved issue | #73 (`status:approved` + `type:feature`) |
| Pull request | #74, `feat/config-security-services`, stacked to `main` |
| Implementation commits | `25ca9a1`, `97097f8`, `69afb6e` |
| Last validated Slice 3 code/evidence head at artifact-correction start | `87906ed576981e03862e06c132d8837beef2d2a9` |
| Validated PR snapshot at correction start | 498 additions / 3 deletions; under 1000; no exception |
| PR label | `type:feature` |
| GHA verify at correction start | Run `32551316744`, Ubuntu `verify` PASS and Windows `profile-windows` PASS |
| GHA admission at correction start | Run `32551316724`, macOS/Ubuntu/Windows admission PASS |
| GHA keyring gate at correction start | Run `32551316689`, macOS/Ubuntu/Windows keyring evaluation PASS |
| Local runtime | Not executed; WDAC blocks local Go test binaries |
| IBM i | No contact; `not_validated_on_ibmi` preserved |
| Safety WIP | Preserved intact and unpushed at `safety/profile-recovery-wip@55ed60b` |

The head above is the last validated Slice 3 code/evidence head when this
artifact correction began. The artifact-only correction commit necessarily
follows that head; this record does not claim that the baseline remains the
current PR head. The correction changes only SDD evidence and routing.

### TDD Cycle Evidence

| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|---|
| 3.1 | `internal/configuration/security_test.go`, existing `internal/credential/keyring_store_red_test.go` | Unit | N/A for new configuration contract; existing credential tests covered by GHA | ✅ Written before production implementation; local execution prohibited by WDAC | ✅ Baseline GHA run `32551316744` at validated head `87906ed576981e03862e06c132d8837beef2d2a9` — `go test -count=1 ./...` PASS | ✅ Present/absent/unavailable, set/rotate/delete, migration confirmation, and bounded opaque outcomes | ✅ Secret input zeroization boundary and no credential material in outcomes |
| 3.2 | `internal/configuration/security_test.go` | Unit/in-process contract | Existing profile/remote suites passed in GHA | ✅ Written before service implementation | ✅ Baseline GHA run `32551316744` at validated head `87906ed576981e03862e06c132d8837beef2d2a9` — `go test -count=1 ./...` PASS | ✅ Manual verified enrollment, warned TOFU inspection, exact confirmation, and mismatch | ✅ Atomic profile update seam, persisted non-secret provenance, no TUI/MCP/serve changes |

### Work Unit Evidence

| Evidence | Result |
|---|---|
| Focused test command and exact result | Baseline GHA run `32551316744` at validated head `87906ed576981e03862e06c132d8837beef2d2a9`: `go test -count=1 ./...` PASS; configuration, credential, profile, and remote packages passed |
| Runtime harness command/scenario and exact result | Baseline GHA exact-head matrix: Ubuntu `verify` PASS (`https://github.com/ddgutierrezc/bac-nexus/actions/runs/32551316744`), Windows `profile-windows` PASS; admission PASS on macOS/Ubuntu/Windows (`https://github.com/ddgutierrezc/bac-nexus/actions/runs/32551316724`); keyring gate PASS on macOS/Ubuntu/Windows (`https://github.com/ddgutierrezc/bac-nexus/actions/runs/32551316689`); no IBM i contact |
| Rollback boundary | Revert PR #74 / commits `25ca9a1..69afb6e`; removes only Slice 3 credential/trust services, native status classification, and host-key provenance/mismatch additions while preserving Slices 1–2 and the safety WIP |

### Scope and Security Notes

- Credential lifecycle results are opaque; transient input is bounded to 1–4096 bytes and zeroized best-effort after use.
- Native status does not return credential bytes; unavailable stores fail closed without vault or remote fallback.
- Manual enrollment requires verified provenance; TOFU inspection requires warning plus exact confirmation and persists only non-secret fingerprint/trust/provenance through atomic profile update.
- Inspection never persists trust, and changed fingerprints return `host_key_changed` before remote discovery.
- No TUI screens, Bubble Tea rendering, MCP wiring, `nexus serve` composition, arbitrary SSH/SQL/shell, or live IBM i validation were added.

### Next Routing

- Parent settles the supplied native attempt using the exact post-correction PR-head GHA evidence and decides the approved merge lifecycle for PR #74.
- Only after settlement/merge is complete: start Slice 4 under a new explicit apply request.
- Slice 4 is not started by this correction; no TUI screens or production behavior are changed.

## Slice 4: TUI Shell and Profile CRUD

### Tasks Complete

- [x] 4.1 RED: Bubble Tea `Update` navigation tests cover empty and populated list states, detail/form/back behavior, delete confirmation, resize, narrow layout, no-color rendering, and configure separation from MCP stdio.
- [x] 4.2 GREEN/REFACTOR: add the optional `nexus configure` shell/router and profile CRUD screens using the admitted Charm v1 family; preserve CLI, catalogspike, and `nexus serve` behavior.

### Delivery Facts

| Field | Value |
|---|---|
| Tasks complete | 1.1, 1.2, 1.3, 2.1, 2.2, 3.1, 3.2, 4.1, 4.2 (9 of 13 total tasks complete) |
| Work unit | `slice4-shell-crud` |
| Approved issue | #75 (`status:approved` + exactly one `type:feature` label) |
| Pull request | #76, `feat/config-tui-crud`, stacked to `main`, merged as `e1df3eff6f62584d23c30a8cbbc8e283f7cd3b1e` |
| Last validated Slice 4 code/evidence baseline at correction start | `915e1d3c9af3f6ff3a8369096243cc444654640d` |
| Baseline GitHub line accounting | 583 additions / 3 deletions; 586 changed lines, under the 1000-line budget and without exception |
| Baseline GHA Go Verification | Run `32552676370` — PASS |
| Baseline GHA Charm admission | Run `32552676360` — PASS on macOS, Ubuntu, and Windows |
| Baseline GHA keyring gate | Run `32552676372` — PASS on macOS, Ubuntu, and Windows |
| Post-merge GHA Go Verification | Run `32553268802` — PASS on merged main `e1df3eff6f62584d23c30a8cbbc8e283f7cd3b1e` |
| Local runtime | Not executed; WDAC blocks local Go test binaries |
| IBM i | No contact; `not_validated_on_ibmi` preserved |
| Safety WIP | Preserved unchanged and unpushed at `safety/profile-recovery-wip@55ed60b73e4a5b612750c9b362d8485991191edb` |

The baseline above is the last validated Slice 4 code/evidence head when this
artifact-only correction began. This correction follows that baseline and
changes only this SDD progress artifact; it does not change production
behavior, tests, dependencies, or Slice 4 implementation facts.

### TDD Cycle Evidence

| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|---|
| 4.1 | `internal/tui/model_test.go` | Unit / Bubble Tea `Update` | N/A for new file | Written before production | Baseline GHA run `32552676370` — PASS | Empty state, detail/back/delete, resize/narrow/no-color | Formatting and reload-after-mutation behavior preserved |
| 4.2 | `cmd/nexus/configure_test.go`, `internal/tui/model_test.go` | Unit/integration adapter | Existing `cmd/nexus` behavior covered by baseline GHA | Written before production | Baseline GHA run `32552676370` — PASS | Configure isolation plus CRUD/navigation paths | Isolated runner seam and typed store seam preserved |

### Work Unit Evidence

| Evidence | Result |
|---|---|
| Focused test command and exact result | Intended command: `go test -count=1 ./internal/tui ./cmd/nexus`; local execution prohibited by WDAC. Baseline GHA executed `go test -count=1 ./...` and passed in run `32552676370`, including both focused packages. |
| Runtime harness command/scenario and exact result | Baseline GHA `go test -count=1 ./...` passed the direct `Model.Update` navigation scenarios, configure adapter isolation, and existing serve/MCP tests; no IBM i or live terminal session was attempted. |
| Rollback boundary | Revert PR #76 / commits `7383146..915e1d3`; remove only Slice 4 TUI/command/dependency/docs/tests/task changes while preserving Slices 1–3 and the safety WIP. |

### Correction Facts

- This commit is artifact-only and follows baseline `915e1d3c9af3f6ff3a8369096243cc444654640d`.
- The corrected filesystem artifact now reconciles Slice 4 with the hybrid Engram progress and task records.
- No native attempt was acquired, settled, or reset by this correction.

### Next Routing

- Parent settlement/merge of PR #76 must happen first.
- Only after parent settlement and merge is complete may Slice 5 security screens begin under a new explicit apply request.
- Slice 6 readiness, diagnostics, and integration preview remain out of scope.

## Slice 5: TUI Security Flows

### Tasks Complete

- [x] 5.1 RED: exact confirmations, cancellation, timeout-bounded progress, status-only messages, and secret-isolation tests were written before production implementation.
- [x] 5.2 GREEN/REFACTOR: credential lifecycle, migration, manual trust, and warned TOFU screens use typed outcomes and a secret-free child-model boundary.

### Delivery Facts

| Field | Value |
|---|---|
| Tasks complete | 1.1–1.3, 2.1–2.2, 3.1–3.2, 4.1–4.2, 5.1–5.2 (11 of 13 total tasks complete) |
| Work unit | `slice5-security-screens` |
| Approved issue | #77 (`status:approved` + exactly one `type:feature` label) |
| Delivery | `feat/config-tui-security`, stacked-to-main after merged Slice 4 |
| Last validated code/evidence baseline before Slice 5 | `e1df3eff6f62584d23c30a8cbbc8e283f7cd3b1e` (Slice 4 merge) |
| Changed-line budget | Baseline snapshot: 514 additions / 3 deletions across 5 files; 517 authored changed lines, under 1000, no exception |
| Local runtime | Not executed; WDAC blocks local Go runtime/test/vet/build evidence |
| IBM i | No contact; `not_validated_on_ibmi` preserved |
| Safety WIP | Preserved unchanged and unpushed at `safety/profile-recovery-wip@55ed60b73e4a5b612750c9b362d8485991191edb` |

### TDD Cycle Evidence

| Task | Test file | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|---|
| 5.1 | `internal/tui/security_test.go` | Unit / Bubble Tea `Update` | Existing Slice 4 GHA baseline `32553268802` PASS | Written first; local execution prohibited | Baseline GHA `32553913784` PASS | Cancellation, status-only outcome, warning/confirmation, and sentinel absence cases | GHA baseline PASS; gofmt/diff check clean |
| 5.2 | `internal/tui/security_test.go`, `internal/tui/model.go` | Unit / child-model integration | Existing Slice 4 GHA baseline `32553268802` PASS | Written first; local execution prohibited | Baseline GHA `32553913784` PASS | Credential, migration, manual trust, TOFU, progress, and responsive views | GHA baseline PASS; gofmt/diff check clean |

### Work Unit Evidence

| Evidence | Result |
|---|---|
| Focused test command | Intended `go test -count=1 ./internal/tui ./internal/configuration`; local execution prohibited. Exact-head baseline GHA Go Verification `32553913784` passed repository verification, including the focused TUI/configuration behavior. |
| Runtime harness command/scenario | Exact-head baseline GHA `32553913784` PASS; direct Bubble Tea Update security navigation, typed outcomes, cancellation, and no-secret assertions executed. Charm admission `32553913803` PASS on macOS, Ubuntu, and Windows; no IBM i contact. |
| Rollback boundary | Revert Slice 5 code/test commit `e7a6004` and the Slice 5 task/apply-progress publication only; preserve merged Slices 1–4 and the safety WIP. |

### Scope Controls

- No Slice 6 readiness, diagnostics, integration preview, or platform evidence.
- No arbitrary SSH/SQL/shell, external-client writes, `nexus serve` composition changes, MCP lifecycle changes, or live IBM i validation.
- The committed baseline wording identifies the last validated code/evidence baseline; later artifact publication head is reported externally rather than mislabeled as that baseline.
