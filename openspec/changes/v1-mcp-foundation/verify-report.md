```yaml
schema: gentle-ai.verify-result/v1
evidence_revision: sha256:7377ee9ebe6b9d08488ff16f7863c125d545bf8f6ccf7a70293725efdb567313
verdict: fail
blockers: 8
critical_findings: 8
requirements: 8/13
scenarios: 34/43
test_command: go test -count=1 ./...
test_exit_code: 0
test_output_hash: sha256:deb92116a8761bf51e346f6540160e9aa94737e96c642c5f1ab1c559af7d3432
build_command: go vet ./...; six-target CGO_ENABLED=0 go build plus manifest verification and artifact upload
build_exit_code: 0
build_output_hash: sha256:63b4c402bcf54cd1466e5452e04a9c9392e6fbec48d8818525d91dab75166b44
```

## Verification Report

**Change**: `v1-mcp-foundation`  
**Version**: `v0.0.0-ci.295e69773c9cd49c0e76c66ab11468903ec3aaa4`  
**Mode**: Strict TDD  
**Candidate**: `295e69773c9cd49c0e76c66ab11468903ec3aaa4` (`main`, clean, equal to `origin/main`)  
**IBM i status**: `ready_for_controlled_ibmi_validation`; `not_validated_on_ibmi`

Live IBM i validation is an external manual rollout gate and is not required for SDD completion. No automated test, fake, package check, or GitHub-hosted runner result in this report is represented as proof of live IBM i behavior. No product IBM i composition was added in this scope.

### Completeness

| Metric | Value |
|---|---:|
| Canonical parent tasks | 42 |
| Tasks checked | 42 |
| Tasks unchecked | 0 |
| Requirements | 13 |
| Runtime-covered compliant scenarios | 34/43 |
| Deferred external IBM i scenario | 1/43, non-blocking and not claimed compliant |

Task checkboxes are complete, but the implementation and current strict-TDD evidence do not support final acceptance.

### Exact Candidate and Runtime Evidence

| Evidence | Result |
|---|---|
| GitHub Actions run | `32517435167`, `Go Verification`, push, success |
| Candidate binding | Run head and checked-out revision both `295e69773c9cd49c0e76c66ab11468903ec3aaa4` |
| URL | https://github.com/ddgutierrezc/bac-nexus/actions/runs/32517435167 |
| Test step | `go test -count=1 ./...`, exit 0; all listed packages passed; no skipped test was reported |
| Static analysis | `go vet ./...`, exit 0 |
| Package step | Six `CGO_ENABLED=0` targets: linux/darwin/windows × amd64/arm64; exit 0 |
| Artifact | ID `9459337358`, 12 files, 13,788,510 bytes, archive digest `sha256:734e637f3c0b3deb343e8cb1efaf6dd77af193cbcdd7741f36f9bbc7b2b73672`, unexpired |
| Full authoritative log | 34,221 bytes; `sha256:7377ee9ebe6b9d08488ff16f7863c125d545bf8f6ccf7a70293725efdb567313` |

WDAC policy was not bypassed. No local Go test binary was executed. Local `gofmt` was used only as a static formatter check against tracked Git blobs.

### Build, Package, and Formatting

| Check | Result | Evidence |
|---|---|---|
| Repository tests | ✅ Passed | Exact-head GHA run `32517435167` |
| `go vet ./...` | ✅ Passed | Exact-head GHA run `32517435167` |
| Six-target binaries | ✅ Passed | Linux, Darwin, Windows; amd64 and arm64 |
| Manifest schema/status/path/checksum/byte length | ✅ Passed | Workflow generated and compared the exact nine-field JSON object for every target |
| Tamper rejection | ✅ Passed | `internal/release.TestManifestRejectsTamperingMismatchAndUnsafePath` in the exact-head suite |
| Artifact upload | ✅ Passed | Artifact `9459337358`, 12 files |
| Formatting | ❌ Failed | The workflow has no formatting step; formatting the tracked Git blob identifies `internal/app/service_test.go` as nonconforming |
| Coverage | ➖ Not available | No coverage command/report is configured in the authoritative workflow |

### Requirement and Scenario Compliance

