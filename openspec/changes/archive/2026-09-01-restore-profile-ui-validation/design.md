# Design: Restore Profile UI Validation

## Technical Approach

Restore only the presentation contracts visible at parent `ab346046a`: responsive shell/panel geometry, semantic feedback, Bubbles cursor/focus, lossless wrapping, action states, and persistent overflow. Implement compact profile-specific primitives over current `homeTheme`, `shellLayout`, and `wrapWizardText`; do not recreate deleted wizard files, steps, progress, drafts, proof types, or password state. `Model` continues to own presentation/orchestration only.

## Architecture Decisions

| Decision | Alternatives | Rationale |
|---|---|---|
| Add profile primitives for shell, panel, rhythm, field, action, feedback, footer, and viewport | Restore wizard renderer; duplicate Create/Edit markup | Preserves proven behavior without retired semantics and keeps geometry measurable once. |
| Keep validation in TUI adapters returning semantic field IDs | Parse `error.Error()`; copy domain rules | Domain validators remain authoritative; localization and focus never depend on error prose. |
| Keep one persistent profile viewport in `Model` | Recreate viewport in `View`; clip content | Offset survives updates and focused controls can be revealed at 40x16. |
| Preserve `OnboardingOperations` and fixed terminal `tea.Exec` boundary unchanged | Put passwords/messages in `Model`; add connectivity to Edit | Maintains secret ownership, stale-result protection, and presentation-only scope. |

## Data and Control Flow

```text
KeyMsg -> Model.Update -> validation adapter -> local validation state
                         | valid
                         +-> existing store operation (Edit)
                         +-> terminal prompt -> OnboardingOperations (Create)
operation/result msg -> identity check -> operation feedback -> profile renderer
WindowSize/focus -> viewport refresh/reveal -> View
```

Create calls `profile.ValidateHost` and then `profile.ValidateUsername` exactly once in that order; failure emits no command and focuses the first invalid field. Edit builds the candidate, then evaluates fields in `Profile.Validate` order: name; endpoint (host/port); username; legacy host-key pair; Java home; Mapepire path; credential mode. Exported validators are called directly; fields only validated inside `Profile.Validate` are isolated by applying one draft field/group to the original valid profile and invoking `Validate`, so no rule or prose classification is duplicated. Final `Profile.Validate` remains the save gate.

Feedback selection is operation error > explicit operation/status > local validation. Editing a field clears that field's validation; entering/leaving a profile context clears all local validation. Operation failure survives edits and clears only on retry start, cancellation/context exit, or a newer terminal result. Invalid actions are blocked but focusable and reveal the first invalid control; truly unavailable/running actions are excluded, while Cancel/Finalize remain reachable.

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/tui/profile_screen.go` | Create | Compact primitives, responsive geometry, feedback, footer, persistent viewport/reveal. |
| `internal/tui/profile_validation.go` | Create | Ordered Create/Edit adapters and semantic field results. |
| `internal/tui/model.go` | Modify | Local validation/viewport state and Edit action orchestration only. |
| `internal/tui/onboarding.go` | Modify | Use direct adapter and shared renderer without changing secret boundary. |
| `internal/tui/profile_screen_test.go` | Create | Primitive and runtime-frame RED tests. |
| `internal/tui/onboarding_test.go`, `internal/tui/render_matrix_test.go`, `internal/tui/localization_test.go` | Modify | State, failure, security, viewport, and locale matrices. |
| `internal/localization/catalogs/{es,en}_direct_onboarding.json`, `internal/localization/catalogs/{es,en}_extra.json` | Modify | Paired semantic field/action feedback; catalogs remain complete and fail closed. |

## Interfaces / Failure Handling

Validation result is presentation data: `{FieldID, MessageID, Cause}`; `Cause` is asserted/logically retained but never parsed or rendered. Invalid Create starts no prompt/operation; unavailable prompt fails closed; stale results are ignored; cancellation calls existing `Cancel`; completion maps only saved, failed, and cleanup-required states. Invalid Edit calls neither `Save` nor `Update`; store failures stay on Edit with sanitized operation feedback.

## Testing Strategy

Strict RED-GREEN-REFACTOR per slice. Table-driven `Update` tests prove validator order, first-invalid focus, clearing/precedence, blocked/disabled navigation, no store/operation calls, stale results, cancellation, and secret absence. Runtime `View()` tests page through entry, validation, operation error, running, saved, failed, cleanup-required, Edit save/cancel at 120x40 true-color, 80x24 `NO_COLOR`, and 40x16, in Spanish and English; assert bounds, Bubbles cursor, complete copy, focused-control reachability, and truthful top/middle/bottom indicators. Required gate: `go test -count=1 ./...`.

## Threat Matrix

| Boundary | Applicability | Design response / RED tests |
|---|---|---|
| Documentation-like paths | N/A: no executable classification | None. |
| Git repository selection | N/A: no Git routing | None. |
| Commit state | N/A: no VCS automation | None. |
| Push state | N/A: no push behavior | None. |
| PR commands | N/A: no PR commands | None. |

The existing fixed in-process terminal capture is not expanded into shell/subprocess execution; security regression tests cover its unchanged secret-free behavior.

## Migration, Delivery, Rollback

No migration or feature flag. Auto-chain two independently reviewable sub-800-line slices: (1) primitives plus Create recovery; (2) Edit adapter plus paired localization/runtime coverage. `sdd-tasks` must validate estimates. Revert slice 2 first, then slice 1; domain, persistence, and operations remain unchanged.

## Open Questions

None.
