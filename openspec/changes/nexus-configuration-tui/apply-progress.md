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
| Changed-line budget | 393 authored lines on the PR head; under 1000; no exception |
| Focused GHA | Run `32549337316`: `go test -count=1 ./...`, `go vet ./...`, gofmt, and build checks PASS after one unrelated SQLite contention rerun |
| Windows GHA | Run `32549337316`: `go test -count=1 ./internal/profile` and profile vet PASS |
| Admission GHA | Run `32549337311`: macOS, Ubuntu, and Windows admission evidence PASS |
| Local runtime | Not executed; WDAC requires GHA-only runtime evidence |
| Safety WIP | Preserved intact and unpushed at `safety/profile-recovery-wip@55ed60b`; no information was removed |
| IBM i | No contact; `not_validated_on_ibmi` preserved |

### Work Unit Evidence

| Evidence | Result |
|---|---|
| Focused test command and exact result | GHA run `32549337316`, Ubuntu `go test -count=1 ./...`: PASS; configuration/profile packages PASS within full suite |
| Runtime harness command/scenario and exact result | GHA run `32549337316`, Windows `go test -count=1 ./internal/profile`: PASS; no local runtime claim due WDAC |
| Rollback boundary | Revert `6f82514..ce5b9b1` / PR #72 to remove only Slice 2B profile recovery and configuration coordination; preserve Slice 2A and safety WIP |

### TDD Cycle Evidence

| Task | RED | GREEN | REFACTOR |
|---|---|---|---|
| 2.1 | GHA exact-head contract tests admitted the new recovery/delete scenarios | GHA full suite and Windows profile suite PASS | Tests use bounded temp roots, typed outcomes, cancellation, and leakage assertions |
| 2.2 | Contract failures were addressed before final implementation evidence | GHA full suite, vet, formatting, and Windows profile evidence PASS | Narrow `ProfilesStore` recovery seam; no TUI, MCP, Slice 3 lifecycle, or IBM i contact |
