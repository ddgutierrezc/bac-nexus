# Proposal: Nexus Configuration TUI

## Intent

Give local developers/operators a safe `nexus configure` workflow for approved IBM i profiles. Today configuration is split between `catalogspike` orchestration and `nexus serve`, with no reusable application layer, lifecycle UI, or honest readiness view. The outcome is complete profile/credential administration without weakening automation, stdio isolation, or validation claims.

## Scope

### In Scope
- Optional Bubble Tea-family TUI: list, create, inspect, update, and delete profiles. Updates atomically replace files after backup and restore on failure; deletion requires exact confirmation, retains a restorable profile backup, and separately asks whether to delete the profile-owned native credential.
- Credential status/presence, set, rotate, delete, and explicit legacy-vault migration. Secret bytes never enter model state, files, audit, logs, MCP, argv/environment, previews, or output.
- Explicit host-key inspection/enrollment: operator-triggered remote inspection is labeled unverified TOFU, requires exact confirmation, pins the fingerprint, and later mismatch fails closed; independently verified manual entry remains available.
- Automatic local readiness plus separately initiated, warned, cancellable, timed remote diagnostics with sanitized results/audit. Readiness exposes missing production recovery/resolver/acquirer/lease composition and preserves `ready_for_controlled_ibmi_validation` / `not_validated_on_ibmi`.
- Schema-validated MCP command/snippet preview and copy only; rollout as stacked-to-main slices within 1,000 changed lines each.

### Out of Scope
- Starting MCP inside the TUI; external config writes; policy editing; persistent audit/sinks/retention; arbitrary remote execution; live-validation claims; fixing `nexus serve` composition.
- Java/Mapepire/JAR as current requirements; they may appear only as labeled legacy diagnostics.

## Product Rules and Architecture

Extract reusable configuration application services from `cmd/catalogspike`; keep CLI automation compatible and TUI, `cmd/nexus`, `internal/mcp`, security policy, and in-memory sanitized audit as separate adapters/boundaries. Profile deletion never implies credential deletion; partial delete/update failures restore the profile backup and report credential outcome independently.

## Capabilities

### New Capabilities
- `nexus-configuration`: CRUD, trust, credentials, readiness/diagnostics, and integration previews.

### Modified Capabilities
- `local-mcp-security`: extend secret-isolation, trust-enrollment, and sanitized-audit requirements to configuration flows.

## Rollout and Affected Areas

| Slice / area | Change |
|---|---|
| `internal/configuration`, `cmd/catalogspike` | Extract services; preserve CLI behavior |
| `cmd/nexus`, `internal/tui` | Add `configure`, then bounded profile/security flows |
| readiness/integration adapters | Add local status, explicit remote actions, validated previews |

## Risks, Gates, and Rollback

Secret retention, false trust/readiness, platform terminal variance, and CLI drift fail closed and require contract/navigation/no-secret tests. Bubble Tea/Bubbles/Lip Gloss version, license, and security approval is a design gate—not approved; fallback is a service-complete non-TUI adapter. Roll back each slice independently, restore profile backups, retain credentials unless explicitly deleted, and keep existing CLI/`serve` paths.

## Success Criteria

- [ ] CRUD, rollback, credential ownership, TOFU mismatch, cancellation/timeout, and no-secret-output contracts pass deterministically.
- [ ] Local readiness never contacts IBM i; remote diagnostics require explicit warned action and never claim live validation.
- [ ] Existing automation remains compatible; previews write no client files; fixed policy/audit remain read-only.

## Residual Decision

Before design, approve exact Charm dependencies and supported terminal/accessibility baseline, or select the fallback adapter.
