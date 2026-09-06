---
schema: gentle-ai.sdd-design/v1
revision: 4
change: add-codefori-companion-provider
artifact_store: openspec
skill_resolution: paths-injected
status: ready_for_bounded_offline_implementation
---

# Design: bounded native-Windows Code for IBM i Companion proof

## Decision summary

Proceed with a fixed, isolated Companion path while leaving the profile-selected Native path unchanged. The implementation starts with a provider-neutral Go domain port and proof-query validation, then adds a fixed loopback connector, isolated MCP/CLI composition, and a 100% TypeScript VS Code Companion under `companion/vscode-codefori/`.

Offline fake-backed implementation is authorized. Native-Windows activation remains fail-closed until Windows ACL tests prove the descriptor is protected, and all live IBM i behavior remains `not_validated_on_ibmi`. No Windows ACL command or test has been executed for this design.

| Topic | Decision |
|---|---|
| Provider boundary | Put the neutral `Provider` port, request/result values, state enums, proof-query canonicalizer, and normalized-result validator in `internal/provider/provider.go`; do not create `internal/provider/codefori.go` or `internal/companion/`. |
| Connector | Put fixed Code for IBM i Companion transport details in `internal/connectors/ibmi/codefori/`. |
| Companion package | Put the independent TypeScript extension in `companion/vscode-codefori/`. |
| Broker | Bind only `127.0.0.1:64139`; one versioned `POST /v1/rpc`; no discovery, fallback, endpoint setting, forwarding, or alternate transport. |
| Query | Admit only `SELECT CURRENT_USER FROM SYSIBM.SYSDUMMY1`, canonicalized to the settled 41-byte ASCII literal in both Go and TypeScript. |
| Result | Accept one row with one own enumerable string column, discard the raw label, and expose only `rows:[{"value":"..."}]`. |
| Admission | Use immediate bounded admission with capacity 16, sufficient for the required ten-request proof. Do not add a waiting or worker queue and do not add a process-wide SQL mutex. |
| Timeout ownership | Once `runSQL` starts, timeout/cancellation suppresses late output but retains its admission capacity until the underlying promise settles. |
| Windows security | Use only fixed descriptor-specific Windows PowerShell publish/cleanup operations. Existing directories are never assumed to inherit the requested ACL and must be validated before `READY`. |
| Release | Add offline verification plus a disabled-by-default, protected, OIDC-only `companion-v*` publication route. No PAT, publication, tag, commit, push, or PR is part of this design phase. |
| Delivery | Use the user-selected Feature Branch Chain through an integration/tracker branch; every implementation slice must remain at or below 400 changed lines unless measured evidence requires a separately approved `size:exception`. |

## Architecture and isolation

```text
MCP client over stdio
        |
        v
nexus serve selection
  | -profile <name>                         | no profile
  v                                         v
existing Native composition                 Companion-only composition
resolve_catalog_candidates                  session.status
read_selected_source                        sql.query
  |                                         |
unchanged                                   provider.Provider
                                            |
                              internal/connectors/ibmi/codefori
                                            |
                               HTTP/JSON 127.0.0.1:64139
                                            |
                            companion/vscode-codefori (TypeScript)
                                            |
                         Code for IBM i 3.0.12 public exports
```

Selection is based only on profile presence and an optional explicit selector:

| Profile | Explicit selector | Result |
|---|---|---|
| absent | absent or `companion` | Companion |
| present | absent or `native` | Native |
| present | `companion` | Fail before composition |
| absent | `native` | Fail before composition |
| either | unknown | Fail before composition |

Companion absence does not block MCP startup. Calls return bounded unavailable states. Availability never switches providers. Companion composition must not construct or read Native profile, credential, eligibility, audit, ownership, recovery, SSH, SFTP, Mapepire, configuration, TUI, or validation dependencies.

## Provider-neutral Go contract

`internal/provider/provider.go` is the consumer-facing domain boundary. The neutral public type name is `Provider`, not a vendor or transport name.

