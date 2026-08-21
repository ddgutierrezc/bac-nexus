# bac-nexus

BAC Nexus is a local-first, deployment-neutral, agent-agnostic Model Context Protocol (MCP) server that exposes bounded IBM i catalog context to MCP-compatible agents.

The v1 surface is read-only and intentionally narrow. It does not provide a generic remote, SSH, SQL, shell, path, listing, or delete operation.

## What this binary is

`nexus` is the MCP server entry point. It accepts stdio JSON-RPC from an MCP client and exposes two typed tools:

| Tool | Purpose |
|---|---|
| `resolve_catalog_candidates` | Resolve up to 50 catalog candidates for a bounded query. Returns no source content. |
| `read_selected_source` | Read a single source page for the exact selection bound to a cursor. Cursor is opaque and never echoed. |

Both tools are typed, fail closed, and honor `context.Context` cancellation. Source content is never returned by the catalog resolve; only the bounded coordinate set. Source read requires an opaque cursor issued by an earlier `resolve_catalog_candidates` call that targeted the same selection.

## Quick path

1. Build: `go build ./...`
2. Test (available runners only): `go test -count=1 ./...`
3. Run: `nexus serve -profile <name>`
4. Wire any MCP-compatible client (Copilot, OpenCode, Codex, etc.) to the resulting `nexus` binary over stdio.

The server uses the official [`github.com/modelcontextprotocol/go-sdk`](https://github.com/modelcontextprotocol/go-sdk) (v1.2.0) and `mcp.NewServer` + `mcp.AddTool` + `StdioTransport`. The wire protocol is the standard Model Context Protocol; no Nexus-specific transport is added.

## Project layout

| Path | Role |
|---|---|
| `cmd/nexus` | The MCP server binary. Composes the service and runs the stdio MCP server. |
| `cmd/catalogspike` | The pre-v1 diagnostic CLI. Behavior is preserved; it is not wired into the MCP server. |
| `internal/mcp` | The typed MCP adapter. Exposes the two allowed tools and forwards to the service. |
| `internal/app` | The local-OS-principal catalog-context service. Owns recovery, freshness, credential, and policy wiring. |
| `internal/source` | Snapshot, lease, ownership, and bounded recovery. No remote/path/shell surface. |
| `internal/security` | Local-principal policy, pinned host trust, no clientInfo, no parent verification. |
| `internal/audit` | Sanitized, allowlisted audit events. Excludes source, paths, credentials, commands, host, user, model content. |
| `internal/credential` | Consumer-owned native-keyring adapter. Windows Credential Manager, macOS Keychain, Linux Secret Service. Fails closed before any remote work. |
| `internal/ownership/sqlite` | Portable SQLite ownership ledger. Rollback-journal `DELETE` + `synchronous=EXTRA`, no WAL. |
| `docs/SECURITY.md` | Canonical threat model, trust boundary, and incident guidance. |
| `openspec/changes/v1-mcp-foundation/` | SDD proposal, design, specs, and tasks for the v1 MCP foundation. |

## What this binary is not

- Not a generic SSH client. There is no shell, command, or remote execution tool.
- Not a SQL client. There is no SQL execution tool, no read/write/delete, no administrative path.
- Not a mutation tool. v1 is read-only by design.
- Not authenticated against Copilot, OpenCode, or any specific product. The current local OS principal is the trust boundary; advisory selectors are not product authentication.

## Verification

```bash
go test -count=1 ./...
go vet ./...
gofmt -l .  # must produce no output
```

The Go test suite uses deterministic fakes and, where policy allows, temporary SQLite databases. The exact-head GitHub Actions run is the canonical runtime evidence; local Windows Defender Application Control may block the test binary and is not bypassed.

## Documentation

- [docs/SECURITY.md](docs/SECURITY.md) — security boundary, trust model, recovery lifecycle, incident guidance.
- [openspec/changes/v1-mcp-foundation/](openspec/changes/v1-mcp-foundation/) — the SDD proposal, design, specs, and tasks that produced this code.
- [AGENTS.md](AGENTS.md) — agent context, engineering rules, and reference projects.

## Rollout

Production rollout is still blocked on approved IBM i access, source/data classification, audit policy, corporate dependency and endpoint policy, signed/approved Nexus distribution, and an approved local database directory. See `docs/SECURITY.md` for the rollout gate list.
