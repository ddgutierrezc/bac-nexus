# Proposal: Mapepire Dual-Transport Onboarding

## Intent

Replace artifact-first onboarding with automatic, trusted Mapepire transport resolution: managed WSS daemon first (policy endpoint, default `8076`), then independently trusted SSH single only when safe. Preserve nine wizard steps; Steps 3–4 establish identity and pre-auth observation, while Step 6 supplies credentials and Step 8 proves `connect`/optional bounded query.

## Scope

### In Scope
- Small custom, no-pool client: `getversion`, `connect`, `prepare_sql_execute`, `sqlmore`, `sqlclose`, `ping`, `exit`; unique IDs, out-of-order safety, WSS text and SSH LF framing, Nexus-owned versioned bounds. Cancellation closes the session/transport; it does not claim remote statement cancellation.
- Daemon TLS identity (CA/hostname preferred; policy-controlled pin or explicit TOFU), bounded post-trust `/version`, and managed endpoint policy.
- SSH single fallback over authenticated stdin/stdout, with separate known-host/pin or explicit TOFU; one IBM i credential reference serves both transports.
- Coordinated Step 3/4 state semantics, resolver, profile/readiness/audit/security deltas, and SSH artifact/runtime lifecycle only after fallback plus credentials/consent.

### Out of Scope
- User transport/JAR/Java selection, pooling, arbitrary remote operations, production UI layout, downloads, or IBM i contact during this change.

## Capabilities

### New Capabilities
- `mapepire-application-protocol`: Bounded transport-independent client and correlation.
- `mapepire-wss-transport`: Daemon WSS and TLS identity.
- `mapepire-ssh-single-transport`: SSH framing and independent SSH identity.
- `mapepire-transport-onboarding`: Security-aware resolver and Steps 3–4 semantics.
- `mapepire-ssh-fallback-runtime`: Artifact/cache/upload/Java/launch dependency for fallback only.

### Modified Capabilities
- `nexus-configuration`: Replace legacy diagnostic-only Mapepire readiness with truthful transport states.
- `local-mcp-security`: Add trust, downgrade, audit, and bounded remote-capability rules.

## Impact and Migration

| Area | Impact |
|---|---|
| `internal/mapepire/`, transports | Split framing from application protocol. |
| `internal/profile/`, `configuration/`, `audit/`, `security/` | Persist policy/trust only; classify readiness and fallback safely. |
| `internal/tui/` | Redesign Step 3/4 responsibilities while preserving order 1–9; no layout contract. |

Existing `MapepireJAR` remains readable only for SSH fallback compatibility; daemon profiles need neither JAR nor Java. No selected transport or observed readiness/version migrates into profile state.

## Approach and Security

Pin Mapepire Server `2.3.5` and reviewed protocol evidence to exact revisions—never `latest`. Authority order: protocol, server, JS, then Go comparison only. After TLS trust, `/version` may observe endpoint/version, not credentials or Db2. Step 4 reports `[OK] Mapepire detected — authentication pending`; only authenticated `connect` is session-ready.

Fallback is limited to classified availability, policy, or verified unsupported-version failures. TLS/SSH identity failure, downgrade/tampering, and credential/authorization failure are terminal. Persist only endpoint/policy reference, fallback permission, trust mode, and approved TLS/SSH pin evidence; transport, version, readiness, and errors are ephemeral.

## Supersession and Compatibility

This supersedes, but does not alter, unimplemented `mapepire-artifact-acquisition` (zero applied lines). Reuse its pinned 2.3.5 verification, private-cache/stable-handle concepts, upload/rollback, Java validation, and SSH process lifecycle solely as fallback dependencies. Supersede its artifact-first Step 4, provider order/GitHub assumptions, mandatory JAR lifecycle, local-ready meaning, and task plan. Current Step 3 is SSH-only **FACT**; current OpenSpec legacy-diagnostic requirements conflict and require deltas.

## Risks, Rollback, and Success

| Risk / approval | Mitigation |
|---|---|
| Protocol/daemon compatibility | Exact evidence pin and security/dependency review. |
| Unsafe downgrade | Fail closed; audit sanitized classifications only. |
| Trust rotation | Managed policy and explicit re-enrollment. |

Rollback disables the new resolver/capabilities and retains existing profiles and old planning untouched; no remote mutation occurs before explicit fallback consent.

- [ ] Daemon works without JAR/Java/SSH; fallback runs only after eligible classification and trust.
- [ ] Pre-auth UI never claims authenticated readiness; no secrets or ephemeral observations persist.
- [ ] Automated evidence uses fakes/loopback and retains `not_validated_on_ibmi`.

## Proposal Question Round

The supplied product/security decisions resolve the proposal questions; future approval is required only for exact protocol revisions, dependency admission, and managed trust-rotation policy before implementation.