```go
type Provider interface {
    SessionStatus(context.Context) SessionStatusResult
    Query(context.Context, QueryRequest) QueryResult
}

type QueryRequest struct { SQL string }
type QueryResult struct {
    State QueryState
    Rows  []NormalizedRow
}
type NormalizedRow struct { Value string }
```

The package also owns:

- allowed session states: `connected`, `companion_unavailable`, `codefori_extension_unavailable`, and `connection_unavailable`;
- allowed query states: `ok`, `unavailable`, `invalid_query`, `limit_exceeded`, `timeout`, `cancelled`, and `failed`;
- the canonical proof-query constant and `CanonicalizeQuery` behavior;
- normalized result validation: `ok` has exactly one row and a valid UTF-8 value of at most 256 bytes; every non-success state has no rows; and
- no endpoint, token, descriptor, profile, credential, HTTP, PowerShell, VS Code, or Code for IBM i type.

The first implementation unit changes only `internal/provider/provider.go` and `internal/provider/provider_test.go`. It uses a fake `Provider`; it performs no descriptor, HTTP, Windows, VS Code, or IBM i work and is forecast at 180–280 changed lines.

## Query and result contract

Both Go and TypeScript independently implement the same finite recognizer:

1. Reject input over 128 UTF-8 bytes or containing any non-ASCII byte.
2. Treat only ASCII space, tab, carriage return, and line feed as whitespace.
3. Permit leading/trailing whitespace and require one or more whitespace bytes between `SELECT`, `CURRENT_USER`, `FROM`, and `SYSIBM.SYSDUMMY1`.
4. Match token letters with ASCII-only case folding.
5. Permit no whitespace inside identifiers or around the dot.
6. Reject comments, semicolons, parameters, arrays, extra tokens, commands, CL, CTEs, calls, DDL, DML, transactions, and every other form.
7. On success use only `SELECT CURRENT_USER FROM SYSIBM.SYSDUMMY1`, exactly 41 ASCII/UTF-8 bytes.

Go validates before descriptor access or IPC and validates the normalized protocol result after IPC. TypeScript validates again before `runSQL`. The TypeScript adapter calls `runSQL(canonicalSQL, {rows: 1})` and accepts only an array containing exactly one object with exactly one own enumerable property whose value is a valid UTF-8 string no larger than 256 bytes. Its unknown raw key is discarded.

## Fixed broker protocol and data flow

The connector and broker use one strict JSON RPC shape over `POST /v1/rpc`. Requests carry protocol version, activation generation, random request ID, method, and bounded parameters; bearer authentication is carried in the authorization header. Responses echo version, generation, and request ID. Unknown or duplicate fields, trailing JSON, mismatches, oversized bodies, unsupported methods, and noncanonical success shapes fail to sanitized states.

```text
MCP input
 -> Go schema check
 -> provider.CanonicalizeQuery
 -> connector validates and reads protected descriptor
 -> authenticated fixed-port request with unique ID
 -> broker authenticates and validates envelope
 -> TypeScript canonicalizes the query independently
 -> Code for IBM i public adapter calls runSQL(canonical, {rows: 1})
 -> adapter validates one row/one arbitrary column and normalizes to value
 -> broker echoes correlation fields
 -> connector verifies correlation and body bounds
 -> provider normalized-result validation
 -> bounded MCP result
```

Fixed bounds are:

| Resource | Bound |
|---|---:|
| Input SQL | 128 bytes |
| Canonical SQL | 41 ASCII bytes |
| Descriptor | 512 bytes |
| Request body | 512 bytes |
| Response body | 1024 bytes |
| Result | 1 row, 1 normalized column |
| Cell value | 256 UTF-8 bytes |
| Immediately admitted broker operations | 16 |
| Active `runSQL` promises | 16 |
| Companion SQL wait | 5 seconds |
| Go response-header timeout | at least 6 seconds |
| Go total query | at least 7 seconds |
| Session status total | 1 second |
| Retries | 0 |

