# Apply Progress: Enable Production Nexus Serve

## Focused Remediation — MCP Shutdown Liveness and Stderr Sanitization

This one maintainer-authorized remediation corrects only the two CRITICAL findings from failed evidence revision `sha256:379fc91d921f46baca09442d584e98e745b4b982eb4f080057eb331d5606704d`. All 19/19 tasks remain checked; none were reopened or expanded.

`internal/mcp.Server.Run` now binds the SDK session to the serve context. On cancellation it stops intake, cancels accepted request contexts through that parent context, closes the MCP session before waiting for handlers, and returns the deterministic context result. Transport/session lifecycle errors map to `mcp lifecycle unavailable`. The command composition boundary maps non-context runner failures to `serve mcp unavailable`, so `main` cannot print raw SDK, transport, runner, peer, path, or secret-bearing error detail to stderr. Stdout remains owned exclusively by MCP protocol transport.

### TDD Cycle Evidence

| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|---|
| Focused MCP lifecycle remediation | `internal/mcp/server_test.go`, `cmd/nexus/main_test.go` | In-process official MCP transport and composition integration | `go test -count=1 ./internal/mcp ./cmd/nexus`: exit 0 before edits | `go test -count=1 ./internal/mcp -run '^(TestServerShutdownCancelsAcceptedHandlerBeforeWaiting|TestServerSanitizesTransportLifecycleErrors)$'`: exit 1; `ErrLifecycleUnavailable` was undefined. `go test -count=1 ./cmd/nexus -run '^TestRunWithDepsSanitizesRunnerLifecycleErrors$'`: exit 1; `errServeMCPUnavailable` was undefined. | `go test -count=1 ./internal/mcp ./cmd/nexus -run '^(TestServerShutdownCancelsAcceptedHandlerBeforeWaiting|TestServerSanitizesTransportLifecycleErrors|TestRunWithDepsSanitizesRunnerLifecycleErrors)$'`: exit 0. | Added `TestRunWithDepsPreservesContextLifecycleErrors`; the focused four-test command exited 0, proving raw lifecycle detail is classified while context deadline remains deterministic. | `gofmt -w internal/mcp/server.go internal/mcp/server_test.go cmd/nexus/main.go cmd/nexus/main_test.go`; focused four-test command exited 0. |

### Work Unit Evidence

| Evidence | Exact result |
|---|---|
| Focused test command | `go test -count=1 ./internal/mcp ./cmd/nexus -run '^(TestServerShutdownCancelsAcceptedHandlerBeforeWaiting|TestServerSanitizesTransportLifecycleErrors|TestRunWithDepsSanitizesRunnerLifecycleErrors|TestRunWithDepsPreservesContextLifecycleErrors)$'`: exit 0; 2 MCP and 2 composition tests passed. |
| Runtime harness command/scenario | `go test -count=1 ./cmd/nexus -run '^TestNexusStdioSubprocessProducesOnlyJSONRPCOnStdout$'`: exit 0. The existing bounded local helper subprocess exercised stdio framing; its 3-second timeout kill-and-Wait branch did not fire. No network, SSH, IBM i, external service, or `TestControlledNexusServe` execution occurred. |
| Focused race | `go test -race -count=1 ./internal/mcp ./cmd/nexus -run '^(TestServerShutdownCancelsAcceptedHandlerBeforeWaiting|TestServerSanitizesTransportLifecycleErrors|TestRunWithDepsSanitizesRunnerLifecycleErrors|TestRunWithDepsPreservesContextLifecycleErrors)$'`: exit 0; no race report. |
| Full verification | `go test -count=1 ./...`, `go vet ./...`, `go build -o /tmp/opencode/nexus-remediation ./cmd/nexus`, and `git diff --check`: all exit 0. The helper binary was removed immediately; `test ! -e /tmp/opencode/nexus-remediation` exited 0. |
| Rollback boundary | Revert only shutdown/session lifecycle handling and tests in `internal/mcp/server.go` and `internal/mcp/server_test.go`, runner-error classification and test in `cmd/nexus/main.go` and `cmd/nexus/main_test.go`, and this remediation record. |

<!-- BEGIN MCP LIFECYCLE REMEDIATION EVIDENCE PREIMAGE v1 -->
```text
change=enable-production-nexus-serve
failed_evidence_revision=sha256:379fc91d921f46baca09442d584e98e745b4b982eb4f080057eb331d5606704d
scope=mcp-shutdown-liveness-and-stderr-sanitization-only
red_mcp=go test -count=1 ./internal/mcp -run '^(TestServerShutdownCancelsAcceptedHandlerBeforeWaiting|TestServerSanitizesTransportLifecycleErrors)$': exit 1 (ErrLifecycleUnavailable undefined)
red_command=go test -count=1 ./cmd/nexus -run '^TestRunWithDepsSanitizesRunnerLifecycleErrors$': exit 1 (errServeMCPUnavailable undefined)
focused=go test -count=1 ./internal/mcp ./cmd/nexus -run '^(TestServerShutdownCancelsAcceptedHandlerBeforeWaiting|TestServerSanitizesTransportLifecycleErrors|TestRunWithDepsSanitizesRunnerLifecycleErrors|TestRunWithDepsPreservesContextLifecycleErrors)$': exit 0
race=go test -race -count=1 ./internal/mcp ./cmd/nexus -run '^(TestServerShutdownCancelsAcceptedHandlerBeforeWaiting|TestServerSanitizesTransportLifecycleErrors|TestRunWithDepsSanitizesRunnerLifecycleErrors|TestRunWithDepsPreservesContextLifecycleErrors)$': exit 0
runtime=go test -count=1 ./cmd/nexus -run '^TestNexusStdioSubprocessProducesOnlyJSONRPCOnStdout$': exit 0
full=go test -count=1 ./...: exit 0
vet=go vet ./...: exit 0
build=go build -o /tmp/opencode/nexus-remediation ./cmd/nexus: exit 0; helper removed; test ! -e /tmp/opencode/nexus-remediation: exit 0
diff=git diff --check: exit 0
```
<!-- END MCP LIFECYCLE REMEDIATION EVIDENCE PREIMAGE v1 -->

Evidence revision: `sha256:7af582b296964a1c9221967238a567c7aa011eca00f0adde399fb25e95eaf292`, distinct from the remediated failed evidence revision. Reproduce it with:

```bash
python3 -c 'import hashlib; p="openspec/changes/enable-production-nexus-serve/apply-progress.md"; s=open(p, encoding="utf-8", newline="").read(); a="<!-- BEGIN MCP LIFECYCLE REMEDIATION EVIDENCE PREIMAGE v1 -->\n```text\n"; b="```\n<!-- END MCP LIFECYCLE REMEDIATION EVIDENCE PREIMAGE v1 -->"; payload=s.split(a, 1)[1].split(b, 1)[0].encode("utf-8"); print("sha256:" + hashlib.sha256(payload).hexdigest())'
```

## CURRENT STATE — Complete, Awaiting Independent Verification

- [x] 7.2a offline release handoff is complete within task 7.2.
- [x] 7.2b controlled IBM i validation gate and operator documentation are complete; task 7.2 is checked.
- [ ] Independent final `sdd-verify` is next.

## 7.2b Controlled Gate and Operator Contract

`integration/ibmi/serve_live_test.go` is build-tagged and requires the exact dedicated opt-in plus approved binary/manifest, profile, target, window, `verified-readonly` policy, Mapepire 2.3.6 artifact, item/library, and local configuration/audit/ownership roots before it creates a `nexus serve` child or permits remote work. It verifies the manifest against the approved binary, uses the official stdio MCP client for both tools, pages source to EOF, verifies cancellation, and closes/waits for shutdown. It never supplies credentials; native keyring access remains child-owned.

The live gate was NOT run. The release remains `ready_for_controlled_ibmi_validation` and `not_validated_on_ibmi`. External evidence retains only classifications, counts, and the approved binary checksum; it excludes source, host/user, paths, commands, SQL, cursors, raw errors, and cleanup details. Abort stops serving, invalidates leases, revokes eligibility, restores approved local state, and limits cleanup to exact ledger-owned paths.

