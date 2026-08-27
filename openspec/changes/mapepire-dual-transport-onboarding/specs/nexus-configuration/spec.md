# Delta for nexus-configuration

## MODIFIED Requirements

### Requirement: Honest Readiness and Step 8 Orchestration

Local readiness SHALL never contact IBM i and SHALL distinguish pre-auth observations from authenticated session and validated fixed proof. Pre-auth `/version` observation SHALL be separate from post-credential orchestration. The endpoint SHALL default to managed port `8076`; only approved deployment policy MAY override it, and free-form user endpoints MUST be rejected. A saved-profile-only Step 8 service SHALL return typed success/failure classifications and SHALL retain `ready_for_controlled_ibmi_validation` plus `not_validated_on_ibmi`.
(Previously: readiness described local diagnostics and a resolver gap but not the production Step 8 result contract.)

#### Scenario: Endpoint policy
- GIVEN no endpoint override, an approved override, or a free-form override
- WHEN configuration resolves the daemon endpoint
- THEN it uses `8076`, accepts only the approved override, and rejects the free-form value

#### Scenario: Pre-auth is not proof
- GIVEN `/version` succeeds before credentials
- WHEN readiness is rendered
- THEN it reports authentication pending, not authenticated or query-ready

#### Scenario: Typed terminal matrix
- GIVEN WSS success, eligible availability, unsupported version, identity/protocol failure, credential/authorization failure, cancellation, or limit failure
- WHEN Step 8 returns
- THEN it returns the corresponding bounded typed classification and never silently downgrades

#### Scenario: Marker invalidation
- GIVEN a historical timestamp/outcome/proof-revision marker
- WHEN endpoint, policy, or trust changes
- THEN the marker is stale/invalid and a fresh proof remains mandatory

### Requirement: Offline Validation Status

Automated verification SHALL use deterministic fakes or loopback only and SHALL report `not_validated_on_ibmi`; it MUST NOT imply live IBM i access.

#### Scenario: Offline acceptance
- GIVEN all local contract tests pass without IBM i
- WHEN verification reports status
- THEN it records `not_validated_on_ibmi` and no live-validation claim
