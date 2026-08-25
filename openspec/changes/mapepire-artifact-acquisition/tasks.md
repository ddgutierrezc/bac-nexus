# Tasks: Mapepire Artifact Acquisition

## Review Workload Forecast

| Field | Value |
|---|---|
| Estimated changed lines | 1,350–1,550 authored lines |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR1 policy/contract → PR2 cache → PR3 providers → PR4 migration/audit → PR5 remote → PR6 Step 4/docs/verification |
| Delivery strategy | auto-chain |
| Chain strategy | feature-branch-chain |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: feature-branch-chain
400-line budget risk: High

The 1,500-line approval guard is respected, but each implementation slice remains near the 400-line review standard. No threat-matrix rows require RED tests; every listed row is explicitly N/A.

Feature-branch order: PR #1 base = feature/tracker branch; PR #2 base = PR #1 branch; PR #3 base = PR #2 branch; PR #4 base = PR #3 branch; PR #5 base = PR #4 branch; PR #6 base = PR #5 branch. No branches or PRs are created by this plan.

### Suggested Work Units

| Unit | Goal / base | Focused test command | Runtime harness | Rollback boundary |
|---|---|---|---|---|
| 1 | Policy, handle, errors; base tracker | `go test ./internal/mapepireartifact` | N/A; no-network fakes | New package contract files/tests |
| 2 | Private verified cache; base PR #1 | `go test ./internal/mapepireartifact -run Cache` | OS temp dirs + lock seams; no IBM i | Cache/lock implementation |
| 3 | Providers and deployment gate; base PR #2 | `go test ./internal/mapepireartifact -run Resolver` | Reader/provider fakes; no network | Provider adapters/policy gate |
| 4 | Profile migration and audit; base PR #3 | `go test ./internal/profile ./internal/configuration ./internal/audit` | Temp profile store/counting fakes | Schema/readiness/audit changes |
| 5 | Remote verified-handle adapter; base PR #4 | `go test ./internal/connectors/ibmi/mapepirestdio ./internal/remote` | `memoryRemote`; no live SSH | Handle-consuming remote path |
| 6 | Step 4 orchestration/docs; base PR #5 | `go test ./internal/tui ./internal/configuration` | Counting IBM i/SSH/credential/upload/launch fakes | Step 4 wiring/docs |

## Phase 1: Contracts and Cache

- [ ] 1.1 RED tests in `internal/mapepireartifact/*_test.go` for the exact 2.3.5 descriptor, stable handle, typed/sanitized outcomes, and immutable policy; then add policy/verifier contracts without changing Mapepire version.
- [ ] 1.2 RED cache tests for bounds, JAR sanity, digest, partial interruption, corruption, coexistence, reverify-on-open, atomic publish, Windows/Unix lock seams, and cross-process convergence; then implement private OS cache in `internal/mapepireartifact/cache*.go`, `lock*.go`.

## Phase 2: Resolution, Compatibility, Security

- [ ] 2.1 RED resolver tests for deterministic cache→gated pinned upstream→optional Code for IBM i→explicit manual order, availability fallback, and terminal security rejection; then implement adapters/resolver with no latest/arbitrary URL.
- [ ] 2.2 RED migration tests for legacy absolute `MapepireJAR`, missing/link/replaced/oversized/wrong-digest paths, old/new schema, and no persisted cache path; then update `internal/profile/` and `internal/configuration/` readiness.
- [ ] 2.3 RED audit tests for source kind, policy/version/digest/size/outcome and URL/secret/path/raw-error/content redaction; then extend `internal/audit/` with allowlisted metadata. Add explicit licensing/compliance/security deployment-gate tests before enabling upstream release configuration.

## Phase 3: Remote Boundary and Step 4

- [ ] 3.1 RED adapter tests for digest-scoped Nexus-owned remote paths, reverify-before-use, consent separation, rollback/concurrency, and rejected-handle blocking; then adapt `internal/connectors/ibmi/mapepirestdio/` and `internal/remote/` without coupling acquisition to upload/launch.
- [ ] 3.2 RED orchestration tests proving Step 4 performs zero IBM i/SSH/credential/Java/upload/launch activity and permits explicit not-ready completion; then wire canonical states in `internal/tui/` and configuration without prescribing layout.

## Phase 4: Documentation and Verification

- [ ] 4.1 Update the wizard source of truth plus behavior-changing `docs/ARCHITECTURE.md` and `docs/SECURITY.md`; do not edit base specs until archive.
- [ ] 4.2 Run every delta scenario, existing CLI/process compatibility tests, `go test -count=1 ./...`, `go vet ./...`, and formatting; record no-network evidence and confirm no downloads, IBM i contact, or real credentials.
