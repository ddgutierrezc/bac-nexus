# Delta for mapepire-ssh-fallback-runtime

## MODIFIED Requirements

### Requirement: Managed Verified Runtime Boundary

Only an eligible, policy-authorized, independently trusted, credentialed, explicitly consented SSH fallback MAY acquire the pinned release-owned artifact, validate Java, upload to a bounded temporary location, verify it, and launch fixed `--single`. It MUST support rollback and complete cleanup for artifact, Java, upload, launch, session, proof, cancellation, and partial failures. It MUST NOT expose arbitrary SSH/SFTP/command/SQL primitives or silently retry another transport.
(Previously: artifact and runtime actions were described as later consented operations without a complete failure lifecycle.)

#### Scenario: Managed runtime succeeds
- GIVEN all fallback gates pass and the artifact and Java checks succeed
- WHEN bounded upload, verification, fixed launch, authenticated session, and fixed proof complete
- THEN bounded proof metadata is returned and all temporary resources are removed

#### Scenario: Consent is absent or declined
- GIVEN fallback is otherwise eligible
- WHEN consent is absent or declined
- THEN no credential, artifact, Java, upload, launch, or SSH runtime action occurs

#### Scenario: Each runtime stage fails safely
- GIVEN artifact verification, Java validation, upload, launch, session, proof, or cleanup fails
- WHEN the stage reports failure
- THEN the result is terminal and sanitized, rollback/cleanup covers every acquired resource, and no silent alternative starts

#### Scenario: Daemon remains independent
- GIVEN authenticated WSS proof succeeds
- WHEN onboarding completes
- THEN SSH, artifact, Java, upload, and cache-runtime calls remain zero

## ADDED Requirements

### Requirement: Fixed Artifact and Upload Bounds

The runtime SHALL use only the pinned verified artifact handle and bounded temporary upload policy; partial, changed, corrupt, latest, or concurrent unsafe artifacts MUST be rejected.

#### Scenario: Unsafe artifact is blocked
- GIVEN an unpinned, corrupt, partial, changed, or unverified artifact
- WHEN fallback prepares runtime
- THEN it fails before remote mutation
