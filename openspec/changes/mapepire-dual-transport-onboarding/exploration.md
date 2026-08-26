## Exploration: mapepire-dual-transport-onboarding

### Current State

BAC Nexus currently implements only wizard Steps 1–3. Step 1 and Step 2 collect local profile and endpoint drafts; Step 3 performs an explicit, bounded, no-auth SSH host-key inspection and retains only an unverified TOFU candidate in memory. There is no Step 4 screen, no transport resolver, and no authenticated Mapepire onboarding path. The conceptual ordering remains Steps 1–9, with credentials at Step 6.

The current Mapepire boundary is single-mode and newline-oriented:

- `internal/mapepire/protocol.go` encodes JSON requests with a trailing LF and `DecodeFrame` reads LF-delimited responses.
- `internal/mapepire/session.go` serializes one exchange at a time, expects responses immediately after each request, and closes a fresh session after one prepared query. It currently sends `connect`, `prepare_sql_execute`, `sqlclose`, and `exit`; it does not implement `getversion`, `ping`, or `sqlmore`.
- `internal/remote/ssh.go` authenticates with password, starts a fixed `java -jar <remote> --single` command, and exposes the resulting stdin/stdout channel. This is the reusable SSH-single execution seam, but it is authenticated and therefore cannot prove Mapepire availability before Step 6 credentials.
- `internal/connectors/ibmi/mapepirestdio` owns the pinned 2.3.5 artifact policy, Code for IBM i 3.0.12/VS Code discovery, local verification, remote upload/rollback, and Java/`--single` launch policy. These are SSH-single lifecycle concerns, not daemon onboarding concerns.
- `internal/profile.Profile` persists an absolute `MapepireJAR` path and `vault|prompt` credential mode. Native keyring storage exists below the configuration layer, but the wizard does not yet compose credential collection or persistence.
- `internal/configuration/readiness.go` only provides local composition-gap reporting and a separately invoked sanitized diagnostic. It preserves `ready_for_controlled_ibmi_validation` and `not_validated_on_ibmi`; it does not establish authenticated Mapepire readiness.

Official evidence was re-checked against the supplied sources. Protocol commit `2ef44166fcb515744fb922b49ed3673b2dac6b26` has no stable release tag and defines string `id`/`type` requests, echoed IDs, `success`, and asynchronous responses that may arrive out of order. The server documentation confirms LF-delimited JSON for SSH single mode and describes the same application dispatch for daemon WebSocket mode. The server guide documents daemon TLS and default port `8076`; `/version` is an unauthenticated HTTPS-level endpoint in the proposed probing model, while database authorization is proven by `connect`. The JS repository documents WebSocket as the default persistent-daemon mode and SSH single as a separate transport, but its released `v0.6.1` maturity must not be assumed to include the current unreleased transport quality. `mapepire-go` remains a secondary comparison and does not supply the required SSH, pinning, or context boundary.

The approved direction therefore changes the unit of onboarding from “obtain a local JAR” to “resolve a trusted, usable transport automatically”:

1. Preferred: managed Mapepire daemon over WSS at port `8076`.
2. Fallback: SSH single (`java -jar ... --single`) only when the daemon is unavailable or unusable for an availability/policy/verified-unsupported-version reason, and only when an independent SSH trust policy permits it.
3. No user choice among daemon, SSH, JAR, or Java. Nexus owns resolution.
4. A small custom Go client owns only the required protocol subset and sits above transport adapters; there is no runtime dependency on `mapepire-go`.

### Affected Areas

