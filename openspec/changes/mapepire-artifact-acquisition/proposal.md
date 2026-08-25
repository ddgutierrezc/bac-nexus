# Proposal: Mapepire Artifact Acquisition

## Intent

Make Step 4 the canonical local Mapepire preparation lifecycle: **resolve → acquire → verify → cache → ready**. This removes mandatory Code for IBM i coupling while preserving a fail-closed, auditable artifact boundary. Wizard completion remains distinct from Mapepire readiness.

## Scope

### In Scope
- A Mapepire-specific policy, resolver, verifier, private per-user cache, and stable verified artifact handle for pinned Mapepire Server **2.3.5**.
- Approved provider order: valid Nexus cache; pinned official upstream adapter; optional Code for IBM i; explicit manual artifact. Providers supply candidates, never policy.
- Exact release/asset, SHA-256, expected/max size, regular-file/sanity checks, staging, locking, corruption rejection, and atomic publication where possible.
- Explicit profile not-ready state; legacy absolute `MapepireJAR` remains readable, is reverified, and is imported into cache before use.
- Sanitized artifact lifecycle outcomes and audit metadata: source kind, policy identity, version, digest, verification result, and sanitized error class.

### Out of Scope
- IBM i contact, credentials, SSH, upload, IFS work, Java checks, Mapepire launch, or Step 4 UI design.
- Generic artifact infrastructure, cache LRU/GC, multi-user caching, signatures/provenance/attestation, source/version/hash overrides, `latest`, or arbitrary URLs.
- Adoption of Mapepire 2.3.6 or treating workstation GitHub evidence as production approval.

## Capabilities

### New Capabilities
- `mapepire-artifact-acquisition`: Local, policy-governed acquisition and verified-cache lifecycle for Mapepire Server artifacts.

### Modified Capabilities
- `nexus-configuration`: Canonical Step 4 readiness, explicit not-ready profiles, and legacy `MapepireJAR` compatibility.
- `local-mcp-security`: Artifact policy, private-cache, audit, and fail-closed controls.

## Approach

Keep acquisition Mapepire-specific and separate from `mapepirestdio` remote activation. A deployment policy enables the pinned official GitHub provider only after recorded licensing/compliance/security approval; approval pending is not an operator-facing lifecycle state. Permitted unavailability may use explicit local fallback; digest, size, sanity, or policy rejection is terminal and never silently falls through. GitHub is a replaceable adapter for a future BAC repository.

## Compatibility and Specification Posture

Profiles may be created while not ready; later Mapepire-dependent work MUST fail closed. Cache paths are derived and never persisted. The `nexus-configuration` delta will **modify**, not erase, historical Requirement **Honest Readiness and Diagnostics**: Mapepire artifact preparation becomes canonical local readiness, while Java/remote validation remain separate and no-live-validation guarantees remain intact. Existing CLI/process compatibility and `local-mcp-security` restrictions remain applicable.

## Affected Areas

| Area | Impact | Description |
|---|---|---|
| `internal/connectors/ibmi/mapepirestdio/` | Modified | Reuse/adapt verification and discovery; retain remote lifecycle boundary. |
| `internal/profile/`, `internal/configuration/` | Modified | Not-ready and legacy-path migration semantics. |
| `internal/tui/` | Modified | Honest Step 4 states only; no UI design in this change. |
| `openspec/specs/nexus-configuration/` | Modified | Primary delta. |
| `openspec/specs/local-mcp-security/` | Modified | Security/audit delta. |

## Risks and Approvals

| Risk | Mitigation |
|---|---|
| Mutable path/partial cache | Stable handle, revalidation, locks, staging, atomic publish. |
| Unsafe fallback | Terminal security rejection; explicit availability-only fallback. |
| Upstream productization | Deployment gate requires recorded licensing/compliance/security approval. |

## Rollback Plan

Disable the deployment-policy provider, retain legacy-path reads, and prevent new cache publication; no remote state is changed.

## Success Criteria

- [ ] Only verified pinned 2.3.5 bytes receive a ready handle or enter cache.
- [ ] Step 4 performs no IBM i, SSH, upload, Java, or launch work.
- [ ] Not-ready profiles remain creatable but fail closed for dependent operations.
