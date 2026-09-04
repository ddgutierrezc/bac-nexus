# Design: Enable Production Nexus Serve

## Technical Approach

`cmd/nexus` remains the composition root: admit one V3/keyring profile and eligibility, open verified audit/ownership stores, recover, compose request-owned SSH/SFTP/Mapepire adapters and leases, then serve only the two existing MCP tools. Remote contact follows recovery. Stdout is JSON-RPC only; diagnostics use stderr.

## Architecture Decisions

| Decision | Choice and rationale | Rejected |
|---|---|---|
| Eligibility transaction | Keep owner-only eligibility v1 beside profiles, bound to profile/target/policy/pin/credential/artifact/proof. Extend the verified journal—not a parallel flow—as `prepare → keyring → profile → pin → eligibility → committed audit → clear`; compensate in reverse. Uncertain recovery retains the journal and makes serve ineligible. Changed bindings revoke before replacement; rollback restores the old pair only when proven. | Embedded/implicit approval. |
| Durable audit | `internal/audit/file.go` uses mandatory retention `1..3650`, UTC `audit-YYYY-MM-DD.jsonl`, owner-only state, and a lifetime-exclusive lock. Validate allowlists and encode canonical JSON before locking; keep exactly the seven spec fields. Cap records at 1023 bytes plus newline and segments at 64 MiB. Under a mutex, rotate, loop writes to the complete newline-framed record, then sync before success. Zero-progress, short/partial write, write/sync/rotation failure poisons the sink and fails startup/operation. Startup scans retained segments under lock for exact names, bounds, framing, schema, values, and retention. Only the newest segment's unterminated tail may be truncated to its last newline, file/directory synced, and recovery recorded; malformed records, old torn tails, unknown files, or failed recovery poison startup. Validated expiry uses UTC dates and directory sync. | In-memory/best-effort audit. |
| Filesystem evidence | Inject `SecurePathPlatform` adapters: open trusted `UserConfigDir`, walk managed components by parent handle without following links, inspect handles, create restrictive nodes, and re-inspect. Unix requires no symlink component, same device, allowlisted local `fstatfs`, effective-UID ownership, directories `0700`, files `0600`. Windows requires no reparse component, fixed/local volume, current-user ownership, and current-user-only DACL. Unsupported, race-changed, missing, or contradictory evidence fails closed. Audit and ownership share this contract. | Path-string checks/unavailable APIs. |
| Remote operations | `EnsureServerJAR` issues an immutable receipt only after activation from `https://github.com/Mapepire-IBMi/mapepire-server/releases/download/v2.3.6/mapepire-server.jar` and remote SHA-256 verification through the authenticated remote-files capability. The receipt binds that capability, authenticated host identity, safe absolute path, fixed SHA, and launch-policy revision. Before `NewSession`, its receipt-owned admission boundary rehashes the path through the same capability and rejects every mismatch before invoking its fixed-start seam. It then renders exactly the four private fixed stdio variables, fixed Java, `-jar`, receipt path, and `--single`; rendering returns owned values and imports cannot mutate launch behavior. Go SSH sends one exec string; no raw path/SHA/Java/environment/argv launch API remains. Preserve 50 candidates, cancellation, cleanup, and sanitized errors. | Shared/WSS-only/generic execution. |

## Data Flow and Lifecycle

```text
parse → profile/keyring/eligibility → audit+ledger → recover → MCP
configure proof → existing journal transaction → eligibility → audit
```

`RunRemoteDiagnostic` uses the persistent auditor and emits only `operation_class=configuration_diagnostic`, `policy_id=verified_read_only`, result `succeeded|cancelled|timed_out|failed`, zero counts, bounded duration, and lifecycle `completed`. Append failure overrides every outcome with `diagnostic evidence unavailable`, preserves the non-claim, and fails. Its seam has no external-client writer.

Shutdown stops intake, awaits handlers/clients, evicts leases, closes ledger, appends/syncs lifecycle audit, then closes audit; uncertain evidence fails.

## File Changes

| Files | Action | Description |
|---|---|---|
| `cmd/nexus/{main,composition}*.go`, `internal/{app,mcp,source,remote,mapepire,connectors/ibmi}/**` | Modify/Create | Admission, remote composition, lifecycle. |
| `internal/profile/eligibility*.go`, `internal/profile/onboarding_commit*.go`, `internal/configuration/{readiness,onboarding}*.go` | Create/Modify | Transaction and diagnostics. |
| `internal/audit/{file,platform_*}*.go`, `internal/ownership/sqlite/platform_policy_*.go` | Create/Modify | Durable audit and platform safety. |
| `integration/ibmi/serve_live_test.go`, `internal/release/*`, `README.md`, `docs/*`, `.github/workflows/*` | Create/Modify | Gate, evidence, operations. |

## Testing Strategy

Strict RED→GREEN→REFACTOR in stacked-to-main units ≤800 changed lines. Table tests cover transaction compensation; audit encoding, limits, short writes, sync, scan, torn tails, corruption, poison, rotation/retention; and diagnostic secret errors, append failure, and unchanged external-client sentinels. Platform-contract fakes run everywhere; tagged Unix/Windows acceptance tests exercise available no-follow/owner/mode or reparse/ACL evidence. Fakes prove admission/recovery order; MCP tests cover both tools, paging, cancellation, cleanup, and no partial source.

## Threat Matrix

| Boundary | Applicability | Safe/failure behavior and planned RED test |
|---|---|---|
| Documentation-like paths | N/A—no executable classification | None |
| Git repository selection | N/A—no Git execution | None |
| Commit state | N/A—no commits | None |
| Push state | N/A—no push | None |
| PR commands | N/A—no PR automation | None |
| Fixed subprocess lifecycle | Applicable | A receipt-only API rehashes the exact path before session allocation, then deterministically renders only release-owned tokens. RED tests reject zero/mismatched receipt capability, host, path, policy, SHA, and remote rehash mismatch before `NewSession`, plus unsafe rendering, launch-resource leaks, and cancellation leaks. |

## Migration / Rollout

Existing profiles start ineligible. Opt in with `go test -tags=ibmi_integration ./integration/ibmi -run '^TestControlledNexusServe$' -count=1`; normal CI excludes it. Prerequisites are approved binary/manifest, profile, target/libraries, window, policy, artifact, roots, real MCP client, and keyring credential—never credentials in argv, files, logs, or evidence. The gate launches real stdio serve, invokes both tools, pages to EOF, cancels, verifies cleanup/shutdown, and stores only redacted counts/classifications/checksum externally. Manifest remains `not_validated_on_ibmi`; approved passing evidence alone authorizes a separate release transition. Rollback stops serve, revokes eligibility, invalidates leases, and recovers ledger-owned paths.

## Open Questions

None.