Admission is immediate: capacity exhaustion returns `limit_exceeded` without starting `runSQL`. There is no waiting queue. At least ten fake calls must enter before any is released and then complete in reverse order with distinct values and exact response correlation. A request cancelled before `runSQL` starts never starts it. A timeout/cancellation after start ends the caller wait and suppresses late output, but the slot is released only when the underlying public promise settles because no cancellation handle is evidenced. This proves local concurrency mechanics only, not shared-job safety.

## Windows descriptor security

The Companion exposes only private `publishDescriptor` and `cleanupOwnedDescriptor` operations under `companion/vscode-codefori/src/windows/`. They invoke the fixed inbox Windows PowerShell executable through an absolute validated `SystemRoot` path, `shell: false`, fixed arguments, fixed script bytes, minimal environment, bounded stdio, and fixed deadlines. No caller can provide an executable, command, script, path, argument list, or environment extension.

The fixed path is derived inside the operation from `LocalApplicationData` plus `BAC Nexus\companion-v1\descriptor.json`. The descriptor contains only protocol version, generation, and token. The generation and token are generated after the child emits exactly `READY\n`, sent only through bounded stdin, and excluded from argv, environment, stdout, stderr, logs, errors, diagnostics, telemetry, settings, profiles, fixtures, and MCP.

Microsoft official documentation has now verified the relevant directory rule: `Directory.CreateDirectory(path, DirectorySecurity)` applies the supplied directory security during creation, avoiding a create-then-repair ACL interval; when the directory already exists it returns the existing directory unchanged. Therefore an existing directory must be independently checked for current-process SID ownership, protected DACL, exact current-SID-only explicit rules, normal directory type, and non-reparse status before `READY`. It must never be repaired in place.

This source evidence does not prove the complete implementation. Before activation is accepted, official API evidence must still support the selected protected file create-new and open-handle validation behavior, fixed executable/spawn semantics, and Go no-follow descriptor validation. Native-Windows tests must then prove owner equality, protected exact DACLs, non-reparse objects, no pre-`READY` secret, no readable transient file, cross-user read denial, adversarial residue rejection, and generation-owned cleanup. No Windows-native evidence has been executed yet; failure keeps the broker unavailable with no weaker fallback.

## Public Code for IBM i adapter

The TypeScript adapter uses only Code for IBM i 3.0.12 public surfaces:

1. Resolve and activate `halcyontechltd.code-for-ibmi` when needed.
2. Read `exports.instance`.
3. Call `instance.getConnection()`.
4. Register `connected` and `disconnected` through `instance.subscribe(context, event, name, callback)`.
5. Treat `subscribe` as returning `void`; store no disposable and promise no unsubscribe.
6. Call only `connection.runSQL(canonicalSQL, {rows: 1})`.

Deactivation closes Companion-owned listener/request resources and drops local references. Raw errors map to fixed states. A disconnect or caller timeout suppresses a later result, but the design does not claim remote SQL cancellation.

## File changes

| Path | Planned change |
|---|---|
| `internal/provider/provider.go` | Neutral provider port, states, canonical query, recognizer, and normalized-result validation. |
| `internal/provider/provider_test.go` | Table and fake tests for the domain contract; this is the first unit's only test file. |
| `internal/connectors/ibmi/codefori/` | Fixed loopback protocol/client, descriptor reader, platform split, and connector-local fakes/tests. |
| `internal/mcp/codefori.go` | Separate Companion two-tool MCP constructor; existing Native `mcp.New` remains separate. |
| `cmd/nexus/codefori.go`, `cmd/nexus/main.go` | Isolated Companion composition and deterministic profile/provider selection only. |
| `companion/vscode-codefori/` | Independent TypeScript extension, query/protocol/broker, public adapter, fixed Windows token store, tests, build, and VSIX package metadata. |
| `.github/workflows/release-companion.yml` | Offline verification and disabled-by-default protected OIDC publication route for `companion-v*`. |
| `docs/CODEFORI_COMPANION.md` | Native-Windows setup, qualified proof record, limitations, and rollback. |

Native onboarding, profiles, credentials, eligibility, audit, ownership, recovery, SSH/SFTP, Mapepire, configuration, TUI, and validation implementation files remain out of scope. If a slice requires changing one, stop for scope review.

