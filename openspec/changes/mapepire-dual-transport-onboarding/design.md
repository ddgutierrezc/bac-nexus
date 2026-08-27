# Design: Mapepire Dual-Transport Onboarding

## Technical Approach

Add saved-profile-only `internal/configuration.Step8ProofService`. TUI imports only `Step8Runner`; `cmd/nexus` composes adapters. Existing packages retain protocol, TLS, SSH/SFTP, and runtime ownership.

## Architecture Decisions

| Decision | Choice and rationale |
|---|---|
| Direction | `tui -> configuration contract <- cmd/nexus`; no TUI implementation imports; composition is not helper evidence. |
| Auth/trust | `/version` is pre-auth; credentials enter `connect`. WSS first. Per-transport confirmed TOFU binds host/port/fingerprint; mismatch blocks. CA/pin is the V1-risk replacement seam. |

```go
type Step8Runner interface{ Run(context.Context, Step8Request) Step8Result }
```

Configuration-owned contracts: `CredentialProvider.Get(ctx,key,mode)`, `PreAuthResolver.Observe(ctx,profile)`, `WSSFactory.Open`, `SSHFactory.Open(ctx,profile,secret,consent)`, `ProofSession.Connect/FixedProof/Close`, `TrustStore.Enroll`, `MarkerWriter.Write`, `Auditor.Record`, `Clock.Now`, and `RequestIDs.New`.

`Decision` is exactly `wss_selected|ssh_eligible|terminal`; `Observation{Decision,Reason,Version}` is pre-auth only. Eligible `Reason` is exactly `daemon_connection_refused|daemon_unavailable|daemon_availability_timeout|daemon_policy_disabled|daemon_version_verified_unsupported`, each mapping to `ssh_eligible`. Pre-auth terminal reason is exactly `identity_hostname_pin_tofu_trust_mismatch_or_rotation|protocol_or_framing_failure|malformed_response|unsafe_downgrade|cancelled|operation_timeout|limit_exceeded`.

Later proof/runtime maps into public end-to-end result/audit `ResultClass`: `identity_failure|trust_mismatch|protocol_failure|framing_failure|malformed_response|downgrade_blocked|credentials_unavailable|authentication_failed|authorization_denied|cancelled|operation_timeout|proof_timeout|cleanup_timeout|cleanup_failure|limit_exceeded|consent_declined_or_absent|artifact_failure|java_failure|upload_failure|launch_failure|session_failure|proof_failure`; all are terminal. `trust_mismatch`, `credentials_unavailable`, and `downgrade_blocked` are distinct exact public constants, never aliases. Broader internal causes/wrapped detail are sanitized and deterministically map to one public class only. Eligible reasons cannot accompany `terminal`; terminal reasons never map to SSH; WSS success maps only to `wss_selected`. No string matching/wrapped-error guessing: unknown decision/reason fails closed. Request has profile/request/consent only; result has request/class/revision/outcome/cleanup only. Table-driven design verification MUST enumerate every decision/reason/result class, assert its sole mapping, and assert unknown values fail closed.

## Data Flow and State

* Step 3: `configure -> Model -> InspectHostKey -> SSH enrollment -> draft`; no auth/runtime. Step 4: `Model -> policy :8076 -> TLS /version -> authentication_pending`; no SSH. Step 7: `ProfilesStore.Save` writes secret-free profile and `ibmi/<profile>` key.
* Step 8: `action -> tea.Cmd -> Runner -> Observe`. `wss_selected -> credential -> WSSFactory -> connect -> prepare_sql_execute(VALUES 1) -> sqlclose -> exit -> close -> audit/marker`; no SQL/rows. `ssh_eligible -> policy -> SSH trust -> consent -> credential -> Dial -> verified artifact/Java/upload/fixed --single -> same proof`. Decline: zero credential/SSH/runtime. Terminals never downgrade; WSS makes SSH/artifact zero.

Availability and operation/proof/cleanup contexts differ; cancellation closes session/transport/process/SFTP/SSH, never claims remote cancellation. TUI holds request ID/cancel/phase/sanitized feedback; `Update` rejects stale IDs; `View` has no I/O.

`cmd/nexus.runConfigure` builds stores, policy, auditor, resolver/factories, and service; passes runner to `runConfigureTUI`; production model stores only runner; action command invokes it and result returns to `Update`. Deterministic `runConfigure` test uses local stores, loopback WSS, counting SSH, and proves invocation/zero daemon SSH. Helper-only, fake-only constructors, and `cmd/catalogspike` are invalid.

