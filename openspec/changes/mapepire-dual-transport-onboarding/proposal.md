# Proposal: Mapepire Dual-Transport Onboarding

## Intent

Preserve the original daemon-first, secure onboarding rationale and completed lower-layer work. Independent verification found production Step 8 orchestration missing: helper seams and the original 13 completed tasks do **not** prove the wizard path. Build the application-owned boundary that makes a saved profile's controlled proof real.

## Scope

### In Scope
- Steps 3–4 remain credential-free and runtime-free pre-auth trust/protocol observation; Step 7 saves the profile; **only Step 8** retrieves credentials, authenticates, selects transport, falls back, and proves access.
- A first-class Step 8 service supports policy-governed `Ask each time` and native-keyring `Store securely` credentials (no plaintext or silent fallback) at the last responsible moment; it uses trusted WSS first, authenticates a typed Mapepire session, runs fixed `VALUES 1`, exposes neither SQL text nor rows, returns bounded proof metadata only, and closes cursor/session/transport on completion or cancellation.
- Strict no-downgrade routing; eligible-only, consented SSH fallback with independent trust/credentials/policy, verified pinned JAR acquisition, Java validation, bounded upload, fixed `--single`, rollback, and complete cleanup.
- Bubble Tea request lifecycle and terminal feedback; sanitized audit; a separate historical marker containing only bounded timestamp, outcome classification, and proof revision. It is never readiness evidence, never skips a later test, and becomes stale/invalid on relevant endpoint/policy/trust changes.
- Six feature-branch-chain slices: contract/domain; authenticated WSS; SSH fallback adapter; production composition root; Bubble Tea lifecycle; audit/docs/integrated verification.

### Out of Scope
- Generic SQL, arbitrary SSH/SFTP/commands, secret/result exposure, normal-test live IBM i, user-configurable endpoints, unsaved-profile execution, marker-as-readiness, and silent trust or fallback.

## Capabilities

### New Capabilities
None.

### Modified Capabilities
- `mapepire-transport-onboarding`: production Step 8 authenticated proof and lifecycle.
- `mapepire-application-protocol`: fixed bounded `VALUES 1` proof operation.
- `mapepire-ssh-fallback-runtime`: post-credential, consented runtime lifecycle.
- `local-mcp-security`, `nexus-configuration`: secret lifetime, audit, marker, and truthful readiness.

## Approach

Compose the service in `cmd/nexus`, not in Bubble Tea. Managed daemon endpoint defaults to `8076`; only approved deployment policy may override it. V1 uses user-confirmed TLS and SSH TOFU per transport (host/port/fingerprint), blocks mismatch/rotation, and has a future CA/pin migration path. This is explicit V1 risk acceptance, not the target banking posture.

## Affected Areas

| Area | Impact | Description |
|---|---|---|
| `internal/configuration/`, `cmd/nexus/main.go` | Modified | Step 8 orchestration and composition. |
| `internal/mapepire/`, `wss/`, `sshstdio/`, `remote/` | Modified | Authenticated sessions and fallback adapters. |
| `internal/tui/`, `profile/`, `credential/`, `audit/` | Modified | Lifecycle, marker, and redacted evidence. |

Current uncommitted remediation (daemon probe, Step 3 trust visibility, pinned fixture, audit metadata) is not acceptance evidence and must remain untouched. The failed `verify-report.md` remains historical evidence; fresh independent verification is required after all slices, with no archive before zero CRITICAL findings.

## Risks, Rollback, and Success

| Risk | Mitigation |
|---|---|
| V1 free TOFU | Explicit confirmation, independent storage, mismatch blocking; migrate to governed CA/pin. |
| Controlled remote mutation | Eligible classification, policy, credentials, consent, fixed command, rollback/cleanup. |

Rollback disables Step 8 composition and removes new adapters/marker behavior without altering saved secret-free profiles or trust policy.

- [ ] Saved-profile Step 8 proves authenticated `VALUES 1` without exposing SQL or rows.
- [ ] WSS failure falls back only through the defined matrix; all terminal failures fail closed.
- [ ] Offline integration proves production composition, cleanup, audit redaction, marker invalidation, and `not_validated_on_ibmi`.

## Proposal Question Round

The confirmed product decisions resolve this round; review may amend specs/design before implementation.