## Test and evidence plan

1. Domain fake tests prove the neutral port, accepted/rejected query table, exact 41-byte canonical value, and normalized-result invariants without transport.
2. Connector loopback tests prove strict bodies, auth/generation/version/ID correlation, 512/1024-byte limits, no retry, timeout mapping, and redaction.
3. MCP/CLI tests prove exact two-tool surfaces, profile-based isolation, contradiction rejection before construction, lazy Companion availability, and unchanged Native ordering.
4. TypeScript tests independently prove query recognition, adapter public API use, arbitrary raw-label normalization, fixed-port broker behavior, and sanitized states.
5. Admission tests hold at least ten fake `runSQL` calls concurrently, complete them in reverse order, prove exact correlation and no global mutex, reject excess work immediately, and prove timed-out active promises retain capacity until settlement.
6. Cross-platform Windows-boundary fakes prove immutable PowerShell invocation, readiness-before-secret, bounded stdio, timeout termination, generation cleanup, redaction, and no general process runner.
7. Separately authorized Windows-native tests prove the actual directory/file DACL and cross-user behavior. Passing fake tests never upgrades this evidence claim.
8. CI runs Go scoped tests plus Companion install, lint, typecheck, test, build, package, and VSIX inspection without VS Code or IBM i; existing Go verification remains unchanged.

## Release, rollout, and rollback

`.github/workflows/release-companion.yml` verifies ordinary changes and `companion-v*` tags. Its publication job is inert unless all of these hold: the ref is a valid `companion-v*` tag, repository variable `COMPANION_PUBLISH_ENABLED` is exactly `true`, and the protected `companion-marketplace` environment approves the job. Only that job receives `id-token: write`; workflow/default permissions remain `contents: read`; no Marketplace PAT secret is accepted. The intended first tag is `companion-v0.1.0`, but identity, trusted-publisher registration, and Windows evidence must be approved before enabling the variable.

The first live rollout is one native Windows VS Code Companion broker and native Windows Nexus process per user. Install Code for IBM i and the Companion, connect Code for IBM i, start `nexus serve` without a profile, inspect `session.status`, and run only the proof query. Multi-window, WSL, SSH, containers, forwarding, discovery, and extension-host identity claims remain unsupported.

Rollback disables the publication variable, removes/disables the Companion package and Companion composition, and removes only the generation-owned descriptor. It does not migrate, modify, delete, or recreate any Native state.

## Delivery strategy and bounded slicing

Use a Feature Branch Chain with draft/no-merge integration branch `feature/add-codefori-companion-provider`. Child 1 targets the integration branch; each later child targets its immediate predecessor. This is the single honest slicing pass. Tests stay with each behavior, no code is compressed to meet the budget, and any measured slice above 400 changed lines stops for a `size:exception` decision.

```text
feature/add-codefori-companion-provider (draft tracker)
  └─ 1 neutral provider domain (180–280) 📍
      └─ 2 Go fixed-loopback connector (330–400)
          └─ 3 MCP/CLI mode isolation (330–400)
              └─ 4 TS package, query, protocol, broker (350–400)
                  └─ 5 public adapter and admission lifecycle (340–400)
                      └─ 6 fixed Windows token setup (350–400)
                          └─ 7 native-Windows consumer/security gates (320–400)
                              └─ 8 package, protected OIDC route, docs (300–390)
```

These eight sub-slices form three major milestones: Go provider path (1–3), TypeScript broker path (4–6), and Windows/release evidence (7–8). No size exception is forecast after this pass, but estimates are not permission to omit tests or documentation.

## Immediate next step

Implement only `internal/provider/provider.go` and `internal/provider/provider_test.go` under strict TDD. Acceptance requires the neutral interface and state/result invariants, the complete ASCII recognizer table, exact canonical 41-byte output, rejection before any fake provider call, and no transport/Windows/vendor imports. Then proceed to the fixed-loopback connector as slice 2. Do not claim Windows functionality, IBM i behavior, publication, or end-to-end readiness from slice 1.
