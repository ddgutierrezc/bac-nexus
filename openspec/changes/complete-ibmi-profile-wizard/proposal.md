# Proposal: Complete IBM i Profile Wizard

## Intent

Complete the fresh-profile journey, which collects metadata but cannot save and finish. Keep configuration, persistence, and optional proof distinct. Delivery is one exception-approved PR with coherent work-unit commits.

## Product Outcome

The canonical V1 journey becomes eight steps: **1 Profile, 2 Connection, 3 Server Identity, 4 Mapepire Readiness, 5 Credentials, 6 Review & Save, 7 Optional Remote Proof, 8 Completion**. Removing user-facing Java renumbers former Steps 6–9; no phantom step remains.

## Scope

### In Scope
- Preserve endpoint/transport-bound TOFU: explicit confirmation, no silent acceptance, and blocked rotation mismatch.
- Offer `prompt` and secure-keyring modes without secrets in model state, profiles, or SQLite; unavailable secure storage fails closed.
- Validate and atomically save V3 non-secret metadata at Step 6; saving completes profile creation.
- Make Step 7 optional: require WSS consent, then separate SSH-fallback consent after an eligible sanitized failure; support cancellation and reject stale results.
- Make Step 8 distinguish saved/local configuration from proof omitted, cancelled, failed, or successful. Use `ready for controlled validation` only when justified; never claim IBM i or `nexus serve` readiness.
- Preserve responsive, keyboard-only, localized, `NO_COLOR`, secret-safe behavior.

### Out of Scope / Non-Goals
- User-facing Java configuration/diagnostics; managed SSH fallback retains internal policy ownership.
- Real Mapepire/IBM i field validation, MCP serve repair, external-client mutation, arbitrary remote execution, or new trust sources.

## Capabilities

### New Capabilities
None.

### Modified Capabilities
- `nexus-configuration`: replace the conceptual nine-step flow with the eight-step lifecycle.
- `local-mcp-security`: specify explicit prompt versus keyring handling and independent WSS/SSH-fallback consent.

## Approach and Affected Areas

| Area | Impact |
|---|---|
| `internal/tui/` | Compose Steps 5–8 with existing primitives. |
| `internal/configuration/`, `internal/credential/`, `internal/profile/` | Enforce consent, secret isolation, atomic creation. |
| `cmd/nexus/main.go` | Inject services; keep remote logic outside TUI. |
| `docs/IBM_I_PROFILE_WIZARD.md`, `openspec/specs/*` | Align journey/security contracts. |

## Compatibility, Risks, and Rollback

Existing V3 profiles, CRUD, and `nexus serve` remain compatible; no migration is introduced. Risks are consent bypass, misleading completion, secret leakage, and review load. Mitigate through fail-closed services, explicit states, secret-safety checks, and work-unit commits. Roll back composition and deltas together; saved V3 profiles remain valid.

## Success Criteria

- [ ] A fresh profile completes all eight steps and is saved exactly once before optional proof.
- [ ] Each remote branch requires explicit consent and supports cancellation, timeout, retry, and sanitized failure.
- [ ] Prompt/keyring modes expose no secret, and unavailable keyring causes no fallback or remote attempt.
- [ ] Completion renders each proof outcome truthfully without IBM i or serve-readiness claims.
- [ ] Existing profiles and CLI behavior remain compatible across supported terminal sizes and `NO_COLOR`.
