# Delta for local-mcp-security

## ADDED Requirements

### Requirement: Deployment-Gated Artifact Source Policy

Each concrete artifact source model MUST have licensing, compliance, and security approval as a deployment/productization precondition; this specification makes no legal conclusion. V1's official upstream provider MAY be enabled only by deployment policy for the exact pinned Mapepire 2.3.5 descriptor. GitHub MUST be an interchangeable adapter, not a domain dependency. A disabled source MUST fail with a sanitized unavailable outcome without exposing approval internals.

#### Scenario: Deployment policy disables a source
- GIVEN the upstream provider is not enabled by deployment policy
- WHEN acquisition would select it
- THEN the source is unavailable and compliance or internal approval details are not exposed

#### Scenario: Adapter replacement preserves behavior
- GIVEN a future BAC repository adapter replaces GitHub
- WHEN the same approved policy is used
- THEN wizard, verifier, cache, profile, and remote lifecycle semantics remain unchanged

### Requirement: Artifact Integrity, Cache, and Execution Boundary

Artifact acquisition MUST use version/digest-namespaced private per-user storage with bounded staging, cross-process coordination, corruption rejection, verification before publication, atomic publication where possible, and re-verification on use. It MUST audit source kind, policy identity, version, digest, size, and verification outcome with sanitized errors. Secrets, sensitive internal repository URLs, raw paths, and artifact contents MUST NOT appear in audit or logs. Rejected artifacts MUST never be uploaded, launched, or executed.

#### Scenario: Audit is sufficient and sanitized
- GIVEN a candidate is accepted or rejected
- WHEN its lifecycle outcome is audited
- THEN the approved identity fields and result are recorded without secrets, sensitive URLs, or raw errors

#### Scenario: Rejected bytes cannot cross the boundary
- GIVEN a candidate fails policy, size, sanity, or digest verification
- WHEN a later remote or execution operation is requested
- THEN it is blocked and no rejected bytes cross the boundary

#### Scenario: Cache publication survives concurrency safely
- GIVEN two processes acquire the same artifact or one download is interrupted
- WHEN publication or recovery runs
- THEN partial data is not ready, the valid publication is singular, and corruption is rejected

### Requirement: Strict Local/Remote Separation

Step 4 MUST NOT access IBM i networks, credentials, SSH, upload, IFS, Java, or Mapepire launch. Later remote activation MUST be a separate explicit, consented operation that consumes only a verified artifact handle. `Existing CLI and Process Compatibility` remains unchanged.

#### Scenario: Local preparation is offline with respect to IBM i
- GIVEN Step 4 is invoked
- WHEN any provider resolves an artifact
- THEN no IBM i network or authentication activity occurs

#### Scenario: Remote use requires verified readiness
- GIVEN a verified handle is absent or invalid
- WHEN upload or launch is requested
- THEN the operation fails closed with a sanitized blocked outcome