### TDD Cycle Evidence

| Task | RED | GREEN | REFACTOR |
|---|---|---|---|
| 7.2b controlled gate | `go test -tags=ibmi_integration ./integration/ibmi -run '^$' -count=1`: failed because the tagged gate package did not exist | Same compile-only command: exit 0, `[no tests to run]`; no gate child or remote work occurred | `gofmt -w integration/ibmi/serve_live_test.go`; relevant release/MCP, full, race, vet, build, workflow assertion, and diff checks exited 0 |

### Work Unit Evidence

| Evidence | Exact result |
|---|---|
| Focused test command | `go test -tags=ibmi_integration ./integration/ibmi -run '^$' -count=1`: exit 0, `[no tests to run]`; the test body never ran, so child/remote contact count is 0/0. |
| Runtime harness | N/A during implementation: the sole external harness is the explicit gate and was NOT run. |
| Rollback boundary | Revert `integration/ibmi/serve_live_test.go`, the 7.2b documentation/workflow contract, this section, and the 7.2 checkbox. |

All 19/19 tasks are implemented. The parent-held native token `sha256:3b3cc3c8d8e283c04a5b3bf0c48b19ae1c6be93f714a6d49ea3e3f3f2869c53d` remains unmodified: no acquire, reset, rescope, settle, review, commit, PR, live gate, network, SSH, IBM i, or external-service action occurred. Next: `sdd-verify`.

### 7.2b Gatekeeper Correction — Pre-Child Admission and Workflow Assertion

The exact repository-local workflow assertion now executes. Before `exec.CommandContext` can create a child, the gate validates the exact opt-in; required target, window, policy, artifact, item, library, and local roots; regular approved binary and manifest handoff; current V3/keyring profile; independently derived eligibility binding and persisted eligibility; and native-keyring capability/reference. The child receives only `XDG_CONFIG_HOME`, optional local keyring runtime values, and Windows `SYSTEMROOT`; unrelated secret and SSH capability values are excluded.

| Task | RED | GREEN | REFACTOR |
|---|---|---|---|
| 7.2b gatekeeper correction | Repository-local Python workflow assertion: exit 1, `IndentationError: unexpected indent` | The corrected repository-local `python3 -c` assertion exited 0; tagged gate environment proof, tagged compile-only, and focused release/MCP tests exited 0 | `gofmt -w integration/ibmi/serve_live_test.go`; no behavior beyond prerequisite ordering and explicit environment proof changed |

| Evidence | Exact result |
|---|---|
| Focused test | `go test -tags=ibmi_integration ./integration/ibmi -run '^TestControlledChildEnvironmentExcludesUnrelatedSecrets$' -count=1`: exit 0. It proves unrelated and SSH secret-bearing values are absent from the explicit child environment. |
| Compile-only | `go test -tags=ibmi_integration ./integration/ibmi -run '^$' -count=1`: exit 0, `[no tests to run]`; the live body did not run, so child and remote counts are 0/0. |
| Runtime harness | N/A: the only external harness is `TestControlledNexusServe`, which was not run. |
| Rollback boundary | Revert the workflow assertion indentation and 7.2b pre-child/environment checks in `.github/workflows/go-verification.yml` and `integration/ibmi/serve_live_test.go`, plus this correction record. |

The offline workflow builds all approved Nexus targets under `build/v1-mcp-foundation/<version>/<goos>-<goarch>/`, then invokes `cmd/release-manifest`. That helper uses `internal/release` to create, decode, and verify the manifest sidecar against its approved binary and sidecar paths, version, revision, byte length, SHA-256, and the explicit non-claim statuses `ready_for_controlled_ibmi_validation` and `not_validated_on_ibmi`. The workflow does not compile or run `ibmi_integration`, contact IBM i, publish a release, use credentials, or transition validation status.

### TDD Cycle Evidence

| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|---|
| 7.2a offline release handoff | `cmd/release-manifest/main_test.go` | Temporary-filesystem command integration | `go test -count=1 ./internal/release`: exit 0 before edits | `go test -count=1 ./cmd/release-manifest`: exit 1 because `run` was undefined | `go test -count=1 ./cmd/release-manifest ./internal/release`: exit 0 after the minimal helper | A valid binary/sidecar verifies all contract fields; incomplete identity and an alternate sidecar path are rejected | `gofmt -w cmd/release-manifest/main.go cmd/release-manifest/main_test.go`; focused tests remained green |

### Work Unit Evidence

| Evidence | Exact result |
|---|---|
| Focused test / generator smoke | `go test -count=1 ./cmd/release-manifest -run '^(TestRunCreatesAndVerifiesReleaseManifest|TestRunRejectsManifestOutsideApprovedReleasePath)$'`: exit 0. The temporary-filesystem command harness creates, reopens, decodes, and verifies a manifest sidecar without an external runtime boundary. |
| Relevant packages | `go test -count=1 ./cmd/release-manifest ./internal/release`: exit 0. |
| Full / vet / build / diff | `go test -count=1 ./...`, `go vet ./...`, `go build ./...`, and `git diff --check`: exit 0. |
| Workflow proof | Repository-available Python structural/semantic assertions: exit 0; it confirms ordinary Go verification, release-manifest invocation, required non-claim statuses, artifact retention, and absence of IBM i integration, SSH, publication, credential, and remote-tag commands. No YAML validator is installed. |
| Runtime harness | `TestRunCreatesAndVerifiesReleaseManifest` writes a temporary binary and runs the production command function through manifest creation, readback, JSON decoding, and `release.VerifyManifest`; N/A for a live runtime because this slice has no permitted external boundary. |
| Rollback boundary | Revert only `.github/workflows/go-verification.yml`, `cmd/release-manifest/main.go`, `cmd/release-manifest/main_test.go`, and this 7.2a progress section. |

The parent retains the active native token; this unit did not acquire, reset, rescope, settle, review, commit, create a PR, invoke a live gate, use network/SSH/IBM i/external services, or alter the release validation claim.

### 7.2a Gatekeeper Correction — Release Path Admission

Before any binary read or manifest-sidecar write, `cmd/release-manifest` now accepts only an exact SemVer release version and one workflow-approved target: Linux, Darwin, or Windows on amd64 or arm64. It derives and compares both approved paths, including the Windows executable suffix, then rejects alternate binary paths and separator-, traversal-, or control-shaped version/target inputs with `release identity is invalid`. Binary and sidecar filesystem failures now map to stable classifications without supplied paths or OS error detail.

| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 7.2a gatekeeper correction | `cmd/release-manifest/main_test.go` | Temporary-filesystem command integration | `go test -count=1 ./cmd/release-manifest ./internal/release`: exit 0 before edits | New identity/filesystem regression tests initially failed to build because stable classifications and file seams were absent | `go test -count=1 ./cmd/release-manifest -run '^(TestRunRejectsUnapprovedIdentityBeforeReadingOrWriting|TestRunSanitizesFilesystemFailures)$'`: exit 0 after minimal admission and sanitizer logic | Four invalid identity shapes prove zero reads/writes; binary-read and sidecar-write failures prove distinct stable errors | `gofmt -w cmd/release-manifest/main.go cmd/release-manifest/main_test.go`; focused helper/release tests remained green |

