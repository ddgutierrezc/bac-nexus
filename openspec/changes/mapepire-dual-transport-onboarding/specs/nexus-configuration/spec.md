# Delta for nexus-configuration

## MODIFIED Requirements

### Requirement: Honest Readiness and Diagnostics

Local readiness MUST never contact IBM i and MUST report only recomputed transport observations: trusted identity, endpoint reachability, detected protocol, authentication pending, authenticated session, or validated query. Remote diagnostics MUST be explicitly initiated, warned, timed, cancellable, sanitized, and auditable; they MUST retain `ready_for_controlled_ibmi_validation` and `not_validated_on_ibmi`. Java, Mapepire, JAR, SSH, and upload checks MUST NOT be implied by daemon readiness and MAY occur only in the separately consented SSH fallback runtime.
(Previously: Java, Mapepire, and JAR checks MAY appear only as legacy diagnostics.)

#### Scenario: Local readiness
- GIVEN no remote action is selected
- WHEN readiness is refreshed
- THEN it performs local checks only and reports recomputed non-authenticated state

#### Scenario: Cancel remote diagnostic
- GIVEN a warned diagnostic is running
- WHEN the operator cancels or timeout expires
- THEN it stops, records a sanitized outcome, and makes no validation claim

#### Scenario: Configured is not Checked
- GIVEN Java or an artifact is configured but no authenticated validation ran
- WHEN readiness is shown
- THEN it does not claim Checked Java, authenticated Mapepire, or query readiness

## ADDED Requirements

### Requirement: Ephemeral Transport Observations

Persisted profiles MUST remain secret-free and MUST NOT store selected transport, observed version, readiness, or raw errors; those values MUST be recomputed.

#### Scenario: Restart recomputes readiness
- GIVEN a profile stores policy and approved trust evidence only
- WHEN Nexus restarts
- THEN transport and readiness are unknown until bounded inspection recomputes them
