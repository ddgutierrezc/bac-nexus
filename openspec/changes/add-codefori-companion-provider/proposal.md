# Proposal: Add a minimal Code for IBM i Companion provider

## Outcome

Add an optional, independently packaged, 100% TypeScript VS Code Companion. Nexus remains the stdio MCP server and talks to the Companion through bounded HTTP/JSON on `127.0.0.1`. Companion mode exposes only `session.status` and `sql.query`; profile-selected Native mode remains unchanged and exposes only its existing catalog/source tools.

This proposal is ready for bounded, fake-backed local implementation. Authenticated native-Windows activation remains evidence-gated: the approved 100% TypeScript Companion may invoke only fixed descriptor-specific inbox Windows PowerShell publish/cleanup operations, with no native helper, addon, external ACL dependency, or general command runner. Microsoft documentation verifies that `Directory.CreateDirectory(path, DirectorySecurity)` applies security during creation and returns an existing directory unchanged; existing directories therefore require validation before any secret is generated. No Windows-native ACL test has been executed, so Windows functionality is not yet proven.

## User-approved scope

- Both GoLand/Nexus and VS Code run natively on Windows for the first live test.
- `nexus serve` without `-profile` selects Companion. Existing `-profile` selects Native.
- Explicit contradictory selections fail closed. There is no automatic fallback in either direction.
- Companion is 100% TypeScript.
- The spike may use HTTP/JSON on `127.0.0.1` and one known port. Automated discovery is documented future evolution, not MVP work.
- `sql.query` admits only `SELECT CURRENT_USER FROM SYSIBM.SYSDUMMY1` after limited ASCII whitespace/case normalization. There is no arbitrary SQL or parser project.
- There is no default global SQL serialization. Offline tests prove bounded admission and correlation, not live shared-job safety.
- The first tag is `companion-v0.1.0`; the proposed workflow is `.github/workflows/release-companion.yml`.
- Native onboarding, profiles, credentials, SSH, Mapepire, configuration, TUI, validation, audit, recovery, and ownership implementation must never be modified, deleted, or restructured by this change.

## Tool and mode contract

| Invocation | Provider | MCP tools |
|---|---|---|
| `nexus serve` with no profile | Companion | `session.status`, `sql.query` |
| `nexus serve -profile <name>` | Native | `resolve_catalog_candidates`, `read_selected_source` |
| Explicit Companion plus profile | Reject | None |
| Explicit Native without profile | Reject | None |

Companion absence never blocks MCP startup. Tool calls return bounded unavailable states. Availability never selects another provider.

## Query contract

Both Go and TypeScript independently recognize only the approved statement, allowing ASCII-only case changes, leading/trailing ASCII whitespace, and one-or-more ASCII whitespace characters between its four tokens. No whitespace is allowed inside identifiers or around the dot. Every admitted form is canonicalized before IPC/execution to:

```sql
SELECT CURRENT_USER FROM SYSIBM.SYSDUMMY1
```

Comments, semicolons, Unicode whitespace, extra tokens, arrays, parameters, CL, commands, CTEs, calls, DDL, DML, transactions, and all other forms are rejected before `runSQL`.

The canonical literal is settled at exactly 41 ASCII bytes: `6+1+12+1+4+1+6+1+9`. No further query-length decision is required.

A successful raw `runSQL` result must have exactly one row object and exactly one own enumerable column with a bounded string value. Because the raw label has not been live-verified, the Companion discards it and returns the stable normalized result `{"state":"ok","rows":[{"value":"..."}]}`. It does not promise that Code for IBM i labels the raw column `CURRENT_USER`.

## Minimal transport

The author recommends fixed `127.0.0.1:64139` for the spike. A collision fails unavailable; there is no scan or fallback. A small descriptor carries only protocol version, per-activation generation, and an ephemeral bearer token. It contains no endpoint because the port is known.

Requests are authenticated, strictly bounded, versioned, generation-bound, and correlated by random request ID. SQL text, row data, token, endpoint, connection metadata, and raw errors are excluded from logs, audit, telemetry, diagnostics, fixtures, and non-success results.

