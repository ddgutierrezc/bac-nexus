## Exploration: mapepire-artifact-acquisition

### Current State

BAC Nexus already has several Mapepire-related lower-layer capabilities, but they are not yet one canonical artifact lifecycle. `internal/connectors/ibmi/mapepirestdio` owns the current fixed release policy (`ServerVersion = 2.3.5`, pinned SHA-256 `41b1cfa67778ac204426f1dda0b51bd3f45fe3b89c91121d968660140acc0876`, 64 MiB ceiling), Code for IBM i `3.0.12` discovery under the standard VS Code extensions directory, local regular/non-link checks, and remote activation/rollback. `artifact.go` already protects local identity during upload, stages exclusive remote files, hashes the staged and activated files, preserves a rollback copy, and fails closed on verification or cleanup failures. `policy.go` also owns the Java-home and fixed `--single` launch command policy. `internal/mapepire` remains correctly transport/protocol-oriented and does not own artifact acquisition.

The current discovery contract is source-specific and path-oriented: it can return `found`, `not-found`, or `ambiguous`, with rejected-candidate and inspection-failure information. Manual selection is represented indirectly by `Profile.MapepireJAR`, which is an absolute local path persisted in profile JSON. The profile validator therefore treats the path as configuration, not as a stable verified artifact capability. No private per-user local artifact cache or stable post-verification handle exists.

The current wizard implements Steps 1–3 only. `docs/IBM_I_PROFILE_WIZARD.md` classifies Step 4 as conceptual/proposed and explicitly records that no current Step 4 exists, that discovery is local-only, and that remote activation/launch is not wizard-composed. The approved change direction makes Step 4 canonical, but only for a local lifecycle: resolve → acquire → verify → cache → ready. It must not contact IBM i, authenticate, use SSH, upload, activate, launch Mapepire, or persist a cache path.

The exact current specification conflict is `openspec/specs/nexus-configuration/spec.md`, Requirement **Honest Readiness and Diagnostics**, line 95: “Java, Mapepire, and JAR checks MAY appear only as legacy diagnostics.” That requirement must be modified by a future delta if Step 4 becomes canonical. Its existing Requirement **Existing CLI and Process Compatibility** (line 121) and `openspec/specs/local-mcp-security/spec.md`'s fail-closed, no-arbitrary-remote-operation rules remain applicable. The archived changes `2026-08-21-v1-mcp-foundation` and `2026-08-22-nexus-configuration-tui` are historical evidence, not authority to erase or rewrite current requirements.

### Affected Areas

- `internal/connectors/ibmi/mapepirestdio/policy.go` — current fixed version, digest, size ceiling, remote release path, and Java launch policy; should remain Mapepire-specific rather than become a generic artifact framework.
- `internal/connectors/ibmi/mapepirestdio/discovery.go` and `discovery_test.go` — current Code for IBM i 3.0.12 / VS Code discovery and outcome model; should become one approved provider/adapter, not the policy owner.
- `internal/connectors/ibmi/mapepirestdio/artifact.go` and `artifact_test.go` — reusable verifier and remote activation/rollback mechanics; remote activation must remain a distinct later operation and must not be called by Step 4.
- `internal/profile/profile.go` and profile tests — `MapepireJAR` currently means an absolute local path. Future design must define backward-compatible read/validation behavior and deprecation/migration without persisting the new private cache path in the profile schema.
- `internal/tui/model.go`, current Step 3 seams, and `.agents/skills/bac-nexus-tui/SKILL.md` — future Step 4 composition must be presentation/orchestration only, reuse shared TUI primitives, and represent honest lifecycle states without designing the screen in this exploration.
- `internal/remote/ssh.go` and tests — remains a later authenticated remote transport; Step 4 must have no SSH dependency or IBM i activity.
- `internal/mapepire/session.go` and tests — remains the later generic `--single` protocol/session boundary; artifact readiness must not imply a started session.
- `internal/configuration/readiness.go` and `openspec/specs/nexus-configuration/spec.md` — local readiness and legacy-diagnostic semantics need a future delta that distinguishes local verified-artifact readiness from remote/IBM i validation.
- `internal/security/policy.go`, `internal/audit/audit.go`, `openspec/specs/local-mcp-security/spec.md`, and `docs/SECURITY.md` — policy approval, audit classifications, sensitive-output exclusions, fail-closed behavior, and compliance gates must cover artifact acquisition without exposing paths, URLs, source, credentials, or raw errors.
- `openspec/specs/nexus-configuration/spec.md` — primary future delta location for canonical Step 4 lifecycle, profile compatibility, state semantics, and no-remote-activity constraints.
- `openspec/specs/local-mcp-security/spec.md` — future delta location for artifact security/audit/compliance controls if existing generic security requirements do not fully cover local cache, provider approval, digest/size rejection, and cross-process locking.
- `openspec/specs/ibmi-catalog-context/spec.md` — do not modify for this change unless a later remote-operation delta changes the already bounded IBM i acquisition contract; Step 4 itself is local and should not alter catalog/source requirements.
- `docs/IBM_I_PROFILE_WIZARD.md`, `docs/ARCHITECTURE.md`, `docs/SECURITY.md`, and the relevant operator/validation documentation — later updates must reconcile current FACT/PROPOSAL labels, local/remote boundaries, release approval, cache ownership, and the explicit separation between artifact readiness and Java/IBM i validation.