| Evidence | Exact result |
| --- | --- |
| Focused correction test | `go test -count=1 ./cmd/release-manifest -run '^(TestRunRejectsUnapprovedIdentityBeforeReadingOrWriting|TestRunSanitizesFilesystemFailures)$'`: exit 0. |
| Focused helper/release tests | `go test -count=1 ./cmd/release-manifest ./internal/release`: exit 0. |
| Workflow check | Repository-local Python structural assertion over `.github/workflows/go-verification.yml`: exit 0; confirms approved targets, helper invocation, ordinary tests/race/vet, and no IBM i/SSH/publication/remote-tag command. |
| Full / vet / build / diff | `go test -count=1 ./...`, `go vet ./...`, `go build ./...`, and `git diff --check`: exit 0. |
| Runtime harness | Temporary-filesystem production-command tests prove pre-I/O rejection, sidecar non-creation, binary-read classification, and sidecar-write classification. N/A for a live boundary: this offline handoff has no permitted live runtime. |
| Rollback boundary | Revert only 7.2a path admission/sanitization and tests in `cmd/release-manifest/{main.go,main_test.go}`, plus this correction note. |

The parent-held native attempt remains ACTIVE (`sha256:26dfd3f2aae521b492f0bcf8d0df1f0ef306162fa04a3479b986d35a8ce955d6`). This one correction did not acquire, reset, rescope, settle, review, commit, create a PR, run a live gate, or contact a network, SSH, or IBM i boundary. Task 7.2 remains unchecked; 7.2b remains next.

### 7.2a Revision Identity Pre-I/O Remediation

Before any binary read or manifest-sidecar write, `cmd/release-manifest` now rejects empty, control, whitespace, separator, and traversal-shaped revisions with the stable `release identity is invalid` classification. The existing SemVer, target, approved-path, manifest generation, and non-claim behavior remain unchanged.

| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 7.2a revision identity remediation | `cmd/release-manifest/main_test.go` | Temporary-filesystem command integration | `go test -count=1 ./cmd/release-manifest ./internal/release`: exit 0 before edits | `go test -count=1 ./cmd/release-manifest -run '^TestRunRejectsUnapprovedIdentityBeforeReadingOrWriting$'`: exit 1 because newline/control revisions returned nil | Same focused command: exit 0 after revision admission | Empty, newline, NUL-control, whitespace, separator, and traversal revision cases each assert zero I/O-seam reads/writes | `gofmt -w cmd/release-manifest/main.go cmd/release-manifest/main_test.go`; focused helper/release suite remained green |

| Evidence | Exact result |
| --- | --- |
| Focused test / helper-release suite | `go test -count=1 ./cmd/release-manifest ./internal/release`: exit 0. |
| Full / vet / build / diff | `go test -count=1 ./...`, `go vet ./...`, `go build ./...`, and `git diff --check`: exit 0. |
| Runtime harness | The temporary-filesystem command test invokes `run` with read/write seams and proves zero calls for newline/control revisions. N/A for a live boundary: no permitted external runtime exists for this offline helper. |
| Rollback boundary | Revert only revision admission in `cmd/release-manifest/main.go`, its regression cases in `cmd/release-manifest/main_test.go`, and this remediation section. |

The parent-held native attempt remains ACTIVE (`sha256:9b09823ce8ad3f2ad255b733ba10db5b7f620c3330b540dab23e9dcf59f81c65`). No acquire, reset, rescope, settlement, review, live gate, network, SSH, IBM i, commit, or PR action occurred. Task 7.2 remains unchecked; 7.2b remains next.

## 6.1e Final Production Serve Composition

`nexus serve` now retains the admitted V3 profile through durable audit and a narrow ownership-open result that contains the one opened `source.OwnershipLedger`, its configured recovery coordinator, and closer. It runs exact-record recovery before resolver, source-acquirer, lease, or MCP server factories. The final graph uses the durable auditor, `newProductionCatalogResolver`, `newProductionSourceAcquirer`, `source.NewLeaseStore(time.Now, crypto/rand.Reader)`, `app.Service`, and the existing MCP server factory. Ownership then audit close in reverse order on all later outcomes.

The production source acquirer holds the opened ownership ledger and the current recovery target digest. It defers all remote work to each request, where it freshly reloads the exact V3/keyring profile, retrieves a native-keyring credential, and opens only the existing request-scoped bounded source adapter. Startup rejection and cancellation tests prove no later resolver, acquirer, lease, server, SSH, SFTP, Mapepire, IBM i, network, or subprocess boundary is contacted.

### TDD Cycle Evidence

| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|---|
| 6.1e final production serve composition | `cmd/nexus/main_test.go` | In-process composition integration | `go test -count=1 ./cmd/nexus` exited 0 before the unit | `go test -count=1 ./cmd/nexus -run '^(TestRunWithDepsBuildsCompleteProductionGraphAfterRecovery|TestNewProductionSourceAcquirerDefersFreshRemoteSetupUntilRequest)$'` exited 1 because the ownership result, graph factories, and source factory were undefined | Same focused command exited 0 after composition wiring | Fresh profile/keyring per source request and rejection/cancellation zero-factory cases pass in the focused suite | `gofmt -w cmd/nexus/main.go cmd/nexus/main_test.go` then focused suite exited 0; source target digest is independently matched to recovery ownership binding |

### Work Unit Evidence

| Evidence | Exact result |
|---|---|
| Focused test | `go test -count=1 ./cmd/nexus -run '^(TestRunWithDepsBuildsCompleteProductionGraphAfterRecovery|TestNewProductionSourceAcquirer|TestRunWithDepsRejectionAndCancellationDoNotBuildProductionFactories)$'` exited 0. |
| Focused package safety | `go test -count=1 ./cmd/nexus` exited 0. |
| Relevant packages | `go test -count=1 ./internal/app ./internal/mcp ./internal/source ./internal/remote` exited 0. |
| Full suite | `go test -count=1 ./...` exited 0. |
| Vet | `go vet ./...` exited 0. |
| Diff | `git diff --check` exited 0. |
| Runtime harness | In-process fakes proved admitted-profile propagation, recovery before every later factory, source factory's per-request fresh profile/keyring setup, reverse ownership/audit close, and zero later factory/remote contact after rejection or cancellation. No network, live SSH, IBM i, subprocess, commit, or PR action occurred. |
| Rollback boundary | Revert only 6.1e composition and its fakes in `cmd/nexus/main.go` and `cmd/nexus/main_test.go`, plus this progress entry and task checkbox. |

Historical routing only: after 6.1e, task 6.2 was next. The current authoritative state below records the corrected 6.2 completion.

## 6.2 Completion Correction — Current Evidence

The durable audit replay validator now accepts the same three capability classes that append encoding accepts: `catalog_resolve`, `source_read`, and `lifecycle_completion`. A lifecycle-completion record can therefore be appended, synced, closed, and replayed at the next `OpenFile` without weakening the exact seven-field schema, policy/result/lifecycle allowlists, numeric bounds, framing, or owner-only storage checks.

The official in-memory MCP transport now has real `app.Service` evidence for a cursor bound to a different catalog selection and for deterministic acquisition failure. The wrong-selection cursor and acquisition-failure responses are MCP errors with zero source/page fields; the acquisition fake executes its cleanup once and the real lease store retains zero resident bytes. These local fakes invoke no network, SSH, IBM i, or external service.

### TDD Cycle Evidence

| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|---|
| 6.2 bounded correction | `internal/audit/file_test.go`, `internal/mcp/server_test.go` | In-process durable-store and official MCP transport integration | `go test -count=1 ./internal/audit ./internal/mcp`: exit 0 before the correction | `go test -count=1 ./internal/audit -run '^TestOpenFileReopensAfterLifecycleCompletionRecord$'`: exit 1 because replay rejected `lifecycle_completion` after a successful append and close | `go test -count=1 ./internal/audit -run '^TestOpenFileReopensAfterLifecycleCompletionRecord$' && go test -count=1 ./internal/mcp -run '^(TestServerUsesRealAppServiceForPagingAndCursorFailures|TestServerUsesRealAppServiceForAcquisitionFailureWithoutPartialSource)$'`: exit 0 | The real-service MCP test covers normal paging/EOF plus malformed, out-of-range, stale, expired, wrong-selection, and acquisition-failure no-partial branches; acquisition cleanup and zero resident leases are explicit | `gofmt -w internal/audit/file.go internal/audit/file_test.go internal/mcp/server_test.go`; focused tests remained green |