Folder location, loopback, local-volume checks, and descriptor generation do not prove extension-host identity. v0.1 supports one native Windows Companion broker per user and documents multi-window, WSL, SSH, container, forwarding, and cross-host discovery as unsupported.

## Public API boundary

The Companion uses only Code for IBM i 3.0.12 public `exports.instance`, `getConnection`, `subscribe`, and `runSQL` surfaces. The evidenced `subscribe` signature returns `void`; the implementation must not store or dispose a nonexistent return value and must not promise an unsubscribe API. Companion-owned listener and request resources are still closed on deactivation.

`runSQL` receives only the canonical string and `{rows: 1}`. Its public promise has no evidenced cancellation handle. Timeout/cancellation stops waiting and suppresses late output but does not claim cancellation on IBM i.

## Bounds and concurrency

- At most 128 input SQL bytes, 512 request bytes, 1024 response bytes, one row, one column, and a 256-byte UTF-8 cell.
- Immediate broker admission is capped at 16 operations, allowing the required ten-request proof; excess work returns `limit_exceeded` without a waiting or worker queue.
- At most 16 `runSQL` promises are active. A timed-out or cancelled started promise retains capacity until the underlying promise settles.
- Companion SQL wait is 5 seconds.
- Go response-header timeout is 6 seconds and total query time is 7 seconds, so a valid five-second execution is not cut off at 500 ms.
- At least ten fake-backed requests enter independently and complete out of order without response crossover.
- No process-wide/global mutex serializes all SQL by default.
- Shared Code for IBM i job behavior remains `not_validated_on_ibmi` until separately authorized live validation.

## Packaging and release

Add an independent TypeScript package and VSIX verification. Preserve existing Go CI. `.github/workflows/release-companion.yml` builds, lints, typechecks, tests, and packages without VS Code or IBM i. Publication is tag-gated under `companion-v*`; the first intended tag is exactly `companion-v0.1.0`. OIDC uses `contents: read` and publish-job `id-token: write`, with no Marketplace PAT.

Marketplace publication remains disabled until extension identity, publisher ownership, trusted-publisher registration, and Windows token security are approved. The workflow still includes the requested inert publication route: only a valid `companion-v*` tag with repository variable `COMPANION_PUBLISH_ENABLED == 'true'` and approval from the protected `companion-marketplace` environment may reach the OIDC publish job. Only that job receives `id-token: write`, and no Marketplace PAT is accepted.

## Explicit non-goals

- No changes to Native onboarding/profile/credential/SSH/Mapepire/configuration/TUI/validation or related behavior.
- No mixed mode, fallback, arbitrary SQL, parser, shell, CL, write operation, alternate query, profile creation, credential transfer, port forwarding, or automatic VS Code launch.
- No claim that filesystem locality proves extension-host identity.
- No claim that `subscribe` returns a disposable or that raw rows use a `CURRENT_USER` key.
- No live IBM i, live VS Code, publishing, or release in this change.

## Evidence boundary and blocked research

Formal research was explicitly deselected after its executor lacked documentation/open-web grants. The unchanged blocked `research.md` remains historical and is not validated evidence. Normal acceptance uses fakes and loopback only and retains `not_validated_on_ibmi`.

## Risks and blockers

| Item | Status | Consequence |
|---|---|---|
| Windows descriptor DACL creation from 100% TypeScript | Mechanism approved; native evidence pending and release-blocking | Fixed PowerShell creation may be implemented behind fail-closed activation, but Windows ACL and cross-user tests must pass before acceptance. |
| Canonical query length | Resolved | Retain the approved 41-byte literal and reject any alternate statement. |
| Raw row label and live session/concurrency behavior | Not live-validated | Use label-independent normalization and avoid live safety claims. |
| Extension/Marketplace identity | Unapproved | Offline package work may proceed later; publication may not. |

## Narrow next step

Begin with the provider-neutral domain unit only: `internal/provider/provider.go` and `internal/provider/provider_test.go`, including the finite query canonicalizer and normalized-result validation with fakes. This unit is independent of Windows ACL work and must remain at or below 300 changed lines. Windows activation, IBM i behavior, release, and publication remain unproven.