| Requirement | Scenario coverage | Result | Runtime/static evidence |
|---|---:|---|---|
| Local-Principal Authorization | 2/2 | ✅ COMPLIANT | `internal/security/policy_test.go`, `internal/app/service_test.go` |
| Native Secret Isolation | 4/4 | ✅ COMPLIANT | Credential package tests plus content-bound Windows/macOS/Linux keyring-gate evidence recorded in apply progress |
| Explicit Native Credential Migration | 2/2 | ✅ COMPLIANT | `TestKeyringCredentialStoreMigrationDeletesVaultOnlyAfterExactReadback`, retention-on-uncertainty test |
| Pinned Host Trust Policy | 1/2 | ❌ PARTIAL | Exact-pin/change tests pass, but no explicit enrollment/provenance recording implementation exists |
| Sanitized Read-Only Surface and Audit | 1/3 | ❌ FAILING | Path-control structural test passes; arbitrary SQL and integrated audit defects remain |
| Operator-Ready Field-Validation Package | 0/3 | ❌ UNTESTED | Static runbook inspection is complete, but no runtime workflow check covers headings, sanitization, or rollback contract |
| Bounded Catalog Resolution | 1/2 | ❌ FAILING | Bound tests pass; zero results return empty success instead of deterministic `not_found` |
| Exact Source Page Contract | 1/2 | ❌ FAILING | Snapshot newline/EOF tests pass; first-page acquisition/cursor contract is absent |
| Immutable Snapshot Lease and Freshness | 2/2 | ✅ COMPLIANT | Lease replay/binding/restart and app freshness tests |
| Bounded Lease Lifecycle | 3/3 | ✅ COMPLIANT | Acquisition, quota, expiry, cleanup, and immutable-source tests |
| Durable Temporary Ownership and Recovery | 15/15 | ✅ COMPLIANT | SQLite integrity/admission/recovery tests and source recovery/coordinator tests |
| Deterministic Page Boundaries and Validation | 1/1 | ✅ COMPLIANT | Snapshot invalid-range/size/no-partial-content tests |
| Prefix Compatibility, SDD Completion, Deferred IBM i Validation | 1/2 SDD-runtime; 1 external | ✅ SDD scope / external pending | Catalogspike suite passes and manifest non-claim test passes; live IBM i scenario remains explicitly external |

**Scenario accounting**: 34 scenarios have passing runtime coverage, 8 required SDD scenarios are failing/untested, and 1 live IBM i scenario is deliberately deferred and non-blocking. The deferred scenario is not counted as automated compliance and is not an SDD failure by itself.

### Critical Findings

1. **CRITICAL — Arbitrary SQL is exposed through the MCP schema.** `internal/mcp/server.go:49-51` accepts caller-controlled `Statement` and `Parameters`; `resolveCatalog` forwards them unchanged to `app.Service`, which forwards the query to `CatalogResolver`. The handler test explicitly uses `SELECT 1`. This violates the prohibition on arbitrary SQL and the narrow catalog-capability contract.
2. **CRITICAL — A source traversal cannot start through the specified MCP contract.** `ReadSelectedSourceInput` requires only a cursor/range and has no exact selection; `ReadSelectedSourceOutput` omits a cursor. `app.Service.ReadSelectedSource` only looks up a pre-existing cursor. `SnapshotAcquirer.Acquire` and `LeaseStore.Acquire` are never reached by the MCP/app first-page path. Tests mint cursors directly in fixtures, bypassing the missing production behavior.
3. **CRITICAL — Zero catalog results do not return deterministic `not_found`.** `app.Service.ResolveCatalog` applies only `BoundedCandidates`; an empty resolver result returns an empty successful slice. This contradicts the bounded-resolution requirement and its absent-query scenario.
4. **CRITICAL — Explicit pinned-TOFU enrollment is not implemented.** `PinnedTrust.Verify` validates a caller-supplied pin but records neither a key nor provenance. Tests prove verification and deliberate non-enrollment, not the required explicit enrollment operation.
5. **CRITICAL — Integrated audit behavior does not satisfy the contract.** App tests use `fakeAuditor`, bypassing `audit.ValidateEvent`. Production app code uses the profile as `PolicyID`, while `ValidateEvent` does not validate `PolicyID`; `recordDenied` passes raw classified errors and ignores recorder failures. For example, `credentials_unavailable` contains the forbidden substring `credential`, so the real recorder rejects it and the denial is silently absent.
6. **CRITICAL — Operator-package scenarios have no runtime covering test.** The workflow does not validate required runbook sections, prohibited evidence fields, abort/rollback language, or external-retention boundaries. Static inspection finds the runbook content complete, but strict verification cannot mark scenarios compliant without a passing runtime check.
7. **CRITICAL — The checked formatting task is false on the final candidate.** `.github/workflows/go-verification.yml` does not run formatting, and a tracked-blob formatter comparison reports `internal/app/service_test.go` as nonconforming. Tasks nevertheless claim GHA formatting completed.
8. **CRITICAL — Current cumulative strict-TDD evidence is incomplete.** The apply-progress TDD table begins at task 2.11; tasks 1.1–2.10 have no current per-task RED/GREEN/safety-net evidence. The task-4.4 table also omits the required Safety Net and Triangulate columns. Therefore 42/42 checkbox completion cannot be independently reconciled with complete strict-TDD evidence.

