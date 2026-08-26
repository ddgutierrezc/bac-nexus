# mapepire-ssh-fallback-runtime Specification

## Purpose

Define the separate SSH fallback runtime dependency without coupling daemon onboarding to artifacts.

## Requirements

### Requirement: Verified Artifact and Runtime Boundary

Only SSH fallback MAY use the pinned Mapepire Server 2.3.5 artifact policy, private cache and stable verified handle, approved source policy, optional Code for IBM i candidate, and explicit manual candidate. No latest, corruption, partial publication, TOCTOU reuse, or unsafe concurrency is permitted. Later authenticated SSH MAY upload a verified artifact, verify it remotely, roll back safely, validate approved Java, and launch `--single`; every remote action requires later consent. Managed daemon MUST NOT invoke artifact acquisition, JAR, Java, SSH, or upload.

#### Scenario: Artifact lifecycle is safe
- GIVEN a valid, corrupt, partial, concurrent, or changed candidate
- WHEN verification/cache publication runs
- THEN only one verified stable handle can proceed; unsafe candidates are blocked

#### Scenario: Runtime action is consent-separated
- GIVEN fallback is eligible and credentials are available
- WHEN explicit later consent is granted
- THEN controlled upload, verification, rollback, Java validation, and `--single` launch may occur

#### Scenario: Daemon path stays independent
- GIVEN WSS resolves successfully
- WHEN onboarding completes Steps 3–4
- THEN no artifact, Java, SSH, or upload dependency is invoked