The proof is exactly one `connect`, one `prepare_sql_execute(VALUES 1)`, no `sqlmore`, then `sqlclose` and `exit`; it accepts at most one row, one page, 256 columns, 1 MiB frame/aggregate, eight cursors, 64 pending IDs, with 5s availability, 15s operation, and 60s session limits.

## Persistence and Audit

Profile v3 validates `prompt|keyring`; v1/v2 `vault` is `migration_required`. Key is validated name -> `ibmi/<name>`, never secret. Prompt/keyring unavailable, denial, not-found, invalid mode, migration/rotation failure => `credentials_unavailable`; only explicit `KeyringStore.Migrate`. Acquire branch-last, zero after close; no downgrade/plaintext/exposure.

Marker is exactly `{schemaVersion:1,atUnixMs,outcome,proofRevision}`. Save/update clears it on endpoint-policy, policy revision, or trust change; old schemas have none. It never gates readiness. Audit allows policy ID, transport attempt, trust, fallback reason, revision/version, result, duration, lifecycle; excludes endpoint/host/user/path/raw error/SQL/results/secrets.

## Security and Threat Matrix

| Risk | Safe failure / RED proof |
|---|---|
| Credential, endpoint, TOFU | fail closed; default 8076/policy override only; exact enrollment and mismatch block |
| Downgrade, artifact | only eligible matrix + consent; changed/unpinned artifact blocks before upload |
| Cleanup/cancel/stale UI | LIFO close; cancel terminal; stale request ignored |
| Marker/audit | prohibited fields rejected; marker clears and never gates |

| Threat-matrix boundary | Applicability |
|---|---|
| Documentation paths; Git/commit/push/PR | N/A — no classification or VCS/PR automation |
| Shell/process integration | Applicable: fixed allowlisted `--single` only; RED rejects user command/path, partial cleanup, cancellation |

Live IBM i is deferred/manual-approved; all automated evidence remains `not_validated_on_ibmi`.

## Six Feature-Branch-Chain Slices

| Slice (base -> target) | Scope, RED/harness, rollback | Estimate |
|---|---|---:|
| 1 `fix/mapepire-dual-transport-verification -> feature/step8-foundation` | v3 mode/key migration, marker, types/fixed proof RED; fake provider/session/auditor; rollback foundation; `go test -count=1 ./internal/profile ./internal/credential ./internal/configuration` | 380 |
| 2 `feature/step8-foundation -> feature/step8-wss` | WSS/proof RED; TLS/WSS loopback proves close/zero SSH; `go test -count=1 ./internal/configuration ./internal/mapepire ./internal/mapepire/wss` | 350 |
| 3 `feature/step8-wss -> feature/step8-ssh` | policy/trust/consent-before-credential RED; fake SSH/process/upload LIFO; `go test -count=1 ./internal/configuration ./internal/remote ./internal/mapepire/sshstdio ./internal/connectors/ibmi/mapepirestdio` | 390 |
| 4 `feature/step8-ssh -> feature/step8-compose` | composition RED; compile assertion and actual configure counting/loopback proof; rollback root; `go test -count=1 ./cmd/nexus ./internal/configuration` | 330 |
| 5 `feature/step8-compose -> feature/step8-tui` | lifecycle RED; Update/View 120x40/80x24/40x16/NO_COLOR; `go test -count=1 ./internal/tui` | 380 |
| 6 `feature/step8-tui -> feature/step8-audit` | audit/docs RED; composition matrix; rollback additions; `go test -count=1 ./internal/audit ./internal/configuration ./internal/tui` | 240 |

Each slice runs its affected package `go test -count=1`; slice 6 also runs full suite/vet/build. A separate planning commit contains only proposal/spec/design/tasks amendments before slice 1.

## Current Remediation and Rollout

Preserve uncommitted `daemon.go`, `wss/http.go`, resolver/audit/fixture changes for slices 2/6 and TUI Step 3/4 changes for slice 5; rebase only after RED tests. `.atl`, `tmp`, proposal/specs/apply-progress stay outside. `verify-report.md` is byte-unchanged historical failure evidence during planning, implementation, and remediation: never reset/alter native evidence. Only the proper later verify phase creates fresh acceptance evidence.

## Open Questions

None: `VALUES 1`, saved-profile gate, credential modes, endpoint, TOFU, marker, and managed runtime are confirmed.
