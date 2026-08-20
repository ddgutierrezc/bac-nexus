# Proposal: v1 MCP Foundation

## Intent

Evolve `catalogspike` into a local-first, deployment-neutral, agent-agnostic MCP for authorized, coherent IBM i Catalogados traversal across Windows/macOS/Linux.

## Scope

### In Scope
- Exactly two typed stdio read tools: bounded `resolve_catalog_candidates` and paginated `read_selected_source`.
- Immutable process-local snapshots: one-based complete UTF-8 lines, opaque client/selection-bound cursors, fresh exact per-page re-query; limits: 4 MiB/member, 16 MiB aggregate, 10-minute idle TTL refreshed on valid access, 200 lines/128 KiB per page.
- Portable SQLite `OwnershipLedger` stores no secrets; recovery iterates at most 65 exact ownership rows.
- One approved `CredentialStore` adapter uses Windows Credential Manager, macOS Keychain, or Linux Secret Service; failure returns `credentials_unavailable` before remote access.
- Current local OS principal is each platform's trust boundary; selectors advisory. Configurable pinned TOFU, deterministic errors, sanitized audit, strict TDD.
- Independent sub-400-line slices preserving `catalogspike` prefix behavior.

### Out of Scope
- Durable source cache, snapshot logging/storage, or cursor resumption after expiry/restart.
- Generic SSH, SQL, or CL tools; mutation, graphs, TUI, or other connectors.
- Silent prompts, plaintext/SQLite secrets, or portable encrypted-vault fallback; automated live IBM i tests; Mapepire JAR redistribution.

## Capabilities

### New Capabilities
- `ibmi-catalog-context`: Bounded resolution, immutable pagination, and durable temporary ownership.
- `local-mcp-security`: Cross-platform OS-principal trust, advisory selectors, native credentials, pinned-host trust, source privacy, and audit.

### Modified Capabilities
None.

## Approach

Credentials resolve before remote access; each authorized page re-queries the exact selection. Page one copies/downloads once, validates/indexes immutable lines, confirms cleanup, and publishes the cursor; later pages use the lease. `snapshot_expired` restarts at line 1. Recovery acts only on each exact validated recorded private remote path, never discovering, scanning, glob-matching, or deleting by prefix. Historical shared `/tmp` paths remain outside automatic recovery.

## Affected Areas

| Area | Impact | Description |
|---|---|---|
| `cmd/nexus`, `internal/mcp`, `internal/app`, `internal/security`, `internal/audit` | New | Tools/security |
| `internal/catalog`, `internal/source`, `internal/remote`, `cmd/catalogspike`, `go.mod`, `docs/` | Modified | Ownership/migration |

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| Source disclosure/resource abuse | Medium | Authorization/redaction/limits |
| Secure store unavailable | Medium | Fail closed pre-remote |
| Remote orphan | Medium | Exact-ledger cleanup |
| PoC gate failure | High | Blocks implementation/delivery; production rollout requires approval |
| Delivery regression | High | Rollbackable TDD slices |

## Rollback Plan

Disable `cmd/nexus`, invalidate leases, revert slices, and retain `catalogspike`; revoke profiles/native-store entries after exposure. Cleanup uses exact recorded private paths only.

## Dependencies

- PoC-only exception approves `modernc.org/sqlite` v1.38.2 (Go 1.23; SQLite 3.50.1; windows/darwin/linux amd64+arm64) and `github.com/zalando/go-keyring` v0.2.8 (Go 1.18). `modernc` v1.39+ requires Go 1.24, so is excluded. Production/corporate rollout remains unapproved. Other prerequisites: approved MCP Go SDK, external checksum-verified Mapepire JAR, IBM i, source/audit policy, validation window, signed/approved Nexus, endpoint-policy acceptance; no WDAC/equivalent bypass. SQLite selects rollback-journal `DELETE` + `EXTRA`; no WAL/reset reliance.

## Success Criteria

- [ ] TDD proves bounded replay/order-independent traversal; TTL/bindings, re-query, line-1 restart, copy-once, cleanup-before-cursor, and exact-ledger recovery.
- [ ] No secret/source/hash/cursor reaches argv/environment/logs/audit/fixtures/artifacts/SQLite; native-store failures return `credentials_unavailable` with no remote attempt/fallback.
- [ ] Exact module graph/SBOM, license inventory, govulncheck/known-vulnerability review, checksum lock, and no runtime download/DLL pass; platform compile/tests cover windows/darwin/linux and supported amd64/arm64 without unavailable-runner promises. Any failure blocks implementation/delivery.
- [ ] Approved validation reaches EOF, verifies newlines/cleanup, and retains no source under corporate policy.