- `internal/mapepire/protocol.go`, `session.go`, and tests — split application protocol from framing/transport, support WSS message framing and SSH LF framing, correlate out-of-order responses, add bounded `getversion`, `connect`, `prepare_sql_execute`, `sqlmore`, `sqlclose`, `ping`, and `exit`, and document session-close cancellation semantics.
- `internal/remote/ssh.go`, `hostidentity.go`, and `internal/hostidentity/` — retain authenticated SSH single execution, but add a transport-neutral SSH trust/pinning/TOFU identity contract distinct from the current host-key-only UI adapter.
- New Mapepire transport/client packages (exact package names for proposal/design) — provide WSS with TLS trust/pinning/TOFU and SSH-single channel adapters without leaking transport details upward.
- `internal/profile/profile.go` and configuration composition — represent transport-neutral policy and transport-specific trust identity without persisting secrets or a derived cache path; reconcile existing `MapepireJAR` compatibility only where the SSH fallback requires it.
- `internal/tui/model.go`, `profile_identity_step.go`, and the future Step 4 renderer — redesign Steps 3 and 4 together, reuse the existing panel/focus/feedback/viewport rules, and keep presentation free of protocol, credential, and connector logic.
- `internal/configuration/readiness.go` and configuration services — distinguish endpoint/TLS/protocol observations from authenticated `connect` success and from later controlled IBM i validation.
- `internal/connectors/ibmi/mapepirestdio/` — retain current artifact verification, remote activation, Java path validation, and `--single` launch as fallback dependencies; do not make daemon onboarding depend on them.
- `internal/security/`, `internal/audit/`, and valid `local-mcp-security` requirements — add allowlisted transport capabilities, trust outcomes, fallback classifications, bounded audit metadata, and fail-closed downgrade rules without exposing hosts, paths, credentials, certificates, SQL, or raw errors.
- `openspec/specs/nexus-configuration/spec.md` — exact conflict: its Honest Readiness and Diagnostics requirement currently allows Java/Mapepire/JAR checks only as legacy diagnostics. A future delta must modify that requirement while preserving credential isolation, explicit diagnostics, and no-live-validation claims.
- `openspec/specs/local-mcp-security/spec.md` — exact relationship: its current surface forbids arbitrary SSH/SQL/shell/remote operations and requires pinned trust, native credential isolation, sanitized audit, and no live-validation claims. The new capability must extend these constraints, not weaken them.
- `docs/IBM_I_PROFILE_WIZARD.md` — change the Step 3/4 proposal only after specification approval; preserve the FACT that current Steps 4–9 are not implemented and preserve the nine-step numbering.
- `openspec/changes/mapepire-artifact-acquisition/` and archived changes — historical planning and completed-slice evidence only. The old unimplemented change has 0 applied lines and must remain untouched while its exact reusable and superseded portions are mapped.

### Approaches

1. **Supersede with a dual-transport onboarding change** — replace the old artifact-first plan with transport-neutral protocol/client contracts, WSS preference, security-safe SSH-single fallback, and coordinated Step 3/4 onboarding.
   - Pros: matches the approved architecture; prevents a local artifact from being mistaken for daemon availability; keeps upper layers transport-neutral; makes downgrade and identity rules explicit.
   - Cons: requires a new protocol/session design, two trust models, asynchronous correlation, and a careful compatibility boundary for old SSH artifact behavior.
   - Effort: High

2. **Amend `mapepire-artifact-acquisition` in place** — append daemon transport and Step 3/4 changes to the existing artifact lifecycle plan.
   - Pros: preserves prior planning references and some cache/provider work.
   - Cons: leaves artifact acquisition as the conceptual center, mixes local and remote readiness, obscures the fact that daemon mode skips JAR/Java/SSH, and makes the old task split materially misleading.
   - Effort: High

3. **Keep the old artifact change and add transport work beside it** — treat daemon onboarding as a later independent change.
   - Pros: minimal change to existing documents.
   - Cons: permits conflicting Step 4 contracts, duplicates readiness semantics, and risks applying the old artifact-first plan before the new transport decision is encoded.
   - Effort: High

### Recommendation

Use **Approach 1: supersede**, without editing, deleting, or archiving `mapepire-artifact-acquisition` yet. The new change should be the authoritative planning surface; the old change should be retained as an audit trail of the interrupted artifact-first direction. Its reusable implementation ideas are limited to the SSH-single fallback boundary: pinned artifact verification, private cache/handle concepts if later approved, remote upload rollback, fixed Java validation, and `StartMapepire` process lifecycle. Its provider order, Step 4 local-ready meaning, mandatory JAR lifecycle, GitHub/provider assumptions, and six-slice task plan are superseded.

The minimum coherent capability boundary should be:

