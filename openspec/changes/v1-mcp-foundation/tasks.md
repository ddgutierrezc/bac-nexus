# Tasks: v1 MCP Foundation

## Review Workload Forecast

| Field | Value |
|---|---|
| Estimated changed lines | 650–850 authored; one final PR slice |
| 1000-line budget risk | Medium |
| Chained PRs recommended | No |
| Suggested split | One coherent PR to `main` |
| Delivery strategy | automatic |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: stacked-to-main
400-line budget risk: Medium
1000-line budget risk: Medium

### Suggested Work Units

| Unit | Goal | Focused test command | Runtime harness | Rollback boundary |
|---|---|---|---|---|
| 1 | Finish 4.4 handoff package and manifest verification | GHA `go test -count=1 ./...`; manifest assertions | GHA build/package job; live IBM i is N/A and external | Revert docs, identity tests, workflow, and manifest packaging only |

## Completed State (preserve)

- [x] 1.1–1.6 Source foundation; [x] 2.1–2.4 remote safety/dependency gate; [x] 2.5–2.7 ledger foundation; [x] 2.8–2.10 filesystem policy.
- [x] 2.11–2.13 transaction retry/readback; [x] 2.13a–2.13c integrity; [x] 2.14–2.16 acquisition; [x] 2.17–2.19 recovery/documentation.
- [x] 3.1 dependency gate; [x] 3.2–3.4 credentials; [x] 3.5–3.7 policy/audit; [x] 3.8–3.10 application service.
- [x] 4.1–4.3 MCP server, wiring, and refactor/documentation.

Current progress: **42/42**. Canonical task 4.4 is complete; live IBM i validation remains an external rollout gate.

## Phase 4: MCP and Acceptance

- [x] 4.4 **Acceptance correction (final PR, strict TDD):**
  - **RED:** Add focused tests in `cmd/nexus/main_test.go` (or a package-local manifest test) for schema fields, checksum/byte length, embedded version/VCS identity, exact `build/v1-mcp-foundation/<version>/<goos>-<goarch>/nexus[.exe]` semantics, sidecar naming, and both status strings; assert no IBM i claim.
  - **GREEN:** Modify `cmd/nexus/main.go` and `.github/workflows/go-verification.yml` to build the platform-specific binary, generate/recompute/verify deterministic `nexus.manifest.json`, and publish the handoff directory at the design paths. Do not add IBM i composition.
  - **GREEN:** Create `docs/IBM_I_VALIDATION.md` with prerequisites, manual line-1→EOF/newline/success-cleanup/cancellation-cleanup/no-retention checklist, sanitized evidence template, abort/rollback, and explicit `ready_for_controlled_ibmi_validation` / `not_validated_on_ibmi` language. Completed evidence remains external and never stores source or sensitive data.
  - **REFACTOR/verify:** Correct contradictory status wording in `README.md` and `docs/SECURITY.md`; run full repository verification through GHA (`go test -count=1 ./...`, `go vet ./...`, formatting, manifest/package checks, and supported build matrix). Successful completion produces **42/42** and enables `sdd-verify`; live IBM i remains an external rollout prerequisite, outside SDD.

Rollback: revert only the task-4.4 documentation, identity/manifest tests and implementation, workflow packaging, and status wording; retain completed 1.1–4.3 behavior.
