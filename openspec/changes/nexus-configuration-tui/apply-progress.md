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
