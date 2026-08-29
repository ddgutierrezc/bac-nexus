# Delta for nexus-configuration

## ADDED Requirements

### Requirement: Canonical Local Mapepire Preparation

Step 4 MUST perform only local resolve, acquire, verify, cache, and ready/not-ready evaluation. `prepared` or `ready` means only that a verified local artifact handle exists; it MUST NOT imply upload, launch, Java validation, IBM i connection, or operational readiness. The flow MUST expose deterministic outcomes such as unavailable, rejected, and blocked without prescribing UI layout.

#### Scenario: Valid cache prepares locally
- GIVEN a valid policy-matching cache entry
- WHEN Step 4 runs
- THEN it returns a stable verified handle without IBM i, SSH, credential, upload, or launch activity

#### Scenario: Verification precedes use
- GIVEN a candidate is acquired from any approved source
- WHEN Step 4 processes it
- THEN no upload or launch occurs before verification succeeds

#### Scenario: Step 4 has no remote activity
- GIVEN an operator completes Step 4
- WHEN local preparation runs
- THEN it performs zero IBM i network/authentication, SSH, IFS, Java, Mapepire, or remote lifecycle operations

### Requirement: Explicit Not-Ready Profile Lifecycle

Profile creation MAY continue without a verified artifact, but it MUST persist an explicit not-ready state. Every later Mapepire-dependent operation MUST fail closed until readiness is restored. A later remote lifecycle is separate, explicit, consented, and may consume only a verified handle. Approval pending is an external deployment gate, not a normal Step 4 legal-workflow state.

#### Scenario: Wizard continues without an artifact
- GIVEN no provider yields a verified artifact
- WHEN profile creation completes
- THEN the profile is saved as explicitly not-ready and the wizard remains usable

#### Scenario: Dependent work is blocked
- GIVEN a profile is not-ready
- WHEN a Mapepire-dependent operation is requested
- THEN it returns blocked and performs no remote or execution action

### Requirement: Legacy Artifact Compatibility

An absolute legacy `MapepireJAR` remains readable, but MUST be reverified and imported into the Nexus cache before use. Missing or invalid paths MUST produce not-ready. New profile semantics MUST NOT persist mutable cache paths.

#### Scenario: Legacy path is valid
- GIVEN a readable absolute legacy artifact satisfies the pinned policy
- WHEN it is used
- THEN it is verified, imported, and represented by a stable handle

#### Scenario: Legacy path is invalid
- GIVEN the legacy path is missing, replaced, linked, oversized, or has a wrong digest
- WHEN readiness is evaluated
- THEN the profile is not-ready and the artifact is never used

## MODIFIED Requirements

### Requirement: Honest Readiness and Diagnostics

Local readiness MUST never contact IBM i and MUST report canonical local Mapepire preparation only as ready when a verified artifact handle exists. It MUST preserve honest not-ready and blocked outcomes. Remote diagnostics MUST be explicitly initiated, warned, timed, cancellable, sanitized, and auditable; they MUST retain `ready_for_controlled_ibmi_validation` and `not_validated_on_ibmi`. Configured Java MUST remain distinct from Checked Java, and Java/remote validation MUST remain outside Step 4.
(Previously: Java, Mapepire, and JAR checks MAY appear only as legacy diagnostics.)

#### Scenario: Local readiness
- GIVEN no remote action is selected
- WHEN readiness is refreshed
- THEN it performs local checks only and reports the verified-artifact or not-ready outcome

#### Scenario: Cancel remote diagnostic
- GIVEN a warned diagnostic is running
- WHEN the operator cancels or its timeout expires
- THEN it stops, records a sanitized outcome, and makes no validation claim

#### Scenario: Configured is not Checked
- GIVEN Java is configured but no remote validation has run
- WHEN local readiness is shown
- THEN it does not claim Checked Java or Mapepire operational readiness
