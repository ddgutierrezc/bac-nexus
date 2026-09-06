# Proposal: Add a deployable Code for IBM i Companion provider

## Intent

Deploy a zero-configuration v1 PoC that lets Nexus agents consume the active Code for IBM i session. Code for IBM i exclusively owns credentials; Nexus and Companion neither request nor persist them.

## Scope

### In Scope
- A TypeScript Companion using Code for IBM i 3.0.12 APIs in its extension host.
- `CodeForIProvider` over HTTP/JSON on `127.0.0.1`, exposing only `session.status` and proof `sql.query`.
- Browser-origin rejection; narrow allowlists; strict request, response, time, depth, and concurrency bounds; deterministic errors; no arbitrary SQL, shell, CL, or mutation.
- Native behavior through explicit `-profile`, without fallback.

### Out of Scope
- Descriptors, tokens, generation authentication, PowerShell publication/cleanup, and Windows ACL work.
- Peer authentication, enterprise hardening, network exposure, endpoint discovery, generic SQL, and credential transfer.

## Capabilities

### New Capabilities
- `codefori-companion-provider`: Bounded access to the active session through a loopback Companion.

### Modified Capabilities
- `local-mcp-security`: Replace token, descriptor, and ACL requirements with explicit v1 local-machine trust.
- `production-nexus-serve`: Preserve isolated no-profile Companion and profile-selected Native modes without fallback.

## Approach

The Companion owns listener lifecycle and invokes allowlisted Code for IBM i APIs. Nexus maps MCP tools through `CodeForIProvider`; both sides validate the query and sanitize results. The broker is loopback-only, origin-restricted, bounded, and fail-closed. v1 trusts processes on the local machine and claims no caller identity.

## Affected Areas

| Area | Impact | Description |
|---|---|---|
| `companion/vscode-codefori/` | Modified | Remove descriptor/authentication/ACL flow; retain bounded broker. |
| `internal/connectors/ibmi/codefori/` | Modified | Use fixed unauthenticated loopback transport. |
| `cmd/nexus/` | Modified | Preserve profile-based provider isolation. |

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| Any local process invokes narrow operations | Medium | Accepted v1 residual risk; retain loopback, origin, allowlist, and bounds. |
| v1 is mistaken for hardened authentication | Medium | State that peer authentication and enterprise hardening are deferred; claim no peer identity. |
| Fixed port is unavailable | Low | Fail deterministically without scanning or fallback. |

## Rollback Plan

Disable/uninstall Companion and revert its wiring. Continue using unchanged Native mode with `nexus serve -profile <name>`; no credential migration is required.

## Dependencies

- Code for IBM i 3.0.12 with an active session.

## Success Criteria

- [ ] A deployed PoC returns bounded status and proof-query results with zero credential configuration.
- [ ] Loopback, origin rejection, allowlists, bounds, deterministic errors, and fail-closed behavior are verified.
- [ ] Descriptor, token, PowerShell, ACL, generation authentication, and credential persistence are absent.
- [ ] Native `-profile` behavior remains unchanged with no silent fallback.
- [ ] Documentation states the any-local-process risk and deferred hardening without claiming peer identity.
