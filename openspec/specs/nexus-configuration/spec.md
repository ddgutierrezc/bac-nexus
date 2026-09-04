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

### Requirement: Shared Responsive Profile Screens
Create and metadata Edit MUST use the shared BAC header, footer, and centered responsive panel. Create MUST use a four-step onboarding flow; Edit SHALL retain its metadata-only Save and Cancel behavior. Entry, capture, running, failure, cancellation, cleanup-required, success, and completion states MUST expose truthful, scoped status and reachable actions.

#### Scenario: Create lifecycle
- GIVEN an operator opens Create
- WHEN they complete the four steps and connect/save
- THEN scoped lifecycle feedback and completion are shown

#### Scenario: Edit lifecycle
- GIVEN an operator opens profile metadata Edit
- WHEN they save or cancel
- THEN the panel shows the resulting state or returns without applying edits

### Requirement: Field-Specific Create Validation and Feedback
Create MUST validate name including case-insensitive duplicate checking/loading, then host, username, and editable SSH port (1–65535); port SHALL default to 22. Invalid Next or Connect and Save MUST remain focusable, start no capture or operation, and focus the first invalid field. Operation errors MUST take precedence over explicit and validation feedback; local validation MUST clear when its field or context changes.

#### Scenario: Invalid create submission
- GIVEN invalid name or endpoint input
- WHEN the operator activates the focused continuation
- THEN no later step, prompt, or operation starts and focus moves to the first invalid field

#### Scenario: Feedback precedence and clearing
- GIVEN validation feedback or a scoped operation failure is visible
- WHEN the related field/context changes or another operation fails
- THEN local validation clears as applicable and the operation failure remains visible

### Requirement: Validated Metadata Edit
Metadata Edit MUST use the authoritative `Profile.Validate` validation order and present field-specific semantic feedback. An invalid Save MUST remain focusable, block persistence, and focus the first invalid field. Edit SHALL retain simple Save and Cancel actions and MUST NOT authenticate, establish or prove connectivity, or run proof.

#### Scenario: Invalid edit save
- GIVEN metadata fields fail validation
- WHEN the operator activates Save
- THEN no profile is changed and focus moves to the first invalid field

#### Scenario: Cancel edit
- GIVEN an operator has changed Edit fields
- WHEN they activate Cancel
- THEN changes are discarded without authentication, connectivity, or proof

### Requirement: Accessible Terminal Rendering and Copy Parity
Create and Edit MUST render in fixed BAC shell/chrome with a responsive centered panel and lossless content wrapping. The four Create steps MUST retain persistent, truthful overflow and reachability at 120x40, 80x24, and 40x16 in Spanish and English, with and without `NO_COLOR`. Semantic status MUST be understandable independently of color, and `NO_COLOR` output MUST contain no ANSI escape sequences.

#### Scenario: Narrow runtime frames
- GIVEN each supported viewport, including 40x16
- WHEN Create or Edit contains feedback exceeding the visible area
- THEN controls, focus, feedback, and exit remain reachable through truthful overflow without lost text

#### Scenario: No-color localized feedback
- GIVEN English or Spanish and `NO_COLOR` are selected
- WHEN a field validation error renders
- THEN equivalent actionable feedback renders without ANSI escape sequences

### Requirement: Componentized Four-Step Onboarding
Create MUST present exactly: 1 Name, 2 Connection, 3 Credentials, 4 Review. Back and Next SHALL preserve entered non-secret values; forward movement SHALL be blocked until the current step is valid. The shell, panel, title, indicator, input/action/feedback primitives, focus reveal, persistent overflow, and lossless terminal-cell wrapping MUST remain responsive.

#### Scenario: Step navigation
- GIVEN a Create session with valid step input
- WHEN the operator moves forward or backward
- THEN the four-step order and prior non-secret input remain intact

### Requirement: Direct Secure Onboarding
Step 3 MUST visibly offer a secure password action that uses `tea.Exec` for hidden terminal-only capture. Password bytes MUST NOT enter model, message, view, logs, audit, or files; capture status MAY report only captured, cancelled, unsupported/failed, or retryable. Step 4 MUST show a secret-free review and MUST NOT restore eight-step, identity/proof-choice, Mapepire, draft, or proof-rerun semantics.

#### Scenario: Connect and save
- GIVEN valid preceding steps and a captured credential
- WHEN Connect and Save is selected from Review
- THEN onboarding completes only through the bounded backend lifecycle

#### Scenario: Password prompt cannot proceed
- GIVEN password prompt failure, cancellation, or unsupported terminal
- WHEN Connect and Save is selected
- THEN it fails closed and starts or persists no operation

#### Scenario: Retired semantics remain absent
- GIVEN any Create or Edit runtime state
- WHEN the UI renders, receives messages, or records output
- THEN it contains no obsolete workflow semantics and no secret in models, messages, views, or logs

### Requirement: Secret Isolation and Credential Policy
Password bytes SHALL NOT enter tea.Model, tea.Msg, View, logs, audit, metadata, or files. Hidden terminal capture SHALL move bytes directly to an application-owned, bounded, expiring, single-use lease; the TUI retains only opaque status and identity. Retry, replacement, Back, identity edits, cancellation, expiry, stale result, shutdown, and compensation SHALL revoke and zero the lease. The backend SHALL use native keyring when supported or memory-only prompt-on-use when unavailable. It SHALL NOT offer storage choice or silently persist insecure credentials.

#### Scenario: Keyring supported
- GIVEN native keyring support is available
- WHEN onboarding succeeds
- THEN metadata is secret-free and the credential is keyring-only

#### Scenario: Keyring unavailable
- GIVEN native keyring support is unavailable
- WHEN onboarding succeeds
- THEN the profile prompts on use and no password is persisted

### Requirement: Bounded Backend Connection and Persistence
Connect and Save MUST be bounded and cancellable, reject stale results by request identity, and persist only after successful inspection/proof and required audit/credential handling. The validated selected port MUST propagate unchanged through the request, inspection, proof, identity comparison, and persisted profile; no downstream default or hardcoded port MAY replace it. For an approved V3 keyring profile, successful proof MUST create serving eligibility bound to the profile target, policy, host pin, and approved Mapepire artifact identity. Reconfiguration that changes any bound value, credential policy, proof outcome, or persisted profile MUST revoke eligibility until a new approved proof succeeds; failed/cancelled updates MUST retain the prior valid profile and eligibility or leave serving ineligible. Create MUST retain exactly the protected four-step UI and MUST NOT reopen visual-polish scope. (Previously: successful proof could persist a profile without serving eligibility or revocation rules.)

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

#### Scenario: Serving eligibility is created or revoked
- GIVEN approved proof or a target/policy/pin/artifact reconfiguration
- WHEN configuration commits
- THEN eligibility is created only after proof or revoked until reproven

#### Scenario: Protected onboarding remains intact
- GIVEN Create or metadata Edit is exercised
- WHEN serving eligibility changes
- THEN the four-step Create and metadata-only Edit behavior remain unchanged

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
