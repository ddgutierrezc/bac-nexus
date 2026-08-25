---
name: bac-nexus-tui
description: "Trigger: BAC Nexus TUI, wizard step, Step 4, Bubble Tea, Bubbles, Lip Gloss. Implement or verify responsive BAC Nexus wizard UI behavior."
license: Proprietary
metadata:
  author: "ddgutierrezc"
  version: "1.1.0"
---

## Activation Contract

Load for BAC Nexus TUI or wizard-step work, especially Step 4, Bubble Tea, Bubbles, Lip Gloss, rendering, focus, spacing, wrapping, feedback, or viewport proof. Prioritize current code/tests, then the approved request, `DESIGN.md`, and this Skill; this Skill never replaces code, design, or tests.

## Hard Rules

- Keep `Model`/`Update`/`View` presentation and orchestration only; keep domain, persistence, security, and connector logic outside TUI.
- Inspect and reuse existing panel, header, step indicator, actions, fields, choices, focus, feedback, footer, wrap, rhythm, and viewport behavior before adding variants. Compose only cohesive reusable pieces; avoid artificial components and monolithic screens.
- Keep focus separate from selection: `▸`, `( )`, `(*)`; preserve real Bubbles cursor behavior and give disabled invalid actions useful feedback.
- Keep focus, selection, readiness, and disabled state separate: progress actions are ready, blocked-interactive, or truly disabled. Blocked stays focusable, emits actionable feedback, and focuses the first invalid control when useful; disabled is excluded from focus.
- Preserve compact hierarchy: title→description, question→supporting, description→controls, controls, control→feedback, actions. Use shared rhythm tokens.
- Wrap losslessly by terminal cells: semantic continuation indent, ordinary words whole on fresh lines, split only overlong tokens, no functional ellipsis or overflow.
- Use `[OK]`, `[INFO]`, `[WARN]`, `[ERR]`, `[--]`; select error > explicit > validation. Preserve semantic colors and NO_COLOR.
- Keep panel geometry, wrapping contracts, feedback gaps, control gaps, action gaps, navigation, approved copy, and shell behavior unless the approved request explicitly changes them.
- Treat screenshot references as visual intent only. Reconcile them with fixed-grid terminal limits, responsive viewport behavior, accessibility through NO_COLOR, and existing runtime evidence.
- Add tests beside affected behavior. Do not prove rendering solely through panel helpers, source strings, or internal viewport content when a runtime `View()` proof is possible.
- Remote wizard actions require explicit consent and a bounded async command: show loading, allow cancellation, sanitize errors, support retry, reject stale results by request identity, and never fall back to trust, persistence, or authentication.

## Decision Gates

| Need | Action |
| --- | --- |
| Existing primitive fits | Reuse it. |
| Repeated cohesive UI behavior | Add a shared primitive with runtime tests. |
| Narrow terminal hides content | Use the native viewport only when needed; preserve header/footer. |
| Invalid progress action | Keep it focusable, block transition, show feedback, and focus the first invalid control. |
| Screenshot conflicts with architecture | Stitch visual reference, `DESIGN.md`, and current behavior; never translate HTML/CSS/pixels literally. |

## Execution Steps

1. Read the referenced sources and inspect current symbols/tests before editing.
2. Send `WindowSizeMsg`; use real cell widths, the shared responsive panel, content-driven height, and stable header/footer.
3. Implement the smallest presentation-only change; preserve responsive breakpoints, navigation, and approved copy.
4. Verify actual `Update`/`View` frames at 120x40, 80x24, and 40x16 in true-color and NO_COLOR. Prove focus, reachability, wrapping, bounds, feedback, and spacing through runtime output, not helpers alone.
5. Run focused package tests, `go test ./...`, `go vet ./...`, `go build ./...`, and `git diff --check` before reporting.

## Output Contract

Report changed files, reused primitives, runtime scenarios, test commands/results, and any remaining risk. State whether wrapping, focus, feedback precedence, and narrow viewport behavior were exercised.

## References

- [Agent context](../../../AGENTS.md)
- [Design system](../../../DESIGN.md)
- [Wizard rendering](../../../internal/tui/wizard_render.go)
- [Wrapping](../../../internal/tui/wrap.go)
- [Wizard viewport](../../../internal/tui/wizard_viewport.go)
- [Wizard tests](../../../internal/tui/wizard_render_test.go)
- [Render matrix](../../../internal/tui/render_matrix_test.go)
