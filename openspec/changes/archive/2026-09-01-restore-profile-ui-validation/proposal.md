# Proposal: Restore Profile UI Validation

## Intent

Repair a user-facing regression caused by wizard retirement: shared visual primitives, UI validation/focus feedback, and runtime coverage disappeared with the obsolete workflow. Recover presentation without restoring retired semantics or changing domain behavior.

## Scope

### In Scope
- Preserve one-screen host + username **Connect and Save** and terminal-only transient password capture.
- Restore BAC shell/header/footer, centered responsive panel, fields/actions, semantic feedback, rhythm, and viewport/overflow for Create, running, completion, and metadata Edit.
- Use domain validators for field-specific errors and first-invalid Bubbles focus in Create/Edit; retain save/cancel.
- Prove real runtime frames at 120x40, 80x24 NO_COLOR, and 40x16 for validation, error, running, completion, save, and cancel.

### Out of Scope
- Wizard steps/progress/drafts/proof, password model/message/view state, or proof reruns.
- Edit authentication/connectivity proof; domain, persistence, security, or onboarding lifecycle changes.

## Capabilities

### New Capabilities
None.

### Modified Capabilities
- `nexus-configuration`: Restore responsive runtime, semantic feedback, field validation, and first-invalid focus requirements for direct creation and metadata editing.

## Approach

Recover selected historical presentation contracts as compact helpers using `homeTheme`, `shellLayout`, and lossless wrapping; never restore deleted wizard files. Adapt Edit over authoritative validators instead of parsing errors or duplicating rules. Clear local validation on edits/context changes; operation errors retain precedence. Forecast two sub-800-line slices: presentation + Create recovery, then Edit validation + coverage/localization; `sdd-tasks` must confirm chaining.

## Affected Areas

| Area | Impact | Description |
|---|---|---|
| `internal/tui/*.go` | Modified | Presentation and focus/feedback adapters |
| `internal/tui/*_test.go` | Modified | Runtime matrix and state coverage |
| `internal/localization/catalogs/*.json` | Modified if needed | Paired field copy |

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| Retired workflow leaks back | Medium | Recover behavior only; prohibit wizard/proof semantics |
| Narrow frames hide controls | Medium | Persistent viewport and runtime bounds/reachability tests |
| Edit validation drifts | Medium | Call domain validators in existing validation order |

## Rollback Plan

Revert presentation, localization, and runtime-test commits together; unchanged domain and operation boundaries preserve current behavior.

## Dependencies

- Existing BAC primitives and `internal/profile` validators; no new dependency or live IBM i.
- Strict TDD with `go test -count=1 ./...`.

## Success Criteria

- [ ] Scoped states retain reachable controls, focus, feedback, wrapping, and overflow at all three sizes.
- [ ] Invalid Create/Edit submissions are blocked, identify the first invalid field, and focus it without authenticating Edit.
- [ ] No secret enters Bubble Tea and no wizard/proof semantic returns.

## Proposal Question Round

Auto-mode assumptions: all local profile operators are affected; copy remains unless field specificity requires localization parity; visual recovery cannot alter outcomes. Confirm during specs or correct before design.
