# Archive Report: v1 MCP Foundation

## Closure

`v1-mcp-foundation` is archived after implementation, independent verification, and maintainer-authorized delivery. The final product status is `ready_for_controlled_ibmi_validation` and `not_validated_on_ibmi`. No live IBM i validation was performed or claimed.

## Final Delivery Evidence

| Evidence | Final state |
|---|---|
| Pull request | #62 merged as squash commit `3af350a40ca3660ac649f8d807cffd2cfe1d89c0`; issue #61 is closed. |
| Post-merge CI | GitHub Actions `32529744090` passed on the exact `main` commit. It covered tests, vet, formatting/operator-contract checks, six-target package/manifest verification, and artifact upload. |
| Independent verification | PASS WITH WARNINGS: 13/13 requirements, 43/43 scenarios accounted, 42/42 SDD-required runtime scenarios passed, and 0 CRITICAL blockers. |
| Native settlement | Completed exactly once at ordinal 75. |
| Runtime limitation | WDAC blocked local Go runtime; no local Go test execution is claimed. |

The final verification fact supplied at archive launch identifies canonical report hash `6f6f7ec41cca454da5232c5e7329538c8e33b93ae375949fa9a0cd6dcd647075`.

## Source-of-Truth Synchronization

| Domain | Action | Details |
|---|---|---|
| `ibmi-catalog-context` | Created | Mechanically copied the full delta as its first canonical spec: 7 requirements and 27 scenarios. |
| `local-mcp-security` | Created | Mechanically copied the full delta as its first canonical spec: 6 requirements and 16 scenarios. |

The canonical specifications are now:

- `openspec/specs/ibmi-catalog-context/spec.md`
- `openspec/specs/local-mcp-security/spec.md`

## Artifact and Task Closure

- Archive path: `openspec/changes/archive/2026-08-21-v1-mcp-foundation/`
- Preserved artifacts: proposal, delta specs, design, tasks, apply progress, and verification report.
- Persisted task artifact: 42/42 canonical tasks checked; no unchecked implementation tasks.
- Native status at archive start: `taskProgress` 5/5 complete, `archive: ready`, no blocked reasons, no remediation required, and repo-local edits only.
- Receipt-driven development was disabled and `reviewGate` was structurally absent; no native review artifact was required or read.

## Byte-Identity Readback

All filesystem archival operations used Git Bash native `cp`/`git mv` and POSIX `diff -r`. The diff output for every operation was empty:

```text
BEGIN diff -r ibmi-catalog-context
END diff -r ibmi-catalog-context (empty)
BEGIN diff -r local-mcp-security
END diff -r local-mcp-security (empty)
BEGIN diff -r archive snapshot -> archived tree
END diff -r archive snapshot -> archived tree (empty)
```

The recursive archive comparison used a pre-move snapshot under the approved temporary directory and confirmed byte identity before this additive report was created.

## Final Warnings and Follow-up Boundaries

The following non-blocking warnings remain final-state facts:

1. Historical design text describing work 4.4 as future/currently absent is stale.
2. The workflow does not invoke `nexus version` or inspect every packaged binary's Go metadata.
3. Real IBM i validation remains an external rollout gate.

These warnings do not block this archive and do not change the product status. A later documentation-only change may address the stale design narrative and packaging-observability gap; it must not represent automated evidence as live IBM i validation.

## Authoritative Git-Blob Identity

There is no verification-report hash discrepancy. The following authoritative committed Git-blob commands both produce canonical SHA-256 `6f6f7ec41cca454da5232c5e7329538c8e33b93ae375949fa9a0cd6dcd647075`:

```text
git show 909d5e1e7296a9dc44eb0acde57e18ef0a1f02d3:openspec/changes/v1-mcp-foundation/verify-report.md | sha256sum
6f6f7ec41cca454da5232c5e7329538c8e33b93ae375949fa9a0cd6dcd647075 *-

git show 27293e8d188bd82474b73dc586a4e977499db3b0:openspec/changes/archive/2026-08-21-v1-mcp-foundation/verify-report.md | sha256sum
6f6f7ec41cca454da5232c5e7329538c8e33b93ae375949fa9a0cd6dcd647075 *-
```

The shared committed blob is `91f3b0506e1ba6af57f84437fc7df368fd8a093f`. A local `Get-FileHash` result over the Windows checked-out CRLF representation is non-authoritative for committed archive identity. The archived report remains an intermediate verification snapshot: its PR/open-head and warning wording describe its verification-time state, not the post-merge closure recorded above.

## Engram Traceability

The following Engram observations were read during archive:

- #2035 — `sdd/v1-mcp-foundation/proposal`
- #2036 — `sdd/v1-mcp-foundation/spec`
- #2038 — `sdd/v1-mcp-foundation/design`
- #2040 — `sdd/v1-mcp-foundation/tasks`
- #2208 — `sdd/v1-mcp-foundation/verify-report`
- #2228 — independent final-verification discovery

The terminal hybrid archive report is persisted under `sdd/v1-mcp-foundation/archive-report`.
