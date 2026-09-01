# Design: Simplify Connection Onboarding

## Technical Approach

```text
Profiles → Create/Edit → Form(host, username) → Connect and Save → Running → Completion
    ↑             Back ───────────────────────────────────────────── Finalize ┘
```

The application owns capture, policy, proof, credentials, and persistence. Bubble Tea owns presentation. Step 8, trust, recovery, and keyring mechanisms remain.

## Architecture Decisions

| Decision | Choice and rationale | Rejected |
|---|---|---|
| V1 identity | `AutomaticFirstUsePolicy` accepts exactly one valid unverified `HostKeyObservation`, stages an immutable pin with provenance `automatic-tofu-v1:first-contact-unverified`, and records `identity_bootstrap_allowed` before authentication. This event means policy admission, not enrollment. Existing profiles use `TrustService.Verify`; ambiguity, missing evidence, or mismatch records `identity_changed` and fails closed. Success additionally requires post-commit `identity_pin_committed`. Completion/detail disclose that first contact was not independently verified; future corporate CA/pins may supersede this policy. | Claiming independent verification or requiring user choice. |
| Fallback consent | `PolicyFallbackAuthorizer` grants one request ID/generation/reason for only the five existing `DecisionSSHEligible` reasons. `PolicySSHConsent.From(grant)` validates that claim and alone sets `Step8Request.SSHConsent=true`; the internal ticket is issued and immediately consumed by `RunSSH`. Identity, trust, protocol, malformed-response, credential, and unknown failures produce no grant, consent, or ticket. | UI consent or security-failure downgrade. |
| Keyring | `Capability{Supported|Unsupported|Unavailable}` is independent of account `Presence{Present|Absent}`. Only classified unsupported/unavailable capability selects memory-only `prompt`; supported capability followed by probe/read/write failure fails truthfully. | Treating account errors as capability absence. |
| Feedback | Scope `{screen, operationID, generation, code}`; cancel invalidates identity. | Global `status`/`err`. |

## Ownership and Contracts

```go
// internal/configuration consumer; implementations return owned bytes.
type SecretPrompt interface {
    Read(context.Context, *os.File, *os.File, string) ([]byte, PromptCode)
}
// internal/tui; all request/result values are secret-free.
type OnboardingOperations interface {
    StartCaptured(context.Context, OnboardingRequest, []byte) (OperationIdentity, StartCode)
    Wait(context.Context, OperationID) OnboardingResult
    Cancel(OperationID)
}
type IdentityPolicy interface { Resolve(context.Context, Endpoint, *profile.Profile) (IdentityDecision, error) }
type FallbackAuthorizer interface { Authorize(context.Context, profile.Profile, Step8Reason, OperationIdentity) (FallbackGrant, error) }
```

`configuration` owns orchestration, audit, and compensation. `cmd/nexus` injects `remote.SecretPrompt{Input, Output, IsTerminal, Read}`, never global `TerminalSecretPrompt()`. `credential` separates capability/presence. `profile` adds a committer over prepared-create locking, `Update`, `Restore`, and exact create rollback. TUI owns navigation; localization owns both locales.

## Secret and Operation Flow

`onboardingExecCommand` implements Bubble Tea's `Run/SetStdin/SetStdout/SetStderr`. `Run` type-asserts stdin/stderr to `*os.File`, validates both terminals, then calls injected `SecretPrompt`. Before successful capture it performs no derivation, audit, pin staging, operation/worker creation, persistence, keyring, or remote work. Non-files, non-terminals, unsupported prompting, interrupt, or EOF return only a secret-free code and start nothing. After capture, `StartCaptured` creates identity/worker and takes ownership: it copies once into worker memory and zeroes supplied bytes; rejection zeroes them without a worker. The command has no shell/executable/arguments or terminal-mode changes. `tea.Exec` restores the terminal before callback emits only `{ID, Generation, Code}`. Escape cancels the worker; every copy is zeroed and stale results cannot transition screens.

```text
validate terminal → prompt/capture → create operation → stage defaults/pin + bootstrap audit
→ prove/fallback grant → keyring stage
→ profile/pin commit → mandatory committed audit → success
```

Only after capture does the worker derive identity and write bootstrap audit before authentication. The pin remains in memory until profile commit. Before mutation, edits snapshot prior keyring secret/presence; the journal records phase without secrets. If profile commit fails, restore/delete keyring. If committed audit fails, compensate in reverse under the profile lock: exact rollback-delete for new profile/pin; `.bak` restore for edit/old pin; restore old keyring secret or delete its new account. Clear the journal only after complete compensation. Any compensation failure returns **not saved / cleanup required**; otherwise **not saved**. Success requires both audits. Capability fallback writes no credential. No infrastructure option reaches users.

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/configuration/{onboarding,identity_policy,step8_onboarding}.go` | Create | Prompt contract, transaction, audited TOFU, fallback consent. |
| `internal/profile/onboarding_commit.go`, `credential/keyring_store.go` | Create/Modify | Rollback seam; capability/presence split. |
| `internal/tui/{onboarding,profile_management}.go`, `model.go`, `home.go` | Create/Modify | Replacement states, scoped results, Finalize to reloaded profile list. |
| `cmd/nexus/main.go`, localization catalogs, wizard doc | Modify | Terminal adapter, composition, disclosure. |
| Eight-step/legacy form, credential, proof, and TOFU UI files | Delete after proof | Remove parallel routes and catalog keys. |

## Testing Strategy

Strict TDD adds RED tests proving prompt failure/cancel/unsupported terminal creates no operation or side effect; restoration/codes/zeroing; both audits; all profile/pin/keyring compensations; policy SSHConsent, claim binding, immediate ticket consumption, forbidden downgrades; capability errors; stale cancellation; Finalize/management/recovery; locales; and runtime 120x40, 80x24 NO_COLOR, 40x16. CI uses fakes/loopback; live IBM i is opt-in or skipped.

## Threat Matrix

All reference rows are **N/A**: no executable classification or VCS/PR automation. The fixed in-process `tea.Exec` boundary receives the RED tests above.

## Migration / Rollout

Land RED tests, transactional services, and replacement UI; switch routes; then delete unreachable legacy states/tests. Preserve schemas, pins, keyring accounts, Step 8 gates/tickets, markers, backups, and recovery. Rollback restores routes only; never migrate credentials or weaken pins.

## Open Questions

None.
