# Archive Report: Nexus Configuration TUI

## Final State

- Change: `nexus-configuration-tui`
- Persistence: hybrid OpenSpec + Engram
- Archive date: 2026-08-22
- Final synchronized main: `33b03112a69ded73898dfe89d6d46945220d4e9f`
- Native review gate: structurally absent; archive proceeded under ordinary repository policy
- Action context: repo-local; all edits stayed within the repository allowed root

The six implementation slices are complete. The historical failed verification was superseded and is not the current verdict. Maintainer-authorized remediation merged in PR #84 as `6fa3b7021c807f164846dc6465b0c6354d6215e6`; the canonical current verification report was merged by PR #86 as `33b03112a69ded73898dfe89d6d46945220d4e9f`.

## Gates

- Tasks: PASS — 13/13 complete; no unchecked implementation tasks in `tasks.md`.
- Verification: PASS WITH WARNINGS — 12/12 requirements, 32/32 scenarios, 0 blockers, 0 critical findings.
- Current verification evidence: GHA `32576672325` passed full tests, race tests, Windows profile tests, static checks, formatting/operator checks, and six-target builds.
- Canonical report SHA-256: `sha256:23908b5074e911c6a72299a7ec71d3c59b0234ff9d521296b84da0e167280007`.
- Sole warning: controlled IBM i validation remains external; status is `ready_for_controlled_ibmi_validation` / `not_validated_on_ibmi`.
- No IBM i contact or live validation occurred. Local Go execution remained prohibited by WDAC.

## OpenSpec Synchronization

- Created `openspec/specs/nexus-configuration/spec.md` from the complete delta.
- Updated `openspec/specs/local-mcp-security/spec.md` by applying the four modified requirements while preserving unrelated requirements, including local-principal authorization and the operator-ready field-validation package.
- No requirements were removed or renamed.

## Mechanical Evidence

The new main specification was copied with native Git tooling and verified before publication:

```text
Command: cp.exe -R openspec/changes/nexus-configuration-tui/specs/nexus-configuration/spec.md openspec/specs/nexus-configuration/.spec.md.archive-copy.tmp
Command: diff.exe -r openspec/changes/nexus-configuration-tui/specs/nexus-configuration/spec.md openspec/specs/nexus-configuration/.spec.md.archive-copy.tmp
Output: [empty]
Command: mv.exe openspec/specs/nexus-configuration/.spec.md.archive-copy.tmp openspec/specs/nexus-configuration/spec.md
```

The change folder was snapshotted before a native `git mv` and verified against that pre-move snapshot:

```text
Command: cp.exe -R openspec/changes/nexus-configuration-tui <temporary-snapshot>\source
Command: git mv openspec/changes/nexus-configuration-tui openspec/changes/archive/2026-08-22-nexus-configuration-tui
Command: diff.exe -r <temporary-snapshot>\source openspec/changes/archive/2026-08-22-nexus-configuration-tui
Output: [empty]
```

The active change directory is absent. The archive contains proposal, exploration, both delta specs, design, tasks, apply-progress, verify-report, and this additive archive report. The byte-identity comparisons excluded only this report because it was created after the pre-move snapshot.

## Engram Traceability

Full artifact observations read before archival:

| Artifact | Observation ID |
|---|---:|
| `sdd/nexus-configuration-tui/proposal` | 2235 |
| `sdd/nexus-configuration-tui/spec` | 2237 |
| `sdd/nexus-configuration-tui/design` | 2238 |
| `sdd/nexus-configuration-tui/tasks` | 2241 |
| `sdd/nexus-configuration-tui/apply-progress` | 2242 |
| `sdd/nexus-configuration-tui/verify-report` | 2269 |

Historical observations were not used as current gate authority: the failed report and issue #81/PR #82 were superseded by the remediation and current report. Safety WIP remains outside the candidate at `safety/profile-recovery-wip@55ed60b73e4a5b612750c9b362d8485991191edb`.

## Rollback Boundary

Before delivery, revert the archive/spec-sync documentation changes only: remove the dated archive folder and newly created main specification, restore the prior `local-mcp-security` main spec, and restore the active change folder from the archive. Production code, tests, merged remediation, safety WIP, and external systems are outside this rollback boundary.

## Verdict

**ARCHIVED WITH WARNING** — the SDD cycle is complete, with only approved controlled IBM i validation remaining as an external follow-up. No production code, IBM i system, external configuration, RDD review, or safety WIP was modified.
