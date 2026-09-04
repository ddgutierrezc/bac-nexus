# Archive Report: Enable Production Nexus Serve

## Closure Status

**Change**: `enable-production-nexus-serve`  
**Archived**: `2026-09-03`  
**Verdict at close**: **PASS WITH WARNINGS**  
**Completion**: 19/19 tasks, 7/7 requirements, 22/22 scenarios  
**Blockers / CRITICAL findings**: 0 / 0

The persisted `tasks.md` is fully checked. The final independent verification report records `pass_with_warnings`, zero blockers, zero CRITICAL findings, and complete requirement/scenario coverage. The archived change is complete.

## Final-State Evidence

The final state supersedes historical intermediate failures and pending claims in `apply-progress.md` and earlier verification snapshots:

- Runtime remediation passed: cancellation/session close precedes handler waits, and lifecycle errors are sanitized. Evidence: `sha256:7af582b296964a1c9221967238a567c7aa011eca00f0adde399fb25e95eaf292`.
- Strict-TDD subprocess remediation passed: completion/reaping is derived from `exec.Cmd.ProcessState.Exited()` after `Wait`; the `childCount` tautology is absent. Evidence: `sha256:83fc3e058438396a93c33e43a2cfb6d20cb45445df45a499fb2c82b4954f8a83`.
- Final independent verification passed with warnings. Evidence: `sha256:be4c1dec65cb666a5a36c3f1dfd1b6ef8d5a9c5881317e3209a88b76cc865e9d`; report SHA-256: `sha256:de633173fb10c8e75b0916ad89abdb368e241d2b60601d59077384fc5d19f2ea`.

## Specification Synchronization

| Domain | Action | Details |
|---|---|---|
| `local-mcp-security` | Updated | Replaced `Sanitized Read-Only Surface and Audit` from its delta; 1 modified requirement, preserving unrelated requirements. |
| `nexus-configuration` | Updated | Replaced `Bounded Backend Connection and Persistence` from its delta; 1 modified requirement, preserving unrelated requirements. |
| `production-nexus-serve` | Created | Mechanically copied the full specification; 5 requirements and 11 scenarios. |

The source-of-truth specifications now reflect the archived behavior:

- `openspec/specs/local-mcp-security/spec.md`
- `openspec/specs/nexus-configuration/spec.md`
- `openspec/specs/production-nexus-serve/spec.md`

## Retained Warnings and Non-Claim

The following non-blocking warnings remain at close:

1. `internal/configuration.CheckLocalReadiness` has a stale diagnostic describing the completed production composition as missing.
2. Historical Strict TDD work-unit evidence is incomplete for 11 of 19 older tasks.
3. Nineteen changed applicable implementation files remain below the informational 80% coverage threshold.
4. The controlled live gate lacks explicit child kill/reaping after its timeout.
5. The `defaultDeps` resolver/acquirer comment is stale.

The retained non-claim is `not_validated_on_ibmi`. `TestControlledNexusServe` was not executed; controlled IBM i validation remains a separate explicit operation.

## Mechanical Archive Evidence

`production-nexus-serve` was copied with `cp` to a temporary file and compared before the atomic move into the main specification path.

```text
diff -r openspec/changes/enable-production-nexus-serve/specs/production-nexus-serve/spec.md <temporary target>
(empty output; byte-identical)
```

The active change directory was copied to a recursive snapshot before moving to its exact archive destination. The moved archive was compared with that snapshot, excluding this additive archive report.

```text
diff -r <pre-move snapshot>/source openspec/changes/archive/2026-09-03-enable-production-nexus-serve
(empty output; byte-identical)
```

## Archive Contents

- `proposal.md`
- `exploration.md`
- `specs/`
- `design.md`
- `tasks.md` (19/19 checked)
- `apply-progress.md`
- `verify-report.md`
- `archive-report.md`

No stale-checkbox reconciliation or partial-archive override was used.