### Work Unit Evidence

| Evidence | Exact result |
|---|---|
| Focused test | `go test -count=1 ./internal/audit -run '^TestOpenFileReopensAfterLifecycleCompletionRecord$' && go test -count=1 ./internal/mcp -run '^(TestServerUsesRealAppServiceForPagingAndCursorFailures|TestServerUsesRealAppServiceForAcquisitionFailureWithoutPartialSource)$'` exited 0. |
| Focused race | `go test -race -count=1 ./internal/audit ./internal/mcp -run '^(TestOpenFileReopensAfterLifecycleCompletionRecord|TestServerUsesRealAppServiceForPagingAndCursorFailures|TestServerUsesRealAppServiceForAcquisitionFailureWithoutPartialSource)$'` exited 0. |
| Bounded subprocess harness | `go test -count=1 ./cmd/nexus -run '^(TestRunWithDepsShutdownEvictsLeasesThenClosesOwnershipAuditsAndClosesAudit|TestNexusStdioSubprocessProducesOnlyJSONRPCOnStdout)$'` exited 0; the existing fixed local helper child retained parseable JSON-RPC-only stdout and no protocol JSON on stderr. |
| Full suite | `go test -count=1 ./...` exited 0. |
| Vet | `go vet ./...` exited 0. |
| Diff | `git diff --check` exited 0. |
| Runtime harness | Official `mcp.NewInMemoryTransports`, `Server.Run`, client connection, and `CallTool` invoke a real lease-backed `app.Service`. A wrong-selection cursor and deterministic acquisition failure both return zero source/page content; acquisition cleanup is exactly once and the lease store resident count is zero. |
| Rollback boundary | Revert only the lifecycle capability replay clause in `internal/audit/file.go`, the reopen regression in `internal/audit/file_test.go`, the real-service MCP wrong-selection/acquisition cases in `internal/mcp/server_test.go`, and this correction section. |

Task 6.2 is complete under the maintainer-approved `size:exception` (cap 1600 lines). The active attempt remains parent-held and ACTIVE; this correction did not acquire, reset, rescope, settle, review, commit, or create a PR. Next implementation task: 7.1.

## Historical 6.2 MCP Protocol and Shutdown Lifecycle — Superseded by Correction

The MCP server connects the official SDK transport explicitly and tracks accepted tool handlers. However, the prior evidence overclaimed task completion: cursor failures are fake-service responses rather than a real lease-backed `app.Service` through MCP; process lifecycle does not yet evict leases, append one lifecycle audit event, and close local owners in the required order; and the SDK's concrete stdio endpoint has no injectable reader/writer seam. A truthful test of the actual stdout/stderr process boundary requires a bounded subprocess harness or a new endpoint seam, neither of which was authorized for this correction.

### TDD Cycle Evidence

| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|---|
| 6.2 MCP protocol and shutdown lifecycle | `internal/mcp/server_test.go` | In-process MCP transport integration | `go test -count=1 ./internal/mcp ./internal/app ./internal/source ./cmd/nexus` exited 0 before the unit | `go test -count=1 ./internal/mcp -run '^TestServerShutdownWaitsForAcceptedHandler$'` exited 1: cancellation closed the peer before an accepted handler completed | Partial: handler ordering test passes, but the required real lease-backed cursor, lifecycle ownership/audit, and actual stdio endpoint proofs are absent | Partial: only fake-service paging/errors and handler ordering are covered | `gofmt -w internal/mcp/server.go internal/mcp/server_test.go`; no new correction code was written |

### Work Unit Evidence

| Evidence | Exact result |
|---|---|
| Focused test | `go test -count=1 ./internal/mcp -run '^(TestServerServesBothToolsOverInMemoryMCPTransport|TestServerProtocolFailuresHaveNoPartialSource|TestServerShutdownWaitsForAcceptedHandler)$'` exited 0. |
| Focused race | `go test -race -count=1 ./internal/mcp ./internal/app ./internal/source ./cmd/nexus` exited 0. |
| Relevant packages | `go test -count=1 ./internal/mcp ./internal/app ./internal/source ./cmd/nexus` exited 0. |
| Full suite | `go test -count=1 ./...` exited 0. |
| Vet | `go vet ./...` exited 0. |
| Diff | `git diff --check` exited 0. |
| Runtime harness | Official `mcp.NewInMemoryTransports`, `Server.Connect` through `Server.Run`, `Client.Connect`, `ClientSession.CallTool`, and peer close prove both registered tools, typed MCP errors, paging, EOF cursor omission, no partial-source output, and cancellation/handler ordering. No process, network, live SSH, or IBM i boundary ran. |
| Rollback boundary | Revert only `internal/mcp/server.go`, `internal/mcp/server_test.go`, this entry, and the 6.2 task checkbox. |

Historical state only: task 6.2 was blocked before the correction above. Do not treat this historical failure as current authority.

## 7.1 Persistent Sanitized Diagnostic Audit

`RunRemoteDiagnostic` now records a bounded metadata-only outcome through `audit.NewPersistentDiagnosticAuditor`. The adapter can write only `configuration_diagnostic`, `verified_read_only`, a diagnostic result class (`succeeded`, `cancelled`, `timed_out`, or `failed`), zero requested/returned counts, duration, and `completed` lifecycle. It exposes no external-client writer, does not copy diagnostic detail into durable data, and retains `ValidationStatus: not_validated_on_ibmi` for every outcome.

Audit append or sync failure overrides an otherwise successful diagnostic to the sanitized `diagnostic evidence unavailable` failure. The durable sink's existing append/sync behavior poisons the sink; this unit verifies the override using its actual sync-failure seam. Timeout and cancellation remain bounded by the existing context deadline/cancellation path and persist only their allowlisted classifications.

### TDD Cycle Evidence

| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|---|
| 7.1 persistent diagnostic audit | `internal/configuration/readiness_test.go` | In-process durable-audit integration | `go test -count=1 ./internal/configuration` exited 0 before edits | `go test -count=1 ./internal/configuration -run '^(TestRemoteDiagnosticPersistsOnlySanitizedFactsAndReopens|TestRemoteDiagnosticAppendFailureOverridesSuccessWithoutLiveClaim)$'` exited 1 because `configuration.NewPersistentDiagnosticAuditor` was undefined | `go test -count=1 ./internal/configuration -run '^(TestRemoteDiagnostic|TestPersistentDiagnosticAuditor)'` exited 0 | Success, timeout, cancellation, and durable sync-failure override use distinct runner/context/storage paths; all retain the non-live validation status | `gofmt -w internal/audit/audit.go internal/audit/file.go internal/configuration/readiness.go internal/configuration/readiness_test.go`; focused tests remained green |

### Work Unit Evidence

| Evidence | Exact result |
|---|---|
| Focused test | `go test -count=1 ./internal/configuration -run '^(TestRemoteDiagnostic|TestPersistentDiagnosticAuditor)'` exited 0. |
| Focused race | `go test -race -count=1 ./internal/configuration ./internal/audit -run '^(TestRemoteDiagnostic|TestPersistentDiagnosticAuditor)'` exited 0; the audit package had no matching focused names. |
| Relevant packages | `go test -count=1 ./internal/configuration ./internal/audit` exited 0. |
| Full suite | `go test -count=1 ./...` exited 0. |
| Vet | `go vet ./...` exited 0. |
| Diff | `git diff --check` exited 0. |
| Runtime harness | An in-process `audit.File` temporary owner-only store wrote a success record, closed, reopened, and read back exactly seven fields. Separate fake contexts proved a 5ms timeout and pre-cancelled path persist only `timed_out`/`cancelled`; a real durable sink sync failure overrides success. No network, SSH, IBM i, external service, or external-client mutation seam exists or ran. |
| Rollback boundary | Revert only the configuration-diagnostic allowlist/adapter in `internal/audit/{audit,file}.go`, diagnostic timing/override behavior and tests in `internal/configuration/readiness{,_test}.go`, this evidence section, and the 7.1 checkbox. |

