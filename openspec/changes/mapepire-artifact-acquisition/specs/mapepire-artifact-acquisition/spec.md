# mapepire-artifact-acquisition Specification

## Purpose

Provide a Mapepire-specific, policy-governed local lifecycle that produces only verified artifacts for later use.

## Requirements

### Requirement: Pinned Policy and Provider Resolution

The policy MUST identify Mapepire Server 2.3.5 from `Mapepire-IBMi/mapepire-server`, release `v2.3.5`, asset `mapepire-server-2.3.5.jar`, SHA-256 `41b1cfa67778ac204426f1dda0b51bd3f45fe3b89c91121d968660140acc0876`, a 64 MiB maximum, expected-size and compatibility metadata, and allowed source kinds. It MUST reject latest, arbitrary URLs, dynamic version selection, 2.3.6, and per-profile policy overrides. Providers MUST be ordered: valid Nexus cache, approved pinned upstream, optional Code for IBM i, then explicit manual input. Availability fallback MAY be policy-defined; integrity, size, or policy rejection MUST be terminal. Providers MUST NOT determine policy.

#### Scenario: Pinned upstream succeeds
- GIVEN the approved deployment gate enables the exact 2.3.5 upstream asset
- WHEN the pinned remote provider acquires it
- THEN the candidate proceeds to the common verifier and no 2.3.6 or arbitrary source is considered

#### Scenario: Cache miss advances in order
- GIVEN the required artifact is absent from the Nexus cache
- WHEN resolution runs
- THEN it evaluates the next approved provider in order and never treats absence as a security rejection

#### Scenario: Network unavailability uses allowed fallback
- GIVEN the approved remote source is unavailable and policy defines a local fallback
- WHEN resolution runs
- THEN only that fallback is attempted and the outcome remains explicit

#### Scenario: Security rejection stops resolution
- GIVEN a candidate has a digest mismatch or size violation
- WHEN verification rejects it
- THEN resolution returns rejected and MUST NOT silently try another provider

#### Scenario: Code for IBM i is replaceable
- GIVEN Code for IBM i supplies a candidate
- WHEN the candidate is accepted or rejected
- THEN policy and domain behavior are unchanged if the adapter is later replaced by a BAC repository adapter

### Requirement: Common Verification and Verified Artifact Contract

Every candidate MUST pass one verifier: approved descriptor, bounded acquisition, expected/max size, regular-file and basic JAR sanity checks, then pinned SHA-256. Nothing unverified may be published, uploaded, launched, or executed. A ready result MUST contain policy identity, version, asset identity, digest, size, operational source, and a stable verified handle/reference; a mutable path alone is insufficient and MUST NOT permit verification-to-use TOCTOU. Signed manifests, provenance, release signatures, and internal attestations are future hardening, not V1 requirements.

#### Scenario: Valid Code for IBM i candidate is imported
- GIVEN Code for IBM i supplies valid compatible bytes
- WHEN common verification succeeds
- THEN Nexus imports the bytes into its cache and returns a verified handle

#### Scenario: Invalid Code for IBM i candidate fails closed
- GIVEN Code for IBM i supplies missing, linked, malformed, incompatible, or corrupt bytes
- WHEN common verification runs
- THEN the candidate is rejected and no cache, upload, or execution occurs

#### Scenario: Manual input is explicit
- GIVEN an operator explicitly selects a manual artifact
- WHEN common verification succeeds
- THEN it may become ready; implicit manual fallback is prohibited

### Requirement: Private Verified Cache

Nexus MUST use a private per-user OS cache namespaced by version and digest. It MUST stage partial downloads, enforce acquisition bounds, verify before publication, publish atomically where possible, coordinate cross-process acquisition, reject corruption, reverify on open/use, and allow multiple versions to coexist. It MUST NOT use `%TEMP%` for production storage or persist a profile cache path. V1 MUST NOT require complex LRU/GC or multi-user cache coordination.

#### Scenario: Cache and concurrency are safe
- GIVEN a valid cache entry or concurrent acquisitions of one identity
- WHEN resolution runs
- THEN a valid handle is returned without network, or processes converge on one verified publication

#### Scenario: Corruption or interruption is harmless
- GIVEN a corrupt cache entry or interrupted partial download
- WHEN the cache is opened or acquisition retries
- THEN it is rejected or ignored, never returned ready, and no partial file is published

#### Scenario: Versions coexist
- GIVEN verified 2.3.5 artifacts with different approved digests exist
- WHEN either is opened
- THEN each remains separately addressable and reverified by its stable identity
