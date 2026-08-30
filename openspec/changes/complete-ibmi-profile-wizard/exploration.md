## Exploration: complete-ibmi-profile-wizard

### Current State
The local `nexus configure` Bubble Tea wizard has real Steps 1–4, followed by an injected Step 8 action surface. Steps 1 and 2 collect and validate non-secret draft data; Step 3 performs explicit, cancellable, pre-auth SSH TOFU inspection and, after acceptance, requires a separate Enter to enter Step 4. Step 4 is a bounded pre-auth HTTPS `/version` observation and does not authenticate, start fallback, or save a profile.

Steps 5 (Java), 6 (credentials), 7 (review/save), and 9 (completion) have no wizard screens or navigation. A production `configuration.Step8Service` and a cancellable/retryable Step 8 action screen are composed, but the fresh wizard never saves the V3 profile required to reach Step 8. Step 4 can only enter Step 8 by finding an existing persisted profile whose name matches the draft. Its newly created action has `Consent: false`, and the UI presents no fallback-consent choice; direct WSS proof can still proceed because the WSS branch does not gate on consent. Step 8 also uses a background context rather than the program lifecycle context.

Current code and tests supersede an older Step 8 exploration that described the entire production route as uncomposed: `cmd/nexus` now injects the production runner and the model has a Step 8 screen. Neither this offline evidence nor the local SSH harness validates Mapepire or IBM i in a real approved environment.

### Affected Areas
- `internal/tui/model.go` — owns wizard screen states, safe draft lifetime, command routing, and production runner injection; must remain presentation/orchestration only.
- `internal/tui/profile_step.go`, `profile_connection_step.go`, `profile_identity_step.go`, and `mapepire_onboarding_step.go` — provide current Steps 1–4 navigation and define the draft data that later steps must preserve without introducing implicit remote effects.
- `internal/tui/step8_action.go`, `step8_view.go`, and `wizard_viewport.go` — need a fresh-profile handoff, explicit consent/fallback UX, lifecycle-bound cancellation, focus/feedback states, and responsive runtime coverage.
- `cmd/nexus/main.go` — is the composition root for the host-identity inspector and production Step 8 runner; it must not move credential, connector, or remote logic into TUI.
- `internal/configuration/step8_service.go` — owns saved-profile proof orchestration; consent must be consistently enforced before every remote attempt or an explicit product decision must define the narrower rule.
- `internal/profile/profile.go` and profile storage — provide V3 secret-free persistence and atomic create semantics; Step 7 needs an agreed validated profile construction and duplicate/conflict handling boundary.
- `internal/credential/` and `internal/configuration/` credential/trust seams — provide keyring/prompt and trust contracts, but require an approved wizard-facing credential policy and ordering relative to profile persistence.
- `docs/IBM_I_PROFILE_WIZARD.md` and `openspec/specs/nexus-configuration/spec.md` — contain current facts and constraints; the valid spec currently classifies Java/Mapepire/JAR checks as legacy diagnostics, which conflicts with treating Steps 4–5 as canonical wizard functionality without a spec decision.

### Approaches
1. **One PR completing Steps 5–9** — add Java, credential, review/save, fresh Step 8 with consent/fallback, and completion screens in one change.
   - Pros: delivers the apparent nine-step journey in one user-visible increment.
   - Cons: requires unresolved credential/persistence, trust, Mapepire/Java scope, consent, and readiness decisions; crosses TUI, profile, credential, configuration, localization, documentation, and runtime-test boundaries. It cannot credibly fit an 800 changed-line review budget with required coverage.
   - Effort: High.

2. **Decision-gated vertical slice: Steps 5–7, save, then fresh-profile Step 8 handoff** — specify Java as configured-only or omit it, choose credential semantics, compose a secret-free review, atomically save a V3 profile at Step 7, and enter Step 8 with an explicit remote/fallback-consent gate.
   - Pros: establishes the missing persistence boundary and makes the existing Step 8 service reachable from a new profile without misrepresenting testing as saving.
   - Cons: still needs product decisions before implementation and is likely near or above 800 lines once responsive runtime tests and localization are included; Step 9 remains separate.
   - Effort: High.

3. **Narrow security correction before journey composition** — correct Step 8 consent and lifecycle context semantics, then plan Steps 5–7 and 9 as separate reviewable slices.
   - Pros: closes the current explicit-consent inconsistency at the remote-action boundary and keeps production behavior honest.
   - Cons: does not make a fresh profile reach Step 8 or complete the wizard.
   - Effort: Medium.

### Recommendation
Do not approve a single 800-line PR to “complete everything missing.” First resolve the product gates below, then use Approach 2 as the first coherent implementation slice, preceded by Approach 3 if the consent defect is confirmed as in-scope. The slice must retain the save-versus-test distinction: Step 7 persists validated non-secret V3 metadata exactly once; Step 8 receives that saved profile and requires explicit consent before any remote attempt or fallback. Step 9 should be a following slice because its honest readiness/preview claims depend on the final save/test state model.

Required decisions before a proposal/spec/design:
- Is TOFU allowed for the targeted V1 environments, and what approved source and `known_hosts` policy supports verified enrollment?
- Is Step 5 a V1 configured-only Java setting, a legacy diagnostic, or omitted under the current configuration specification?
- Which user-facing credential choices map to V3 `prompt|keyring`, and what is the keyring/profile save ordering and migration behavior?
- Does explicit Step 8 consent cover the WSS attempt and separately authorize SSH fallback, or are two confirmations required? What timeout, audit class, and recovery rules apply?
- What completion labels distinguish saved, locally configured, test attempted/completed, and ready for controlled validation without claiming `nexus serve` or IBM i readiness?

### Risks
- The current fresh-profile route is broken: it never produces the persisted V3 profile that Step 8 requires.
- The current Step 8 UI does not collect or display consent, while the WSS path can make a remote authenticated attempt without `request.Consent`; this conflicts with the documented explicit-consent invariant.
- Background-context Step 8 commands are not bound to the TUI program lifecycle; cancellation and shutdown behavior need a single parent context.
- The valid OpenSpec configuration spec still limits Java, Mapepire, and JAR checks to legacy diagnostics; promoting them into canonical Steps 4–5 requires explicit specification approval.
- No real Mapepire HTTPS `/version`, keyring, IBM i identity/authority, WSS/SSH fallback, Java, or fixed proof validation has occurred. These remain separately approved, read-only field-validation work.
- Completing Steps 5–9 with required tests, responsive frames, `NO_COLOR`, secret-safety proof, and documentation is not feasible within one 800-line PR without an explicit review-budget exception.

### Ready for Proposal
No — obtain the five product/security decisions above and either permit chained reviewable slices or explicitly accept a larger review budget. After that, propose the Step 5–7/save/fresh-Step-8-consent vertical slice first; do not claim live IBM i validation or operational `nexus serve` readiness.
