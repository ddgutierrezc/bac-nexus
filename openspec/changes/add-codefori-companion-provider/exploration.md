# Exploration: add-codefori-companion-provider

## Scope and evidence boundary

This is a reconciled exploration record only. No production code, command, test, network request, IBM i action, publication, or commit was performed in the corrective pass. Native onboarding, profile, credential, SSH, Mapepire, configuration, TUI, validation, audit, recovery, and ownership implementation are outside the allowed change surface.

The repository had no usable CodeGraph index/tool surface during the original exploration, so its targeted filesystem inspection remains the documented fallback. Formal research was later explicitly deselected after the research executor lacked evidence grants. The unchanged blocked `research.md` is historical and is not validated evidence.

## Existing integration boundaries

- `cmd/nexus/main.go` owns `serve` parsing and current profile-selected Native composition.
- `internal/mcp/server.go` currently exposes only `resolve_catalog_candidates` and `read_selected_source`.
- `internal/app.Service` and its credential/audit/recovery dependencies are direct-provider behavior and are not the place to add Companion SQL.
- Current Native catalog/source operations use existing SSH/SFTP, Mapepire, Catalogados, profile, eligibility, ownership, and recovery boundaries. This change must not alter or reuse those internals.
- The direct onboarding and saved-profile validation routes remain unrelated to Companion discovery and must remain untouched.
- Existing controlled IBM i live tests remain Native-only evidence.
- The repository is currently Go-oriented; the Companion is an independent TypeScript package rather than a root workspace conversion.

## Confirmed external API facts

The retained official-source exploration identified the Code for IBM i 3.0.12 public integration through VS Code extension exports:

1. Resolve `halcyontechltd.code-for-ibmi` and activate it when necessary.
2. Access `exports.instance`.
3. Use `instance.getConnection()`.
4. Use `instance.subscribe(context, 'connected'|'disconnected', name, callback)`.
5. Use `connection.runSQL(statement, options)`.

Corrections to earlier assumptions:

- The public `subscribe` API returns `void`. There is no evidenced unsubscribe handle, so the Companion must not promise disposal of a subscription return value.
- Public `runSQL` accepts broader inputs than this product permits. The Companion must independently constrain input before calling it.
- The public `runSQL` promise has no evidenced cancellation handle.
- No live evidence establishes the exact property name used for the one-column raw row. Result normalization must not require a raw `CURRENT_USER` key.
- Code for IBM i extension-host placement means loopback is host-local, but a folder path or local-volume check does not cryptographically identify a specific extension host.

The type package remains pinned exactly to `@halcyontech/vscode-ibmi-types@3.0.12`. Marketplace OIDC publishing is available through current `vsce` with `contents: read` and `id-token: write`, but publisher identity is not approved.

## Reconciled user decisions

| Area | Confirmed decision |
|---|---|
| First live topology | Native Windows GoLand/Nexus and native Windows VS Code. |
| Serve selection | No profile means Companion; `-profile` means Native; contradictory explicit flags fail closed; no fallback. |
| Companion implementation | 100% TypeScript. |
| IPC | HTTP/JSON on `127.0.0.1`; a known port is acceptable for the spike. |
| Discovery | Automated discovery is later evolution, not MVP work. |
| SQL | One proof statement with bounded ASCII whitespace/case normalization; no parser or arbitrary SQL. |
| Concurrency | No global default serialization; prove correlation with at least ten reordered fake requests. |
| Release | Tag `companion-v0.1.0`; workflow `.github/workflows/release-companion.yml`. |
| Native scope | Never modify/delete/restructure Native onboarding/profile/credential/SSH/Mapepire/configuration/TUI/validation implementation. |

## Minimal recommended architecture

This section is an author recommendation, not a record of additional user approval.