Task 7.1 is complete. The parent retains the active native token; this unit did not acquire, reset, rescope, settle, review, commit, or create a PR. Next implementation task: 7.2 only. Final `sdd-verify` remains blocked.

## Historical Current State — Superseded by the Complete 19/19 State Above

- Historical completed state: 18/19 tasks, including Phase 6.1, corrected 6.2 evidence, and 7.1 persistent diagnostic audit evidence.
- Historical pending state: 7.2 only. Phase 6 and task 7.1 were complete.
- Delivery decision: maintainer-approved `size:exception` for 6.2, capped at 1600 lines.
- Execution control: no automatic reset/retry loops. Make one attempt and at most one scoped correction per unit, then stop for maintainer decision.
- Validation control: validators evaluate only the current subtask; deferred requirements are not gate failures.
- Acquire control: declare all relevant untracked files at acquire and classify each as existing, new, or out-of-scope.
- Completion evidence: exact commands/results and the rollback boundary are required; hashes are optional integrity metadata, not standalone completion authority.
- Established decisions remain: fixed 30-day eligibility, operator retention file, Mapepire 2.3.6, the 6.2 `size:exception`, and OpenSpec authority.
- Historical next action: 7.2 only. This is superseded: all 19 tasks are complete and independent `sdd-verify` is next.

## 6.1d Authenticated Bounded Catalogados Resolver

`catalogados.AuthenticatedExecutor` opens one typed Mapepire session for each Catalogados request, authenticates with the V3 profile username and a request-fresh native-keyring credential, executes only the resolver-owned prepared query with the existing 51-row sentinel, closes any returned cursor, exits the protocol, and always closes the session and request-owned SSH client. Cancellation, deadline, startup, authentication, query, cursor, and exit failures map only to deterministic catalog unavailable/cancelled/timed-out categories.

`newProductionCatalogResolver` is a factory seam only. It freshly loads the exact named V3/keyring profile, obtains the exact profile credential from `credential.NewNativeCredentialStore()`, starts Mapepire exclusively through the existing verified receipt path, and returns the narrow Catalogados resolver. It is intentionally not wired into `defaultDeps` or the final serve graph; 6.1e owns that composition.

### TDD Cycle Evidence

| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|---|
| 6.1d authenticated bounded Catalogados resolver | `internal/connectors/ibmi/catalogados/catalogados_test.go`, `cmd/nexus/main_test.go` | In-process connector and production-factory integration fakes | `go test -count=1 ./internal/connectors/ibmi/catalogados ./cmd/nexus` exited 0 before this unit | `go test -count=1 ./internal/connectors/ibmi/catalogados -run '^TestAuthenticatedExecutor'` exited 1 because authenticated executor symbols were undefined; `go test -count=1 ./cmd/nexus -run '^TestProductionCatalogResolver'` exited 1 because factory seams were undefined | `go test -count=1 ./internal/connectors/ibmi/catalogados ./cmd/nexus -run '^(TestAuthenticatedExecutor|TestProductionCatalogResolver)'` exited 0 | Authentication, fixed prepared query, 51-row bound, cursor/exit/session cleanup, cancellation sanitization, and fresh exact V3/keyring profile loading are asserted | `gofmt -w internal/connectors/ibmi/catalogados/catalogados.go internal/connectors/ibmi/catalogados/catalogados_test.go cmd/nexus/main.go cmd/nexus/main_test.go` then focused suite exited 0; no behavior changes |

### Work Unit Evidence

| Evidence | Exact result |
|---|---|
| Focused test | `go test -count=1 ./internal/connectors/ibmi/catalogados ./cmd/nexus -run '^(TestAuthenticatedExecutor|TestProductionCatalogResolver)'` exited 0. |
| Focused package safety/coverage | `go test -count=1 ./internal/connectors/ibmi/catalogados ./cmd/nexus` exited 0. |
| Full suite | `go test -count=1 ./...` exited 0. |
| Vet | `go vet ./...` exited 0. |
| Diff | `git diff --check` exited 0. |
| Runtime harness | In-process typed-session and production-factory fakes proved request authentication, fixed query/bound, deterministic failure categories, and session cleanup without network, live SSH, IBM i, or subprocess activity. |
| Rollback boundary | Revert only the 6.1d executor/session helper in `internal/connectors/ibmi/catalogados/catalogados.go`, its tests, the unconsumed factory seam in `cmd/nexus/main.go`, its test, and this progress section. |

Task 6.1d is complete. Task 6.1e remains pending. No native lifecycle, review, commit, PR, network, live SSH, IBM i, or subprocess operation was performed. The intended stacked-to-main boundary is this resolver-only unit.

### 6.1d Bounded Correction — Narrow Catalogados Executor API

The executor seam now accepts only `catalog.Search`; it constructs `PreparedSearch(search)` internally and has no method accepting SQL, row limits, or parameters. The legacy catalog-spike fixed session now uses `NewFixedSessionExecutor`, which also accepts only `catalog.Search` and constructs the same fixed query internally. This preserves the factory, authenticated lifecycle, cleanup, and 6.1e composition boundary.

| Evidence | Exact result |
|---|---|
| Safety net | `go test -count=1 ./internal/connectors/ibmi/catalogados ./cmd/nexus` exited 0 before correction. |
| RED | `go test -count=1 ./internal/connectors/ibmi/catalogados -run '^(TestResolver|TestAuthenticatedExecutor)'` exited 1 because the narrow `Resolve` executor methods were absent. A second RED, `go test -count=1 ./internal/connectors/ibmi/catalogados -run '^TestFixedSessionExecutorOwnsCatalogadosQuery$'`, exited 1 because the fixed-session narrow adapter was absent. |
| GREEN | `go test -count=1 ./internal/connectors/ibmi/catalogados ./cmd/nexus -run '^(TestResolver|TestAuthenticatedExecutor|TestFixedSessionExecutorOwnsCatalogadosQuery|TestProductionCatalogResolver)'` exited 0. |
| Refactor | `gofmt -w internal/connectors/ibmi/catalogados/catalogados.go internal/connectors/ibmi/catalogados/catalogados_test.go cmd/catalogspike/main.go` then the GREEN command exited 0. |
| Focused packages | `go test -count=1 ./internal/connectors/ibmi/catalogados ./cmd/nexus ./cmd/catalogspike` exited 0. |
| Full / vet / diff | `go test -count=1 ./...`, `go vet ./...`, and `git diff --check` each exited 0. |
| Runtime harness | In-process Mapepire-session and resolver fakes proved only a validated search can reach the fixed prepared query with the 51-row bound; no network, live SSH, IBM i, or subprocess activity. |
| Rollback boundary | Revert the narrow executor API/adapter and its tests in `internal/connectors/ibmi/catalogados/`, plus the one catalog-spike adapter call; do not alter factory or 6.1e wiring. |

The correction is within the existing 6.1d ≤800-line unit budget. Task 6.1d remains checked with corrected evidence.

### 6.1d Remediation — Forged Search Rejection

Both Catalogados executor implementations canonically revalidate `catalog.Search` through `catalog.NewSearch` and require the canonical result to equal the supplied value. Invalid or non-canonical forged fields therefore return only `ErrCatalogUnavailable` before either executor opens/authenticates a session or derives query parameters. The fixed prepared query, 51-row bound, authenticated factory, cleanup, cancellation, and sanitized failure behavior remain unchanged.

