# Code for IBM i Companion v1

The Companion provides zero-configuration, bounded proof access to the active
Code for IBM i session. Code for IBM i retains all IBM i credentials.

## Quick path

1. Start Code for IBM i with an active session.
2. Start `nexus serve` without `-profile` to select Companion mode.
3. Use `session_status`, the canonical proof query, program inspection tools, or `read_selected_source` after an exact catalog selection.

## Per-instance local endpoint

| Topic | Decision |
|---|---|
| Address | Each Companion extension host binds only to an OS-assigned `127.0.0.1` port. |
| Operations | Public MCP tools are `session_status`, `sql_query`, `resolve_program`, metadata-only `find_program_source`, `resolve_catalog_candidates`, and `read_selected_source`. The latter accepts an exact selected candidate for its first bounded page and only an opaque cursor thereafter. |
| Discovery | Each host atomically publishes a private v2 instance record with its random identity, endpoint, 256-bit token, connection generation, eligibility, and bounded lease. |
| Browser requests | Any request with an `Origin` header is rejected with `browser_origin_rejected`. |

## Local-machine trust boundary

This local endpoint requires the private 256-bit loopback token in
`X-Nexus-Companion-Token` before it parses a request body or performs work. On
each successful Companion listener start, the extension generates and rotates
the token, then stores endpoint and token together in a private per-instance v2
record. Nexus reads and validates that record locally;
the token is not an MCP input or output, and users do not configure it.

The token state is protected for the current OS principal where the platform
permits. The endpoint does not distinguish individual processes running under
that same principal: a process that can read the private token state can call
the loopback endpoint. It is not an Internet-facing or remote-access boundary,
and the port must not be exposed or forwarded.

Nexus selects exactly one connected, focused, unexpired v2 instance. If it
cannot prove a unique eligible instance, it fails closed before transport; it
never uses startup order or silently fails over after disappearance or an
authentication failure. A same-instance token refresh may retry once. Legacy v1
fixed-endpoint state is a temporary fallback only when no valid v2 record exists.
The extension renews its owned record every ten seconds with a 30-second lease;
shutdown stops that heartbeat before disposal and removes only its matching
registration. An empty registry or expired-only crash residue permits v1
fallback. A malformed, insecure, or otherwise invalid v2 sibling fails closed
and never falls back to v1.
Shutdown atomically quarantines a record before deleting it. If its content no
longer proves ownership after that move, Nexus retains the private quarantine
artifact rather than risk deleting a replacement registration; it consequently
fails closed until approved local cleanup or lease handling resolves it.

The endpoint rejects browser-origin requests and accepts no arbitrary SQL,
shell, CL, mutation, endpoint discovery, forwarding, or remote access.

`resolve_catalog_candidates` returns bounded metadata through fixed, parameterized Catalogados queries. Users configure no tokens, SQL, or allowlists.

## Fixed limits

The endpoint accepts a 512-byte request and produces at most a 4096-byte metadata
response. It permits one normalized row and column, a 256-byte value, 16
immediately admitted operations and active queries, and no queue. A proof query
waits at most five seconds; excess work returns `limit_exceeded`.

## Program inspection

`resolve_program` supports only `*PGM`. With no explicit library it searches
Code for IBM i's configured `currentLibrary` followed by `libraryList`, in stable
de-duplicated order; this is not runtime `*LIBL`. The configured scope is capped
at 16 libraries and matches at 8. Truncated scope never mints a selection.

An opaque Nexus-local selection is replayable for five minutes so read-only MCP
calls may retry; the server retains at most 256 live selections and fails closed
when full. `find_program_source` accepts only that selection and returns metadata
only. Documented Code for IBM i APIs do not yet provide independently evidenced
compiled-object source coordinates, so it returns `unavailable`.

## Selected source paging

`read_selected_source` reads at most 200 one-based source lines from the exact
candidate selected from `resolve_catalog_candidates`. Later requests provide
only the opaque cursor returned by a non-EOF page. Source lines preserve their
content, including trailing spaces; Companion newline normalization is accepted
because the MCP result is line-oriented. At EOF, the response omits both cursor
and next start line, and Nexus disposes the Companion artifact before returning.
Ambiguous, expired, unavailable, malformed, oversized, and cleanup outcomes
return no source content. Nexus never chooses among ambiguous candidates.

## Credential ownership and hardening

Code for IBM i exclusively owns IBM i credentials. The Companion and Nexus do
not request, receive, persist, or log IBM i credentials. The Companion persists
only the private local loopback authentication state; it is not an IBM i
credential. Current authentication is a shared local token, not per-process or
enterprise peer identity; stronger endpoint hardening remains a future concern.
On Windows, the registry stays below the current user's application-data
directory and rejects symlink and non-regular entries. Nexus does not claim to
independently validate Windows owner or DACL information, and it uses no shell
or PowerShell check to attempt one.

## Verification limits

Offline Go and Node checks prove protocol, bounds, and fake/loopback behavior.
They do not prove a live VS Code extension host, IBM i connectivity, or proof
query execution against an IBM i system. Live validation requires separate
authorization.

## Marketplace pre-release automation

Companion pre-releases are published only from an explicit `companion-v*` Git
tag after the same offline checks used by pull requests and branch pushes pass.

### Release checklist

1. Create the repository secret named `VSCE_PAT`. Its short-lived Azure DevOps
   Marketplace PAT must have **Marketplace > Manage** scope and access to all
   accessible organizations. Do not place the token in source code, a tag, or
   workflow input.
2. Update `companion/vscode-codefori/package.json` to the release version.
3. Create a matching tag. For the first pre-release, use `companion-v0.1.0`.
4. Monitor the run in the repository's
    [Actions tab](https://github.com/ddgutierrezc/bac-nexus/actions).

| Topic | Rule |
|---|---|
| Tag convention | `companion-v<package-version>`; for example, `companion-v0.1.0`. |
| Version gate | The text after `companion-v` must exactly equal `package.json`'s `version`; otherwise the workflow stops before packaging or publishing. |
| Published bytes | One VSIX is packaged and that same file is published as a Marketplace pre-release. |
| Authentication | The workflow maps the `VSCE_PAT` secret only to the publishing step's environment. It never supplies the token as a command argument. |

See the official [VS Code extension publishing guide](https://code.visualstudio.com/api/working-with-extensions/publishing-extension) and [Azure DevOps PAT guidance](https://learn.microsoft.com/azure/devops/organizations/accounts/use-personal-access-tokens-to-authenticate?view=azure-devops).

## Rollback

Disable or uninstall the Companion and run `nexus serve -profile <name>` to use
the unchanged Native path. No credentials, descriptors, tokens, or data require
migration, and Companion unavailability never selects Native mode automatically.

Deleting a release tag stops no publication that has already completed. Removing
or deprecating a Marketplace version is a separate Marketplace operation.
