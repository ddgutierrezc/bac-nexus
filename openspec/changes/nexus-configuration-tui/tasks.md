# Tasks: Nexus Configuration TUI

## Review Workload Forecast

| Slice | Estimate / risk | PR boundary / focused verification | Runtime harness / rollback |
|---|---:|---|---|
| 1 Services/admission | 700 / Low | Approved issue; `feat/config-services`; `go test -count=1 ./cmd/catalogspike ./internal/configuration` | GHA Windows/Linux build; revert admission/services only |
| 2 Profile recovery | 850 / Medium | Approved issue; `feat/profile-recovery`; `go test -count=1 ./internal/profile ./internal/configuration` | GHA Windows atomic tests; revert store/recovery only |
| 3 Credentials/trust | 900 / High | Approved issue; `feat/config-security-services`; `go test -count=1 ./internal/credential ./internal/configuration` | GHA platform tests; revert credential/trust only |
| 4 Shell/CRUD | 950 / High | Approved issue; `feat/config-tui-crud`; `go test -count=1 ./internal/tui ./cmd/nexus` | GHA Windows Terminal build; revert TUI/command only |
| 5 Security screens | 900 / High | Approved issue; `feat/config-tui-security`; `go test -count=1 ./internal/tui ./internal/configuration` | Teatest navigation in GHA; revert security screens only |
| 6 Readiness/preview | 850 / Medium | Approved issue; `feat/config-readiness-preview`; `go test -count=1 ./internal/configuration ./internal/integrationpreview/...` | GHA matrix/race; revert adapters/status only |

Total: ~5,150 changed lines. All PRs stack to `main` in order (PR *n* base: merged PR *n-1*); every PR records RED → GREEN → REFACTOR evidence, linked `status:approved` issue, one `type:*` label, behavior docs, and task-checkbox updates.

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
1000-line budget risk: High

First autonomous slice: Slice 1, which remains service-only if Charm admission fails closed.

## Phase 1: Service Extraction and Dependency Admission

- [x] 1.1 RED: add `cmd/catalogspike` compatibility and `internal/configuration` contract tests; extract reusable profile/credential/remote orchestration without changing setup or `serve`.
- [x] 1.2 GREEN: record exact v1 Charm versions, licenses, vulnerability scan, supported builds and Windows Terminal feasibility in `.github/workflows/`/docs before any Charm import; denial/unavailability blocks TUI imports.
- [x] 1.3 REFACTOR: wire service-only composition, document fallback and stdio isolation, then run the Slice 1 command and GHA harness.

## Phase 2: Profile Recovery CRUD

- [x] 2.1 RED: add bounded List≤128/Create/Read/Update/Delete tests for validation, conflicts, sanitized outcomes, symlink/non-regular/in-root defenses, crash points, backup restore, and Windows atomic replacement.
- [x] 2.2 GREEN/REFACTOR: implement platform atomic replace/fsync/recovery and exact two-stage delete semantics; update behavior docs and run Slice 2 evidence.

## Phase 3: Credential and Trust Services

- [x] 3.1 RED: prove Status/Set/Rotate/Delete/Migrate opaque outcomes, 1–4096 bounds, explicit readback migration, fixed `/usr/bin/security` macOS Keychain stdin transport, deterministic Windows/Linux failures, and a separate secret-free clipboard preview/copy adapter that never receives credential material.
- [x] 3.2 GREEN/REFACTOR: implement transient `SecretInput`, native status/migration, manual verified and warned timed/cancellable TOFU enrollment, and fail-closed `host_key_changed`; document and run Slice 3 evidence.

## Phase 4: TUI Shell and Profile CRUD

- [ ] 4.1 RED: Bubble Tea `Update`/teatest navigation tests cover list empty state, detail/form/back/quit, safe focus, resize, 80x24/narrow/no-color rendering, and no MCP stdio lifecycle.
- [ ] 4.2 GREEN/REFACTOR: add `nexus configure` shell/router/controller and CRUD screens using admitted Charm v1 only; preserve CLI/`catalogspike` behavior, update docs, run Slice 4 evidence.

## Phase 5: TUI Security Flows

- [ ] 5.1 RED: test exact destructive confirmations, cancellation/timeouts, progress focus, credential status-only messages, and sentinel-secret absence from models/messages/views/snapshots/clipboard/previews.
- [ ] 5.2 GREEN/REFACTOR: add credential, migration, trust and TOFU screens with typed outcomes only; update docs and run Slice 5 evidence.

## Phase 6: Readiness, Preview, and Platform Evidence

- [ ] 6.1 RED: test offline local readiness exposes the `nexus serve` composition gap; warned remote diagnostics cancel/timeout, sanitize/audit, and retain both non-validation statuses.
- [ ] 6.2 GREEN/REFACTOR: add read-only status and versioned `internal/integrationpreview/<client>` schema adapters; unknown versions fail closed, copy never writes files; GHA validates Windows atomic/credential builds and `go test -race ./...` without WDAC bypass.
