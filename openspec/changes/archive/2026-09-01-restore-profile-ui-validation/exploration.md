## Exploration: Restore Profile UI Validation

### Current State
The direct Create route is correctly secret-free and calls the application-owned onboarding operation, but `renderDirectOnboarding`, running, and completion use raw wrapped text through `renderLegacyViewport`; the form editor also renders raw lines. Creation validates with `profile.ValidateHost` and `profile.ValidateUsername`, but exposes one aggregate error and only then moves focus. Editing validates through `Profile.Validate` on submit and shows only a global error.

The pre-PR #118 parent (`ab346046a`) contains presentation-only primitives that match the requested standard: responsive centered panel geometry, fixed shell/header/footer with a Bubbles viewport and overflow indicator, field/action rows, focus markers, semantic feedback, spacing rhythm, and lossless wrapping. Its old connection step also proves the desired validation behavior: reuse profile validators, report one field-specific error, block the action, and focus the first invalid real input. Its multi-step states, draft data, step labels, proof flow, and password-related workflow are retired semantics and are not reusable.

### Affected Areas
- `internal/tui/onboarding.go` — recover the direct Create, running, and completion presentation while retaining the current secret-free `tea.Exec` and operation lifecycle.
- `internal/tui/model.go` — route the direct and edit views through shared presentation, retain simple edit save/cancel behavior, and restore field-validation/focus state without changing domain ownership.
- `internal/tui/theme.go`, `internal/tui/home.go`, `internal/tui/wrap.go` — reuse existing BAC theme, shell geometry, footer, semantic styles, and lossless wrapping; add only cohesive form-panel composition if necessary.
- `internal/tui/onboarding_test.go`, `internal/tui/model_test.go`, `internal/tui/render_matrix_test.go` — restore runtime proof for creation entry/running/failure/cleanup/success and edit validation/save/cancel at 120x40, 80x24 NO_COLOR, and 40x16.
- `internal/localization/catalogs/{es,en}_direct_onboarding.json`, `internal/localization/catalogs/{es,en}_extra.json` — add paired, field-specific UI feedback/copy only if no suitable approved message exists.
- `internal/profile/profile.go` — validation source only; it must not be changed. `ValidateHost`, `ValidateUsername`, `ValidatePort`, `ValidateName`, and `Profile.Validate` remain authoritative.

### Approaches
1. **Recover selected historical primitives as small current-state helpers** — adapt panel, input/action, feedback, rhythm, and viewport concepts from `ab346046a` to the current direct route and editor, preserving current screens and operations.
   - Pros: restores the established visual/runtime standard; retains tested responsive contracts; avoids duplicating domain validation; explicitly excludes retired workflow state.
   - Cons: requires careful extraction because the historical helpers refer to deleted wizard types and screens.
   - Effort: Medium

2. **Build new route-local renderers from the home theme** — use only `homeTheme` and write separate direct-create/edit layouts without historical helper recovery.
   - Pros: less compatibility code.
   - Cons: likely duplicates panel, wrapping, feedback, focus, and overflow rules already proven in history; higher visual-regression risk.
   - Effort: Medium-High

### Recommendation
Use Approach 1, but recover behavior rather than restoring files wholesale. Create a compact shared profile-form presentation seam using the current `homeTheme`, `shellLayout`, `wrapWizardText`, and historical panel/viewport contracts. Keep the direct route as exactly host + username + Connect and Save, with password capture remaining outside Bubble Tea. Map invalid direct fields in deterministic host-then-username order; map editor failures to the first field implied by the existing profile validation order, render one `[ERR]` field-specific message, block save, and focus the actual Bubbles input. Clear local validation feedback when the user edits or changes context; explicit operation errors remain higher precedence.

Likely delivery slices under the 800-line review budget:
1. Shared presentation seam plus direct Create/running/completion recovery, field-specific direct validation, and runtime matrix tests.
2. Edit panel recovery, deterministic editor validation mapping/focus, save/cancel coverage, and any localization additions.

### Risks
- Copying the deleted wizard files would accidentally restore prohibited multi-step state, proof screens, or workflow semantics; adapt only presentation contracts.
- `Profile.Validate` returns generic errors in a fixed order, so edit mapping must be an explicit UI adapter over the existing validators, not parsed error prose or duplicated domain rules.
- A panel that fits at 120x40 can hide actions or feedback at 40x16; a persistent viewport with truthful overflow indicators and real `View()` frame tests is required.
- New localization IDs require complete Spanish and English catalogs or localization validation fails.
- Preserve the existing `.atl` modifications and `stash@{0}`; this exploration made no change outside the new artifact.

### Ready for Proposal
Yes — propose a presentation-only TUI recovery in two reviewable slices. State explicitly that it restores shell/panel/feedback/focus/runtime coverage for direct creation and metadata editing, while excluding wizard steps, password model/message/view state, authentication/proof reruns, and all domain-validation changes.
