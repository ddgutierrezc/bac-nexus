# Design: Nexus Configuration TUI

## Technical Approach

Add `nexus configure` as an optional adapter over `internal/configuration`; it never imports `internal/mcp`, reuses `cmd/catalogspike` flags, or owns MCP stdio. Extract catalog-spike orchestration while preserving its commands. Use the observed v1 Charm family (`bubbletea v1.3.10`, `bubbles v1.0.0`, `lipgloss v1.1.0`, `github.com/charmbracelet/...`); mixing `charm.land/.../v2` is prohibited. Admission precedes imports; denial leaves services usable without the TUI.

```text
cmd/nexus ──→ internal/tui ──→ internal/configuration ──→ profile/credential/remote
cmd/catalogspike ────────────────────────┘                    │
internal/mcp ──→ internal/app (separate)          audit/security (read-only)
```

## Architecture Decisions

| Decision | Choice and rationale | Rejected tradeoff |
|---|---|---|
| UI structure | Small shell router plus child models: list, detail, form, confirmation, progress, result/error; stack-based back navigation, centralized keymap/help, `WindowSizeMsg`, 80x24 and narrow single-column layouts, explicit no-color renderer. Only the active child updates. | Monolithic model couples operations and focus; nested programs break lifecycle. |
| Operations | Child models emit secret-free intents; an operation controller runs cancellable, timed services and returns typed IDs/classes only. Progress disables destructive focus; completion focuses Back/Done, errors focus Retry/Back. Escape cancels or pops; quit requires no pending mutation. | Raw goroutines/messages permit stale results and sensitive payloads. |
| Secrets | `SecretInput.WithSecret(ctx, profile, func([]byte) error) Outcome` owns terminal entry and zeroization outside model/message/view/clipboard. Set/rotate/migrate callbacks invoke native storage and return presence/outcome only. Go/string/native APIs may copy memory, so zeroization is best-effort, not a guarantee. | Password Bubbles retain values in model state. |
| Persistence | Platform atomic-replace adapter, not ad-hoc command logic. | In-place writes and rename chains expose truncation/gaps. |

## Contracts and Data Flow

| Service | Bounded contract |
|---|---|
| `Profiles` | `List(limit≤128)`, `Create`, `Read`, `Update`, `Delete`; typed validation/conflict/not-found/recovery outcomes. |
| `Credentials` | `Status`, `Set`, `Rotate`, `Delete`, `MigrateLegacy`; opaque independent outcomes. |
| `Trust` | manual verified enrollment; explicit `remote.InspectHostKey` TOFU inspection with deadline/cancel, exact fingerprint confirmation, immutable provenance; mismatch=`host_key_changed`, no discovery/rotation. |
| `Readiness` | local-only typed checks; separately warned remote diagnostic preserving `ready_for_controlled_ibmi_validation` and `not_validated_on_ibmi`. It reports nil recovery/resolver/acquirer/lease in current `nexus serve` composition without repairing it. |
| `IntegrationPreview` | Configuration services own generic secret-free semantic invocation only. Each dedicated `internal/integrationpreview/<client>` adapter—not TUI, configuration core, MCP, or profile—accepts it, validates the client's versioned schema/format, owns quoting/path rules, and renders preview/copy only; never external files. Unknown/unsupported versions fail closed without plausible snippets. |

Writes validate first; canonicalize the configured root; reject linked/non-regular root, target, backup, or temp; keep all files in-root. Create uses exclusive publication. Update writes/fsyncs a 0600 temp and restorable backup, atomically replaces, verifies, and restores backup on failure. Delete first creates/fsyncs a backup, then removes; credential deletion needs a second exact confirmation and its failure restores the profile while reporting both outcomes. Directory-fsync limits and Windows replacement semantics are implementation gates; every crash point leaves old/new valid or triggers deterministic recovery.

Audit extends closed operation/result enums only for lifecycle/trust/diagnostic classes; events contain no identifiers or raw errors. Policy and in-memory status remain immutable/read-only; no history or sink configuration.

## File Changes

| Path | Action |
|---|---|
| `internal/configuration/*` | Create services, semantic invocation, outcomes, readiness, clipboard port. |
| `internal/integrationpreview/*` | Create versioned client format adapters and schema validation. |
| `internal/profile/profile.go` | Modify bounded list/update/backup/restore and path defenses. |
| `internal/tui/*` | Create shell, screens, keymaps, renderer, controller. |
| `cmd/nexus/main.go`, `cmd/catalogspike/main.go` | Modify dispatch/composition; preserve `serve`, setup flags, and automation. |
| `internal/credential/*`, `internal/audit/*`, `.github/workflows/*` | Modify status/transient adapters, closed audit classes, admission/platform evidence. |

## Testing Strategy

RED-first tests use `t.TempDir`, fault injection, crash-window/property cases, symlink races, bounds, exact confirmations, cancellation/timeouts, mismatch, and byte-sentinel assertions across model/messages/views/errors/audit/clipboard. Test `Update` directly; use `teatest` only for navigation; goldens only for stable 80x24/narrow/no-color views. GHA supplies platform/race evidence because WDAC is not bypassed; dependency admission runs before TUI imports.

## Threat Matrix

| Boundary | Applicability / reason |
|---|---|
| Documentation-like paths | N/A — no executable classification. |
| Git repository selection | N/A — no VCS selection. |
| Commit state | N/A — no commits. |
| Push state | N/A — no pushes. |
| PR commands | N/A — delivery strategy does not automate PR commands. |

Fixed macOS credential and clipboard process adapters remain applicable process boundaries: fixed executable/arguments, stdin for secrets, no shell/environment, bounded output, cancellation, and RED tests.

## Rollout, Compatibility, and Gates

Stacked-to-main slices, each under 1,000 changed lines: (1) service extraction; (2) profile recovery CRUD; (3) credential/trust services; (4) TUI shell+CRUD; (5) security flows; (6) readiness/preview/platform evidence. Each slice preserves CLI/profile JSON/legacy vault and can revert independently; `catalogspike setup`, automation, and `nexus serve` remain valid. No automatic migration occurs.

Gates: Charm admission; Windows atomic-replace/durability proof; fixed clipboard adapter approval; terminal resize/no-color evidence. Non-goals: MCP hosting, serve-gap repair, policy editing, persistent audit, external config writes, arbitrary remote execution, automatic trust/rotation, or live-validation claims.
