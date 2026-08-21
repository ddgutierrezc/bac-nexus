# Proposal: v1 MCP Foundation

## Intent

Deliver the approved local, agent-agnostic MCP for authorized IBM i Catalogados traversal. SDD closes after implementation, internal-contract verification, and an operator-ready package as **ready for controlled IBM i validation**, never validated on IBM i.

## Scope

### In Scope
- Existing v1 behavior and security: two typed read tools; immutable bound snapshots; exact re-query; 4 MiB/member, 16 MiB aggregate, 10-minute TTL, and 200-line/128-KiB page limits.
- Secret-free SQLite ownership; at most 65 exact private paths; no recovery scanning or prefix deletion.
- Native credentials, OS-principal trust, advisory selectors, pinned TOFU, deterministic errors, sanitized audit, and fail-closed remote access.
- Operator runbook/checklist, sanitized evidence template, binary version/checksum, rollback guidance, and prerequisites.

### Out of Scope
- Existing exclusions: durable cache, generic infrastructure tools, mutation, graphs, TUI, other connectors, insecure secret handling, and JAR redistribution.
- Automated IBM i validation or equivalence between fakes/CI and live IBM i evidence.

## Capabilities

### New Capabilities
- `ibmi-catalog-context`: Bounded resolution, pagination, and temporary ownership.
- `local-mcp-security`: Principal trust, credentials, host trust, privacy, and audit.

### Modified Capabilities
None.

## Approach

Preserve the implemented fail-closed flow and automated contract tests. An authorized operator later runs the checklist and records sanitized IBM i evidence. That evidence is an external rollout gate, not an SDD completion requirement.

## Follow-up Artifacts

- Specs: operator package and non-claim requirements.
- Design/tasks: sanitization, identity, prerequisites, rollback, and artifact verification. Task 4.4 remains unchanged now.

## Affected Areas

| Area | Impact | Description |
|---|---|---|
| Existing v1 code | Unchanged | Product/security scope |
| `docs/`, release artifacts | Modified | Validation package |

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| Live behavior unproven at SDD close | High | Non-claim plus operator gate |
| Evidence disclosure | Medium | Sanitized template and retention rules |

## Rollback Plan

Withhold before rollout. During validation, stop Nexus, invalidate leases, restore the approved binary/configuration, revoke affected credentials, and clean only exact recorded paths.

## Dependencies

- Existing approved runtime/dependency constraints remain.
- Validation requires an authorized operator/identity, supported reachable IBM i, approved libraries/policies/window/binary, endpoint acceptance, and no control bypass.

## Success Criteria

- [ ] Automated verification proves internal behavior/security contracts only.
- [ ] Build, dependency admission, checksum lock, and portability checks pass.
- [ ] Versioned/checksummed binary and operator package are ready for controlled IBM i validation.
- [ ] Live EOF/newline/cleanup/retention proof remains an external gate; SDD makes no IBM i validation claim.