- **Mapepire application protocol/client:** typed subset only; request IDs are unique; response correlation is independent of send order; bounded frame/message/result sizes; no pooling initially.
- **Transport adapters:** WSS text-message framing and TLS identity policy; SSH-single LF framing over an authenticated process channel. Neither adapter owns business operations.
- **Transport resolver:** daemon first, then fallback only for availability, policy, or verified unsupported-version classifications. Identity mismatch, trust failure, protocol tampering, credential failure, and unsafe downgrade are terminal and MUST NOT trigger fallback.
- **Onboarding identity:** Step 3 must capture either trusted daemon TLS certificate identity or trusted SSH host-key identity under approved manual verification, TOFU, or pinning policy. The operator must never select the mechanism; the policy chooses the permitted path.
- **Step 4 readiness:** show a truthful pre-auth state and trigger only bounded, transport-appropriate probes. It may report a trusted endpoint and observed protocol version, but “Mapepire session ready” requires authenticated `connect` and belongs after credentials or to the later optional test boundary.
- **SSH fallback acquisition:** keep full artifact acquisition/upload/Java work out of this change unless proposal approval explicitly makes it a separately bounded dependency. The new change should define the fallback contract and blocked outcome, not silently pull the old artifact plan into daemon onboarding.

### Truthful Step 3/4 and Credential Ordering

| Point in flow | Daemon WSS | SSH single fallback |
|---|---|---|
| Before credentials | After TLS trust, Nexus may prove endpoint identity and possibly `/version`/`getversion`; this proves endpoint/protocol presence only, not IBM i credentials, authorization, or Db2 job. | Nexus may inspect/pin the SSH host key without authentication. It cannot prove Mapepire protocol availability before SSH authentication, approved artifact availability, Java launch, and process start. |
| Step 3 result | “Daemon endpoint identity observed/trusted” or equivalent; never “connected.” | “SSH server identity observed/trusted” or equivalent; never “Mapepire available.” |
| Step 4 result | “Daemon endpoint reachable,” “protocol version observed,” “authentication required,” or “pending authenticated check.” | “Fallback permitted but not verified,” “requires authenticated SSH and approved runtime,” or “blocked”; never “ready.” |
| After credentials | `connect` can prove authenticated Mapepire/Db2 session and job; this is the first honest session-ready result. | SSH authentication, approved artifact/runtime, launch, and `connect` can prove a session; each sub-result must remain explicit. |

The current conceptual order collects credentials at Step 6, so Step 4 cannot honestly claim authenticated Mapepire readiness. No order change is recommended during exploration: redesign Steps 3 and 4 together, retain credential collection at Step 6, and reserve authenticated proof for Step 8’s explicit optional test or a later post-credential operation. If product requires Step 4 to establish a live session, the nine-step ordering must be explicitly changed rather than hidden behind UI wording.

`/version` may be used without credentials only after TLS identity is accepted under policy, with bounded timeout, response size, supported-version checks, and no claim beyond endpoint/version observation. An unauthenticated HTTP response must not bypass certificate trust or authorize fallback. A TLS identity mismatch or downgrade attempt is a security failure, not daemon unavailability.

Session cancellation must close the local transport/session when necessary. Because the subset exposes no per-query cancellation operation, the product must state that cancellation stops local waiting and may close the entire session; it must not claim remote statement cancellation. `sqlmore` must remain bounded by row, byte, deadline, and session limits.

### Risks

- Current LF-only framing and serialized exchange logic are unsafe for WSS text messages and out-of-order asynchronous responses; adapting only the socket would create correlation and truncation defects.
- The daemon server’s current WebSocket upgrade check reportedly validates only Basic-header shape; TLS identity and authenticated `connect` must remain Nexus-side gates, not assumptions about the upgrade handler.
- A trusted TLS endpoint can still reject credentials or Db2 authorization; endpoint/version proof must never be represented as operational Mapepire readiness.
- SSH single cannot perform protocol work before SSH authentication and Java/JAR launch, so a pre-credential Step 4 fallback label must remain explicitly unverified.
- Automatic fallback can become a silent security downgrade. Trust-domain mismatch, certificate/host-key mismatch, protocol tampering, credential failure, and unsupported-but-unverified versions must be classified separately from availability.
- Supported Mapepire server versions and protocol revision pinning are not yet approved. The protocol repository has no stable release tag at the researched commit, so “latest” behavior is unsafe.
- Existing `MapepireJAR` persistence and old `vault|prompt` semantics are compatibility hazards; daemon mode must not require either, while SSH fallback may require an approved artifact lifecycle.
- WSS dependency and certificate handling add a Go dependency/security review boundary; custom code must remain small and avoid adopting the incomplete `mapepire-go` SDK.
- Step 4 could overclaim “available” before `connect`; copy and readiness vocabulary need product approval and runtime tests at the wizard `View()` boundary.
- No real IBM i contact or live credential proof is permitted in this exploration; all automated evidence must use fakes/loopback and retain `not_validated_on_ibmi`.

