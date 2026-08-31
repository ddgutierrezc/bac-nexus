# nexus-configuration Specification

## Purpose

Provide an optional, keyboard-operated local configuration lifecycle for approved IBM i profiles without changing MCP serving, external client configuration, or IBM i validation status.

## Requirements

### Requirement: Optional Configuration Lifecycle
`nexus configure` MUST be optional and MUST provide a profile list, detail views, and reversible navigation. It SHALL use Bubble Tea, Bubbles, and Lip Gloss only after exact-version, license, and vulnerability admission; otherwise a service-complete non-TUI adapter remains the fallback. It SHALL not start MCP or consume MCP stdio.

#### Scenario: Enter and leave the shell
- GIVEN a local operator invokes `nexus configure`
- WHEN the shell starts or the operator exits
- THEN it shows the profile list or returns control without changing configuration

#### Scenario: No profiles exist
- GIVEN no profiles are stored
- WHEN the shell opens
- THEN it offers creation and an empty-state explanation

### Requirement: Complete Profile CRUD and Recovery
The profile store MUST add bounded List and Update contracts. Create and Update MUST validate before atomically replacing a profile, retain a restorable backup, and restore the prior file if replacement fails; failures MUST identify only a sanitized outcome.

#### Scenario: Create, inspect, and update
- GIVEN valid non-secret profile fields
- WHEN an operator creates, reads, or updates a profile
- THEN List and detail views show the committed profile and no secret

#### Scenario: Update replacement fails
- GIVEN an existing profile and its backup
- WHEN atomic replacement fails or the process is interrupted before commit
- THEN the prior profile remains or is restored and the result is failure

#### Scenario: Invalid update
- GIVEN malformed or conflicting profile input
- WHEN the operator submits it
- THEN no profile or backup is replaced

### Requirement: Deliberate Profile Deletion
Profile deletion MUST require exact confirmation of the selected profile, retain a restorable backup, and MUST NOT delete its native credential unless a separate exact credential confirmation succeeds. A partial failure MUST restore the profile and report credential outcome separately.

#### Scenario: Delete profile only
- GIVEN exact profile confirmation and no credential-deletion confirmation
- WHEN deletion succeeds
- THEN the backup is retained and the credential remains

#### Scenario: Credential deletion fails
- GIVEN both exact confirmations and an available profile backup
- WHEN credential deletion fails after profile deletion
- THEN the profile is restored and the credential failure is sanitized

### Requirement: Native Credential Administration
A lower-layer credential-status contract MUST report presence or unavailability without returning secret bytes. Set, rotate, delete, and explicit legacy-vault migration MUST use transient input services that return opaque outcomes; secret bytes MUST NOT enter Bubble Tea commands, messages, model state, views, serializable artifacts, previews, logs, audit, argv, or environment.

#### Scenario: Presence is displayed safely
- GIVEN a profile credential is present, absent, or unavailable
- WHEN credential status is requested
- THEN the UI reports only that classification

#### Scenario: Set or rotate succeeds
- GIVEN transient secret entry and an available native store
- WHEN Set or Rotate completes
- THEN the UI receives an opaque success outcome and renders no secret

#### Scenario: Credential deletion is explicit
- GIVEN an operator confirms deletion of a profile-owned credential
- WHEN Delete completes
- THEN the UI receives only an opaque outcome

#### Scenario: Migration is explicit
- GIVEN a legacy vault is detected
- WHEN the operator explicitly confirms migration
- THEN migration follows the native-store verification policy or retains the vault on failure

### Requirement: Host-Key Enrollment and Pinning
Manual fingerprint entry MUST support independently verified host keys. Remote inspection MUST be an explicit, warned, timed, cancellable action labeled unverified TOFU; exact confirmation MUST precede pinning. A mismatch MUST fail closed.

#### Scenario: Manual enrollment
- GIVEN an independently verified fingerprint
- WHEN the operator confirms manual enrollment
- THEN the key and verified provenance are pinned

#### Scenario: TOFU enrollment is inspected deliberately
- GIVEN a remote inspection is warned and labeled unverified TOFU
- WHEN the operator exactly confirms the inspected fingerprint
- THEN it is pinned with unverified provenance

#### Scenario: TOFU mismatch
- GIVEN an enrolled fingerprint differs from a later presented key
- WHEN a remote action is attempted
- THEN it returns `host_key_changed` without remote discovery

### Requirement: Honest Readiness and Diagnostics
Local readiness MUST never contact IBM i and MUST report missing production recovery, resolver, acquirer, or lease composition. Remote diagnostics MUST be explicitly initiated, warned, timed, cancellable, sanitized, and auditable; they MUST retain `ready_for_controlled_ibmi_validation` and `not_validated_on_ibmi`. Java, Mapepire, and JAR checks MAY appear only as legacy diagnostics.

#### Scenario: Local readiness
- GIVEN no remote action is selected
- WHEN readiness is refreshed
- THEN it performs local checks only and exposes the serve-composition gap

#### Scenario: Cancel remote diagnostic
- GIVEN a warned diagnostic is running
- WHEN the operator cancels or its timeout expires
- THEN it stops, records a sanitized outcome, and makes no validation claim