| Evidence | Exact result |
|---|---|
| Safety net | `go test -count=1 ./internal/connectors/ibmi/catalogados ./cmd/nexus` exited 0 before remediation. |
| RED | `go test -count=1 ./internal/connectors/ibmi/catalogados -run '^TestAuthenticatedExecutorRejectsForgedSearchBeforeOpeningSession$'` exited 1: forged searches reached the opener (`opened=1`). |
| GREEN | `go test -count=1 ./internal/connectors/ibmi/catalogados -run '^(TestAuthenticatedExecutor|TestFixedSessionExecutorOwnsCatalogadosQuery|TestResolver)'` exited 0. |
| Refactor | `gofmt -w internal/connectors/ibmi/catalogados/catalogados.go internal/connectors/ibmi/catalogados/catalogados_test.go` then the GREEN command exited 0. |
| Focused packages | `go test -count=1 ./internal/connectors/ibmi/catalogados ./cmd/nexus ./cmd/catalogspike` exited 0. |
| Full / vet / diff | `go test -count=1 ./...`, `go vet ./...`, and `git diff --check` each exited 0. |
| Runtime harness | In-process forged `catalog.Search` values assert zero opener calls and deterministic sanitized rejection; no network, live SSH, IBM i, or subprocess activity. |
| Rollback boundary | Revert only canonical search validation and its regression proof in `internal/connectors/ibmi/catalogados/{catalogados.go,catalogados_test.go}`, plus this evidence section. |

Task 6.1d passes only with this remediation evidence. Later tasks remain pending.

## Historical Cumulative Task State — Superseded by CURRENT STATE

- [x] Phase 1 protected onboarding baseline.
- [x] Phase 2 proof-bound eligibility.
- [x] Phase 3 secure local state.
- [x] Phase 4 durable audit append, recovery, retention, and corrections.
- [x] 5.1 fixed Mapepire 2.3.6 receipt-only SSH stdio launch.
- [x] 5.2 fixed source acquisition.
- [ ] Historical Phase 6.1 status: independent eligibility consumer admission was complete before durable audit/ownership and recovery-before-server composition.
- [ ] Phase 6.2 and Phase 7 remain pending.

## Historical Evidence — Independent Serve Eligibility Consumer Admission

`runWithDeps` now performs strict operator configuration/retention admission first. Only after it succeeds does it load the named V3 profile, derive the expected six-dimensional binding through `profile.DeriveEligibilityBinding`, and invoke the production-bound `EligibilityStore.Check` seam. Recovery, server construction, and server run remain later boundaries.

Expected eligibility is derived only from the current controlled V3 profile and authoritative constants: normalized target, `verified-readonly`, persisted pin/trust, exact native keyring identity, Mapepire 2.3.6 identity, and `values-1-v1`. Persisted eligibility is never used to derive the expectation. Missing, malformed, expired, and target/policy/pin/credential/artifact/proof mismatches fail closed without secret-bearing errors.

The persisted eligibility validator accepts the fixed `verified-readonly` policy ID, allowing a producer-issued canonical record to be saved and consumed.

## Historical Product Correction

- Corrected the gate order from eligibility-before-retention to retention-before-eligibility.
- The operator-retention failure sentinel proves eligibility is not evaluated and `Recovery`, `ServerFactory`, and `Run` remain zero.
- Every eligibility-rejection case proves strict operator admission ran once and recovery, `ServerFactory`, and `Run` remain zero.
- A changed current-profile target with an otherwise valid stored record proves the consumer does not self-compare `Eligibility.Binding()`.

## Historical TDD Cycle Evidence

| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|---|
| 6.1 independent eligibility consumer admission | `cmd/nexus/main_test.go` | Production composition integration with temporary eligibility store and fakes | `go test -count=1 ./cmd/nexus` exited 0 before the prior consumer slice | Initial consumer RED failed because the required admission seams were absent | Consumer-focused test passed after the admission wiring | Valid, missing, malformed, expired, six binding mutations, and changed-current-profile target are covered | `admitServeEligibility` keeps the independently derived check narrow |
| 6.1 ordering/factory correction | `cmd/nexus/main_test.go` | Production composition integration with fakes | Existing consumer-focused suite passed before correction | `TestRunWithDepsRetainsOperatorAdmissionBeforeEligibilityAndFactory` failed: observed `eligibility,operator-retention` instead of `operator-retention` | Focused ordering/factory suite passed after reordering gates | Retention failure and all eligibility rejection cases assert later counters remain zero | No further refactor required |

## Historical Work Unit Evidence

| Evidence | Exact result |
|---|---|
| Focused RED | `go test -count=1 ./cmd/nexus -run '^TestRunWithDepsRetainsOperatorAdmissionBeforeEligibilityAndFactory$'` exited 1 with observed order `eligibility,operator-retention`. |
| Focused GREEN | `go test -count=1 ./cmd/nexus -run '^(TestRunWithDepsRetainsOperatorAdmissionBeforeEligibilityAndFactory|TestRunWithDepsAdmitsOnlyTheCurrentIndependentEligibilityBinding|TestRunWithDepsRejectsStoredBindingSelfComparison)$'` exited 0. |
| Focused race | The same focused suite with `-race` exited 0 with no race report. |
| Runtime harness | `runWithDeps` with a real temporary `EligibilityStore`, controlled profile loader, strict-retention sentinel, recovery sentinel, factory sentinel, and runner sentinel; no network, SSH, IBM i, subprocess, or live keyring access occurred. |
| Rollback boundary | Revert only this slice's admission-order lines in `cmd/nexus/main.go`, consumer/order tests in `cmd/nexus/main_test.go`, the fixed policy validation adjustment in `internal/profile/eligibility.go`, and this progress artifact. |

## Historical Verification and Accounting

- `go test -count=1 ./...`, `go vet ./...`, `go build ./...`, and `git diff --check` exited 0.
- Native settlement is authoritative. No acquire, settle, reset, rescope, review, or other native lifecycle action was performed.
- Exact current tracked worktree delta: 42 files, 3,232 additions, 504 deletions. This is the worktree total, not a claim that all changes belong to this slice.
- Exact current untracked files: `internal/audit/file.go`, `internal/audit/file_test.go`, `internal/localstate/platform.go`, `internal/localstate/platform_linux.go`, `internal/localstate/platform_test.go`, `internal/localstate/platform_unsupported.go`, `internal/localstate/platform_windows.go`, `internal/ownership/sqlite/platform_secure_path_test.go`, `internal/profile/eligibility.go`, `internal/profile/eligibility_test.go`, `internal/tui/profile_screen.go`, `internal/tui/profile_screen_test.go`, `internal/tui/profile_validation.go`, `internal/tui/wizard_progress.go`, `internal/tui/wizard_progress_test.go`, `internal/tui/wizard_render.go`, `internal/tui/wizard_viewport.go`, `nexus`, and the untracked OpenSpec archive/change paths listed by `git ls-files --others --exclude-standard`.

## Historical Current Routing — Superseded by CURRENT STATE

`apply/continue`: durable audit/ownership plus recovery-before-server production composition remain in task 6.1. Do not mark task 6.1 complete or start 6.2/Phase 7.

## Historical Evidence Finalization — Phase 6.1 Serve Eligibility Consumer

Product validator PASS after correction: strict operator retention is admitted before independent eligibility; every rejection, including a factory that must remain at zero, stays before recovery, server construction, and run; a valid profile reaches the next boundary. The consumer derives its expected binding from controlled current inputs, performs no stored self-comparison, and emits no secret. The producer's fixed 30-day eligibility policy remains preserved.

The canonical evidence preimage below is UTF-8 with LF line endings, no trailing spaces, and a final LF. Its hashed byte range starts immediately after the opening ` ```text\n` fence line and ends immediately before the closing ` ```\n` fence line; the enclosing HTML markers and fences are excluded. The evidence hash is intentionally outside that range to avoid self-reference.

