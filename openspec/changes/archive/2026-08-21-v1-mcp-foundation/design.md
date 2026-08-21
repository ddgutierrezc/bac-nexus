# Design: v1 MCP Foundation

## Technical Approach

Close SDD after implementation, automated internal-contract verification, and binary handoff assembly. Status is `ready_for_controlled_ibmi_validation` and `not_validated_on_ibmi`. Live IBM i validation is a later external manual gate; fakes, loopback, CI, and package checks are not equivalent evidence.

No product IBM i composition is added. `cmd/nexus` keeps production resolver, acquirer, recovery, and lease dependencies unwired.

## Architecture Decisions

| Option | Tradeoff | Decision |
|---|---|---|
| Several documents / one document | Separate files drift | Create `docs/IBM_I_VALIDATION.md` with prerequisites, runbook, checklist, blank evidence template, abort/rollback, and statuses. |
| Evidence in repository / external evidence | Repository retention increases disclosure risk | Commit only the blank template. Operators record completed evidence in an approved external system; completed evidence and attachments never enter the repository or release bundle. |
| Filename / embedded identity plus checksum | Filenames are mutable | Embed release version and VCS revision, expose them through `nexus version`, and SHA-256 hash final bytes. |
| Automated live gate / deferred manual gate | Automation is unavailable and would overclaim | Automated acceptance verifies internal contracts and package structure only. An authorized operator later performs the live checklist. |

## Package and Data Flow

    repository + approved revision
      -> build/v1-mcp-foundation/<version>/<goos>-<goarch>/nexus[.exe]
      -> build/v1-mcp-foundation/<version>/<goos>-<goarch>/nexus.manifest.json
      -> binary handoff with docs/IBM_I_VALIDATION.md
      -> authorized operator performs external gate
      -> sanitized evidence stored outside repository

For Linux and macOS, the handed-off binary is `build/v1-mcp-foundation/<version>/<goos>-<goarch>/nexus`; Windows uses the same path with `nexus.exe`. Its sidecar is always `build/v1-mcp-foundation/<version>/<goos>-<goarch>/nexus.manifest.json`. These generated paths are package outputs, not committed artifacts.

Current state: `.github/workflows/go-verification.yml` runs Go tests and static analysis only; it does not generate, verify, or publish a package.

Future task 4.4 WILL EXTEND `.github/workflows/go-verification.yml` as the single repository-owned generation and automated-verification recipe. The extension will build the exact target, embed release version and VCS revision, write the sidecar, recompute SHA-256, verify manifest identity against `nexus version` and Go build metadata, and publish the directory as the handoff artifact. The manifest contains only schema version, release version, VCS revision, target OS/architecture, byte length, binary SHA-256, and both statuses. Binary checksum is allowed; sensitive-content hashes are forbidden. Generation needs no IBM i. `docs/IBM_I_VALIDATION.md` is the operator-owned verification recipe: after transfer, the operator recomputes SHA-256 and requires exact identity equality without storing command output. Mismatch, missing field, dirty/unapproved revision, or version mismatch aborts handoff.

External evidence permits only checklist completion, bounded requested/returned counts, outcome/newline/lifecycle classifications, EOF reached, binary version/checksum, and window classification. It excludes source, paths, coordinates, credentials, host/user IDs, raw errors, commands, SQL, cursors, sensitive-content hashes, and remote-cleanup details.

## Manual Validation Contract

Prerequisites are an authorized operator/identity, approved reachable/supported IBM i, libraries/policies/window, endpoint controls, exact binary/configuration, and no bypass. Using the approved MCP client, the operator:

1. traverse from line 1 until EOF while recording only bounded counts and outcome classes;
2. classify empty, LF-terminated, and final-unterminated behavior without recording text;
3. finish one traversal and record only whether cleanup was confirmed;
4. cancel one traversal and record only cancellation and cleanup classifications;
5. stop/invalidate leases and confirm no source appears in retained logs, evidence, attachments, or package artifacts.

Any failed prerequisite, mismatch, unexpected result, unconfirmed cleanup, or prohibited-data exposure aborts validation. Stop Nexus, invalidate leases, restore approved binary/configuration, revoke affected credentials, and request approved cleanup of only exact recorded owned paths. Never broaden cleanup, delete ledger rows for capacity, or retain cleanup details.

## File Changes for Future Tasks

| File | Action | Description |
|---|---|---|
| `docs/IBM_I_VALIDATION.md` | Create | Single runbook, checklist, blank evidence template, prerequisites, abort, and rollback contract. |
| `cmd/nexus/main.go`, `cmd/nexus/main_test.go` | Modify | Embed/report package identity only; do not add IBM i composition. |
| `.github/workflows/go-verification.yml` | Modify | Task 4.4 will extend the current test-and-vet workflow to generate and verify the exact `build/v1-mcp-foundation/<version>/<goos>-<goarch>/nexus[.exe]` and `nexus.manifest.json` pair, then publish that directory as the binary handoff. |
| `README.md`, `docs/SECURITY.md` | Modify | State the corrected SDD status and external live gate; remove wording that implies v1 is live-wired or live-validated. |
| `openspec/changes/v1-mcp-foundation/tasks.md` | Modify later | Replace task 4.4 live acceptance with automated package/identity/doc acceptance; retain live validation as an external rollout gate, not a task checkbox. |

## Testing Strategy

Unit tests verify deterministic `nexus version` output and rejection of unset identity. Under future task 4.4, the extended `.github/workflows/go-verification.yml` will validate required runbook headings/statuses and manifest fields, recompute the packaged binary SHA-256, compare binary/build/manifest identity, and scan the blank template/package metadata for prohibited fields. Existing tests remain internal-contract evidence only. No automated test claims IBM i behavior.

## Threat Matrix

| Boundary | Applicability | Design response / RED test |
|---|---|---|
| Documentation-like paths | N/A: documentation is never classified or executed | None |
| Git repository selection | N/A: no VCS automation is added | None |
| Commit state | N/A: no commit automation is added | None |
| Push state | N/A: no push automation is added | None |
| PR commands | N/A: no PR automation is added | None |

## Migration / Rollout

No migration is required. Withhold the package until automated acceptance passes. Live IBM i validation and rollout authorization remain external. The prior contradiction is resolved by separating SDD/package completion from field validation; future tasks edit exactly the files listed above.

## Open Questions

None.