### Requirement: Preview, Fixed Status, and Terminal Behavior
MCP integration MUST provide schema-validated command/snippet preview and copy only and MUST NOT mutate external-client files. Policy and audit status MUST be read-only; audit history or persistence MUST NOT be added. The UI MUST support Windows Terminal and modern ANSI terminals, 80x24 keyboard-only operation, resize handling, no-color mode, and a narrow-layout fallback.

#### Scenario: Preview is copied only
- GIVEN valid integration fields
- WHEN preview or copy is selected
- THEN output is validated and no external configuration is written

#### Scenario: Narrow no-color terminal
- GIVEN an 80x24 or narrower no-color terminal
- WHEN the operator navigates with the keyboard or resizes
- THEN controls remain usable and essential state remains discernible

### Requirement: Existing CLI and Process Compatibility
Existing automation and `nexus serve` behavior MUST remain compatible. Configuration services, CLI, TUI, MCP, policy, and audit adapters SHALL remain separate; the TUI MUST NOT repair serve composition or claim live validation.

#### Scenario: Existing automation remains valid
- GIVEN an existing catalogspike or serve invocation
- WHEN `configure` is introduced
- THEN its established contract remains unchanged

### Requirement: Direct Secure Onboarding
The persistent Bubble Tea form SHALL contain only host, IBM i username, and **Connect and Save**. Selection SHALL prompt terminal-only transient password capture below Bubble Tea; together they SHALL request exactly those three values in one logical flow without a configuration step. Infrastructure terms and advanced controls SHALL NOT appear; bounded support MAY expose them after failure.

#### Scenario: Connect and save
- GIVEN valid host and username
- WHEN Connect and Save is selected and the prompt returns a password
- THEN onboarding completes in the same logical flow

#### Scenario: Password prompt cannot proceed
- GIVEN password prompt failure, cancellation, or unsupported terminal
- WHEN Connect and Save is selected
- THEN it fails closed and starts or persists no operation

### Requirement: Secret Isolation and Credential Policy
Password bytes SHALL NOT enter tea.Model, tea.Msg, View, logs, audit, metadata, or files. The backend SHALL use native keyring when supported or memory-only prompt-on-use when unavailable. It SHALL NOT offer storage choice or silently persist insecure credentials.

#### Scenario: Keyring supported
- GIVEN native keyring support is available
- WHEN onboarding succeeds
- THEN metadata is secret-free and the credential is keyring-only

#### Scenario: Keyring unavailable
- GIVEN native keyring support is unavailable
- WHEN onboarding succeeds
- THEN the profile prompts on use and no password is persisted

### Requirement: Bounded Backend Connection and Persistence
The backend SHALL fail closed while deriving profile name, port, identity/trust, transport, Mapepire/fallback, credential policy, proof, audit, and secret-free persistence. Connect and Save SHALL be bounded/cancellable and discard stale results.

#### Scenario: Proof before persistence
- GIVEN approved derived configuration
- WHEN Connect and Save succeeds
- THEN proof precedes persistence and audit is secret-free

#### Scenario: Save failure
- GIVEN proof succeeds but persistence fails
- WHEN the action completes
- THEN it reports not saved, retained credentials, and retry/cleanup guidance

#### Scenario: Cancelled or stale
- GIVEN an action is running or superseded
- WHEN cancelled, timed out, or stale
- THEN it changes neither screen nor success state

### Requirement: Safe Completion, Feedback, and Responsive Shell
**Finalize** SHALL work. Feedback SHALL fail closed, sanitized, actionable, Spanish-first with English parity, and scoped. The shell SHALL create, open, delete, back, and exit; deletion requires confirmation and recovery. Valid profiles SHALL remain manageable; unsupported profiles fail closed with guidance.

#### Scenario: Completion is finalized
- GIVEN onboarding reports a saved profile
- WHEN the operator selects Finalize
- THEN the shell returns to the profile list without replaying feedback

#### Scenario: Managed-profile navigation
- GIVEN a valid or newly saved profile
- WHEN the operator creates, opens, deletes, navigates back, or exits
- THEN it takes effect or returns scoped feedback

#### Scenario: Feedback does not leak
- GIVEN an operation fails and another screen or operation starts
- WHEN feedback is rendered
- THEN it is absent from the unrelated context

#### Scenario: Responsive runtime
- GIVEN 120x40, 80x24 NO_COLOR, or 40x16 runtime
- WHEN the shell is navigated
- THEN controls, focus, feedback, and exit remain reachable without color

### Requirement: Deterministic Test Boundaries
Normal CI SHALL verify behavior without live IBM i using deterministic fakes. Live validation SHALL use explicit opt-in seams. Unrelated CLI and serve contracts remain unchanged.

#### Scenario: Normal CI execution
- GIVEN normal CI runs the test suite
- WHEN onboarding behavior is exercised
- THEN fakes provide deterministic results and no IBM i connection is attempted

#### Scenario: Live validation
- GIVEN approved live integration configuration is absent
- WHEN integration tests are invoked normally
- THEN live IBM i tests are skipped rather than inferred or attempted
