# Delta for nexus-configuration

## MODIFIED Requirements

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