### Warnings

1. **WARNING — Design artifact is stale.** `design.md` still says the workflow currently runs only tests/vet and repeatedly describes task 4.4 as future work, although the final candidate packages artifacts.
2. **WARNING — Package identity verification deviates from design.** The workflow embeds version/revision and verifies manifest values but does not execute `nexus version` or inspect Go build metadata as the design requires. Unit tests cover the formatter-level identity contract, not each packaged binary's reported identity.
3. **WARNING — Security documentation overstates the MCP output boundary.** `docs/SECURITY.md` says typed outputs never include source, while `read_selected_source` intentionally returns `Page.Lines` containing source text. The correct restriction applies to audit/logging, not the requested MCP source result.

### Suggestions

1. **SUGGESTION — Add end-to-end package-contract tests inside GHA.** Verify runbook markers/prohibited terms, execute the native Linux/amd64 `nexus version --json`, inspect build metadata, and perform a deliberate binary-tamper rejection check.

### TDD Compliance

| Check | Result | Details |
|---|---|---|
| TDD evidence reported | ⚠️ Partial | Extensive evidence exists from task 2.11 onward; tasks 1.1–2.10 are absent from the current cumulative table |
| All tasks have tests | ❌ | Product tests exist broadly, but first-page traversal, explicit enrollment, real integrated audit, and runbook contracts lack covering tests |
| RED confirmed | ⚠️ Partial | Recorded CI RED runs exist for later slices; current artifact is not complete for all 42 parent tasks |
| GREEN confirmed | ❌ | Exact-head suite is green, but required scenarios remain bypassed or absent |
| Triangulation adequate | ⚠️ Partial | Bounds/recovery/security cases are strong; task 4.4 omits triangulation evidence and key MCP gaps are fixture-bypassed |
| Safety net for modified files | ⚠️ Partial | Later slices record baselines; task 4.4 omits the required Safety Net field |

**TDD compliance**: FAIL — the exact-head suite passes, but strict-TDD provenance and scenario coverage are incomplete.

### Test Layer Distribution

| Layer | Tests | Files | Authority |
|---|---:|---:|---|
| Go package unit/component tests | 209 top-level test functions | 30 | Exact-head GHA `go test -count=1 ./...` |
| Native keyring integration | Platform scenarios recorded in apply progress | 4 credential test files plus keyring workflow | Content-bound prior Windows/macOS/Linux GHA evidence |
| Live IBM i E2E | 0 | 0 | Deliberately external; not claimed |

### Changed File Coverage

Coverage analysis skipped — no authoritative coverage command/report is configured.

### Assertion Quality

No tautology, assertion-without-production-call, or ghost-loop pattern was found in the reviewed change tests. However, `internal/app/service_test.go` uses a permissive `fakeAuditor`, so its audit assertions do not exercise production validation; that gap is included as a CRITICAL integrated-audit finding.

### Quality Metrics

**Static analysis**: ✅ `go vet ./...` passed in exact-head GHA.  
**Formatting**: ❌ tracked candidate is not fully gofmt-conforming and GHA has no formatting gate.  
**Coverage**: ➖ Not available.  
**Secret/surface review**: No credential, private key, generic remote tool, arbitrary shell, generic SSH tool, remote listing/deletion tool, or unbounded traversal was found. Synthetic test values exist but no real secret was identified. Arbitrary SQL remains exposed as finding 1.  
**Catalogspike preservation**: ✅ `bac-nexus/cmd/catalogspike` passed in exact-head GHA; no task-4.4 change modified that command.

### Design Coherence

| Decision | Result | Notes |
|---|---|---|
| No product IBM i composition in this scope | ✅ Followed | Resolver/acquirer/recovery/lease production dependencies remain unwired; this report does not treat live IBM i validation as required |
| External sanitized operator evidence | ✅ Followed statically | Blank template committed; completed evidence remains external |
| Six-target versioned package | ✅ Followed | Exact-head GHA built and uploaded six binaries plus six manifests |
| Binary/build/manifest identity comparison | ⚠️ Partial | Manifest/checksum verified; packaged `nexus version` and Go metadata comparison omitted |
| No live IBM i claim | ✅ Followed | Status is explicitly `not_validated_on_ibmi` |

### Verdict

**FAIL**

The exact-head GitHub Actions suite and six-target packaging job pass, and the operator documentation correctly preserves the IBM i non-validation boundary. Final SDD acceptance is nevertheless blocked by substantive MCP/security/audit contract failures, missing runtime coverage, a false formatting completion claim, and incomplete strict-TDD evidence. Live IBM i validation is not one of the blockers.

## Bounded Native Remediation Attempt

This section preserves the original failed report above and records the single authorized correction attempt. It does not claim live IBM i validation.