### Architecture Boundaries

1. **`MapepireArtifactPolicy`** — Mapepire-specific, fixed-release policy for version `2.3.5`, pinned digest, maximum size, sanity/type/link rules, approved source models, licensing/compliance approval precondition, and provider order. It MUST not expose `latest` or silently accept an unapproved upgrade. Corporate evidence that GitHub assets were reachable and that v2.3.6 matched metadata is technical evidence only; it neither approves GitHub nor upgrades policy beyond 2.3.5.
2. **`MapepireArtifactResolver`** — orchestrates approved providers and returns typed outcomes, not a path trusted by later code. Candidate sources should be ordered explicitly: verified private cache first; an interchangeable approved remote source second (GitHub may be only a replaceable technical adapter pending BAC approval); optional Code for IBM i discovery next; explicit manual selection last/only when requested. A security failure (policy, checksum, size, unsafe link/type) MUST stop rather than fall through. Network unavailability MAY use a policy-defined fallback to local cache, but only when the fallback is not a security failure and its outcome is explicit.
3. **Provider adapters** — Code for IBM i is optional and read-only. It may locate a candidate but cannot set policy version or digest. A remote provider supplies bytes plus provenance/metadata to the resolver; the policy, not the provider, decides compatibility. Manual input is an explicit operator-selected source and remains subject to the same policy and verifier.
4. **`MapepireArtifactVerifier`** — opens/reads a bounded regular non-link artifact, checks exact pinned digest and size/sanity, detects corruption or replacement, and returns a verified identity. Nothing rejected or unverified may be cached as ready, uploaded, activated, or executed.
5. **Private cache** — Nexus owns a private per-user OS cache, keyed by approved version/digest, with multi-version coexistence. V1 requires partial staging, bounded writes, cross-process locking, corruption rejection, and atomic publication where the platform permits it. It does not require complex LRU/GC or multi-user coordination. Cache path is derived by Nexus and is not persisted in profile JSON.
6. **Stable verified handle** — the successful result must be an opaque/stable verified artifact reference/handle, not merely a mutable path. The handle must bind the verified artifact identity and a cache-owned immutable/published object sufficiently to prevent a later path replacement from being mistaken for the verified bytes (TOCTOU). Any later operation must revalidate the handle according to its own boundary.
7. **Later remote operation** — upload/activation/rollback and Mapepire launch remain separate authenticated operations. Step 4 produces only a local approved verified artifact and never invokes `EnsureServerJAR`, SSH, Java, `mapepire.Session`, or a remote launch path.

### Required Outcomes and Scenarios for Future Specs

Future delta specs should define deterministic, sanitized outcomes equivalent to `not-resolved`, `checking-cache`, `acquiring`, `verifying`, `ready`, `unavailable`, `rejected`, and `blocked`; they should not prescribe screen layout. Minimum scenarios:

- valid cache hit returns a verified stable handle without network or IBM i activity;
- absent cache acquires from the next approved provider, verifies, atomically publishes, and returns ready;
- corrupt cache is rejected/quarantined and never returned as ready;
- successful approved remote acquisition verifies and caches the pinned 2.3.5 artifact;
- network unavailable follows only the explicitly approved fallback policy and reports an honest unavailable/ready outcome;
- remote/provider digest mismatch fails closed with no cache publication and no fallback after the security failure;
- size or sanity violation fails closed before publication;
- Code for IBM i finds a valid candidate and supplies no version/digest authority;
- Code for IBM i candidate is absent, invalid, linked, ambiguous, or unreadable without unsafe silent fallback;
- explicit manual source is accepted only after the same policy verification;
- concurrent processes serialize acquisition through a lock and converge on one valid published artifact;
- interrupted/partial download leaves no ready artifact and is cleaned or safely ignored on the next attempt;
- no Step 4 path contacts IBM i, authenticates, opens SSH, reads credentials, uploads, activates, launches Java/Mapepire, or performs a remote operation;
- no rejected, unverified, corrupted, or policy-incompatible artifact can be uploaded or executed;
- configured Java remains distinct from checked Java, and a verified local artifact remains distinct from a remote Mapepire test.

### Approaches

1. **Extend `mapepirestdio` directly** — move cache/provider orchestration into the existing IBM i launch package.
   - Pros: reuses current policy, verifier, discovery, and tests with minimal package movement.
   - Cons: couples local acquisition to SSH/stdio/Java and makes Step 4 depend on a later remote operation; weakens the approved local/remote invariant.
   - Effort: Medium

