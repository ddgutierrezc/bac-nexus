# Delta for nexus-configuration

## ADDED Requirements

### Requirement: Shared Responsive Profile Screens

Direct Create and metadata Edit MUST use the shared BAC header, footer, and centered responsive panel. Create MUST present host and username with one **Connect and Save** action; the password boundary SHALL remain terminal-only and transient. Entry, running, failure, cleanup-required, success, and Edit states MUST expose truthful, scoped status and reachable actions.

#### Scenario: Direct create lifecycle
- GIVEN an operator opens Create with no operation running
- WHEN they submit valid host and username and complete the password prompt
- THEN the same screen transitions through running to success, failure, or cleanup-required feedback

#### Scenario: Edit lifecycle
- GIVEN an operator opens profile metadata Edit
- WHEN they save or cancel
- THEN the panel shows the resulting state or returns without applying edits

### Requirement: Field-Specific Create Validation and Feedback

Create MUST use authoritative host and username validation to show field-specific feedback. An invalid **Connect and Save** action MUST remain focusable, block the operation, and focus the first invalid field in validator order. Operation errors MUST take precedence over explicit and validation feedback; local validation MUST clear when its field or context changes.

#### Scenario: Invalid create submission
- GIVEN host and username contain invalid values
- WHEN the operator activates Connect and Save
- THEN no password prompt or operation starts and focus moves to the first invalid field

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

Create and Edit MUST render in fixed BAC shell/chrome with a responsive centered panel and lossless content wrapping. Rendering MUST retain persistent, truthful overflow and reachability at 120x40, 80x24 with `NO_COLOR`, and 40x16. Spanish and English feedback MUST have equivalent meaning; semantic status MUST be understandable independently of color, and `NO_COLOR` output MUST contain no ANSI escape sequences.

#### Scenario: Narrow runtime frames
- GIVEN each supported viewport, including 40x16
- WHEN Create or Edit contains feedback exceeding the visible area
- THEN controls, focus, feedback, and exit remain reachable through truthful overflow without lost text

#### Scenario: No-color localized feedback
- GIVEN English or Spanish and `NO_COLOR` are selected
- WHEN a field validation error renders
- THEN equivalent actionable feedback renders without ANSI escape sequences

## MODIFIED Requirements

### Requirement: Direct Secure Onboarding

The persistent Bubble Tea form SHALL contain only host, IBM i username, and **Connect and Save**. Selection SHALL prompt terminal-only transient password capture below Bubble Tea; together they SHALL request exactly those three values in one logical flow without a configuration step. Infrastructure terms and advanced controls SHALL NOT appear; bounded support MAY expose them after failure. Wizard steps, progress, drafts, password model/message/view state, proof semantics, and proof reruns MUST NOT be restored.

(Previously: The direct form prohibited configuration steps and advanced controls but did not explicitly prohibit retired wizard and proof semantics.)

#### Scenario: Connect and save
- GIVEN valid host and username
- WHEN Connect and Save is selected and the prompt returns a password
- THEN onboarding completes in the same logical flow

#### Scenario: Password prompt cannot proceed
- GIVEN password prompt failure, cancellation, or unsupported terminal
- WHEN Connect and Save is selected
- THEN it fails closed and starts or persists no operation

#### Scenario: Retired semantics remain absent
- GIVEN any Create or Edit runtime state
- WHEN the UI renders, receives messages, or records output
- THEN it contains no wizard/proof semantics and no secret in models, messages, views, or logs
