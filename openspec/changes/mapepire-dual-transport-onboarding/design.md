# Design: Mapepire Dual-Transport Onboarding

## Technical Approach

Replace the current LF-only, serialized `internal/mapepire` session with a typed application client over distinct WSS and SSH-single adapters. A policy-owned resolver prefers trusted daemon WSS (`:8076`); SSH runtime is invoked only after eligible resolution, credentials, and consent. Catalog callers retain their `Execute` adapter and never select a transport.

## Architecture Decisions

| Area | Decision | Trade-off / rationale |
|---|---|---|
| Ownership | `internal/mapepire` owns typed envelopes, IDs, correlation, cursors, limits, and session lifecycle; `internal/mapepire/wss` and `internal/mapepire/sshstdio` own their wire contracts; `internal/configuration` owns resolver/orchestration; `profile` owns persisted policy/trust. | Upper catalog/application layers depend only on the client, never transport. |
| Wire boundary | WSS adapter accepts/sends exactly one JSON **text** message; SSH adapter accepts/sends exactly one JSON LF record. Each owns context/deadlines, size checks, peer evidence, EOF/exit/close mapping. | A generic `io.ReadWriteCloser` leaks LF semantics into WSS; rejected. |
| WSS library | Admit `github.com/coder/websocket` only after dependency/security approval, pinned to its approved version. Configure TLS through an injected `http.Client`/`tls.Config`; use context-aware Dial/Reader/Writer, `MessageText`, `SetReadLimit`, compression disabled, and loopback `httptest` TLS tests. | Small maintained zero-dependency library with first-class context; Gorilla lacks equivalent context-first I/O. No dependency is added by this design. |
| TLS evidence | CA+hostname is default. Pin/TOFU stores `sha256/<base64url(SHA-256(leaf DER))>` plus mode/provenance; never a certificate/host/URL. Any mismatch, expiry, hostname failure, or leaf rotation requires approved re-enrollment (no overlap). | Leaf pin makes rotation explicit and auditable; plaintext and `MP_UNSECURE` are prohibited. |
| Protocol/session | Replace `Request`/`Response` with operation-specific request/response structs for `getversion`, `connect`, `prepare_sql_execute`, `sqlmore`, `sqlclose`, `ping`, `exit`. A cryptographically random unique ID is registered before write; one reader loop dispatches a bounded pending map. Wrong, duplicate, malformed, or unknown IDs fail and close the whole session. | Out-of-order safe without pooling. Cursor IDs remain client-owned, close only after completion, and `sqlmore` totals are bounded. |
| Failure routing | Eligible: daemon disabled by policy, bounded refusal/timeout/unavailability, or TLS-trusted `/version` proving unsupported 2.3.5. Terminal: TLS/SSH identity failure, tampering/protocol violation, unsafe downgrade, credential/authorization, malformed/unknown version, limits, cancellation. | Prevents silent downgrade; selected transport/reason/version/readiness are ephemeral. |
| Alternatives | Reject `mapepire-go` runtime, SDK/pool, unified framing, silent fallback, persisted selection, and Step-4 authenticated readiness. | Each either lacks required boundaries or creates stale/unsafe claims. |

## Data Flow

```text
Step 3: policy -> TLS inspection -> [eligible] SSH host-key inspection
Step 4: resolver -> WSS trust -> bounded /version -> detected/auth pending
Step 6/8: same credential reference -> WSS Basic+connect OR SSH auth->runtime->connect
client -> typed envelope -> WSS text | SSH LF -> reader loop -> correlated caller
```

`connect` (job required) is the first honest session proof; optional bounded read-only query is the only validated-query proof. Session context cancellation closes its adapter and wakes all pending calls; it never claims remote statement cancellation. SSH `getversion` requires matching ID and `success=true`; EOF/process exit is a classified process failure. Daemon path never touches JAR, Java, SSH, upload, or cache.

## Interfaces / Contracts

```go
type PeerEvidence struct { Mode TrustMode; Fingerprint string; PolicyRef string }
type Transport interface { Send(context.Context, []byte) error; Receive(context.Context) ([]byte, error); Close() error }
type Resolver interface { ResolvePreAuth(context.Context, ProfilePolicy) (Observation, error) }
```

`Transport` is internal to `mapepire`; adapters are constructed with their concrete WSS or SSH dependencies, not exposed upward. Versioned Nexus policy initially retains the existing 1 MiB frame and 1,000-row cap; adds 1 MiB aggregate response, 200 rows/page, 256 columns, 8 cursors, 64 pending IDs, 5s handshake/probe, 15s request, and 60s session. These align with current `MaxFrameBytes`/`MaxQueryRows`, source's 200-line/128 KiB bounds, and 2.3.5 evidence; they are release constants, never profile inputs.

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/mapepire/{protocol,session,limits,errors}_*.go` | Modify/Create | Typed client, correlation, compatibility `Execute` adapter, fixtures. |
| `internal/mapepire/{wss,sshstdio}/` | Create | Concrete adapters and trust/process mapping. |
| `internal/configuration/`, `internal/profile/` | Modify | Resolver, readiness, schema-v2 reader/writer and migration. |
| `internal/remote/`, `internal/connectors/ibmi/mapepirestdio/` | Modify | Reuse no-auth inspector; fallback-only verified handle, upload rollback, Java validation, fixed `--single`. |
| `internal/tui/`, `internal/audit/`, `internal/security/` | Modify | Service-result state/copy and sanitized allowlisted audit; no layout work. |

Schema v2 adds managed endpoint policy/fallback reference and independent TLS/SSH evidence. It reads v1 strictly using its existing allowlist, writes v2 with `schemaVersion`, and rejects unknown versions/keys. Legacy `HostKeyFingerprint`, `MapepireJAR`, and `vault|prompt` remain readable but require revalidation; no secret, cache path, selected transport, readiness, version, or error is persisted.

## Testing Strategy

| Layer | RED coverage | Approach |
|---|---|---|
| Unit | typed validation, IDs, reverse responses, duplicates, unknown IDs, cursors/sqlmore, cancellation/exit | table fakes; pin official protocol fixtures at `2ef44166fcb515744fb922b49ed3673b2dac6b26`. |
| Integration | TLS CA/pin/rotation, text-only WSS; LF/process EOF; resolver no-downgrade; migration/audit redaction | loopback TLS/WSS and fake process only. |
| Boundary | Step 3/4 truthful copy; Step 8 credential lifecycle; daemon zero fallback calls | counting fakes and TUI `View()` tests; normal suite makes zero IBM i contact. |

## Threat Matrix

| Boundary | Applicability | Safe behavior / RED test |
|---|---|---|
| Documentation-like paths | N/A — no executable classification | N/A |
| Git repository selection | N/A — no VCS operation | N/A |
| Commit state | N/A — no commit operation | N/A |
| Push state | N/A — no push operation | N/A |
| PR commands | N/A — no PR automation | N/A |

Fixed SSH process construction remains allowlisted; LF fake tests cover process integration, EOF, cancellation, and no arbitrary command input.

## Migration / Rollout

Deliver schema/compatibility first, then protocol/adapters, resolver/trust, fallback runtime, and wizard composition as reviewable feature-branch-chain slices. Disable resolver to roll back; v1 reads remain safe and fallback artifacts are inert. This supersedes (without deleting or editing) `mapepire-artifact-acquisition`: retain only its 2.3.5 verified handle/cache, approved source, optional Code for IBM i, upload verification/rollback, Java, and consent mechanics; artifact-first Step 4 and provider assumptions are superseded.

## Open Questions

None — dependency admission is an explicit implementation gate, not a design ambiguity.