### Product Decisions Required Before Proposal

1. Which Mapepire server versions and protocol revision are supported, and what exact compatibility policy replaces the unavailable stable protocol tag?
2. Is daemon port `8076` fixed in V1, or may deployment policy supply an approved endpoint/port without making it a user transport choice?
3. Which daemon TLS trust modes are approved: enterprise CA validation, independently verified certificate pinning, explicit TOFU, or a defined combination? What is the certificate identity representation and rotation procedure?
4. Which SSH trust modes are approved for fallback: independent fingerprint, pinned known-host material, TOFU, or a defined combination? Is SSH fallback prohibited when only unverified trust exists?
5. Is unauthenticated `/version` permitted in the target environments, and does policy require an explicit operator action in Step 4 before probing it?
6. What exact vocabulary is approved for pre-auth states: “endpoint reachable,” “server identity trusted,” “protocol detected,” “authentication pending,” and “session ready” need product-level definitions.
7. Does Step 4 remain before Step 6 credentials, with authenticated proof deferred to Step 8, or is a change to the conceptual nine-step ordering approved?
8. What counts as daemon unusable and fallback-eligible, and which failures are terminal security failures that MUST block all fallback?
9. Is SSH fallback artifact acquisition part of this change, or a separate dependent capability with its own approval, cache, licensing, upload, Java, and rollback contract?
10. What are the local bounds for frame size, response size, rows, `sqlmore` paging, concurrent requests, session lifetime, and cancellation deadlines?
11. Is the initial client strictly single-session/no-pooling, and which protocol subset is approved beyond `getversion`, `connect`, `prepare_sql_execute`, `sqlmore`, `sqlclose`, `ping`, and `exit`?
12. Which transport and fallback facts may be persisted in profile state, and which must remain ephemeral observations? No secrets, certificate material beyond approved fingerprints, cache paths, or raw endpoint diagnostics should enter unsafe stores.

### Ready for Proposal

**No — pending product/security answers above.** The architecture is sufficiently clear to supersede the old planning change, but proposal should wait for supported-version policy, daemon/SSH trust modes, fallback classifications, readiness vocabulary, Step 4 versus Step 6 proof boundary, and ownership of SSH fallback artifact acquisition. Once answered, the proposal should authorize the smallest transport-neutral contract slice first and explicitly state that the old artifact change is superseded, not applied as-is.

### Evidence Index

- `internal/mapepire/protocol.go`, `internal/mapepire/session.go`, and package tests — current single-mode framing, lifecycle, bounds, and correlation limitations.
- `internal/connectors/ibmi/mapepirestdio/policy.go`, `discovery.go`, and `artifact.go` — pinned 2.3.5 policy, local discovery, Java/`--single`, upload, verification, rollback, and reusable SSH fallback mechanics.
- `internal/remote/ssh.go`, `internal/remote/hostidentity.go`, `internal/hostidentity/inspection.go` — authenticated SSH, no-auth host-key inspection, and current transport-specific UI seam.
- `internal/tui/profile_connection_step.go`, `profile_identity_step.go`, and `model.go` — implemented Steps 2–3 and transport-free draft messages; no Step 4 transition.
- `internal/profile/profile.go`, `internal/configuration/readiness.go`, and `internal/configuration/service.go` — persisted profile fields, native credential boundary, and honest readiness/diagnostic contracts.
- `openspec/specs/nexus-configuration/spec.md` and `openspec/specs/local-mcp-security/spec.md` — valid current requirements and exact legacy-diagnostic/security conflicts.
- `openspec/changes/mapepire-artifact-acquisition/{proposal.md,exploration.md,design.md,tasks.md}` — unimplemented artifact-first plan; no edits made and no lines applied.
- `docs/IBM_I_PROFILE_WIZARD.md` — current FACT/PROPOSAL distinction and conceptual credential ordering.
- Official references: [mapepire-protocol commit](https://github.com/Mapepire-IBMi/mapepire-protocol/tree/2ef44166fcb515744fb922b49ed3673b2dac6b26), [mapepire-server](https://github.com/Mapepire-IBMi/mapepire-server), [mapepire-js](https://github.com/Mapepire-IBMi/mapepire-js), [mapepire-go](https://github.com/Mapepire-IBMi/mapepire-go), [server guide](https://mapepire-ibmi.github.io/guides/sysadmin/), and [Node.js usage](https://mapepire-ibmi.github.io/guides/usage/nodejs/).
