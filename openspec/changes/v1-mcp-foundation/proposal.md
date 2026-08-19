# Proposal: v1 MCP Foundation

## Intent

Turn `catalogspike` into a local-first, agent-agnostic MCP foundation for explicitly authorized, coherent IBM i Catalogados resolution and source traversal.

## Scope

### In Scope
- Typed stdio tools: bounded `resolve_catalog_candidates` and paginated `read_selected_source`.
- Immutable process-local leases with one-based complete-line UTF-8 pages, opaque client/selection-bound cursors, and fresh exact re-query per page.
- Limits: 4 MiB complete-member ceiling, 16 MiB aggregate snapshot quota, 10-minute idle TTL refreshed on valid access, and 200 lines/128 KiB per response.
- Credential Manager, authorized clients, configurable pinned TOFU, deterministic errors, sanitized audit, and strict TDD.
- Independent sub-400-line slices preserving `catalogspike` prefix behavior.

### Out of Scope
- Durable source cache, sensitive snapshot logging/storage, or cursor recovery across expiry/restart.
- Arbitrary infrastructure operations, mutation, graphs, central deployment, TUI, or other connectors.
- Automated live IBM i tests or Mapepire JAR redistribution.

## Capabilities

### New Capabilities
- `ibmi-catalog-context`: Bounded catalog resolution and coherent arbitrary line pagination for freshly revalidated exact selections.
- `local-mcp-security`: Client authorization, native secrets, pinned-host trust, sensitive-data handling, and sanitized audit outcomes.

### Modified Capabilities
None.

## Approach

Authorize and run a fresh exact catalog re-query on every page. The first page performs one fixed copy/download, validates and indexes immutable UTF-8 lines, confirms cleanup, then publishes the bound cursor. Later pages read the lease. Expiry returns `snapshot_expired`; coherent traversal restarts at line 1. Bounded recovery targets only stale Nexus-owned temporaries, never generic cleanup.

## Affected Areas

| Area | Impact | Description |
|---|---|---|
| `cmd/nexus`, `internal/mcp`, `internal/app` | New | Tools and pages |
| `internal/security`, `internal/audit` | New | Policy and redaction |
| `internal/catalog`, `internal/source`, `internal/remote` | Modified | Selection, leases, cleanup |
| `cmd/catalogspike`, `go.mod`, `docs/` | Modified | Prefix migration |

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| Full-source disclosure | Medium | Authorization and redaction |
| Memory/cursor abuse | Medium | Hard resource limits and bindings |
| Remote orphan | Medium | Confirmed cleanup; bounded recovery |
| Delivery regression | High | TDD; rollbackable sub-400-line slices |

## Rollback Plan

Disable `cmd/nexus`, invalidate leases, revert slices independently, and retain `catalogspike`; revoke client profiles and Credential Manager entries after suspected exposure. Cleanup stays Nexus-owned only.

## Dependencies

- Approved MCP Go SDK, Windows APIs, separately supplied checksum-verified Mapepire JAR, IBM i access, source/audit policy, and manual validation window.

## Success Criteria

- [ ] Strict TDD proves complete-line UTF-8 traversal, replay/order independence, limits, TTL, bindings, fresh re-query, and line-1 restart.
- [ ] Each snapshot performs one copy/download; cursor publication follows confirmed cleanup; stale recovery is prefix-bounded.
- [ ] No secret, source, hash, or cursor reaches argv, environment, logs, audit, fixtures, or artifacts.
- [ ] Security failures close safely; approved manual validation reaches EOF, confirms newlines/cleanup, and retains no source.