- Keep Native composition unchanged.
- Add a sibling Companion-only MCP composition with `session.status` and `sql.query`.
- Use a small 100% TypeScript broker on a fixed loopback port; `127.0.0.1:64139` is the current design recommendation.
- Use a bounded descriptor only for protocol generation and an ephemeral bearer token; do not build port scanning or multi-host discovery.
- Load the descriptor lazily per operation so Companion absence never blocks Nexus startup.
- Use independent Go and TypeScript finite recognition of the single statement, then call `runSQL` only with a canonical literal and `{rows: 1}`.
- Normalize a raw result only if it has exactly one row object, exactly one own enumerable column, and one bounded string value. Return that value under a stable protocol field rather than preserving an unverified raw label.
- Admit bounded concurrent operations without a global mutex. Do not claim shared-job parallel safety from fake evidence.

## Query count and recognizer finding

The approved literal is:

```text
SELECT CURRENT_USER FROM SYSIBM.SYSDUMMY1
```

Its ASCII/UTF-8 byte count is 41:

```text
SELECT(6) + space(1) + CURRENT_USER(12) + space(1) + FROM(4)
+ space(1) + SYSIBM(6) + dot(1) + SYSDUMMY1(9) = 41
```

The non-whitespace characters total 38. There is no defensible 39-byte count for this literal. The corrective request's 39-byte assertion therefore remains a factual conflict requiring confirmation rather than a documentation value to copy.

Limited normalization can remain parser-free: ASCII-only case folding; permitted leading/trailing ASCII space/tab/CR/LF; one-or-more of those characters between the four tokens; no whitespace around the dot or inside identifiers; no other tokens or bytes. Every accepted form becomes the 41-byte canonical statement before execution.

## Windows security feasibility finding

The token descriptor is security-sensitive. On native Windows:

- Standard Node/VS Code filesystem APIs can write atomically but do not expose full Windows security-descriptor/DACL creation and verification.
- Node `chmod`, `%LOCALAPPDATA%` placement, `globalStorageUri`, local-volume checks, and non-reparse checks are not proof of a protected current-user-only DACL.
- Go can verify a DACL before reading, but consumer verification cannot retroactively prevent exposure caused by a permissive creation directory.
- Folder location, DACL, local volume, token possession, and loopback reachability still do not prove extension-host identity.
- A native addon/helper or external ACL library is not approved and would conflict with the unqualified 100% TypeScript packaging expectation unless separately authorized.
- A TypeScript call to a fixed built-in Windows ACL utility is a possible focused option, not an approved mechanism and not yet security-reviewed.

Therefore Windows owner-only descriptor publication remains a genuine blocker. Failing activation closed is the only currently justified strict behavior.

## Bounds finding

A 500 ms Go response-header timeout cannot permit a five-second Companion SQL execution because HTTP response headers arrive after method execution in the proposed single-response protocol. A coherent bounded starting point is:

- Companion SQL wait: 5 seconds.
- Go response-header timeout: 6 seconds.
- Go total query deadline: 7 seconds.
- Status total deadline: 1 second, which remains the earlier context deadline.

These are design recommendations subject to TDD, not live performance evidence.

## Non-goals

- No changes to the Native path or its data/security lifecycle.
- No mixed tool surface, availability fallback, broad SQL grammar, parser, alternate query, shell, CL, mutation, or credential transfer.
- No automatic VS Code launch, port forwarding, WSL/Remote bridge, extension-host identity claim, or multi-window discovery.
- No promised unsubscribe API, raw `CURRENT_USER` row key, remote SQL cancellation, or live parallel-safety claim.
- No live IBM i or live VS Code validation in normal CI.

## Remaining decisions

1. Approve a Windows token-file ACL mechanism callable from the TypeScript package, or explicitly accept an inherited-ACL spike exception and its residual exposure risk.
2. Confirm the 41-byte literal or supply the intended different 39-byte SQL literal.
3. Later, approve extension/Marketplace identity and OIDC registration before publication.
4. Separately authorize live Windows/VS Code/IBM i validation; until then retain `not_validated_on_ibmi`.

The narrow next action is review of items 1 and 2, not implementation.
