## Exploration: simplify-connection-onboarding

### Current State
`nexus configure` currently enters a modern Home shell but creates profiles through an eight-screen wizard: profile name, host/user/port, explicit SSH identity inspection and TOFU acceptance, Mapepire readiness, credential-mode choice, review/save, proof, and completion. The production composition now injects a prompt-only `ProfileCreator` and Step 8 runner, so the current uncommitted correction can save a local prompt-mode profile; it does not simplify the journey.

The wizard model intentionally holds metadata only. Its global `status`/`err` feedback is consumed by several screens, so feedback can cross a transition unless every transition clears it. Completion has no key handler, so its visible Finalize action is inert. Existing profile list/detail/edit/delete screens use the legacy renderer and are reached from the modern Home shell.

Connection proof is already backend-owned through `configuration.Step8Service`: it keeps credentials outside the TUI, prefers WSS, permits SSH only through policy, independent trust, and a consumed fallback ticket, and emits sanitized audit metadata. Profile validation currently requires explicit port, SSH trust, and V3 policy/trust metadata. Native-keyring failures fail closed; prompt mode gets an ephemeral password only at authorized use.

### Affected Areas
- `internal/tui/model.go` — replace the eight-screen onboarding route, scope feedback per operation, and route completion and profile management through the modern shell.
- `internal/tui/profile_{step,connection,identity,credentials,review,proof,completion}_step.go`, `mapepire_onboarding_step.go`, `step8_action.go` — retire normal-path infrastructure screens and preserve only reusable presentation/lifecycle primitives.
- `internal/configuration/profile_creation.go` and `internal/configuration/step8*.go` — introduce an application-owned automatic connect/save orchestration boundary; retain fail-closed policy, proof, and audit behavior below it.
- `internal/profile/profile.go` — accept internally derived profile defaults and policy/trust evidence without making them user inputs.
- `internal/credential/` and `cmd/nexus/main.go` — keep native keyring/prompt capture and zeroization outside Bubble Tea; compose the new onboarding service.
- `internal/tui/home.go` and legacy profile screens — move list/detail/edit/delete into the modern shell or provide a modern management route.
- `internal/localization/catalogs/` and `internal/localization/localization.go` — replace normal-path onboarding copy with Spanish-first direct connection copy and English parity.
- `internal/tui/*_test.go`, `internal/configuration/*_test.go`, `cmd/nexus/configure_test.go` — replace eight-step assertions with strict-TDD coverage of the direct flow, secret isolation, policy failures, feedback scope, completion, and responsive frames.
- `openspec/specs/nexus-configuration/spec.md`, `docs/IBM_I_PROFILE_WIZARD.md` — update the approved configuration contract and remove obsolete eight-step claims.

### Approaches
1. **Direct connection orchestration (recommended)** — expose one logical form for host, IBM i username, and password, then invoke an application-owned onboarding service that derives name/defaults, applies identity and transport policy, connects/proves, persists non-secret metadata, attempts native storage when supported, and returns a sanitized completion result.
   - Pros: matches the approved user experience; preserves secret isolation and backend policy; makes WSS/SSH/Mapepire decisions implementation details.
   - Cons: requires a new, carefully bounded orchestration contract and migration from the existing state machine.
   - Effort: High.

2. **Hide labels but retain the eight-step state machine** — cosmetically remove technical wording while continuing to require the same transitions.
   - Pros: smaller initial visual diff.
   - Cons: does not meet the approved direct-flow product decision; preserves duplicate/inert states and leaks workflow complexity.
   - Effort: Medium.

### Recommendation
Adopt Approach 1 as one approved size-exception change, while preserving security behavior behind the UI. The smallest safe product slice is: one logical connection form; one explicit Connect and Save action; automatic policy-controlled identity/transport/credential handling; one localized success-or-actionable-failure completion state; and modern-shell profile management. Do not expose a password through `tea.Model`, messages, views, logs, or persisted data: its capture must remain a transient service callback invoked by the orchestration layer.

Retain the current uncommitted production composition, prompt-mode local save, catalog infrastructure, feedback renderer, viewport shell, request identity/cancellation patterns, Step 8 policy/proof/audit services, and keyring adapter. Remove or make unreachable from normal onboarding the name, port, TOFU/fingerprint, Mapepire, credential-mode, review, proof/fallback, validation, and legacy-management UI machinery. Retention does not imply keeping those concepts visible; advanced/support flows may reuse backend services later.

### Risks
- A literal Bubble Tea password input would violate the existing no-secret-in-model/message/view invariant; secure transient capture must be designed before implementation.
- Current V3 profile validation requires trust and policy fields that the new service must derive from approved backend policy, not invent or weaken.
- Existing Step 8 runs from `context.Background()` in the action model; the direct flow must use the program lifecycle context, bounded child contexts, cancellation, and stale-result rejection.
- `m.status` and `m.err` are global wizard context; scoped feedback is needed to prevent the reported leakage.
- `screenProfileCompletion` currently ignores Enter, confirming the reported inert Finalize defect.
- Current OpenSpec and wizard documentation explicitly endorse the eight-step flow and must be superseded before implementation; live IBM i validation remains out of scope.
- The current worktree contains 545 changed lines including unrelated `.atl` changes. This exploration neither modifies nor incorporates `.atl`; the maintainer-approved single-PR exception and 800-line review budget must still be enforced in task planning.

### Ready for Proposal
Yes — the product decisions are settled: direct three-value onboarding, internal policy resolution, automatic keyring when available, memory-only fallback when unavailable, Spanish-first copy with English parity, backend-enforced fail-closed security, and modern-shell management. The proposal should explicitly define the transient password-capture boundary, derived-profile contract, secure connect/save ordering, failure/completion taxonomy, and removal/migration plan for the eight-step normal path.
