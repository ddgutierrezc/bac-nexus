# Proposal: Enable Production Nexus Serve

## Intent

Make `nexus serve -profile <name>` usable by agents for catalog resolution and real IBM i source retrieval. Today startup fails because profile admission and the resolver, acquirer, recovery, and lease dependencies are not composed.

## Supersession

**Supersedes: `mapepire-artifact-acquisition`.** Maintainer decision `enable-production-nexus-serve` is the authoritative Mapepire 2.3.6 remote receipt/launch policy. The superseded change remains historical and unimplemented (0/9 tasks); it is not archived or completed by this relationship.

## Scope

### In Scope
- Admit one approved V3/keyring profile per stdio process; reject prompt, legacy, missing, stale, or mismatched eligibility.
- Compose fixed SSH Mapepire Catalogados resolution, request-scoped SSH/SFTP source acquisition, durable ownership recovery, crypto-random leases, bounded lifecycle, and sanitized errors.
- Add append-only redacted local audit with mandatory retention configuration, operator/release documentation, deterministic fake-based tests, and an explicit opt-in IBM i gate.
- Deliver reviewable stacked slices of at most 800 changed lines, beginning by stabilizing the verified uncommitted onboarding baseline.

### Out of Scope
- Further TUI visual parity; WSS catalog runtime; generic shell, SQL, SSH, SFTP, path, list, or delete tools.
- Other connectors, graph storage, mutation, cloud, or multi-tenant operation.

## Capabilities

### New Capabilities
- `production-nexus-serve`: Fail-closed profile admission, runtime composition, recovery, stdio lifecycle, and operational readiness for both MCP tools.

### Modified Capabilities
- `local-mcp-security`: Persist bounded redacted audit and enforce profile-bound SSH/Mapepire eligibility and noninteractive credential policy.
- `nexus-configuration`: Create proof-bound serving eligibility while preserving the protected four-step onboarding behavior.

## Approach

First preserve and deliver the dirty four-step onboarding candidate without altering `.atl`, `stash@{0}`, archived evidence, or the historical screenshot branch. Then stack ≤800-line slices for admission/eligibility, SSH-backed composition/recovery, persistent audit/lifecycle, and documentation plus controlled validation. Reserve stdout for MCP JSON-RPC; send sanitized diagnostics to stderr.

## Affected Areas

| Area | Impact | Description |
|---|---|---|
| `cmd/nexus/`, `internal/{profile,configuration,credential}/` | Modified | Admission, eligibility, composition |
| `internal/{connectors/ibmi,mapepire,remote,source,ownership,audit}/` | Modified | Bounded runtime, recovery, audit |
| `README.md`, `docs/`, `.github/workflows/` | Modified | Accurate operations and release evidence |

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| Eligibility or audit policy permits unsafe startup | High | Bind target/policy/pin/artifact identity; fail closed |
| Dirty onboarding work is overwritten | Medium | Stabilize it first; exclude TUI polishing from later slices |

## Rollback Plan

Revert each stacked slice independently, restore the last approved binary/configuration, stop serving, revoke affected credentials and eligibility, invalidate leases, and clean only ledger-owned paths after validated recovery.

## Dependencies

- Approved fixed Mapepire artifact policy, audit retention, target/libraries/identity, and local ownership root `<UserConfigDir>/BAC Nexus/ownership/ownership.db` with restrictive permissions.

## Success Criteria

- [ ] Both MCP tools work through fakes with bounded cancellation, cleanup, deterministic errors, and no IBM i dependency.
- [ ] Invalid profiles/eligibility fail before remote access; recovery precedes serving; stdout remains protocol-only.
- [ ] Release remains `not_validated_on_ibmi` until the explicit controlled live gate succeeds.
