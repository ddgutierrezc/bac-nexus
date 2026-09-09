# Code for IBM i Companion v1

The Companion provides zero-configuration, bounded proof access to the active
Code for IBM i session. Code for IBM i retains all IBM i credentials.

## Quick path

1. Start Code for IBM i with an active session.
2. Start `nexus serve` without `-profile` to select Companion mode.
3. Use `session_status`, the canonical proof query, `resolve_program`, or `find_program_source`.

## Fixed local endpoint

| Topic | Decision |
|---|---|
| Address | The Companion binds only to `127.0.0.1:64139`. |
| Operations | Public MCP tools are `session_status`, `sql_query`, `resolve_program`, metadata-only `find_program_source`, and `resolve_catalog_candidates`. Internal dotted RPC methods include `session.status`, `sql.query`, `program_inspection.v1.resolve`, and `program_inspection.v1.find_source`. |
| Port collision | The Companion is unavailable; it does not scan, retry another port, or fall back to Native mode. |
| Browser requests | Any request with an `Origin` header is rejected with `browser_origin_rejected`. |

## Local-machine trust boundary

This v1 endpoint has no caller authentication. Any local process that can reach
the loopback port may call its narrow operations. It does not establish an OS
user, session, process, extension-host, or other caller identity.

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
compiled-object source coordinates, so it returns `unavailable`. `read_source_member`
is explicitly absent pending a separate approved bounded source-acquisition design.

## Credential ownership and future hardening

Code for IBM i exclusively owns IBM i credentials. The Companion and Nexus do
not request, receive, persist, or log them. Peer authentication and stronger
enterprise endpoint hardening are deliberately deferred from v1.

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