<!-- BEGIN CURRENT EVIDENCE PREIMAGE v1 -->
```text
CANONICAL EVIDENCE PREIMAGE v1
Encoding: UTF-8; LF line endings; no trailing spaces; final LF included.
Scope: Phase 6.1 serve eligibility consumer correction only; product/test source remains unchanged in this finalization.
Focused command: go test -count=1 ./cmd/nexus -run '^(TestRunWithDepsRetainsOperatorAdmissionBeforeEligibilityAndFactory|TestRunWithDepsAdmitsOnlyTheCurrentIndependentEligibilityBinding|TestRunWithDepsRejectsStoredBindingSelfComparison)$'
Focused outcome: exit 0 (PASS).
Focused race command: go test -race -count=1 ./cmd/nexus -run '^(TestRunWithDepsRetainsOperatorAdmissionBeforeEligibilityAndFactory|TestRunWithDepsAdmitsOnlyTheCurrentIndependentEligibilityBinding|TestRunWithDepsRejectsStoredBindingSelfComparison)$'
Focused race outcome: exit 0 (PASS; no race report).
Full command: go test -count=1 ./...
Full outcome: exit 0 (PASS).
Vet command: go vet ./...
Vet outcome: exit 0.
Build command: go build ./...
Build outcome: exit 0.
Diff check command: git diff --check
Diff check outcome: exit 0.
Scoped diff preimage: concatenate, in the listed order and without separators, the exact stdout bytes from `git diff --no-ext-diff --binary -- cmd/nexus/main.go cmd/nexus/main_test.go`, `git diff --no-index --binary /dev/null internal/profile/eligibility.go`, and `git diff --no-index --binary /dev/null internal/profile/eligibility_test.go`; each no-index exit 1 means a difference and is normalized by the shell.
Scoped diff hash command: { git diff --no-ext-diff --binary -- cmd/nexus/main.go cmd/nexus/main_test.go; git diff --no-index --binary /dev/null internal/profile/eligibility.go || [ $? -eq 1 ]; git diff --no-index --binary /dev/null internal/profile/eligibility_test.go || [ $? -eq 1 ]; } | sha256sum
Scoped diff hash outcome: exit 0; sha256:7befb285fa982cca0a16445cf77c9c8134c5ee95381df8c2bb6411a0b023b2f1.
```
<!-- END CURRENT EVIDENCE PREIMAGE v1 -->

Evidence hash: `sha256:d0620741889c77ef552893260f17900220e3ce275a1ecb1e0ed3fd6aea39b3d6` (new and distinct from failed evidence `sha256:5f074690e0437569982ef663632022d3aab29baae61971dffeec5c5c03cb8ca8`). Reproduce it from the persisted artifact with:

```bash
python3 -c 'import hashlib; p="openspec/changes/enable-production-nexus-serve/apply-progress.md"; s=open(p, encoding="utf-8", newline="").read(); a="<!-- BEGIN CURRENT EVIDENCE PREIMAGE v1 -->\n```text\n"; b="```\n<!-- END CURRENT EVIDENCE PREIMAGE v1 -->"; payload=s.split(a, 1)[1].split(b, 1)[0].encode("utf-8"); print("sha256:" + hashlib.sha256(payload).hexdigest())'
```

Native baseline now includes the relevant untracked files declared at acquire. Native settlement owns the exact line-count record and the `<=800` decision; this artifact-only finalization does not acquire, settle, reset, rescope, or review.

Task 6.1 remains unchecked. Next: `apply/continue` for durable audit/ownership plus recovery-before-server production composition. Final verify remains blocked.

## Historical Evidence — 6.1b Durable Local-State and Recovery-Before-Server Slice

The admitted V3 profile is passed to both durable local-state openers. `runWithDeps` opens the retention-approved audit first, then the secure ownership ledger, uses the ownership-provided recovery coordinator before constructing the server, and closes ownership then audit on every later outcome. Open and recovery failures map to sanitized serve classifications; cancellation after audit opening closes audit without opening ownership or reaching recovery, factory, or run.

The default audit opener uses `audit.OpenFile` with `LoadOperatorRetention` and `localstate.NewPlatform`; the default ownership opener uses `ownership/sqlite.Open` under the user configuration root. This slice deliberately stops at the injected recovery boundary: the final remote cleanup opener plus resolver/acquirer/lease/MCP adapter composition remain a later slice.

### Historical Product Evidence

- `TestRunWithDepsDurableLocalStateAndRecoveryBeforeServer/audit_open_failure_blocks_ownership_recovery_and_server` proves ownership, recovery, factory, and run are zero.
- `.../ownership_open_failure_closes_audit` proves audit closes and recovery, factory, and run are zero.
- `.../recovery_failure_closes_ownership_then_audit_before_server` proves reverse close order and zero factory/run with a sanitized recovery error.
- `.../valid_path_recovers_before_factory_and_closes_in_reverse_order` proves audit → ownership → recovery → factory → run → ownership close → audit close.
- `TestRunWithDepsCancellationAfterAuditOpenClosesWithoutRemoteWork` proves cancellation closes audit and leaves recovery, factory, and run at zero.
- Existing operator-retention and eligibility tests retain zero-side-effect rejection gates before any local-state or recovery boundary.

### Historical TDD Cycle Evidence

| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|---|
| 6.1 durable local state/recovery | `cmd/nexus/main_test.go` | Production composition integration with fakes | `go test -count=1 ./cmd/nexus` exited 0 before this slice | Focused test initially failed because `mainDeps` lacked durable opener seams; it then exposed raw recovery text and an incomplete ownership-order assertion | Focused suite exited 0 after opener wiring, sanitized mapping, and reverse-close handling | Audit-open, ownership-open, recovery, cancellation, valid ordering, and existing admission rejection cases | No additional refactor needed |

### Historical Work Unit Evidence

| Evidence | Exact result |
|---|---|
| Focused | `go test -count=1 ./cmd/nexus -run '^(TestRunWithDepsDurableLocalStateAndRecoveryBeforeServer|TestRunWithDepsCancellationAfterAuditOpenClosesWithoutRemoteWork)$'` exited 0. |
| Race | `go test -race -count=1 ./...` exited 0. |
| Full | `go test -count=1 ./...` exited 0. |
| Vet | `go vet ./...` exited 0. |
| Build | `go build ./...` exited 0. |
| Diff | `git diff --check` exited 0. |
| Runtime harness | In-process production composition fakes exercised audit/ownership/recovery/factory/run ordering and close paths without network, SSH, IBM i, subprocess, or live keyring access. |
| Rollback boundary | Revert only this durable opener/recovery wiring in `cmd/nexus/main.go`, its focused cases in `cmd/nexus/main_test.go`, and this appended progress section. |

### Historical Canonical Slice Evidence

The preimage is UTF-8 with LF line endings and a final LF. Hash exactly the bytes after the opening ` ```text\n` and before the closing ` ```\n`; exclude fences and marker lines.

<!-- BEGIN DURABLE LOCAL-STATE EVIDENCE PREIMAGE v1 -->
```text
change=enable-production-nexus-serve
task=6.1-durable-local-state-recovery-before-server
focused=go test -count=1 ./cmd/nexus -run ^(TestRunWithDepsDurableLocalStateAndRecoveryBeforeServer|TestRunWithDepsCancellationAfterAuditOpenClosesWithoutRemoteWork)$: exit 0
race=go test -race -count=1 ./...: exit 0
full=go test -count=1 ./...: exit 0
vet=go vet ./...: exit 0
build=go build ./...: exit 0
diff=git diff --check: exit 0
```
<!-- END DURABLE LOCAL-STATE EVIDENCE PREIMAGE v1 -->

Evidence hash: `sha256:7ed77d84c9aafaaaec1582d3e2da5a2bbeb40420c44f49eb5a4796609a4d7d99`.

Reproduce from this persisted artifact:

```bash
python3 -c 'import hashlib; p="openspec/changes/enable-production-nexus-serve/apply-progress.md"; s=open(p, encoding="utf-8", newline="").read(); a="<!-- BEGIN DURABLE LOCAL-STATE EVIDENCE PREIMAGE v1 -->\n```text\n"; b="```\n<!-- END DURABLE LOCAL-STATE EVIDENCE PREIMAGE v1 -->"; payload=s.split(a, 1)[1].split(b, 1)[0].encode("utf-8"); print("sha256:" + hashlib.sha256(payload).hexdigest())'
```

## Historical Current Routing — Superseded by CURRENT STATE

