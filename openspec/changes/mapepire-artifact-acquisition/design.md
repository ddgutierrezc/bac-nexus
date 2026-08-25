# Design: Mapepire Artifact Acquisition

## Technical Approach

Add a Mapepire-specific local lifecycle: resolve → acquire → verify → atomically cache → return a verified handle. Extract the local regular-file/hash guard in `mapepirestdio.openVerifiedLocalJAR` into its verifier; retain `EnsureServerJAR`, `RemoteFiles`, SSH/SFTP activation, launch policy, and `mapepire.Session` at the remote boundary. Step 4 consumes only the local service; it neither dials nor receives credentials.

## Architecture Decisions

| Decision | Choice | Rejected alternative / rationale |
|---|---|---|
| Ownership | Create `internal/mapepireartifact`; it owns the descriptor, resolver, providers, verifier, cache, outcomes, and handle. `mapepirestdio` retains remote activation/launch. | Generic artifact package: rejected until a second consumer proves shared semantics. |
| Policy | `MapepireArtifactPolicy` hard-codes 2.3.5, `Mapepire-IBMi/mapepire-server`, `v2.3.5`, `mapepire-server-2.3.5.jar`, pinned SHA-256, 64 MiB, expected-size/compatibility metadata and source kinds. | `latest`, overrides, arbitrary URLs, and 2.3.6: non-deterministic or out of scope. |
| Resolution | Sequential providers: verified cache; deployment-gated pinned-upstream adapter; optional Code for IBM i; explicit manual. Only availability/unreachable outcomes may advance to policy-approved fallback; rejected/policy/security outcomes stop. | Provider racing: loses deterministic auditing and may bypass policy. GitHub transport is an adapter, so a BAC repository replacement changes no domain, profile, cache, wizard, or remote contract. |
| Verified use | Return `VerifiedMapepireArtifact` with immutable identity and an opaque cache reference; `Open` reopens and re-verifies the namespaced entry immediately before streaming. | Persisting/exposing a mutable cache path: rejected to prevent verification-to-use TOCTOU. |
| Cache | Private `os.UserCacheDir()/BAC Nexus/mapepire/<version>/<digest>/artifact.jar`; same-volume `partial-*` staging, 0600 files/0700 directories, bounded stream, regular-file/non-link and basic ZIP/JAR sanity, SHA-256, sync, rename, reopen/reverify. Identity-scoped cross-process lock serializes publishers; platform lock/filesystem operations are injected seams. | `%TEMP%`, LRU/GC, and multi-user cache: excluded. Interrupted partials are removed/ignored; corrupt finals are quarantined/removed and never ready; approved versions/digests coexist. |

## Data Flow

```text
Step 4 / configuration service
  -> Resolver(policy, deployment policy, providers)
  -> cache hit OR one candidate provider
  -> Verifier -> Cache.Publish -> Cache.OpenVerified
  -> VerifiedMapepireArtifact(identity, source metadata, opaque reference)
```

Each `MapepireArtifactSource` yields a bounded candidate reader plus operational source kind, never a URL-derived policy. Cancellation/deadlines stop acquisition and clean partials. Transient availability may retry only within its deadline before allowed fallback; operations are idempotent by identity. Outcomes are `ready`, `not_ready/unavailable`, `blocked`, or `rejected`; errors are typed (`cancelled`, `deadline`, `unavailable`, `policy`, `integrity`, `cache`) and sanitized.

## Interfaces / Contracts

```go
type VerifiedMapepireArtifact struct { Identity ArtifactIdentity; Source SourceMetadata; Ref VerifiedRef }
type MapepireArtifactSource interface { Acquire(context.Context, MapepireArtifactPolicy) (Candidate, error) }
type Resolver interface { Resolve(context.Context, ResolveRequest) (VerifiedMapepireArtifact, Outcome, error) }
```

`SourceMetadata` records source kind and approved descriptor identity, never URL, raw path, credentials, or bytes. The audit extension adds allowlisted artifact capability/result/reason classes and version, digest, size, source kind, and verification outcome; current `audit.Event` forbids digest, so it must be evolved deliberately without allowing raw error material.

## Profile and Remote Lifecycle

Version the profile schema explicitly and add artifact readiness state/identity (not a cache path). Old profiles remain readable: an absolute legacy `MapepireJAR` is reverified and imported; missing, linked, replaced, oversized, or mismatched files become not-ready. The wizard may persist not-ready; every dependent operation blocks before SSH, upload, Java, or launch. Configured Java remains distinct from Checked Java.

Future remote activation accepts only the handle, reopens/reverifies it, then adapts its stream to the existing `mapepirestdio.EnsureServerJAR` protections. It uses a digest-scoped Nexus-owned remote path, verifies before activation and before `StartMapepire`/`mapepire.Session`, requires separate explicit consent, and preserves current temporary-upload, backup, rollback, and cleanup semantics. Step 4 has no remote dependency or UI design.

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/mapepireartifact/` | Create | Policy, resolver/providers, verifier, cache/locks, handle, errors, tests. |
| `internal/configuration/`, `internal/profile/` | Modify | Local readiness service and schema migration/state. |
| `internal/connectors/ibmi/mapepirestdio/`, `internal/remote/` | Modify | Later handle-consuming remote adapter; retain launch/protocol boundaries. |
| `internal/audit/` | Modify | Sanitized artifact audit allowlists. |
| `internal/tui/` | Modify | Future state consumption only; no UI design. |
| `docs/ARCHITECTURE.md`, `docs/SECURITY.md` | Modify | Future implementation documentation only. |

## Testing Strategy

| Layer | Coverage | Approach |
|---|---|---|
| Unit | All delta scenarios: descriptor, ordering, gate, terminal rejection, manual explicitness, bounds/JAR/digest, cache corruption/coexistence, legacy migration, audit sanitization | Table-driven fakes, temp directories, injected clock/lock/filesystem/providers. |
| Integration | Atomic publish, concurrent processes, crash partial recovery, reopen TOCTOU, remote adapter rollback | OS-backed temp cache and existing `memoryRemote`; platform lock seam. |
| Boundary | Step 4 performs zero IBM i/SSH/credential/IFS/Java/upload/launch calls; invalid/no handle prevents upload/launch | Counting fakes wired into configuration and remote seams. |

## Threat Matrix

| Boundary | Applicability | Safe behavior / RED test |
|---|---|---|
| Documentation-like paths | N/A — no executable-file classification | N/A |
| Git repository selection | N/A — no VCS operation | N/A |
| Commit state | N/A — no commits | N/A |
| Push state | N/A — no pushes | N/A |
| PR commands | N/A — no PR automation | N/A |

Remote command construction remains in existing fixed `BuildCommand`; this change introduces no shell routing or arbitrary process execution.

## Migration / Rollout

Ship schema reader/writer compatibility first, default legacy/unknown profiles to not-ready, and keep the upstream adapter disabled until recorded licensing/compliance/security approval. Roll back by disabling that provider and new publication; retained cache is inert, legacy reads remain safe, and no remote state changes.

## Open Questions

- [ ] Confirm the approved deployment-policy configuration location and cross-platform file-lock primitive before implementation.
