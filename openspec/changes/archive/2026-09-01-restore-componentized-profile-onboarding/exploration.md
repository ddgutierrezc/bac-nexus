## Exploration: Restore Componentized Profile Onboarding

### Current State
The dirty worktree has restored the BAC shell, responsive profile viewport, scoped validation, localization parity, and direct two-field onboarding presentation. Its active contract intentionally excludes wizard/progress/draft semantics: Create collects only host and username, then uses `tea.Exec` for terminal-only password capture. The current `OnboardingRequest` has no port and `OnboardingService` hardcodes `Port: 22`, so a selected endpoint port cannot reach inspection, proof, or persistence.

Historical baseline `e4449a1` contained the original componentized wizard. The requested reusable sources are byte-identical in preferred revision `d027210`: `wizard_render.go`, `wizard_viewport.go`, `wizard_progress.go`, `profile_step.go`, and `profile_connection_step.go`. `d027210` later replaced that UI with the direct bridge while retaining those component sources, making it the safest source revision for selective restoration.

### Affected Areas
- `internal/tui/wizard_render.go`, `wizard_viewport.go`, `wizard_progress.go` — restore shared panel geometry (about 74% wide, 88% medium, near-full narrow), rhythm, semantic feedback, focus reveal, and persistent overflow.
- `internal/tui/profile_step.go`, `profile_connection_step.go` — adapt historical identity and endpoint components into steps 1–2 of the approved four-step flow; retain name validation, case-insensitive duplicate detection, host → username → port validation, and editable default port 22.
- `internal/tui/onboarding.go`, `model.go`, `profile_screen.go` — compose four steps without putting secret bytes in Model, Msg, View, logs, audit, or files; preserve cancellation, retry, request identity, stale-result rejection, and sanitized lifecycle feedback.
- `internal/configuration/onboarding.go` — extend the non-secret request and profile construction so the validated selected port is used for inspection, proof, identity comparison, and persistence rather than hardcoded `22`.
- `internal/localization/catalogs/*.json`, `internal/localization/localization.go` — add Spanish-default and English-equivalent four-step copy and retain `NO_COLOR` semantics.
- `internal/tui/*_test.go`, `internal/configuration/onboarding_test.go` — recover/adapt historical contract tests and add port propagation and secret-boundary regression tests.
- `docs/IBM_I_PROFILE_WIZARD.md`, `DESIGN.md`, `openspec/specs/nexus-configuration/spec.md` — replace the direct two-field Create contract with the approved four-step onboarding contract during later SDD phases.

### Approaches
1. **Selective historical component restoration** — bring forward the byte-identical shared render/viewport/progress primitives and adapt Step 1/2 controllers to the approved four-step lifecycle; keep today’s terminal-only secret and async backend seams.
   - Pros: recovers proven visual contracts and runtime evidence without reviving obsolete identity/proof/mapepire steps; minimizes presentation invention; preserves current security behavior.
   - Cons: requires careful reconciliation with dirty recovery work and current direct-onboarding models.
   - Effort: High.

2. **Extend the current direct screen incrementally** — add name, port, progress, viewport, and review behavior directly into `onboarding.go`.
   - Pros: fewer historical files initially restored.
   - Cons: recreates already-proven components, risks visual/behavioral drift, and tangles the secret boundary with presentation orchestration.
   - Effort: High.

### Recommendation
Use selective historical component restoration. Reuse the exact shared components from `d027210` (verified identical to `e4449a1`), reduce them to four approved steps, and retain the current direct flow’s `tea.Exec` capture and service lifecycle. The four-step concept is: (1) profile identity/name; (2) endpoint host, username, and validated editable SSH port; (3) focused password action that suspends Bubble Tea and returns secret-free capture status only; (4) secret-free review followed by bounded connect/save, completion, retry/cancel as applicable. Do not restore the eight-step state machine, identity/proof choice UI, Mapepire configuration screens, or password state.

### Risks
- The current dirty UI recovery must be treated as an input, not overwritten: preserve its profile screen/viewport, edit validation, localization, and runtime test work; reconcile it deliberately in a later implementation branch.
- Port loss is currently a functional/security defect: adding a UI port without extending `OnboardingRequest` and all downstream inspection/proof/persistence paths would still inspect port 22 while displaying or saving another endpoint.
- `tea.Exec` must be isolated at the password action: only the command owns and zeroes secret bytes; completion messages, request identity, feedback, audit, and persisted profile data remain secret-free.
- Reintroduced progress controls must distinguish focus, selection, readiness, blocked, and disabled. Blocked forward actions stay focusable, display `[ERR]`/`[WARN]` feedback, and reveal the first invalid control.
- Historical rendering tests must be adapted, not copied blindly: prove lossless cell-width wrapping with continuation indent, viewport overflow indicators, focus reveal, Spanish/English parity, and `NO_COLOR` at 120x40, 80x24, and 40x16 through runtime `Update`/`View` frames.

### Ready for Proposal
Yes — no additional product question blocks proposal. The proposal should explicitly supersede the archived direct two-field Create semantics while preserving its useful shell, validation, terminal-secret boundary, cancellation/retry, stale-result rejection, sanitized errors, and persistence behavior. It should forecast stacked review slices under the 800-line budget and begin strict TDD with component-contract, lifecycle, port-propagation, and runtime-frame tests before implementation.