2. **Mapepire-specific local acquisition boundary with adapters** — introduce Mapepire-owned policy, resolver, verifier, cache, and stable-handle concepts; retain `mapepirestdio` as the later IBM i activation/launch implementation and adapt current discovery/verification into providers.
   - Pros: matches the approved architecture, preserves Mapepire-specific ownership, supports replaceable BAC-approved sources, makes TOCTOU and cache lifecycle explicit, and keeps Step 4 offline/local.
   - Cons: requires careful compatibility treatment for persisted `MapepireJAR`, provider approval modeling, and new concurrency/staging tests.
   - Effort: High

3. **Generic enterprise artifact framework first** — create a connector-neutral artifact registry/cache for future ecosystems.
   - Pros: apparent reuse for future connectors.
   - Cons: premature abstraction, broader security/compliance surface, unclear lifecycle semantics, and no current requirement beyond Mapepire.
   - Effort: High

### Recommendation

Choose Approach 2. The future design should add a Mapepire-specific local acquisition capability rather than a generic framework or a remote-launch extension. Make `MapepireArtifactPolicy` the authority for release/digest/size/source approval, keep providers replaceable and non-authoritative, use a Nexus-owned private cache with atomic publication and locking, and return a stable verified handle. Preserve `Profile.MapepireJAR` as a backward-compatible legacy/manual input during migration, but stop treating the persisted path as proof of readiness; future profile semantics should store only an explicit source preference or compatible legacy reference, never the derived cache path.

The proposal/spec phase should decide exact provider fallback semantics, cache root and file naming without exposing them, handle invalidation/revalidation, lock timeout, partial-staging recovery, and whether a legacy path is reverified on every use before being imported into the cache. It should also modify the configuration specification's legacy-diagnostic wording only through a complete future delta, preserving all unrelated requirements and history.

### Risks

- The valid `nexus-configuration` Requirement **Honest Readiness and Diagnostics** currently limits Java/Mapepire/JAR checks to legacy diagnostics; a future delta must explicitly change this without weakening the no-live-validation contract.
- A path-only result is vulnerable to TOCTOU; a stable verified handle and publication/revalidation protocol are mandatory.
- Provider fallback can accidentally turn a security rejection into an unapproved download or manual bypass; security failures must be terminal, while only policy-approved availability fallback may continue.
- Cache bytes are sensitive enterprise software artifacts and require private permissions, bounded disk use, corruption handling, and compliance/licensing approval; no legal conclusion should be inferred from technical reachability evidence.
- GitHub availability and v2.3.6 hash/metadata evidence does not approve GitHub as a source or Mapepire 2.3.6; release policy remains pinned to 2.3.5.
- Existing remote activation/rollback is robust but must remain outside Step 4; accidentally reusing it in the wizard would violate the local-only invariant.
- Profile backward compatibility is non-trivial because `MapepireJAR` currently persists an absolute path; silent migration or path deletion could break existing profiles or conceal provenance.
- Cross-process locking and interrupted publication need platform-specific tests; atomic rename semantics alone do not prove durable publication after power loss.
- Future hardening such as signed manifests, provenance, release signatures, or internal attestation is deliberately deferred unless existing approved specifications require it.

### Documents Impacted Later

- `openspec/changes/mapepire-artifact-acquisition/specs/nexus-configuration/spec.md` — primary delta for canonical Step 4, lifecycle outcomes, local-only invariant, and profile compatibility.
- `openspec/changes/mapepire-artifact-acquisition/specs/local-mcp-security/spec.md` — security/audit/cache/provider deltas if required after exact overlap analysis.
- `docs/IBM_I_PROFILE_WIZARD.md` — change Step 4 from conceptual proposal to approved specification-backed capability; preserve Step 5 Java separation and honest status vocabulary.
- `docs/ARCHITECTURE.md` — document Mapepire-specific policy/resolver/verifier/cache/handle boundaries and retain `mapepirestdio` as the remote activation boundary.
- `docs/SECURITY.md` — document private cache ownership, artifact confidentiality, fail-closed provider behavior, and no upload/launch before verification.
- `docs/IBM_I_VALIDATION.md` and relevant spike/operator guides — update only where local artifact preparation and later remote validation/approval prerequisites are clarified.
- No base specification, Java behavior, MCP catalog contract, or production code should be changed during exploration; future deltas must preserve archived history.

### Ready for Proposal

Yes, conditionally. The architectural direction is sufficiently bounded for `sdd-propose`, provided the proposal carries forward the exact legacy-diagnostics conflict, the local-only Step 4 invariant, stable verified-handle/TOCTOU semantics, pinned 2.3.5 policy, explicit compliance approval gate, and the provider/fallback distinction. Proposal should not authorize implementation, downloads, IBM i access, remote upload, Java/Mapepire launch, or Step 4 screen design.
