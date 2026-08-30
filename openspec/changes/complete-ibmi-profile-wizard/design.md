# Design: Complete IBM i Profile Wizard

## Technical Approach

Extend the Bubble Tea shell to eight steps. Construction, credentials, persistence, and proof remain injected services; `Model` retains only non-secret drafts, request/generation identities, and sanitized outcomes.

## Architecture Decisions

| Decision | Choice and rationale |
|---|---|
| Prepared create | A cross-process, profile-scoped lock in the profile root serializes every Nexus profile create and keyring Set/Delete. Under it, write a non-secret journal, reject an existing profile/credential, provision, then use atomic `profile.Store.Save`. This follows current boundaries without racy preflight. |
| Safe compensation | The journal records transaction ID, phase, and a backend ownership token when conditional delete exists. Compensation may delete only through atomic `DeleteIfOwned(profile, token)`. Current native `CredentialStore` has no compare-delete; therefore compensation **must not call `Delete`**. It leaves the profile absent, records `credential_cleanup_required`, reports cleanup failure, and blocks retry/proof until explicit operator recovery. Uncertain/crashed preparation follows the same no-delete path, so pre-existing or concurrently replaced secrets are never deleted. |
| Step 6 idempotency | One save intent receives random `CreateRequestID`, immutable draft digest, and generation. A service single-flight entry makes matching duplicates join or replay the cached terminal result; ID/digest mismatch fails closed. The model emits once while pending, accepts only the current identity, caches terminal results, ignores stale messages, and transitions visibly once. Retry creates a new identity; successful replay returns the saved profile, never conflict. |
| SSH admission | An eligible sanitized WSS result contains opaque `FallbackTicket`, a 192-bit random capability. Its server-side digest record binds saved profile, WSS request, generation, eligible class, and five-minute expiry. Eligible classes are refused, unavailable, availability-timeout, policy-disabled, and verified-unsupported-version. SSH consent atomically consumes it once; mismatch, expiry, cancellation, supersession, staleness, forgery, or replay is rejected before SSH effects. |
| Completion truth | Completion uses exactly `omitted`, `cancelled`, `failed`, or `successful`; readiness requires an accepted current-generation fixed proof with successful cleanup. |

## Data Flow

```text
Steps 1–5 → Step 6 lock/journal → optional keyring provision → atomic profile save
SavedProfile → consented WSS → eligible FallbackTicket → separate SSH consent
             → sanitized terminal result → truthful completion
```

The TUI already passes lifecycle context to Bubble Tea and identity inspection. Remaining work is reuse by Step 4/save/proof commands, bounded child contexts, supersession cancellation, and deadline-versus-operator classification.

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/configuration/profile_creation.go` | Create | Lock/journal coordinator, single-flight create, safe compensation. |
| `internal/profile/profile.go`, `internal/profile/recovery.go` | Modify | Profile-scoped lock and prepared-create recovery records. |
| `internal/configuration/security.go` | Modify | Route Nexus credential mutations through the shared lock. |
| `internal/configuration/step8*.go` | Modify | Ticket issuance/consumption and split WSS/SSH consent. |
| `internal/tui/model.go`, `mapepire_onboarding_step.go`, `profile_{credentials,review,proof,completion}_step.go` | Modify/Create | Steps 5–8, lifecycle requests, stale rejection, rendering. |
| `cmd/nexus/main.go`, `internal/localization/localization.go`, `docs/IBM_I_PROFILE_WIZARD.md` | Modify | Composition, copy, and eight-step documentation. |

## Interfaces / Contracts

`CreateProfileRequest` carries request ID, generation, and draft digest; `CreateProfileResult` returns the exact immutable saved profile. `WSSProofResult` carries request ID, generation, sanitized class, and optional `FallbackTicket`. Ticket storage and credential ownership tokens never enter TUI state or persisted profiles.

## Completion Mapping

| Event | Completion outcome | `ready_for_controlled_validation` |
|---|---|---|
| Omitted | `omitted` | No |
| User cancellation | `cancelled` | No |
| Deadline timeout | `failed` | No |
| Terminal WSS/SSH failure | `failed` | No |
| Successful fixed proof and cleanup | `successful` | Yes |
| Cleanup/audit/marker failure | `failed` | No |
| Stale result | No transition; retain accepted outcome | Unchanged, never granted by stale data |
| Retry supersession | No transition; latest generation decides | No until latest success |

## Testing Strategy

RED tests cover lock contention, create replay, safe compensation, ownership mismatch, ticket binding/expiry/replay/cancel, zero SSH effects, stale generations, exactly-once transitions, and every completion row. Direct `Update` tests state; runtime views cover three supported sizes and `NO_COLOR`; integration uses `t.TempDir` and loopback transports.

## Threat Matrix

| Boundary | Applicability | Safe/failure behavior | Planned RED tests |
|---|---|---|---|
| Documentation-like paths | N/A — no executable classification | No boundary changed | None |
| Git repository selection | N/A — no Git execution | No boundary changed | None |
| Commit state | N/A — no commit automation | No boundary changed | None |
| Push state | N/A — no push automation | No boundary changed | None |
| PR commands | N/A — no PR automation | No boundary changed | None |
| Remote process/transport | Applicable | Fixed proof only after consent/trust/ticket; rejection blocks downstream effects | Cancellation, timeout, stale, invalid/replayed ticket; zero transport calls |

## Migration / Rollout

No profile migration. Unresolved prepared journals block create/proof and require explicit recovery; rollback must not discard them.

## Open Questions

None.
