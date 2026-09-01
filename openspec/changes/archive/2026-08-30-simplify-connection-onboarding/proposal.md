# Proposal: Simplify Connection Onboarding

## Intent

Replace the confusing eight-step IBM i journey with one secure flow: provide host, username, and password, choose **Connect and Save**, then receive an actionable completion result. This supersedes the eight-step onboarding UX, not its backend security mechanisms.

## Scope

### In Scope
- One normal-path form, one direct action, and one completion state.
- Backend-owned profile name, port, identity, Mapepire, transport, fallback, proof, keyring, audit, and persistence defaults/policy.
- Modern-shell profile list/detail/edit/delete with working keyboard navigation.
- Fix inert Finalize behavior and prevent feedback leaking across screens or operations.
- Strict-TDD proof before making eight-step and legacy normal paths unreachable.

### Out of Scope
- Exposing infrastructure terminology, advanced controls, or storage-mode choices in normal onboarding.
- Weakening backend security controls.
- Automated live IBM i testing; it remains optional, explicit, and unavailable in normal CI.

## Product Behavior

Password capture remains outside Bubble Tea models, messages, views, logs, audit, and persistence. An application service derives approved metadata, applies fail-closed policy, connects/proves through bounded services, uses native keyring automatically when supported, and commits secret-free metadata. Without keyring support, it selects memory-only prompt-on-use; no password is stored insecurely. Failures are sanitized, actionable, cancellable, and never claim partial success.

Existing valid profiles remain readable and manageable without forced rewriting; new profiles use compatible derived metadata. Legacy credentials are never silently migrated or deleted. Unsupported profiles fail closed with management guidance.

## Capabilities

### New Capabilities
None.

### Modified Capabilities
- `nexus-configuration`: Direct onboarding, automatic credential policy, modern management, scoped feedback, and truthful completion replace legacy UX requirements.

## Approach and Affected Areas

| Area | Impact |
|---|---|
| `internal/tui/`, `internal/localization/` | Replace routes/copy; preserve responsive, secret-free shell. |
| `internal/configuration/`, `internal/profile/`, `internal/credential/` | Orchestrate compatible defaults under existing policy. |
| `cmd/nexus/` | Compose transient capture and onboarding. |
| Tests and `docs/IBM_I_PROFILE_WIZARD.md` | Replace obsolete claims with TDD evidence. |

## Risks

- **High:** secret leakage or weakened trust. Mitigate with lower-layer capture and regression tests.
- **Medium:** profile incompatibility or premature route removal. Preserve schema compatibility and retire old reachability only after replacement proof.

## Rollback Plan

Revert new routing and orchestration while retaining profiles and all backend security mechanisms. Never weaken credential, trust, audit, or persistence enforcement.

## Success Criteria

- [ ] Normal onboarding requests only host, username, and password and completes through one action.
- [ ] Secrets never cross the Bubble Tea or persistence boundaries; keyring fallback follows policy.
- [ ] Modern profile management, Finalize, scoped feedback, cancellation, and responsive navigation pass strict-TDD coverage.
- [ ] Existing profiles remain usable and normal CI requires no live IBM i.