| Finding | Triage and correction |
|---|---|
| Arbitrary SQL | Confirmed candidate defect. MCP input now accepts item/library fields and constructs the bounded catalog query internally. |
| First-page acquisition | Confirmed candidate defect. The source contract now accepts an exact selection without a cursor; the app resolves, acquires, leases, and reads page one. |
| Absent catalog result | Confirmed candidate defect. Empty bounded results now return `catalog.ErrCandidateNotFound` and emit a deny event. |
| TOFU enrollment | Confirmed candidate defect. `PinnedTrust.Enroll` is an explicit operation that returns non-secret provenance; `Verify` never enrolls. |
| Required audit | Confirmed candidate defect. App operations now propagate required auditor failures and use a fixed sanitized denial classification. |
| Runbook runtime checks | Confirmed candidate gap. GHA now asserts required headings, prohibited evidence markers, and exact-path rollback wording. |
| Formatting | Confirmed candidate gap, not yet proven resolved. GHA now has a formatting gate; exact-head GHA must establish its result. |
| Cumulative strict-TDD evidence | Confirmed artifact gap. Historical evidence remains preserved; this correction adds focused candidate coverage, but complete 42-task provenance requires independent review. |

### Correction Evidence

- Native work unit: `final-verification-v1-mcp-foundation`.
- Native token: `sha256:9e2d9b57381903e9a6ce1966d7d3956bdc4d4aa715d6249eb65ef96471ac2ed4`.
- Failed evidence being remediated: `sha256:7377ee9ebe6b9d08488ff16f7863c125d545bf8f6ccf7a70293725efdb567313`.
- Local compile-only checks passed for `./internal/app`, `./internal/mcp`, and `./cmd/nexus`; no local Go test binary was executed because WDAC remains authoritative.
- Exact-head GitHub Actions tests, vet, formatting, package, and runbook checks are still required before settlement.
- IBM i remains `ready_for_controlled_ibmi_validation` / `not_validated_on_ibmi`.

### Exact-head GHA result for this attempt

- Run `32519777214` reached the correction head `ec49d9b3b1acda6a0b935f9c2748a2a525ff818d`; tests and vet passed on reruns, but the formatting gate failed on `internal/app/service.go`.
- The same run also recorded the pre-existing flaky SQLite contention failure on an earlier rerun (`TestLedgerAdmissionUsesExactRetrySchedule`); it is not attributed to this correction.
- No passing remediation evidence revision exists. This correction therefore cannot settle `passed`.

## Final correction in progress

The maintainer-authorized final correction is limited to canonical formatting of
`internal/app/service.go` and cumulative Strict-TDD provenance. The provenance
reconciliation is recorded in `apply-progress.md`; it preserves the original
report and does not claim local Go runtime execution or live IBM i validation.
The exact failed evidence revision remains
`sha256:351160a11f7aa97c26d4f33d3be8db66ecc2273c05a45570cdc3d853e37f3abf`.

## Independent verification of the eight original blockers

**Candidate:** `d1238cf` / exact workflow head `d1238cf`  
**GHA:** `32520734406` — success  
**IBM i:** `ready_for_controlled_ibmi_validation`; `not_validated_on_ibmi`

| Original blocker | Independent result | Evidence |
|---|---|---|
| Arbitrary SQL in MCP schema | Resolved | Typed item/library input and bounded internal query; no statement/parameters surface remains. |
| Missing first-page acquisition | Resolved | Exact selection path resolves, acquires, leases, and reads page one before cursor-only continuation. |
| Empty catalog result | Resolved | Empty bounded result returns deterministic `catalog.ErrCandidateNotFound` and deny audit. |
| Missing explicit TOFU enrollment | Resolved | `PinnedTrust.Enroll` is explicit and returns sanitized provenance; `Verify` does not enroll. |
| Integrated audit contract | Resolved | Production app propagates recorder failures and records fixed sanitized denial classifications. |
| Operator-package runtime checks | Resolved | GHA validates required runbook headings, prohibited evidence markers, and exact-path rollback wording. |
| Formatting | Resolved | GHA formatting step passes across all tracked Go files. |
| Cumulative Strict-TDD provenance | Resolved as documentation reconciliation | `apply-progress.md` now maps tasks 1.1–4.4 to retained RED/GREEN/REFACTOR commits and exact GHA IDs, without inventing local runtime or tests-first claims. |

### Independent result

All eight original blockers are independently closed: **0 blockers**. The
authoritative GHA run passed repository tests, vet, formatting, six-target
packaging, manifest verification, artifact upload, and operator runbook
checks. The sole remaining non-automated scenario is the explicitly external
live IBM i rollout gate; it is not claimed or treated as an SDD blocker.
