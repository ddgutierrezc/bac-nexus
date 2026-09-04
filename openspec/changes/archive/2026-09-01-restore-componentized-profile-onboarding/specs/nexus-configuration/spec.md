# Delta for nexus-configuration

## ADDED Requirements

### Requirement: Componentized Four-Step Onboarding

Create MUST present exactly: 1 Name, 2 Connection, 3 Credentials, 4 Review. Back and Next SHALL preserve entered non-secret values; forward movement SHALL be blocked until the current step is valid. The restored shell, panel, title, indicator, input/action/feedback primitives, readiness states, rhythm, focus reveal, persistent overflow, and lossless cell wrapping MUST remain responsive.

#### Scenario: Step navigation
- GIVEN a Create session with valid step input
- WHEN the operator moves forward or backward
- THEN the four-step order and prior non-secret input remain intact

#### Scenario: Identity and endpoint validation
- GIVEN an empty, historically invalid, or duplicate name, or invalid endpoint fields
- WHEN Next is activated
- THEN it stays focusable, reports feedback, and focuses the first invalid field: name; then host, username, port

#### Scenario: Responsive localized rendering
- GIVEN English or Spanish and a 120x40, 80x24, or 40x16 viewport, with or without `NO_COLOR`
- WHEN each step and overflow render through runtime `Update`/`View`
- THEN all text wraps losslessly, controls remain reachable, meaning is color-independent, and no ANSI escapes appear in `NO_COLOR`

## MODIFIED Requirements

### Requirement: Shared Responsive Profile Screens

Create and metadata Edit MUST use the shared BAC header, footer, and centered responsive panel. Create MUST use the four-step componentized onboarding flow; Edit SHALL retain its current metadata-only Save and Cancel behavior. Entry, capture, running, failure, cancellation, cleanup-required, success, and completion states MUST expose truthful scoped status and reachable actions.
(Previously: Create was a direct host/username screen with one action.)

#### Scenario: Create lifecycle
- GIVEN an operator opens Create
- WHEN they complete the four steps and connect/save
- THEN scoped lifecycle feedback and completion are shown

#### Scenario: Edit lifecycle
- GIVEN an operator opens metadata Edit
- WHEN they save or cancel
- THEN the panel preserves the established Edit result or discard behavior

### Requirement: Field-Specific Create Validation and Feedback

Create MUST validate name including case-insensitive duplicate checking/loading, then host, username, and editable SSH port (1–65535); port SHALL default to 22. Invalid Next or Connect and Save MUST remain focusable, start no capture or operation, and focus the first invalid field. Operation errors take precedence and local feedback clears with related changes.
(Previously: Create validated only host and username for direct submission.)

#### Scenario: Invalid submission
- GIVEN invalid name or endpoint input
- WHEN the focused continuation is activated
- THEN no later step, prompt, or operation starts and first-invalid focus is applied

#### Scenario: Feedback precedence
- GIVEN validation feedback or scoped operation failure
- WHEN related input changes or another operation fails
- THEN applicable validation clears and operation failure remains visible

### Requirement: Direct Secure Onboarding

Step 3 MUST visibly offer a secure password action that uses `tea.Exec` for hidden terminal-only capture. Password bytes MUST NOT enter model, message, view, logs, audit, or files; capture status MAY report only captured, cancelled, unsupported/failed, or retryable. Step 4 MUST show a secret-free review and MUST NOT restore eight-step, identity/proof-choice, Mapepire, draft, or proof-rerun semantics.
(Previously: direct submission captured a password without steps or review.)

#### Scenario: Capture and retry
- GIVEN valid preceding steps
- WHEN the password action captures, cancels, or fails
- THEN only secret-free status returns and retry is available where applicable

#### Scenario: Review connects and saves
- GIVEN a captured credential and secret-free review
- WHEN Connect and Save is selected
- THEN onboarding completes only through the bounded backend lifecycle

#### Scenario: Password capture cannot proceed
- GIVEN password capture is cancelled, unsupported, or fails
- WHEN the operator attempts to continue
- THEN it fails closed and starts or persists no operation

#### Scenario: Retired semantics remain absent
- GIVEN any Create or Edit runtime state
- WHEN it renders, receives messages, or records output
- THEN no obsolete workflow semantics or secret is present

### Requirement: Bounded Backend Connection and Persistence

Connect and Save MUST be bounded and cancellable, reject stale results by request identity, sanitize failures, and persist only after successful inspection/proof and required audit/credential handling. The validated selected port MUST propagate unchanged through the request, inspection, proof, identity comparison, and persisted profile; no downstream default or hardcoded port MAY replace it.
(Previously: onboarding constructed the profile with port 22.)

#### Scenario: Selected port succeeds
- GIVEN a validated non-default port and captured password
- WHEN Connect and Save completes successfully
- THEN inspection, proof, identity comparison, and saved profile use that port

#### Scenario: Save failure
- GIVEN proof succeeds but persistence fails
- WHEN the action completes
- THEN it reports sanitized not-saved guidance and preserves credential cleanup semantics

#### Scenario: Cancelled or stale result
- GIVEN an action is running or superseded
- WHEN it is cancelled, times out, or returns stale
- THEN it changes neither review nor completion state and persists nothing
