# Delta for nexus-configuration

## ADDED Requirements

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

## REMOVED Requirements

### Requirement: Native Credential Administration
(Reason: Credential handling is automatic policy.)
(Migration: Use Secret Isolation and Credential Policy.)

### Requirement: Host-Key Enrollment and Pinning
(Reason: Trust is backend-derived.)
(Migration: Preserve fail-closed enforcement.)