Task 6.1 remains unchecked. Next: `apply/continue` for the final remote cleanup opener plus connector/app/MCP composition. Final verify remains blocked.

## 6.1c Default Bounded Recovery Coordinator

`openDurableOwnership` now returns a non-nil `source.RecoveryCoordinator` with the opened exact-record SQLite ledger. Its recovery guards load each ledger-record profile afresh from the default profile store, retrieve the exact profile credential through `credential.NewNativeCredentialStore()` (native service `BAC Nexus`), and open only `remote.NewRecoveryRemote` over authenticated SSH. The remote exposes only `Remove`, `Stat`, and `Close`; record validation, exact path confirmation, target binding, and deletion remain enforced by `source.RecoveryCoordinator`. Context cancellation is checked before and after local credential access and by the coordinator and remote adapter before each remote operation. Recovery continues to run before server construction through the existing 6.1b ordering.

### TDD Cycle Evidence

| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|---|
| 6.1c default recovery coordinator | `cmd/nexus/main_test.go` | In-process production-composition integration | `go test -count=1 ./cmd/nexus` exited 0 before the new behavior test | `go test -count=1 ./cmd/nexus -run '^TestOpenDurableOwnershipBuildsBoundedExactRecoveryCoordinator$'` exited 1: required ownership/recovery composition seams were undefined | Same focused command exited 0 after coordinator and narrow remote wiring | Valid exact row verifies fresh exact-name load → keyring lookup → cleanup/confirmed delete/close; cancelled recovery verifies no additional ledger access | No further refactor needed |

### Work Unit Evidence

| Evidence | Exact result |
|---|---|
| Focused test | `go test -count=1 ./cmd/nexus -run '^TestOpenDurableOwnershipBuildsBoundedExactRecoveryCoordinator$'` exited 0. |
| Focused package safety/coverage | `go test -count=1 ./cmd/nexus` exited 0; `go test -count=1 ./internal/remote` exited 0. |
| Full suite | `go test -count=1 ./...` exited 0. |
| Vet | `go vet ./...` exited 0. |
| Diff | `git diff --check` exited 0. |
| Runtime harness | In-process coordinator harness used a fake exact-record ledger, profile loader, native-credential-store seam, and narrow cleanup remote. It verified recovery completion before any factory boundary without network, live SSH, IBM i, or subprocess execution. |
| Rollback boundary | Revert only 6.1c recovery wiring in `cmd/nexus/main.go`, the narrow recovery adapter in `internal/remote/ssh.go`, its focused composition test in `cmd/nexus/main_test.go`, and this progress section. |

Task 6.1c is complete. Tasks 6.1d and 6.1e remain pending. No native lifecycle, review, commit, PR, network, live SSH, IBM i, or subprocess operation was performed. The intended stacked-to-main boundary is this coordinator-only unit.

## Focused Remediation — Observable Stdio Helper Child Invocation

This maintainer-authorized remediation corrects only the CRITICAL tautological assertion identified by failed evidence revision `sha256:c287fda6abf41e3e9bbc9df3169f4fd526799bc9df1ec64b39ae3ef4803e9c79`. All 19/19 tasks remain complete and unchanged.

`TestNexusStdioSubprocessProducesOnlyJSONRPCOnStdout` now derives its child assertion from the actual `exec.Cmd` process boundary: after the bounded helper has completed through `command.Wait`, it requires a non-nil exited `command.ProcessState`. A failed `Start` still fails immediately; a child that does not complete and reap fails the test, while the existing timeout branch kills and reaps it before failure. No product code changed.

### TDD Cycle Evidence

| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|---|
| Focused child-invocation evidence remediation | `cmd/nexus/main_test.go` | Bounded local subprocess integration | `go test -count=1 ./cmd/nexus -run '^TestNexusStdioSubprocessProducesOnlyJSONRPCOnStdout$'`: exit 0 before edit | Static assertion-quality guard exited 1 while both `childCount := 1` and `if childCount != 1` were present, demonstrating the old evidence was tautological; no production RED was fabricated because production behavior was already correct | Same focused test exited 0 with `command.ProcessState.Exited()` observed after `command.Wait` | Skipped: this surgical correction has one fixed subprocess path; existing JSON-RPC frame assertions and the new process-exit assertion prove distinct observable outcomes without widening scope | `gofmt -w cmd/nexus/main_test.go`; focused test remained green |

### Work Unit Evidence

| Evidence | Exact result |
|---|---|
| Focused test command | `go test -count=1 ./cmd/nexus -run '^TestNexusStdioSubprocessProducesOnlyJSONRPCOnStdout$'`: exit 0; the fixed helper child completed and produced valid JSON-RPC stdout. |
| Runtime harness command/scenario | The same focused command exercised the real local `exec.Cmd` helper boundary. `command.Wait` returned successfully and the assertion observed an exited `command.ProcessState`; the three-second kill-and-Wait timeout branch did not fire. |
| Rollback boundary | Revert only the observable process-state assertion in `cmd/nexus/main_test.go` and this remediation section; no product behavior or unrelated evidence is removed. |

<!-- BEGIN STDIO CHILD INVOCATION REMEDIATION EVIDENCE PREIMAGE v1 -->
```text
change=enable-production-nexus-serve
failed_evidence_revision=sha256:c287fda6abf41e3e9bbc9df3169f4fd526799bc9df1ec64b39ae3ef4803e9c79
scope=cmd/nexus/main_test.go subprocess child-invocation assertion only
safety_net=go test -count=1 ./cmd/nexus -run ^TestNexusStdioSubprocessProducesOnlyJSONRPCOnStdout$: exit 0
red=python3 -c static assertion guard for childCount := 1 and if childCount != 1: exit 1 (tautological assertion detected)
green=go test -count=1 ./cmd/nexus -run ^TestNexusStdioSubprocessProducesOnlyJSONRPCOnStdout$: exit 0
package=go test -count=1 ./cmd/nexus: exit 0
focused_race=go test -race -count=1 ./cmd/nexus -run ^TestNexusStdioSubprocessProducesOnlyJSONRPCOnStdout$: exit 0
race=go test -race -count=1 ./...: exit 0
full=go test -count=1 ./...: exit 0
vet=go vet ./...: exit 0
build=go build -o /tmp/opencode/nexus-tautology-remediation ./cmd/nexus: exit 0; binary removed; test ! -e /tmp/opencode/nexus-tautology-remediation: exit 0
diff=git diff --check: exit 0
cleanup=bounded local helper completed; command.Wait reaped it; 3-second kill-and-Wait branch did not fire; no network, SSH, IBM i, external service, or TestControlledNexusServe execution
tasks=19/19 unchanged
```
<!-- END STDIO CHILD INVOCATION REMEDIATION EVIDENCE PREIMAGE v1 -->

Evidence revision: `sha256:83fc3e058438396a93c33e43a2cfb6d20cb45445df45a499fb2c82b4954f8a83`, distinct from and explicitly remediating `sha256:c287fda6abf41e3e9bbc9df3169f4fd526799bc9df1ec64b39ae3ef4803e9c79`.

Reproduce it with:

```bash
python3 -c 'import hashlib; p="openspec/changes/enable-production-nexus-serve/apply-progress.md"; s=open(p, encoding="utf-8", newline="").read(); a="<!-- BEGIN STDIO CHILD INVOCATION REMEDIATION EVIDENCE PREIMAGE v1 -->\n```text\n"; b="```\n<!-- END STDIO CHILD INVOCATION REMEDIATION EVIDENCE PREIMAGE v1 -->"; payload=s.split(a, 1)[1].split(b, 1)[0].encode("utf-8"); print("sha256:" + hashlib.sha256(payload).hexdigest())'
```

The parent-held token `sha256:d8636dc980533bb0e0f9f84a1989a223f5753be223d8878df5045e4838464236` remains untouched. No acquire, settle, reset, rescope, verify, archive, review, commit, PR, live gate, network, SSH, IBM i, or external-service operation occurred. Independent `sdd-verify` remains the next gate.
