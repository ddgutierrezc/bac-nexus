# Proposal: Restore Componentized Profile Onboarding

## Intent

Supersede archived direct Create—host and username followed by **Connect and Save**—with secure four-step onboarding. Restore the exact component system from `d027210`/`e4449a1`, retain recovery work, and fix `OnboardingService` hardcoding port `22`, which prevents selected-port propagation to inspection, proof, identity comparison, and persistence.

## Scope

### In Scope
- Four steps: name; host, editable SSH port default 22, and IBM i username; visible password action using `tea.Exec` and hidden terminal capture; secret-free review, bounded connect/save, and sanitized completion.
- Exact historical render, viewport, progress, identity, and connection components adapted to four steps.
- Preserve dirty recovery work, cancellation/retry, request identity, stale-result rejection, i18n, `NO_COLOR`, viewport behavior, validation, and sanitized errors.
- Propagate the validated port through all backend paths.
- Strict TDD: write failing component-contract, lifecycle, port-propagation, secret-boundary, and runtime-frame tests before implementation.

### Out of Scope
- Restoring the old eight-step workflow, identity/proof choices, Mapepire screens, drafts, proof reruns, or password state.
- Changing Edit, MCP/CLI, trust, or credential contracts; live IBM i tests.

## Capabilities

### New Capabilities
None.

### Modified Capabilities
- `nexus-configuration`: replace direct Create requirements with four-step onboarding, port propagation, progress/review behavior, and preserved security/lifecycle guarantees.

## Approach

Restore byte-identical components from `d027210`, adapt steps 1–2, and compose steps 3–4 over current `tea.Exec` and async seams. Deliver stacked-to-main slices under 800 changed lines: component contracts, onboarding, backend port/lifecycle, then localization/docs/runtime evidence.

## Affected Areas

| Area | Impact | Description |
|---|---|---|
| `internal/tui/` | Modified | Components, orchestration, viewport, localization, tests |
| `internal/configuration/onboarding.go` | Modified | Carry selected port through connect/save |
| `docs/IBM_I_PROFILE_WIZARD.md`, `DESIGN.md` | Modified | Document approved flow and component behavior |
| `openspec/specs/nexus-configuration/spec.md` | Modified | Replace superseded direct-onboarding requirements |

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| Restore overwrites useful dirty work | High | Selective reconciliation with regression tests |
| Secret or stale async result escapes its boundary | Medium | Secret-negative tests, request IDs, cancellation |
| UI port differs from backend port | High | End-to-end port-propagation tests |

## Rollback Plan

Revert slices in reverse order to direct Create and prior spec/docs while retaining independent recovery fixes. Remove port UI and request changes together to prevent misleading endpoints.

## Dependencies

- Historical revisions `d027210` and `e4449a1`; current Bubble Tea/Bubbles/Lip Gloss stack.

## Success Criteria

- [ ] All four steps remain usable at 120x40, 80x24, and 40x16 in color and `NO_COLOR`, with Spanish/English parity.
- [ ] No secret enters TUI state, messages, views, logs, audit, or files; selected ports propagate through proof and persistence.
- [ ] Strict-TDD unit/runtime suites prove validation, cancellation/retry, stale rejection, bounded completion, and sanitized failures.
