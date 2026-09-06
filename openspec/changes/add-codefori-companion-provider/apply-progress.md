# Apply progress: add-codefori-companion-provider

## Completed Slice 1 — provider-neutral domain contract

Completed the five implementation-owned Slice 1 tasks and marked each corresponding checkbox `[x]` in `tasks.md`:

- RED query-recognizer table tests for allowed ASCII formatting and prohibited broader input.
- RED fake `Provider` tests for pre-call validation, canonical forwarding, malformed-result rejection, and cancellation.
- RED normalized-result validation tests for state, row count, UTF-8, and 256-byte bounds.
- GREEN neutral port, state types, proof-query canonicalizer, validation, and service in `internal/provider/provider.go`.
- REFACTOR confirmation that the package has no transport, descriptor, token, profile, credential, PowerShell, VS Code, or Code for IBM i imports or names.

## Files changed — Slice 1

- `internal/provider/provider.go`
- `internal/provider/provider_test.go`
- `openspec/changes/add-codefori-companion-provider/tasks.md` (completion reconciliation)
- `openspec/changes/add-codefori-companion-provider/apply-progress.md`

## TDD Cycle Evidence

| Task | Test file | Layer | Safety net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|---|
| Slice 1 RED query recognition | `internal/provider/provider_test.go` | Unit | N/A (new package) | `go test -count=1 ./internal/provider` failed to build because production symbols were undefined | Passed after the canonicalizer implementation | 2 accepted formatting/case cases and 9 prohibited-input cases | Removed redundant empty-token condition; scoped test passed |
| Slice 1 RED provider service | `internal/provider/provider_test.go` | Unit | N/A (new package) | Same compile-failure RED run referenced the absent port/service types | Passed after `RunProofQuery` implementation | Valid canonical forwarding, invalid input no-call, malformed-result rejection, and pre-start cancellation cover independent paths | Scoped test passed after cleanup |
| Slice 1 RED normalized results | `internal/provider/provider_test.go` | Unit | N/A (new package) | Same compile-failure RED run referenced absent state/result validation | Passed after `ValidateQueryResult` implementation | Valid 256-byte value, six valid non-success states, and six malformed shapes cover result branches | Scoped test passed after cleanup |

## Completed Slice 2 — fixed-loopback Go connector

Completed the three implementation-owned Slice 2 tasks and marked each corresponding checkbox `[x]` in `tasks.md`:

- RED tests prove invalid SQL never reads a descriptor or creates an HTTP request; fixed-port authenticated request construction; exact version/generation/request-ID correlation; strict unknown, duplicate, and trailing JSON rejection; byte limits; fixed deadlines; single-attempt behavior; and bounded token-free result states.
- GREEN implements only the fixed `127.0.0.1:64139` `POST /v1/rpc` connector with an injected descriptor reader, cryptographic request IDs, bearer authentication, strict protocol decoding, and no endpoint setting, discovery, fallback, or reusable transport abstraction.
- TRIANGULATE tests cover version, generation, and request-ID mismatch; missing, stale, denied, and nil descriptors; malformed and oversized responses; status handling; and `internal/provider.ValidateQueryResult` validation of returned normalized results. The non-Windows constructor uses the same nil-reader fail-closed path.

## Files changed — Slice 2

- `internal/connectors/ibmi/codefori/client.go`
- `internal/connectors/ibmi/codefori/protocol.go`
- `internal/connectors/ibmi/codefori/descriptor.go`
- `internal/connectors/ibmi/codefori/descriptor_other.go`
- `internal/connectors/ibmi/codefori/client_test.go`
- `internal/connectors/ibmi/codefori/protocol_test.go`
- `openspec/changes/add-codefori-companion-provider/tasks.md` (Slice 2 completion reconciliation)
- `openspec/changes/add-codefori-companion-provider/apply-progress.md`

## TDD Cycle Evidence — Slice 2

| Task | Test file | Layer | Safety net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|---|
| Slice 2 RED strict bounded connector contract | `client_test.go`, `protocol_test.go` | Unit with fake `http.RoundTripper` | N/A (new connector package; initial scoped package lookup confirmed no prior package) | `go test -count=1 ./internal/connectors/ibmi/codefori` failed to build because `NewClient`, `Descriptor`, protocol types, and limits did not exist | Passed after the minimal fixed client, descriptor contract, and protocol adapter were added | Accepted canonical request and rejected invalid SQL; unknown, duplicate, and trailing JSON cases exercise distinct parser paths | Removed the unused descriptor error after focused tests stayed green |
| Slice 2 GREEN fixed bounded HTTP RPC | `client_test.go`, `protocol_test.go` | Unit with fake `http.RoundTripper` | N/A (new connector package) | Existing RED tests specified fixed URL, POST, bearer header, canonical SQL, strict response shape, and bounded outcomes before client code | Initial production run exposed strict-decoding failures; full scoped package test passed after strict object decoding, size checks, correlation, and timeout configuration | Version, generation, request-ID, malformed, oversized, status, and descriptor cases cover independent outcome paths; the non-Windows constructor reaches the tested nil-reader fail-closed behavior | Focused test passed after cleanup |
| Slice 2 TRIANGULATE provider result and unavailable mapping | `client_test.go`, `protocol_test.go` | Unit with fake `http.RoundTripper` | N/A (new connector package) | Oversized but syntactically valid envelope test failed because direct envelope decoding did not enforce the 1024-byte bound | Passed after `decodeEnvelope` enforced the response bound before parsing | Returned query results are decoded exactly and validated with `provider.ValidateQueryResult`; non-success result rows, invalid descriptor variants, transport failures, and correlation failures return bounded states | Focused test passed after cleanup |

## Slice 2 test commands and results

1. `go test -count=1 ./internal/connectors/ibmi/codefori` — initial RED: failed to build because connector production symbols did not exist.
2. `go test -count=1 ./internal/connectors/ibmi/codefori` — RED after initial production: strict unknown, duplicate, and trailing JSON tests failed, proving permissive decoding was insufficient.
3. `go test -count=1 ./internal/connectors/ibmi/codefori` — RED during triangulation: oversized syntactically valid envelope was accepted before the explicit response-length validation.
4. `go test -count=1 ./internal/connectors/ibmi/codefori` — GREEN/REFACTOR: passed (`ok bac-nexus/internal/connectors/ibmi/codefori`).
5. `go test -count=1 ./internal/provider ./internal/connectors/ibmi/codefori` — passed (`ok bac-nexus/internal/provider`; `ok bac-nexus/internal/connectors/ibmi/codefori`).
6. `go vet ./internal/provider ./internal/connectors/ibmi/codefori` — passed (exit 0; no diagnostics).
7. `go test -race -count=1 ./internal/provider ./internal/connectors/ibmi/codefori` — passed (`ok bac-nexus/internal/provider`; `ok bac-nexus/internal/connectors/ibmi/codefori`).

### Slice 2 Test Summary

- Total tests written: 12 top-level connector tests, plus table-driven subcases.
- Total tests passing: 12 top-level connector tests under the focused package command; provider and connector package checks pass together.
- Layers used: Unit with injected descriptor readers and fake `http.RoundTripper` (12).
- Approval tests: None — the connector package is new.
- Pure functions created: strict request/response envelope, normalized-result, session-state, and bounded-body parsers.

## Work Unit Evidence — Slice 2

| Evidence | Exact result |
|---|---|
| Focused test command and exact result | `go test -count=1 ./internal/provider ./internal/connectors/ibmi/codefori` exited 0: both packages reported `ok`. `go vet ./internal/provider ./internal/connectors/ibmi/codefori` exited 0 with no diagnostics. `go test -race -count=1 ./internal/provider ./internal/connectors/ibmi/codefori` exited 0: both packages reported `ok`. |
| Runtime harness command/scenario and exact result | N/A — Slice 4 has not created a runnable Companion broker, and this Slice 2 authorization forbids network, generated binaries/JARs, and IBM i execution. Fake `http.RoundTripper` tests exercise actual `http.Client` request construction and protocol handling without opening a socket. |
| Rollback boundary | Remove only `internal/connectors/ibmi/codefori/{client.go,protocol.go,descriptor.go,descriptor_other.go,client_test.go,protocol_test.go}` and revert the three Slice 2 task checkboxes/progress section. This removes the fixed-loopback provider without changing `internal/provider` or any Native behavior. |

## Slice 2 design deviations

None. The implementation uses no configurable endpoint, discovery, retry, fallback, profile, Native dependency, connector-generic transport, external network access, Windows execution, VS Code access, or IBM i access.

## Slice 2 workload and PR boundary

Single maintainer-approved `size:exception` work unit: fixed-loopback Go connector. Slice 2 contributes exactly 873 authored source/test additions and zero source/test deletions across its six permitted connector files; no commit, branch, push, PR, tag, release, or publication was created.

## Completed Slice 3 — isolated MCP and CLI selection

Completed the three implementation-owned Slice 3 tasks and marked each corresponding checkbox `[x]` in `tasks.md`:

- RED tests prove the Companion server registers exactly `session.status` and `sql.query`; the existing Native two-tool registration remains covered by its unchanged package test; no-profile selection dispatches only to Companion, profile selection only to Native, and contradictory or unknown selectors return before either composition is invoked.
- GREEN adds a distinct `CodeForIServer` and a Companion-only composition root. `nexus serve` now selects Companion without `-profile`; a nonblank profile uses the unchanged `runWithDeps` Native path. The optional `-provider` may only restate the profile-derived mode.
- TRIANGULATE uses in-memory MCP transport and a fake Companion provider to prove unavailable status/query results remain on the Companion surface, accepted formatting reaches the provider boundary as a normalized result, and a non-empty `session.status` input is rejected by the typed MCP schema. The Companion dependency input has only `Provider` and `ServerFactory`, and its source imports only the Companion connector, MCP adapter, and neutral provider port.

## Files changed — Slice 3

- `internal/mcp/codefori.go`
- `internal/mcp/codefori_test.go`
- `cmd/nexus/codefori.go`
- `cmd/nexus/main.go`
- `cmd/nexus/main_test.go`
- `openspec/changes/add-codefori-companion-provider/tasks.md` (Slice 3 completion reconciliation)
- `openspec/changes/add-codefori-companion-provider/apply-progress.md`

## TDD Cycle Evidence — Slice 3

| Task | Test file | Layer | Safety net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|---|
| Slice 3 RED isolated tools, selectors, and unavailable path | `internal/mcp/codefori_test.go`, `cmd/nexus/main_test.go` | In-memory MCP integration and unit | `go test -count=1 ./internal/mcp ./cmd/nexus` passed before modifications | `go test -count=1 ./internal/mcp ./cmd/nexus` failed to build because `NewCodeForI`, Companion request/output types, selector types, and composition dispatch symbols did not exist | Passed after the separate Companion MCP server, profile-derived selection, and isolated composition were added | In-memory transport verifies bounded unavailable results without a Native provider; selector table covers both valid modes, both contradictions, and an unknown selector | No further production refactor was needed; the final focused package checks passed |
| Slice 3 GREEN separate constructor and preserved Native path | `internal/mcp/codefori_test.go`, `cmd/nexus/main_test.go` | In-memory MCP integration and unit | Same passing baseline | Same absent-symbol RED run specified the constructor and `runServe` selection seam before implementation | Initial GREEN exposed only the obsolete expectation that no-profile serve must reject; updating that existing acceptance test to the approved no-profile Companion contract produced a passing run | The Companion factory observes exactly two Companion tools while the existing unchanged Native server test continues to assert its exact two Native tools | No further production refactor was needed; the final focused package checks passed |
| Slice 3 TRIANGULATE isolated construction and strict empty status input | `internal/mcp/codefori_test.go`, `cmd/nexus/main_test.go` | In-memory MCP integration and structural unit | Same passing baseline | New status-input test initially failed because the generic helper treats schema rejection as a fatal MCP call error; this confirmed the schema rejected the non-empty object | The test now directly asserts the MCP validation error, and passes without production changes | `companionDeps` permits only `Provider` and `ServerFactory`; no-profile dispatch never invokes Native composition and profile dispatch never invokes Companion composition | No further production refactor was needed; the final focused package checks passed |

## Work Unit Evidence — Slice 3

| Evidence | Exact result |
|---|---|
| Focused test command and exact result | `go test -count=1 ./internal/provider ./internal/connectors/ibmi/codefori ./internal/mcp ./cmd/nexus` exited 0: all four packages reported `ok`. `go vet ./internal/provider ./internal/connectors/ibmi/codefori ./internal/mcp ./cmd/nexus` exited 0 with no diagnostics. `go test -race -count=1 ./internal/provider ./internal/connectors/ibmi/codefori ./internal/mcp ./cmd/nexus` exited 0: all four packages reported `ok`. |
| Runtime harness command/scenario and exact result | `go test -count=1 ./internal/mcp` exercised the actual SDK in-memory MCP transport: Companion-only tool registration, status/query calls, unavailable outcomes, canonical query output, and typed rejection of non-empty status input all passed. No live runtime command applies because this slice must not contact a broker, VS Code, or IBM i; no generated Nexus binary or JAR was run. |
| Rollback boundary | Remove only `internal/mcp/codefori.go`, `internal/mcp/codefori_test.go`, `cmd/nexus/codefori.go`, and the Slice 3 portions of `cmd/nexus/main.go` and `cmd/nexus/main_test.go`, then restore the three Slice 3 checkboxes and this section. This restores the existing profile-required Native entry path without modifying Native composition or earlier provider/connector work. |

## Slice 3 design deviations

None. The Companion path depends only on the neutral provider port, fixed-loopback connector construction, and separate MCP adapter. It does not construct a profile, credential, eligibility, audit, ownership, recovery, SSH/SFTP, Mapepire, configuration, TUI, or Native validation dependency. Companion startup stays lazy: the nil descriptor reader produces bounded unavailable calls rather than loading Native dependencies or falling back.

## Slice 3 workload and PR boundary

Single maintainer-approved `size:exception` work unit: isolated MCP and CLI mode selection. Slice 3 adds 560 authored source/test lines and deletes 16 source/test lines (576 changed lines) across the five permitted source/test files. It remains within the session's 2,000-line review budget. The separate lifecycle-safe MCP facade and in-memory transport tests require this cohesive size; no code was compressed or tests removed to meet an earlier forecast. No commit, branch, push, PR, tag, release, publication, Windows execution, or live IBM i work was performed.

## Completed Slice 4 — TypeScript package, query, protocol, and broker

Completed the two implementation-owned Slice 4 tasks and marked each corresponding checkbox `[x]` in `tasks.md`:

- RED/GREEN adds an independently testable TypeScript package with `@halcyontech/vscode-ibmi-types` pinned exactly to `3.0.12`, a finite proof-query recognizer, strict protocol codec, and a fake-backed fixed-loopback broker scaffold.
- TRIANGULATE adds strict malformed/duplicate/unknown/trailing/oversized body cases, bearer authentication, version/generation/request-ID rejection, collision behavior, and sanitized non-success assertions without VS Code, Windows, IBM i, or a live network listener.

The broker has no endpoint setting, discovery, fallback, wildcard/`localhost` bind, generic RPC method, queue, adapter, descriptor, or Code for IBM i host access. Its only listener seam receives the fixed `127.0.0.1:64139` pair and is exercised by a fake server; the public Code for IBM i adapter and admission lifecycle remain exclusively in Slice 5.

## Files changed — Slice 4

- `companion/vscode-codefori/package.json`
- `companion/vscode-codefori/package-lock.json`
- `companion/vscode-codefori/eslint.config.mjs`
- `companion/vscode-codefori/tsconfig.json`
- `companion/vscode-codefori/src/extension.ts`
- `companion/vscode-codefori/src/query.ts`
- `companion/vscode-codefori/src/query.test.ts`
- `companion/vscode-codefori/src/protocol.ts`
- `companion/vscode-codefori/src/protocol.test.ts`
- `companion/vscode-codefori/src/broker.ts`
- `companion/vscode-codefori/src/broker.test.ts`
- `openspec/changes/add-codefori-companion-provider/tasks.md` (Slice 4 completion and handoff reconciliation)
- `openspec/changes/add-codefori-companion-provider/apply-progress.md`

## TDD Cycle Evidence — Slice 4

| Task | Test file | Layer | Safety net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|---|
| Slice 4 RED/GREEN package, recognizer, protocol, and broker | `src/query.test.ts`, `src/protocol.test.ts`, `src/broker.test.ts` | Unit with injected fixed-loopback server fake | N/A (new package) | `npm test` failed before production code because `query.js`, `protocol.js`, and `broker.js` did not exist; 3 suites could not load their absent modules | Focused command passed 7/7 tests after the finite recognizer, strict codec, and fixed broker were added | Added a distinct permitted formatting case, prohibited SQL forms, successful correlated response, collision, and unauthenticated response paths | Fixed the `ValidQueryResult` discriminated union after `npm run typecheck` identified optional-row access; focused tests, lint, typecheck, full test, and build passed |
| Slice 4 TRIANGULATE strict envelopes and sanitization | `src/query.test.ts`, `src/protocol.test.ts`, `src/broker.test.ts` | Unit with injected fixed-loopback server fake | N/A (new package) | Tests were written after the initial GREEN and before their first execution; the generalized implementation passed without a production change | Focused command passed 20/20 tests | Duplicate/unknown/trailing/incomplete/oversized bodies, semicolons/comments/Unicode/dot whitespace/extra tokens, invalid auth/version/generation/ID/path/method, handler failures, and token-free sanitized output cover independent paths | The type-only refactor above preserved all strict-path results under the focused and full commands |

## Work Unit Evidence — Slice 4

| Evidence | Exact result |
|---|---|
| Focused test command and exact result | `npm test -- --run src/query.test.ts src/protocol.test.ts src/broker.test.ts` exited 0: 3 test files and 20 tests passed. |
| Runtime harness command/scenario and exact result | N/A — this Slice is explicitly fake-only and prohibits a live network listener, VS Code host, Windows ACL operation, and IBM i access. The focused command exercised the fixed-host/port broker lifecycle through an injected fake listener, including one collision and authenticated request handling, without opening a socket. |
| Rollback boundary | Remove only `companion/vscode-codefori/{package.json,package-lock.json,eslint.config.mjs,tsconfig.json,src/extension.ts,src/query.ts,src/query.test.ts,src/protocol.ts,src/protocol.test.ts,src/broker.ts,src/broker.test.ts}` and revert the two Slice 4 checkboxes/handoff and this progress section. This removes the TypeScript scaffold without changing Go work, Native behavior, Windows descriptor work, or future adapter/admission files. |

## Slice 4 command outcomes and dependency lock

1. `npm install --package-lock-only --ignore-scripts --no-audit --no-fund` initially failed with `ERESOLVE`: `typescript-eslint@8.38.0` requires TypeScript `<5.9.0`, while the initial package declared `typescript@5.9.2`. The package was corrected to the compatible exact `typescript@5.8.3`; the lockfile creation command then exited 0.
2. `npm ci` exited 0 and installed 176 packages from the lockfile. npm emitted an `eslint@9.31.0` deprecation warning, reported one critical audit finding, and reported the transitive `esbuild@0.28.2` postinstall as not covered by the environment's allow-scripts policy; no audit remediation or script approval was performed.
3. `npm run lint` exited 0.
4. `npm run typecheck` initially failed with `TS18048` because the validated success-row union still exposed an optional `rows` property. The discriminated union was corrected; the final `npm run typecheck` exited 0.
5. `npm test` exited 0: 3 test files and 20 tests passed.
6. `npm run build` exited 0. Generated `dist/` output was removed after verification and is not part of the change.

The reproducible lockfile resolves exact top-level development dependencies: `@halcyontech/vscode-ibmi-types@3.0.12`, `@eslint/js@9.31.0`, `@types/node@24.3.0`, `eslint@9.31.0`, `typescript-eslint@8.38.0`, `typescript@5.8.3`, and `vitest@3.2.4`.

## Slice 4 design deviations

None. The extension entrypoint is deliberately a `companion_unavailable` scaffold because descriptor-backed activation is reserved for Slice 6; no public Code for IBM i adapter is implemented before Slice 5.

## Slice 4 workload and PR boundary

Single maintainer-approved `size:exception` work unit: TypeScript package, finite query recognizer, strict protocol, and fake-backed fixed-loopback broker. The source/test/config files total 825 authored additions and zero deletions; the generated npm lockfile adds 3,344 reproducibility-only lines, for 4,169 raw added lines. No commit, branch, push, PR, tag, release, publication, Windows execution, live network listener, VS Code host, or live IBM i work was performed.

## Slice 4 native-runtime attempt settlement

- Attempt work unit: `typescript-broker-scaffold`.
- The attempt settled with passed evidence after `npm ci`, `npm run lint`, `npm run typecheck`, the focused 20-test command, the full 20-test command, and `npm run build` succeeded.
- Native accounting recorded 4,169 changed lines: 825 authored source/test/config lines plus 3,344 generated `package-lock.json` lines.
- This exceeded the 2,000-line objective and yielded a maintainer decision requirement.
- The already-approved maintainer `size:exception` authorized the reset; the reset completed successfully, preserved the historical passed evidence, and cleared the active objective.
- Current native state after reset: no active attempt; next action: begin.

## Test commands and results

1. `go test -count=1 ./internal/provider` — RED: failed to build because `QueryRequest`, `QueryResult`, `SessionStatusResult`, `CanonicalProofQuery`, and related production symbols were undefined.
2. `go test -count=1 ./internal/provider` — RED after completing the test set: failed to build for the same absent production symbols.
3. `gofmt -w internal/provider/provider.go internal/provider/provider_test.go && go test -count=1 ./internal/provider` — initial GREEN exposed an implementation defect: rejected trailing tokens returned a canonical string with `false`.
4. `gofmt -w internal/provider/provider.go && go test -count=1 ./internal/provider` — GREEN: passed (`ok bac-nexus/internal/provider`).
5. `gofmt -w internal/provider/provider.go internal/provider/provider_test.go && go test -count=1 ./internal/provider` — REFACTOR: passed (`ok bac-nexus/internal/provider`).

No broad tests, generated Nexus/JAR work, IBM i, network, commit, publication, or lifecycle command was run. Process inspection found no remaining `go test`, `go tool`, or `go build` process.

### Test Summary

- Total tests written: 5 top-level unit tests.
- Total tests passing: 5 top-level unit tests under `go test -count=1 ./internal/provider`.
- Layers used: Unit (5).
- Approval tests: None — this is a new package.
- Pure functions created: 4 (`CanonicalizeQuery`, `ValidateQueryResult`, and two private recognizer helpers).

## Design deviations

None. The service is transport-neutral and only invokes `Provider.Query` after canonicalization and pre-start context validation. It suppresses a malformed provider result as `failed` and a cancelled context as `cancelled`.

## Workload and PR boundary

Feature Branch Chain, child 1: provider-neutral domain contract, targeting draft tracker `feature/add-codefori-companion-provider`. The two source files are new and total exactly 300 added source lines with zero deletions, meeting the delegated Slice 1 hard cap. No commit or publication was created.

## Structured status

```yaml
schemaName: gentle-ai.sdd-status v2
changeName: add-codefori-companion-provider
artifactStore: openspec
planningHome:
  root: /home/david-gutierrez/Desarrollo/Repositorios/bac-nexus/openspec
  changesDir: openspec/changes
changeRoot: openspec/changes/add-codefori-companion-provider
artifacts:
  proposal: done
  specs: done
  design: done
  tasks: done
  applyProgress: done
  verifyReport: missing
  syncReport: missing
taskProgress:
  total: 28
  complete: 20
  remaining: 8
applyState: ready
dependencies:
  apply: ready
  verify: ready
  sync: blocked
  archive: blocked
actionContext:
  mode: repo-local
  workspaceRoot: /home/david-gutierrez/Desarrollo/Repositorios/bac-nexus
  allowedEditRoots:
    - /home/david-gutierrez/Desarrollo/Repositorios/bac-nexus
  warnings:
    - Slice 4 was restricted to the fake-backed TypeScript package/query/protocol/broker files and task/progress reconciliation required by the apply contract.
nextRecommended: sdd-apply
isNonAuthoritative: false
```

## Cumulative task state

- [x] **RED:** Prove no descriptor read or HTTP call occurs for invalid SQL; prove strict version/generation/request-ID correlation, bearer auth, unknown/duplicate/trailing JSON rejection, 512-byte request and 1024-byte response limits, one-second status, six-second-or-greater response-header timeout, seven-second query total, no retry, and token-free errors. <!-- sdd-owner: implementation -->
- [x] **GREEN:** Implement the fixed `127.0.0.1:64139` `POST /v1/rpc` client and protocol adapter without endpoint configuration, discovery, fallback, or generic transport abstraction. <!-- sdd-owner: implementation -->
- [x] **TRIANGULATE:** Validate all returned normalized results through `internal/provider`; map missing, malformed, stale, denied, unreachable, mismatched, and unsupported-platform state to bounded results. <!-- sdd-owner: implementation -->
- [x] **RED:** Prove Companion exposes exactly `session.status` and `sql.query`, Native retains exactly its existing two tools, contradictory selectors fail before dependency construction, and unavailable calls never switch providers. <!-- sdd-owner: implementation -->
- [x] **GREEN:** Add a separate Companion MCP constructor and composition root; change only deterministic selection/help text in `main.go`; preserve the existing profile-selected `runWithDeps` path. <!-- sdd-owner: implementation -->
- [x] **TRIANGULATE:** Prove Companion mode constructs no profile, credential, eligibility, audit, ownership, recovery, SSH/SFTP, Mapepire, configuration, TUI, or Native validation dependency. <!-- sdd-owner: implementation -->
- [x] **RED/GREEN:** Pin `@halcyontech/vscode-ibmi-types` to 3.0.12; independently recognize only the proof query; bind only `127.0.0.1:64139`; accept only bounded authenticated `POST /v1/rpc`; reject collision without scan/fallback; and expose no endpoint setting or generic RPC method. <!-- sdd-owner: implementation -->
- [x] **TRIANGULATE:** Prove strict auth/version/generation/ID/body handling and sanitized non-success output without VS Code or IBM i. <!-- sdd-owner: implementation -->
- [x] **RED/GREEN:** Use only `exports.instance`, `getConnection`, `subscribe`, and `runSQL`; treat `subscribe` as `void`; call `runSQL` only with canonical SQL and `{rows: 1}`; normalize one arbitrary raw column label to `value`; suppress raw errors and malformed/late/disconnected results. <!-- sdd-owner: implementation -->
- [x] **RED/GREEN:** Implement immediate capacity 16 with no waiting/worker queue and no global SQL mutex. Admit at least ten barrier-held fakes, complete them in reverse order, and prove exact correlation. <!-- sdd-owner: implementation -->
- [x] **TRIANGULATE:** Prove excess work returns `limit_exceeded`, pre-start cancellation never calls `runSQL`, and a timed-out started promise retains capacity until it settles. Keep evidence `not_validated_on_ibmi`. <!-- sdd-owner: implementation -->
- [x] **RED:** Prove absolute inbox PowerShell resolution, `shell: false`, fixed scripts/arguments, minimal environment, bounded stdio/deadlines, readiness-before-RNG, stdin-only secret transfer, fixed output, no retry, sanitized errors, and generation-owned cleanup. <!-- sdd-owner: implementation -->
- [x] **GREEN:** Create/validate only the fixed LocalApplicationData descriptor path. For a new directory use `Directory.CreateDirectory(path, DirectorySecurity)`; for an existing directory validate it unchanged before `READY`. Create the descriptor with a protected current-SID-only DACL before writing, and fail closed if selected file semantics cannot be evidenced. <!-- sdd-owner: implementation -->
- [x] **TRIANGULATE:** Keep the operation private and descriptor-specific; reject any caller-controlled executable, command, script, path, arguments, or environment. <!-- sdd-owner: implementation -->
- [x] **PREREQUISITE:** Prepare `.github/workflows/release-companion.yml` with a `windows-latest` validation job that runs the scoped Go consumer tests and opt-in TypeScript token-store Windows tests, uses only `contents: read`, no secrets, publication, or IBM i, bounded logs/artifacts, and leaves `.github/workflows/go-verification.yml` unchanged. <!-- sdd-owner: implementation -->
- [ ] Prove current-process SID owner, protected exact owner-only DACL, regular/non-reparse directory and file, 512-byte descriptor bound, version/generation checks, and no token use before Go validation. <!-- sdd-owner: implementation -->
- [ ] Prove clean creation, existing valid directory handling, no pre-`READY` secret, cross-user read denial, no readable transient file, and generation-owned cleanup. <!-- sdd-owner: implementation -->
- [ ] Prove wrong-owner, inherited, extra-ACE, permissive, reparse, oversized, locked, malformed, and wrong-generation residue keeps the broker unavailable without repair or Native fallback. <!-- sdd-owner: implementation -->
- [ ] Record the evidence honestly as Windows ACL validation only; do not claim extension-host identity or IBM i behavior. <!-- sdd-owner: implementation -->
- [ ] Add ordinary offline verification for the Companion and validate `companion-v*` SemVer tags, with first intended tag `companion-v0.1.0`. <!-- sdd-owner: implementation -->
- [ ] Add a publish job that runs only for a valid `companion-v*` tag when repository variable `COMPANION_PUBLISH_ENABLED == 'true'`, uses protected environment `companion-marketplace`, grants `id-token: write` only in that job, uses current `vsce` OIDC, and accepts no Marketplace PAT. The unset/default state must be inert. <!-- sdd-owner: implementation -->
- [ ] Add contract tests proving ordinary CI cannot publish, non-Companion tags cannot publish, the enable variable and protected environment are required, no PAT name is present, and existing Go verification remains independent. <!-- sdd-owner: implementation -->
- [ ] Document native-Windows install order, one broker per user, the status/proof-query flow, ACL/cross-user evidence recording without secrets, unsupported multi-window/WSL/SSH/container/forwarding topologies, `not_validated_on_ibmi`, and rollback. <!-- sdd-owner: implementation -->

## Completed Slice 5 — public adapter and immediate admission

Completed the three implementation-owned Slice 5 tasks and marked each corresponding checkbox `[x]` in `tasks.md`:

- RED/GREEN adds a public Code for IBM i adapter seam that uses only `exports.instance`, `getConnection`, `subscribe`, and `runSQL`. It treats `subscribe` as `void`, calls `runSQL` only with the canonical proof SQL and `{ rows: 1 }`, discards an arbitrary raw label, and returns only normalized `value` rows or fixed failure states.
- RED/GREEN adds immediate admission with the fixed capacity of 16, no queue, worker, or global mutex. A fake-backed broker test admits ten operations through the real broker handler, resolves them in reverse order, and verifies each request ID receives its own normalized response.
- TRIANGULATE proves `limit_exceeded` for the seventeenth operation, pre-start abort prevents work invocation, and a started promise that timed out retains its capacity until its underlying promise settles. Evidence remains `not_validated_on_ibmi`.

## Files changed — Slice 5

- `companion/vscode-codefori/src/codeforiAdapter.ts`
- `companion/vscode-codefori/src/codeforiAdapter.test.ts`
- `companion/vscode-codefori/src/admission.ts`
- `companion/vscode-codefori/src/admission.test.ts`
- `companion/vscode-codefori/src/broker.ts`
- `companion/vscode-codefori/src/broker.test.ts`
- `openspec/changes/add-codefori-companion-provider/tasks.md` (Slice 5 completion and handoff reconciliation)
- `openspec/changes/add-codefori-companion-provider/apply-progress.md`

## TDD Cycle Evidence — Slice 5

| Task | Test file | Layer | Safety net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|---|
| Slice 5 public adapter | `src/codeforiAdapter.test.ts` | Unit with public-API-shaped fakes | `npm test -- --run src/broker.test.ts` exited 0: 1 file, 5 tests passed | `npm test -- --run src/codeforiAdapter.test.ts src/admission.test.ts` failed because `codeforiAdapter.js` and `admission.js` did not exist | Focused adapter/admission command exited 0: 2 files, 8 tests passed after the minimum adapter and admission implementation | Canonical formatting, rejected SQL, arbitrary labels, raw errors, malformed multi-column/symbol-column rows, and disconnected late results exercise independent paths | Added own-enumerable symbol-column rejection; its first focused run failed with an incorrectly accepted row, then passed after strict enumerable-key normalization |
| Slice 5 immediate capacity and correlation | `src/admission.test.ts`, `src/broker.test.ts` | Unit with fake public connection and injected fixed-loopback server | Same 5/5 broker baseline | Same absent-module RED run specified the admission class and adapter surface | Focused adapter/admission command exited 0: 2 files, 8 tests passed | Broker integration admits 10 barrier-held fakes, completes in reverse order, and asserts exact request ID/value pairs; capacity, pre-start abort, and timeout settlement use distinct paths | Exported the narrow broker handler seam; final 3-file focused command exited 0: 3 files, 14 tests passed |
| Slice 5 bounded failure lifecycle | `src/admission.test.ts`, `src/codeforiAdapter.test.ts` | Unit with deferred promise and abort-controller fakes | Same 5/5 broker baseline | Same absent-module RED run | Focused adapter/admission command exited 0: 2 files, 8 tests passed | Seventeenth admission, cancellation before work, timeout after work start, settlement-based release, raw rejection, and disconnect generation change cover distinct outcomes | No further production refactor was needed; final focused command remained green |

## Work Unit Evidence — Slice 5

| Evidence | Exact result |
|---|---|
| Focused test command and exact result | `npm test -- --run src/codeforiAdapter.test.ts src/admission.test.ts src/broker.test.ts` exited 0: 3 test files and 14 tests passed. |
| Runtime harness command/scenario and exact result | N/A — Slice 5 is explicitly offline and prohibits VS Code-host, live-network, and IBM i execution. The injected fixed-loopback server and public-API-shaped Code for IBM i fakes exercise actual broker request handling, ten concurrent admissions, reverse settlement, correlation, timeout, cancellation, and disconnect suppression without opening a socket. Evidence remains `not_validated_on_ibmi`. |
| Rollback boundary | Remove only `companion/vscode-codefori/src/{codeforiAdapter.ts,codeforiAdapter.test.ts,admission.ts,admission.test.ts}` and the Slice 5 portions of `broker.ts` and `broker.test.ts`, then restore the three Slice 5 task checkboxes and this progress section. This restores the Slice 4 fake broker without changing Go, Native behavior, Windows descriptor work, workflow, or documentation. |

## Slice 5 API-contract evidence

- The installed exact package is `@halcyontech/vscode-ibmi-types@3.0.12`. Its `typings.d.ts` declares `CodeForIBMi.instance`; `Instance.d.ts` declares `getConnection()` and `subscribe(...): void`; and `api/IBMi.d.ts` declares `runSQL(statements, options)` with `rows?: number`.
- The adapter exposes a minimal structural seam matching only those public members. It calls `codeForI?.instance`, registers `connected` and `disconnected` without storing a subscription result, and invokes only `connection.runSQL(canonicalSQL, { rows: 1 })`.
- No unsubscribe/dispose behavior, raw `CURRENT_USER` label, generic SQL, command execution, endpoint discovery, VS Code-host execution, or IBM i behavior is claimed.

## Slice 5 command outcomes and delivery blocker

1. `npm test -- --run src/broker.test.ts` safety-net baseline exited 0: 1 test file and 5 tests passed.
2. `npm test -- --run src/codeforiAdapter.test.ts src/admission.test.ts` RED exited 1 because both production modules were absent.
3. The first GREEN run found an invalid test deferred helper and then a release-turn assertion; after test-helper correction, the focused adapter/admission command exited 0: 2 files and 8 tests passed.
4. A stricter own-enumerable-column triangulation test initially failed because `Object.keys` ignored an enumerable symbol property; `normalizeRows` now rejects every raw row except exactly one own enumerable string column, and the 3-file focused command exited 0: 3 files and 14 tests passed.
5. `npm ci` exited 0 and installed 176 packages. It reported one critical audit finding and the existing `esbuild@0.28.2` postinstall allow-scripts warning; no audit remediation or script approval was performed.
6. `npm run lint` exited 0; `npm run typecheck` exited 0; `npm test` exited 0: 5 files and 29 tests passed; `npm run build` exited 0; and the final focused 3-file command exited 0: 14 tests passed. Generated `dist/` output was removed after verification.

The user explicitly deferred the critical `GHSA-5xrq-8626-4rwp` advisory affecting `vitest@3.2.4`. It remains a pre-delivery blocker and was not remediated in this Slice; no `npm audit fix` was run.

## Slice 5 design deviations

None. The adapter stays a public-API seam; descriptor-backed activation is reserved for Slice 6. The isolated adapter and admission fakes prove local mechanics only and retain `not_validated_on_ibmi`.

## Slice 5 workload and PR boundary

Single maintainer-approved `size:exception` work unit: public Code for IBM i adapter and immediate admission lifecycle. Slice 5 adds 502 authored source/test lines and deletes zero source/test lines: 413 lines in the four new adapter/admission files, 12 targeted broker implementation lines, and 77 targeted broker-test lines. No commit, branch, push, PR, tag, release, publication, Windows execution, live network listener, VS Code host, or IBM i work was performed.

## Current handoff

- Cumulative task state: 20/28 complete; 8 remain.
- Next action: await separate commit/push authorization before executing the Slice 7 native-Windows consumer and ACL gates.
- Slice 7 must not weaken the private descriptor-operation boundary or claim Windows/IBM i evidence before its dedicated native tests and approval gates pass.

## Completed Slice 6 — fixed Windows descriptor operations

Completed the three implementation-owned Slice 6 tasks and marked each corresponding checkbox `[x]` in `tasks.md`:

- RED tests prove one absolute inbox PowerShell executable, `shell: false`, fixed scripts and arguments, the two-variable environment, bounded stdio/deadline behavior, exact outputs, readiness-before-RNG, stdin-only descriptor transfer, sanitization, no publication retry, and generation-owned cleanup.
- GREEN adds private descriptor-specific publish/cleanup operations only. The publish script derives the one `LocalApplicationData\\BAC Nexus\\companion-v1\\descriptor.json` path internally, creates a new directory with `Directory.CreateDirectory(path, DirectorySecurity)`, validates either resulting or existing directory unchanged before `READY`, and creates the descriptor with a protected current-SID-only DACL before writing secret bytes.
- TRIANGULATE proves cleanup receives only the generated generation, cleans after failed publication without retrying publication, rejects child stderr and malformed/oversized output, and keeps the scripts parameter-free. The extension lifecycle remains intake-disabled: no broker starts and no native Windows activation claim is made in this slice.

## Files changed — Slice 6

- `companion/vscode-codefori/src/windows/tokenStore.ts`
- `companion/vscode-codefori/src/windows/tokenStore.test.ts`
- `companion/vscode-codefori/src/windows/publish.ps1`
- `companion/vscode-codefori/src/windows/cleanup.ps1`
- `openspec/changes/add-codefori-companion-provider/tasks.md` (Slice 6 completion and handoff reconciliation)
- `openspec/changes/add-codefori-companion-provider/apply-progress.md`

## TDD Cycle Evidence — Slice 6

| Task | Test file | Layer | Safety net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|---|
| Slice 6 RED immutable process/secret contract | `src/windows/tokenStore.test.ts` | Unit with fake child process and filesystem reads | `npm test -- --run src/broker.test.ts` exited 0: 1 file, 6 tests passed | `npm test -- --run src/windows/tokenStore.test.ts` exited 1 because `tokenStore.js` did not exist | Initial four token-store behavior tests passed after the fixed operation implementation | Added malformed output, bounded stderr/deadline, exact stderr rejection, fixed cleanup, and unavailable-host cases; final focused command passed 8/8 | No further refactor was needed; focused test, lint, and typecheck stayed green |
| Slice 6 GREEN protected descriptor publication | `src/windows/tokenStore.test.ts` | Unit with source-contract filesystem read and fake child process | N/A (new Windows files) | The static file-semantics assertion failed because the publish script had no descriptor-file reparse check | Passed after pre-write normal/non-reparse file validation was added beside protected DACL handle validation | New-directory create-with-security, protected DACL setup, exact access rule validation, create-new file semantics, and owned cleanup are all asserted without executing PowerShell | No further refactor was needed; final focused and full Companion tests passed |
| Slice 6 TRIANGULATE private cleanup and sanitization | `src/windows/tokenStore.test.ts` | Unit with fake child process | N/A (new Windows files) | The post-publication cleanup test failed because a failed publish did not invoke cleanup | Passed after the failed-publish path invoked only the fixed cleanup operation with its generated generation | A subsequent stderr-success-shape test failed until any stderr became a fail-closed sanitized result; no raw child text is returned | Removed no behavior; the final branch remains two fixed operations with no caller-controlled process values |

## Work Unit Evidence — Slice 6

| Evidence | Exact result |
|---|---|
| Focused test command and exact result | `npm test -- --run src/windows/tokenStore.test.ts` exited 0: 1 test file and 8 tests passed. `npm run lint` exited 0. `npm run typecheck` exited 0. |
| Runtime harness command/scenario and exact result | N/A — this Linux-host Slice explicitly forbids live PowerShell/Windows ACL execution. The focused fake-child harness exercised the actual Node process configuration, readiness sequencing, descriptor stdin payload, deadline termination, cleanup sequencing, and sanitization; it did not invoke PowerShell, VS Code, network, or IBM i. |
| Rollback boundary | Remove only `companion/vscode-codefori/src/windows/{tokenStore.ts,tokenStore.test.ts,publish.ps1,cleanup.ps1}` and restore the three Slice 6 task checkboxes and this section. This removes descriptor publication/cleanup proof code without changing the broker, adapter/admission lifecycle, Native behavior, workflow, documentation, or future Windows consumer gates. |

## Slice 6 fixed-operation security evidence

- The executable is derived only as `SystemRoot\\System32\\WindowsPowerShell\\v1.0\\powershell.exe`; both script paths are absolute inbox module-relative paths. `shell` is false, arguments are fixed, `windowsHide` is true, and the child environment contains only `SystemRoot` and `LOCALAPPDATA`.
- Publish accepts no caller value. It waits for exactly `READY\\n` before drawing generation/token bytes, then sends the bounded descriptor only over stdin. Neither secret is placed in argv, environment, stdout, stderr, returned errors, or fixed diagnostics.
- Both operations require their exact fixed output, use 1,000 ms process/deadline bounds with 64-byte stdout and 256-byte stderr caps, reject all stderr, terminate on uncertainty, and never retry publication. A failed publish after descriptor generation invokes only its generation-owned cleanup path.
- The PowerShell scripts take no parameters and derive only the fixed descriptor location. `publish.ps1` applies protected current-SID-only directory security at creation, validates existing directory owner/DACL/type/reparse state before `READY`, and uses protected `CreateNew` file security plus pre-write handle/type/reparse checks. `cleanup.ps1` deletes only when the persisted descriptor generation equals its stdin generation.
- This is fake-backed source/process evidence only. No actual Windows ACL, cross-user denial, transient-file, extension-host identity, VS Code, or IBM i behavior is claimed; `not_validated_on_ibmi` remains unchanged.

## Slice 6 command outcomes and delivery blocker

1. `npm ci` exited 0 and installed 176 packages. npm reported one critical audit finding and the existing `esbuild@0.28.2` postinstall allow-scripts warning; no audit remediation, script approval, or non-ordinary network access was performed.
2. `npm test -- --run src/broker.test.ts` safety-net baseline exited 0: 1 file and 6 tests passed.
3. The initial token-store RED command exited 1 because `tokenStore.js` did not exist. The generated-descriptor cleanup RED and stderr sanitization RED assertions also failed before their corresponding minimal production changes.
4. Final focused token-store command exited 0: 1 file and 8 tests passed. `npm run lint` and `npm run typecheck` each exited 0.
5. `npm test` exited 0: 6 files and 37 tests passed. `npm run build` exited 0; generated `dist/` output was removed after verification.

The user-deferred critical `GHSA-5xrq-8626-4rwp` advisory affecting `vitest@3.2.4` remains a pre-delivery blocker. It was not remediated, and no `npm audit fix` was run.

## Slice 6 design deviations

None. The private operation intentionally remains unwired from broker activation because native Windows producer/consumer and ACL proof belongs exclusively to Slice 7. Intake remains disabled rather than falling back or weakening descriptor security.

## Slice 6 workload and PR boundary

Single maintainer-approved `size:exception` work unit: private fixed Windows descriptor operations. Slice 6 adds 605 authored source/test/script lines and no source/test/script deletions across the four permitted Windows files; the task/progress reconciliation is additional SDD artifact text. No commit, branch, push, PR, tag, release, publication, Windows execution, VS Code host, live network listener, or IBM i work was performed.

## Slice 7 preparation — native Windows consumer and ACL gates deferred

Linux preparation added a fail-closed Windows-only descriptor consumer and native-Windows test surfaces. The local workflow-harness prerequisite is complete; the four original Slice 7 native-evidence tasks remain unchecked because native Windows hosted/self-hosted execution, ACL inspection, cross-user denial, and transient-file evidence have not run.

### Prepared files

- `internal/connectors/ibmi/codefori/descriptor_windows.go` adds `NewPlatformClient`/`NewWindowsClient`, validates the fixed LocalApplicationData descriptor directory and file before any descriptor read, verifies the current process SID owner, protected exact one-owner DACL, expected inheritance flags, regular/non-reparse type, handle-bound file DACL, 512-byte bound, strict version, generation, and token shape, and rejects duplicate/unknown/trailing JSON.
- `internal/connectors/ibmi/codefori/descriptor_windows_test.go` prepares Windows-only table cases for wrong-owner, inherited, extra-ACE, permissive, reparse, locked, oversized, malformed, wrong-version, wrong-generation, duplicate, and trailing descriptor residue. It proves through an injected reader that no token bytes are read until both directory and file validation pass.
- `internal/connectors/ibmi/codefori/descriptor_other.go` and `cmd/nexus/codefori.go` are minimal composition wiring: Windows selects the guarded reader, while other platforms retain deterministic unavailable behavior.
- `companion/vscode-codefori/src/windows/tokenStore.windows.test.ts` prepares opt-in (`CODEFORI_WINDOWS_NATIVE_TESTS=1`) native Windows producer checks for clean creation, valid existing-directory reuse, protected owner-only DACLs, regular/non-reparse descriptor shape, 512-byte bound, and generation-owned cleanup. Its cross-user test is separately opt-in (`CODEFORI_WINDOWS_CROSS_USER_TEST=1`) and intentionally requires a runner-provided second ordinary-user probe; it supplies no account, credential, fallback, or ACL-repair mechanism.

### TDD Cycle Evidence — Slice 7 preparation

| Task | Test file | Layer | Safety net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|---|
| Consumer descriptor ACL gate | `internal/connectors/ibmi/codefori/descriptor_windows_test.go` | Windows unit/native boundary | New Windows-only files; existing `go test -count=1 ./cmd/nexus` passed before minimal composition wiring | `GOOS=windows GOARCH=amd64 go test -c -o /tmp/opencode/codefori-windows.test.exe ./internal/connectors/ibmi/codefori` failed because `maxDescriptorBytes`, `newWindowsDescriptorReader`, and `windowsDescriptorSource` were undefined | Cross-compilation of the same Windows test binary exited 0 after the fail-closed reader was added; binary was removed without execution | Valid descriptor plus 15 unsafe-residue and strict-JSON cases cover validation/read ordering, owner/DACL residue classes, bounds, syntax, and generation/version paths | Handle-bound read validation prevents a path-only file check from authorizing token bytes; native execution remains required |
| Producer Windows ACL gate | `companion/vscode-codefori/src/windows/tokenStore.windows.test.ts` | Native Windows integration, opt-in | Existing fake process suite remains covered by the focused command | Native test cases were written for the fixed producer before any native execution claim; Linux cannot execute their PowerShell/ACL path | Prepared only: Linux focused command discovered the file and skipped all three native-only cases | Clean/existing directory and generation cleanup are executable native cases; cross-user denial stays separately environment-gated to a runner-provided second-user harness | No TypeScript production change was needed; the test uses only the existing fixed token-store boundary and fixed PowerShell probe |

### Work Unit Evidence — Slice 7 preparation

| Evidence | Exact result |
|---|---|
| Focused test command and exact result | `go test -count=1 ./internal/connectors/ibmi/codefori ./cmd/nexus && go vet ./internal/connectors/ibmi/codefori ./cmd/nexus` exited 0; both Go packages reported `ok` and vet emitted no diagnostics. `GOOS=windows GOARCH=amd64 go vet ./internal/connectors/ibmi/codefori` exited 0 with no diagnostics. `GOOS=windows GOARCH=amd64 go test -c -o /tmp/opencode/codefori-windows.test.exe ./internal/connectors/ibmi/codefori` exited 0; `/tmp/opencode/codefori-windows.test.exe` was removed immediately and never executed. `npm test -- --run src/windows/tokenStore.test.ts src/windows/tokenStore.windows.test.ts` exited 0: 1 test file/8 tests passed and 1 native-only test file/3 tests skipped. `npm run lint` and `npm run typecheck` each exited 0. |
| Runtime harness command/scenario and exact result | N/A on Linux by explicit user decision. No Windows binary, PowerShell, ACL operation, VS Code host, IBM i, or network path ran. The native harness is prepared but not evidence: run its guarded producer/consumer/cross-user cases on a controlled native Windows hosted or self-hosted runner. |
| Rollback boundary | Revert only `internal/connectors/ibmi/codefori/{descriptor_windows.go,descriptor_windows_test.go,descriptor_other.go}`, the single `NewPlatformClient` wiring line in `cmd/nexus/codefori.go`, `companion/vscode-codefori/src/windows/tokenStore.windows.test.ts`, and this preparation section. This restores deterministic Companion unavailability on every platform without changing Native behavior, the private producer scripts, workflow, documentation, or prior slices. |

### Remaining native Windows evidence

- Run the Windows Go test binary and the opt-in TypeScript producer suite on a controlled native Windows runner; compilation is not runtime validation.
- Prove current-process SID owner, protected exact owner-only DACL, regular non-reparse directory/file, 512-byte descriptor limit, version/generation rejection, and no token use before consumer validation against the real Windows APIs.
- Prove clean and existing-directory producer behavior, no pre-`READY` secret, no readable transient file, cross-user read denial, and generation-owned cleanup.
- Exercise wrong-owner, inherited, extra-ACE, permissive, reparse, oversized, locked, malformed, and wrong-generation residue without repair, Native fallback, VS Code-host claims, or IBM i claims.

### Slice 7 preparation risks and blockers

- **Blocker:** native Windows hosted/self-hosted runtime evidence remains required before any of the four original Slice 7 native-evidence checkboxes can be marked complete or authenticated activation can be accepted.
- The cross-user test intentionally depends on a controlled runner-provided ordinary-user probe; this preparation does not create users, accept credentials, or weaken the ACL boundary to simulate it.
- `not_validated_on_ibmi` remains in force. No extension-host identity, VS Code behavior, IBM i behavior, or shared-job concurrency claim is added.
- The user-deferred critical `GHSA-5xrq-8626-4rwp` advisory affecting `vitest@3.2.4` remains a pre-delivery blocker. No audit remediation was performed.

### Slice 7 preparation workload and PR boundary

Single maintainer-approved `size:exception` work unit: native Windows consumer and ACL-gate preparation only. The workflow-harness prerequisite below adds no source or test code; the original four Slice 7 native-evidence checkboxes remain unchecked. Task progress is **20/28**, the current unit remains **Slice 7 native evidence**, and workflow execution remains blocked on separate commit/push authorization.

## Completed Slice 7 prerequisite — local Windows validation workflow

Created `.github/workflows/release-companion.yml` as a passive, least-privilege Windows Actions harness. It is triggered only by `workflow_dispatch`, pull requests, and branch pushes that affect the harness, scoped CodeForI connector, Companion package, or Go module files; it has no tag trigger.

- Workflow/default permission is only `contents: read`; checkout uses `persist-credentials: false` and shallow history.
- The sole `windows-latest` job has a 15-minute timeout and concurrency cancellation. It uses `actions/checkout@v7`, `actions/setup-go@v7`, and `actions/setup-node@v7` with explicit Go/module and Node/npm-lockfile inputs.
- The Go command is `go test -count=1 -timeout=2m ./internal/connectors/ibmi/codefori`.
- The lockfile-only dependency command is `npm ci --no-audit --no-fund`, and the native TypeScript command is `npm test -- --run src/windows/tokenStore.windows.test.ts` with the exact opt-in `CODEFORI_WINDOWS_NATIVE_TESTS=1`.
- No secret, `id-token: write`, write permission, Marketplace token, artifact upload, publication, release, package/VSIX step, SemVer tag route, IBM i, external service integration, or Slice 8 route is present. `.github/workflows/go-verification.yml` remains unchanged.

### TDD Cycle Evidence — Slice 7 prerequisite

| Task | Test file | Layer | Safety net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|---|
| Passive Windows workflow harness | None — this prerequisite adds configuration only | Local structural validation | N/A (new workflow) | Not claimed — no executable behavior or test was added | Pending native Windows run — deliberately not executed | Skipped — one static workflow declaration | None — focused configuration is already minimal |

No RED/GREEN cycle is fabricated: the user explicitly authorized passive workflow preparation only and prohibited local Windows runtime execution.

### Work Unit Evidence — Slice 7 prerequisite

| Evidence | Exact result |
|---|---|
| Focused test command and exact result | Structural readback confirmed the 77-line workflow declares only the scoped `windows-latest` job, `contents: read`, bounded timeouts, no tag trigger, `CODEFORI_WINDOWS_NATIVE_TESTS=1`, the scoped Go command, and lockfile-based `npm ci`. `git diff --check` exited 0. Referenced Go test, TypeScript test, PowerShell scripts, package files, `go.mod`, and `go.sum` all exist. No YAML parser/actionlint was already available; none was installed. |
| Runtime harness command/scenario and exact result | Pending — a real native Windows boundary exists. The workflow was not executed; no Windows binary, PowerShell, ACL operation, VS Code host, IBM i, external publication, commit, push, or tag ran. Execution remains blocked on separate commit/push authorization. |
| Rollback boundary | Remove only `.github/workflows/release-companion.yml`, restore the single Slice 7 prerequisite checkbox, and remove this prerequisite progress section/status reconciliation. This leaves prior Windows producer/consumer preparation, the four native-evidence tasks, all existing workflows, and all Slice 8 work untouched. |

### Slice 7 prerequisite risks and constraints

- Native Windows ACL, SID, cross-user, transient-file, and consumer evidence remains pending; this harness neither proves nor executes it.
- `not_validated_on_ibmi` remains in force; the workflow contains no IBM i access.
- The deferred critical `GHSA-5xrq-8626-4rwp` advisory for `vitest@3.2.4` remains a pre-delivery blocker and was not remediated.
- GitHub-hosted runner execution requires separate commit/push authorization; the orchestrator owns runtime-acquire settlement.

## Bounded remediation — native Windows token-store deadline

### Diagnosis

- Failed evidence revision: GitHub Actions run `34003682639` at commit `7ef09199b3ba25dcacde972860fb3f90b8ed4746`.
- The scoped native TypeScript command `npm test -- --run src/windows/tokenStore.windows.test.ts` failed the clean-creation and generation-cleanup tests at lines 68 and 97 because `createWindowsTokenStore().publish()` returned `undefined` after about 1003–1013 ms.
- `tokenStore.ts` applied the same fixed 1,000 ms deadline to the PowerShell child-process option and its fail-closed fallback timer. That budget is shorter than observed hosted-runner startup, so the test evidence supports correcting only this deadline.

### Correction

- Increased the private, fixed, non-user-configurable process deadline from 1,000 ms to 2,000 ms in both deadline paths by changing the shared `PROCESS_DEADLINE_MS` constant.
- Updated the fake process expectations to retain proof that publish and cleanup receive the exact same fixed 2,000 ms timeout and that timeout termination remains fail-closed.
- No stdio cap, ACL validation, secret lifecycle, fixed executable/script/arguments/environment, retry behavior, or generation-owned cleanup behavior changed.

### TDD Cycle Evidence — bounded remediation

| Task | Test file | Layer | Safety net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|---|
| Extend the fixed PowerShell deadline for native hosted-runner startup | `companion/vscode-codefori/src/windows/tokenStore.test.ts` | Unit with fake child process | `npm test -- --run src/windows/tokenStore.test.ts` exited 0: 1 file, 8 tests passed before the test change | After changing only the expected timeout and fake-timer advance to 2,000 ms, the focused command exited 1: 2 assertions expected 2,000 but observed 1,000 | After changing only `PROCESS_DEADLINE_MS`, the focused command exited 0: 1 file, 8 tests passed | Publish spawn, cleanup spawn, and fallback deadline use distinct assertions against the same immutable bound | No further refactor was needed; one shared constant keeps child and fallback deadlines aligned |

### Work Unit Evidence — bounded remediation

| Evidence | Exact result |
|---|---|
| Focused test command and exact result | `npm test -- --run src/windows/tokenStore.test.ts` exited 0: 1 test file and 8 tests passed. `npm run typecheck` exited 0. `npm run lint` exited 0. `git diff --check` exited 0. |
| Runtime harness command/scenario and exact result | A fresh native Windows run is required and was not launched by this remediation. The prior failing command was `npm test -- --run src/windows/tokenStore.windows.test.ts`; no native Windows, PowerShell, VS Code, IBM i, or live network success is claimed. Parent retains settlement of token `sha256:46cfad30cc31a43c9239fca84bb5dae721569e762f4f18ee6d621e839796d13a`. |
| Rollback boundary | Revert only the `PROCESS_DEADLINE_MS` value in `companion/vscode-codefori/src/windows/tokenStore.ts` and the corresponding 2,000 ms assertions in `companion/vscode-codefori/src/windows/tokenStore.test.ts`. This restores the prior deadline without changing unrelated Companion or Native behavior. |

### Native-evidence status after remediation

- The four Slice 7 native Windows evidence tasks remain unchecked. This remediation does not satisfy ACL, cross-user, transient-file, descriptor-consumer, extension-host, or IBM i validation.
- The cross-user test remains skipped. A fresh Windows validation run must repeat the scoped Go consumer and opt-in TypeScript producer checks before any native task can be marked complete.
- Remediation source/test delta is 4 additions and 4 deletions (8 changed lines) before this evidence append; it remains within the 120-line remediation cap when this progress entry is included.

## Bounded remediation — Windows pre-READY stdout handshake

### Superseding diagnosis

- Failed evidence revision: GitHub Actions run `34004033450` at commit `4da55d8242af38904cd301e08da151a659f95dc4` repeated the two native `publish()` failures after approximately 2005–2013 ms.
- This supersedes only the earlier startup-duration inference: the fixed 2,000 ms deadline is still reached because `publish.ps1` writes `READY\n` and immediately blocks in `ReadToEnd()` without flushing stdout.
- The corrected root cause is a bounded pre-READY handshake emission failure, not evidence that the deadline should be relaxed again. The fixed 2,000 ms deadline remains unchanged.

### Correction

- `publish.ps1` now performs the fixed `[Console]::Out.Flush()` immediately after writing `READY\n` and before reading descriptor stdin.
- The static focused test proves that exact write-flush-read order while retaining all protected directory/file, `CreateNew`, ownership, and cleanup assertions.
- The change emits no diagnostic, path, raw error, or secret; it does not alter ACL validation, readiness-before-RNG, stdin-only secret transfer, fixed process values, retry behavior, cleanup ownership, or bounds.

### TDD Cycle Evidence — pre-READY handshake remediation

| Task | Test file | Layer | Safety net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|---|
| Flush fixed PowerShell stdout before blocking descriptor input | `companion/vscode-codefori/src/windows/tokenStore.test.ts` | Unit/static source contract | `npm test -- --run src/windows/tokenStore.test.ts` exited 0: 1 file, 8 tests passed | Added the exact adjacent `READY` → `Flush` → `ReadToEnd` expectation; focused command exited 1: 1 assertion failed | Added one `[Console]::Out.Flush()` call; focused command exited 0: 1 file, 8 tests passed | Skipped: this is one fixed structural ordering with one valid output; existing test independently asserts security-sensitive script content | No further refactor was needed; the explicit call is the minimal fixed handshake correction |

### Work Unit Evidence — pre-READY handshake remediation

| Evidence | Exact result |
|---|---|
| Focused test command and exact result | `npm test -- --run src/windows/tokenStore.test.ts` exited 0: 1 test file and 8 tests passed. `npm run typecheck` exited 0. `npm run lint` exited 0. `git diff --check` exited 0. |
| Runtime harness command/scenario and exact result | Fresh native Windows evidence is required and was not launched. Parent owns the final runtime attempt and settlement token `sha256:37db2d7edfca670152200caa33a557786da1bda5b2c5c5d143708570ddde935d` for failed evidence revision `sha256:cb5f61d3a562e653e8d48121470879f1a045bd7340690d8356b50af2a3fb689b`. No native Windows, VS Code, IBM i, or live network success is claimed. |
| Rollback boundary | Revert only the `Out.Flush()` line in `companion/vscode-codefori/src/windows/publish.ps1` and its ordering assertion in `companion/vscode-codefori/src/windows/tokenStore.test.ts`. This removes the handshake correction without changing ACL, secret, timeout, or Native behavior. |

### Native-evidence status after final remediation

- The four Slice 7 native Windows evidence tasks remain unchecked, and the cross-user test remains skipped.
- This is the final authorized runtime correction attempt. A passing fresh Windows run must validate the scoped Go consumer and opt-in TypeScript producer checks before task completion can be considered.

## Maintainer-approved diagnostic objective — windows-fixed-stage-diagnostic

### Fixed transcript and interpretation

- `publish.ps1` emits and flushes only this ordered secret-free transcript: `PROCESS_ENTRY\nDIRECTORY_VALIDATED\nREADY\n`.
- The token store accepts only prefixes of that exact transcript across stdout chunk boundaries and writes descriptor stdin only after the complete transcript.
- Native tests retain only the last fixed stage and report it only when `publish()` returns unavailable: `none` means no complete stage, `process_entry` means before validated directory/ACL state, `directory_validated` means before READY, and `ready` means after the input handshake began.
- Production still returns only its existing bounded result; no stage reaches MCP, settings, telemetry, production logs, errors, or callers.

### TDD Cycle Evidence — fixed-stage diagnostic

| Task | Test file | Layer | Safety net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|---|
| Record only fixed pre-READY stages | `src/windows/tokenStore.test.ts`, `src/windows/tokenStore.windows.test.ts` | Fake process and native-test seam | Focused suite exited 0: 1 file, 8 tests passed | Added transcript, chunk, and last-stage assertions; focused suite exited 1: 6 tests failed; native failure-wrapper contract then exited 1: 1 test failed | Added fixed transcript parser, observer, script stages, and native failure wrapper; focused suite exited 0: 1 file, 9 tests passed | A split transcript and a non-prefix after `process_entry` prove chunk handling and sanitization independently | Tightened stage reporting to retain the completed valid prefix before rejecting the invalid suffix; all focused tests stayed green |

### Work Unit Evidence — fixed-stage diagnostic

| Evidence | Exact result |
|---|---|
| Focused test command and exact result | `npm test -- --run src/windows/tokenStore.test.ts` exited 0: 1 test file and 9 tests passed. `npm run typecheck` exited 0. `npm run lint` exited 0. `git diff --check` exited 0. |
| Runtime harness command/scenario and exact result | Not run locally. Parent owns the single workflow attempt and token `sha256:85f478f19e5e33f4270d5b7ca6819b9a797f00c4fd5394a484db4809a6a057ae` for failed evidence revision `sha256:7090a2bc2186bee3e917f01d0cc9bf989c0ae517736c058602cf750f899d994a`. |
| Rollback boundary | Revert only the fixed transcript/observer changes in `src/windows/tokenStore.ts`, the new `PROCESS_ENTRY` and `DIRECTORY_VALIDATED` write/flush pairs in `src/windows/publish.ps1`, the two focused tests, and this section. |

### Native-evidence status

- The four Slice 7 native Windows evidence tasks remain unchecked; this one-run diagnostic localizes failure and does not claim a fix.

## Authorized correction — windows-phase-specific-deadlines

### Fixed deadline model

- GitHub Actions run `34006280843` reached `none` on a cold first publish and `process_entry` before `directory_validated` on a warm second publish, proving the former global 2,000 ms limit cannot cover pre-secret Windows startup and SID/ACL validation.
- Publish now uses a fixed 10,000 ms pre-READY JavaScript deadline, resets to a fixed 2,000 ms deadline only after the complete exact transcript, and sets the fixed spawn hard cap to 12,000 ms.
- Cleanup uses a separate fixed 10,000 ms JavaScript and spawn bound. It receives only generation ownership input; no token handling, transcript, ACL rule, process value, retry, or public result behavior changed.

### TDD Cycle Evidence — phase-specific deadlines

| Task | Test file | Layer | Safety net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|---|
| Bound pre-READY, post-READY, hard-cap, and cleanup phases | `src/windows/tokenStore.test.ts` | Fake child-process unit | Focused suite exited 0: 1 file, 9 tests passed | Changed fixed timeout expectations and added a pre/post timer scenario; focused suite exited 1: 3 tests failed | Added the fixed phase deadlines and timer reset; focused suite exited 0: 1 file, 10 tests passed | The pre-READY 9,999 ms wait has no RNG or kill; the complete transcript resets to 2,000 ms and timeout triggers generation-owned cleanup | Added the cleanup fake required by the post-READY timeout path; final focused suite stayed green |

### Work Unit Evidence — phase-specific deadlines

| Evidence | Exact result |
|---|---|
| Focused test command and exact result | `npm test -- --run src/windows/tokenStore.test.ts` exited 0: 1 test file and 10 tests passed. `npm run typecheck` exited 0. `npm run lint` exited 0. `git diff --check` exited 0. |
| Runtime harness command/scenario and exact result | Not run locally. Parent owns the single Windows attempt token `sha256:e658aeb871f3e84f11d841d4cf5bec5a859f8fef2229a204894af82f2d9a55e9`; a passing settlement must remediate `sha256:9b65201bdf3d2ad37c3b0b854de1b256a180f5bbd3f3e95279d3d80f742f68d0`. |
| Rollback boundary | Revert only the phase deadline constants, `FixedOperation` deadline fields, timer reset, and corresponding focused test expectations in `src/windows/tokenStore.{ts,test.ts}`, plus this section. |

### Native-evidence status

- The four Slice 7 native Windows evidence tasks remain unchecked until the remote scoped Go consumer and opt-in TypeScript producer evidence passes.

## Authorized harness correction — windows-native-test-timeout-alignment

- GitHub Actions run `34006973636` was preempted by Vitest's default 5,000 ms per-test timeout, before the 10,000 ms pre-READY and 12,000 ms production bounds could be evaluated.
- The two active native producer tests now use the fixed test-only `NATIVE_TEST_TIMEOUT_MS = 60_000`. This exceeds the first test's bounded 44,000 ms sequence (publish 12s, cleanup 10s, publish 12s, afterEach cleanup 10s) with bounded runner overhead; the skipped cross-user contract is unchanged.

### TDD Cycle Evidence — native timeout alignment

| Task | Test file | Layer | Safety net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|---|
| Align active native test timeout with bounded production sequence | `src/windows/tokenStore.test.ts`, `src/windows/tokenStore.windows.test.ts` | Static harness contract | Focused suite exited 0: 1 file, 10 tests passed | Added static expectations for the fixed 60,000 ms declaration and active-test use; focused suite exited 1: 1 test failed | Added the fixed constant to both active tests; focused suite exited 0: 1 file, 10 tests passed | The 44,000 ms composed bound establishes that 15,000 ms is insufficient while 60,000 ms remains bounded | No refactor needed |

### Work Unit Evidence — native timeout alignment

| Evidence | Exact result |
|---|---|
| Focused test command and exact result | `npm test -- --run src/windows/tokenStore.test.ts` exited 0: 1 test file and 10 tests passed. `npm run typecheck` exited 0. `npm run lint` exited 0. `git diff --check` exited 0. |
| Runtime harness command/scenario and exact result | Not run locally. Parent owns token `sha256:beff880be7b39ed6f79a1a0844b5c3a5d98e9262c3e8b0d4ce116cf4ef443ee0`; a passing settlement must remediate `sha256:5f2ac30d16f78d8f968f8fa98ffddedf328a27204a8757c945b7623c0b8443f8`. |
| Rollback boundary | Revert only `NATIVE_TEST_TIMEOUT_MS`, its two active-test arguments, the static assertions, and this section. |

- Native Slice 7 task checkboxes remain unchecked until the remote run passes.

## Completed work unit 1 — direct Go client

Completed only the PR 1 task boundary and marked the corresponding current-revision checkboxes in `tasks.md`:

- [x] 1.3 RED: construction counters prove no-profile selects Companion, `-profile` selects Native, and an unavailable Companion never switches composition.
- [x] 1.4 RED: direct fixed-loopback request tests remove descriptor, bearer, token, and generation contracts while retaining strict correlation, request/result bounds, and deadlines.
- [x] 2.1 GREEN: fixed unauthenticated `127.0.0.1:64139` client and profile-only CLI selection without Native fallback.

### Files changed — work unit 1

- `cmd/nexus/main.go`
- `cmd/nexus/codefori.go`
- `cmd/nexus/main_test.go`
- `internal/connectors/ibmi/codefori/client.go`
- `internal/connectors/ibmi/codefori/client_test.go`
- `internal/connectors/ibmi/codefori/protocol.go`
- `internal/connectors/ibmi/codefori/protocol_test.go`
- `openspec/changes/add-codefori-companion-provider/tasks.md`
- `openspec/changes/add-codefori-companion-provider/apply-progress.md`

### TDD Cycle Evidence — work unit 1

| Task | Test file | Layer | Safety net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|---|
| 1.3 | `cmd/nexus/main_test.go` | Unit with composition counters | `go test -count=1 ./internal/connectors/ibmi/codefori ./cmd/nexus` exited 0: both packages reported `ok` before changes | Same command exited 1 because `selectServeMode` still required the removed provider selector | Passed after profile-only selection and `-provider` removal | No-profile Companion, explicit Native profile, unavailable Companion no-fallback, and removed-flag rejection cover distinct selection paths | No further refactor was needed; final focused command passed |
| 1.4 | `internal/connectors/ibmi/codefori/{client_test.go,protocol_test.go}` | Unit with fake `http.RoundTripper` | Same focused command exited 0 before changes | Same command exited 1 because the old client required `DescriptorReader` and the old selector signature | Passed after direct unauthenticated request/protocol changes | Version and request-ID mismatch, oversized response, removed generation rejection, normalized result validation, and independent status/query deadlines cover distinct protocol paths | No further refactor was needed; final focused command passed |
| 2.1 | `cmd/nexus/main_test.go`, `internal/connectors/ibmi/codefori/{client_test.go,protocol_test.go}` | Unit with fake `http.RoundTripper` and composition counters | Same focused command exited 0 before changes | Tests written for no descriptor/bearer/token/generation request fields and no `-provider` flag failed against the previous construction and protocol contracts | Passed after fixed direct `POST /v1/rpc`, correlation-only envelopes, and profile-only selection | Direct request asserts no authorization header or removed JSON fields; unavailable Companion proves no Native construction; protocol rejects removed generation fields | No further refactor was needed; final focused command passed |

### Work Unit Evidence — work unit 1

| Evidence | Exact result |
|---|---|
| Focused test command and exact result | `go test -count=1 ./internal/connectors/ibmi/codefori ./cmd/nexus` exited 0: `ok bac-nexus/internal/connectors/ibmi/codefori` and `ok bac-nexus/cmd/nexus`. |
| Runtime harness command/scenario and exact result | N/A — this unit has no runtime boundary beyond fake HTTP transport and construction counters. No network listener, VS Code host, Windows process, generated binary/JAR, or IBM i operation ran. |
| Rollback boundary | Revert only `cmd/nexus/{main.go,codefori.go,main_test.go}`, `internal/connectors/ibmi/codefori/{client.go,client_test.go,protocol.go,protocol_test.go}`, and these three task checkboxes/progress section. This restores the pre-unit descriptor-authenticated client and optional provider selector without removing unrelated work. |

### Work unit 1 design deviations

The obsolete descriptor factories remain untouched for work unit 3. `NewClient` temporarily accepts and ignores a compatibility `DescriptorReader` variadic argument so those unchanged files keep compiling; the direct client does not read it or include descriptor, bearer, token, or generation data in requests.

### Work unit 1 delivery and evidence

- Delivery: feature-branch-chain, PR 1 targeting the feature/tracker branch; no branch, commit, staging, tag, release, or PR was created.
- Authored source/test diff: 112 additions and 190 deletions, 302 changed lines. Current task/progress artifact reconciliation is additional planning text; pre-existing proposal/spec/design/task planning changes are excluded.
- RED command: `go test -count=1 ./internal/connectors/ibmi/codefori ./cmd/nexus` exited 1 with `NewClient` arity and `selectServeMode` signature build failures, proving the requested no-auth/profile-only contracts were absent.
- GREEN and final focused command: `go test -count=1 ./internal/connectors/ibmi/codefori ./cmd/nexus` exited 0: `ok bac-nexus/internal/connectors/ibmi/codefori`; `ok bac-nexus/cmd/nexus`.
- Evidence revision: `sha256:a3f1d5ada0c1bcf8edb9dde5d71c89d97bc258f6a9421e2e3adaae0b6e47f613`, computed from the seven scoped source/test diffs plus the normalized exact focused-test outcome stated above.
- Current revised task state: 8/17 complete; remaining work begins with tasks 1.1, 1.2, 1.5, and 2.2.

## Completed work unit 2 — activated HTTP Companion

Completed only the PR 2 boundary and marked the corresponding current-revision checkboxes in `tasks.md`:

- [x] 1.1 RED: Origin-bearing valid, malformed, and oversized bodies are rejected before decode or broker execution.
- [x] 1.2 RED: the real listener uses only `127.0.0.1:64139`, fails a fixed-port collision, and safely repeats start/stop.
- [x] 1.5 RED: activation obtains the exported Code for IBM i instance through a fake host boundary, starts the no-auth broker, and deactivation closes owned resources.
- [x] 2.2 GREEN: activation, fixed loopback HTTP lifecycle, Origin-first gate, strict bounds/parsing, existing adapter/admission, correlation, sanitization, and idempotent teardown are wired.

### Files changed — work unit 2

- `companion/vscode-codefori/src/{httpServer.ts,httpServer.test.ts,extension.ts,extension.test.ts,broker.ts,broker.test.ts,protocol.ts,protocol.test.ts}`
- `openspec/changes/add-codefori-companion-provider/{tasks.md,apply-progress.md}`

### TDD Cycle Evidence — work unit 2

| Task | Test file | Layer | Safety net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|---|
| 1.1 | `src/httpServer.test.ts` | Offline loopback integration | `npm test -- --run src/broker.test.ts src/protocol.test.ts` exited 0: 2 files, 13 tests passed | New focused command exited 1: `httpServer.js` was absent; the existing extension did not call the host boundary | Passed: valid, malformed, and 513-byte Origin-bearing bodies each return exact 403 body and invoke the broker zero times | Mixed-case and empty Origin headers are separately recognized; three body shapes prove the gate precedes decoding | Removed dead header forwarding after authentication removal; final focused command stayed green |
| 1.2 | `src/httpServer.test.ts` | Offline loopback integration | Same 13/13 safety net | Same absent-server RED run | Passed: first fixed listener starts, second collides, repeated start/stop is safe, and a replacement listener starts after cleanup | Actual listener test proves collision has no alternate bind and post-stop port reuse | Listener close is idempotent; final focused command stayed green |
| 1.5 | `src/extension.test.ts` | Unit with fake Code for IBM i host | Same 13/13 safety net | Initial focused command exited 1: the scaffold never queried the host or started a broker | Passed: fake host export flows into the existing adapter, canonical query runs without auth fields, and double deactivation closes once | Host lookup, async export activation, adapter query, fixed bind, and teardown assert distinct lifecycle paths | Replaced an over-complex inferred factory type with a local structural boundary; final focused command stayed green |
| 2.2 | `src/{httpServer,extension,broker,protocol}.test.ts` | Offline loopback integration and unit fakes | Same 13/13 safety net | Missing HTTP server and inactive extension prevented the new acceptance tests | Passed after fixed server, activation wiring, and no-auth correlation-only protocol changes | The protocol suite passed 7/7; the broker suite proves generation is rejected as an unknown field while preserving 16-slot admission, five-second deadline, late-result suppression, and correlation | No mutating formatter is configured; lint/typecheck and final focused tests pass |

### Work Unit Evidence — work unit 2

| Evidence | Exact result |
|---|---|
| Focused test command and exact result | `npm test -- --run src/httpServer.test.ts src/extension.test.ts src/broker.test.ts` exited 0: 3 test files and 9 tests passed. `npm test -- --run src/protocol.test.ts` exited 0: 1 test file and 7 tests passed. `npm run typecheck` and `npm run lint` each exited 0 with no diagnostics. |
| Runtime harness command/scenario and exact result | The final focused command used an actual offline `127.0.0.1:64139` listener plus fake Code for IBM i host. It returned exact Origin rejections for valid, malformed, and oversized bodies; a concurrent second fixed listener failed; after repeated stops, a replacement listener bound and closed successfully. No non-loopback network, VS Code host, generated Nexus/JAR, Windows process, or IBM i operation ran. |
| Rollback boundary | Revert only `companion/vscode-codefori/src/{httpServer.ts,httpServer.test.ts,extension.ts,extension.test.ts,broker.ts,broker.test.ts,protocol.ts,protocol.test.ts}` and these four task checkboxes/progress section. This removes the activated Companion HTTP boundary without changing work unit 1 Go/CLI behavior or later Windows, workflow, documentation, or live IBM i work. |

### Work unit 2 delivery and evidence

- Delivery: feature-branch-chain, PR 2 based on PR 1. No branch, commit, staging, tag, release, publication, or PR was created.
- Authored source/test diff: 334 additions and 65 deletions, 399 changed lines. Required OpenSpec bookkeeping is additional planning text; pre-existing work is excluded.
- Final source-mutating normalization: no mutating formatter is configured in the Companion package; no formatter was invented. `npm run lint` is check-only and passed.
- Listener cleanup evidence: the test closes the first and collision brokers, repeats first stop, then starts and stops a replacement listener on the same fixed port.
- Evidence revision: `sha256:fa032abecf87e3fa7b4ff47dcf8fb79249f4b8bda75803f525ea5921d63eb3ee`, computed from the eight scoped source/test diffs plus normalized final focused-test outcome (`exit=0`, 3 files, 9 tests).
- Current revised task state: 12/17 complete; remaining work begins with tasks 3.1, 3.2, 3.3, 4.1, and 4.2.

## Completed work unit 3 — retire obsolete security and document v1

Completed the three PR 3 tasks and preserved all prior progress evidence.

- [x] 3.1 Removed descriptor readers, platform constructors, and the temporary variadic `NewClient` compatibility seam.
- [x] 3.2 Removed Windows token/PowerShell artifacts, replaced the workflow with offline Go/Node checks, and pinned only Vitest to `3.2.7`.
- [x] 3.3 Added the v1 local-machine trust, limits, verification, and rollback documentation.

### Files changed — work unit 3

- Deleted `internal/connectors/ibmi/codefori/{descriptor.go,descriptor_windows.go,descriptor_other.go,descriptor_windows_test.go}`.
- Deleted `companion/vscode-codefori/src/windows/{tokenStore.ts,tokenStore.test.ts,tokenStore.windows.test.ts,publish.ps1,cleanup.ps1}`.
- Modified `internal/connectors/ibmi/codefori/client.go`, `.github/workflows/release-companion.yml`, and `companion/vscode-codefori/{package.json,package-lock.json}`.
- Created `docs/CODEFORI_COMPANION.md`.

### TDD Cycle Evidence — work unit 3

| Task | Test file | Layer | Safety net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|---|
| 3.1 | `client_test.go` | Go unit | `go test -count=1 ./internal/connectors/ibmi/codefori ./cmd/nexus` exited 0 before deletion | N/A — deletion-only retirement; no new runtime behavior | Same focused command exited 0 after removal | N/A — no branches added | Removed descriptor seam; production `client.go` has no descriptor, token, or generation-authentication reference |
| 3.2 | `tokenStore.test.ts` | Vitest unit | `npm ci` exited 0; token-store suite: 10 passed, 3 Windows-only skipped | N/A — deletion-only retirement and deterministic dependency pin | Final offline suite exited 0: Vitest 3.2.7, 7 files, 32 tests | N/A — no new behavior | Workflow contains no PowerShell, ACL, or native Windows execution; lock diff updates only Vitest and its transitive packages |
| 3.3 | N/A | Documentation | N/A — new document | N/A — no production contract | `git diff --check` exited 0 | N/A — one prescribed documentation surface | Concise English document records all required v1 limits and rollback |

### Work Unit Evidence — work unit 3

| Evidence | Exact result |
|---|---|
| Focused test command and exact result | `go test -count=1 ./internal/connectors/ibmi/codefori ./cmd/nexus` exited 0 before and after removal. `npm ci` exited 0; `npm run lint`, `npm run typecheck`, `npm test`, and `npm run build` each exited 0. Vitest 3.2.7 reported 7 files and 32 tests passed. `git diff --check` exited 0. |
| Runtime harness command/scenario and exact result | N/A — this cleanup unit creates no runtime boundary. Offline retained tests cover fake/loopback behavior only; no Windows/PowerShell, VS Code host, IBM i, generated Nexus/JAR, or non-loopback network path ran. |
| Rollback boundary | Revert only the nine deleted descriptor/Windows files, the `NewClient` signature cleanup, offline workflow, Vitest pin/lock entries, `docs/CODEFORI_COMPANION.md`, the three task checkboxes, and this section. Native `-profile` behavior and the activated Companion remain unchanged. |

### Work unit 3 scope and evidence

- Approved delivery: feature-branch-chain PR 3 based on PR 2, with maintainer-approved `size:exception` capped at 1,650 changed lines.
- Scoped implementation count: 1,230 deleted obsolete lines; 4 client-seam lines; 49 workflow lines; 2 manifest lines; 100 lockfile lines; 54 documentation lines; 6 task-checkbox lines; and 40 progress lines: **1,485 additions + deletions**.
- `docs/CODEFORI_COMPANION.md` is intentionally untracked for native settlement.
- `dist/` was removed after `npm run build`; process inspection found no `go`, `npm`, `node`, `vitest`, or `tsc` process and no listener on `127.0.0.1:64139`.
- Evidence revision: `sha256:77567fef89aa3690dd454cc530ebcdca99756d163583a5b2d3df3a617c2bf3ed`, computed from the scoped patch before this self-referential progress entry plus normalized focused outcomes.
- Remaining tasks: 4.1 offline verification evidence and 4.2 separately authorized Extension Development Host proof.

## Completed task 4.1 — independent offline verification

- [x] Ran check-only formatting, focused Go test/vet/race validation, and deterministic Node install/lint/typecheck/test/build/package checks.
- [x] Confirmed fake/loopback evidence only; it is not live VS Code or IBM i proof and does not satisfy task 4.2.

### TDD Cycle Evidence — task 4.1

| Task | Test file | Layer | Safety net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|---|
| 4.1 | Existing Go and Vitest suites | Offline verification | N/A — verification-only task | N/A — no production behavior or test was added | All required focused checks passed | N/A — no new behavior | No source refactor performed |

### Work Unit Evidence — task 4.1

| Evidence | Exact result |
|---|---|
| Focused test command and exact result | `gofmt -l` over the seven existing changed Go files exited 0 with no output. `go test -count=1 ./internal/provider ./internal/connectors/ibmi/codefori ./internal/mcp ./cmd/nexus` exited 0: all four packages reported `ok`. `go vet` over the same packages exited 0 with no diagnostics. `go test -race -count=1` over the same packages exited 0: all four packages reported `ok`. |
| Runtime harness command/scenario and exact result | `npm ci`, `npm run lint`, `npm run typecheck`, `npm test`, `npm run build`, and `npm pack --dry-run` each exited 0. Vitest 3.2.7 reported 7 files and 32 tests passed; the dry-run package listed 31 files and created no archive. The deterministic Node suite exercised and released its local `127.0.0.1:64139` lifecycle test. |
| Cleanup and proof limit | Removed `dist/`; confirmed no `.tgz`, Go/npm/node/Vitest/tsc process, or listener on `127.0.0.1:64139` remained. No generated Nexus/JAR, Windows/PowerShell, VS Code Extension Development Host, non-loopback network, IBM i, or live proof query ran. |
| Rollback boundary | Revert only the 4.1 checkbox and this progress section; no production behavior changed. |

- `git diff --check` exited 0.
- Evidence revision: `sha256:cf3bc2599efefff11a362cb2d4af7607848098164d0f3ab574b0b81ba81e4204`, computed from the task-state patch before this self-referential progress entry plus normalized offline outcomes.
- Remaining task: 4.2, separately authorized live Extension Development Host proof only.

## Task 4.2 — authorized live proof unavailable

- Status: `unavailable`.
- The required local capability gate did not pass: the available `code` executable reported version `1.136.1`, but its extension inventory contained no `halcyontechltd.code-for-ibmi@3.0.12` entry. A graphical display was available and three generic Extension Development Host candidates were observed, but neither establishes the required Code for IBM i 3.0.12 installation or an active session.
- Primary failure: Code for IBM i exactly `3.0.12` was not available to the local VS Code executable.
- Verification consequence: no Extension Development Host was launched, no listener or process was created, no request was sent to `127.0.0.1:64139`, and task 4.2 remains unchecked.

### TDD Cycle Evidence — task 4.2

| Task | Test file | Layer | Safety net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|---|
| 4.2 | None — authorized live proof only | Extension Development Host | N/A — no source files were modified | Not applicable — capability gate failed before execution | Not run — no suitable host | Not applicable — no execution | Not applicable — no source refactor |

### Work Unit Evidence — task 4.2

| Evidence | Exact result |
|---|---|
| Focused test command and exact result | Not run. The task authorizes one bounded live host operation, not an offline or automated test suite; the required Code for IBM i 3.0.12 capability was unavailable. |
| Runtime harness command/scenario and exact result | Capability-only inspection: `code --version` exited 0 and reported `1.136.1`; `code --list-extensions --show-versions` produced no `halcyontechltd.code-for-ibmi@3.0.12` entry; graphical-display availability was `true`; generic Extension Development Host candidate count was `3`. Result: `unavailable`. Exact permitted operation shape, not executed: `POST http://127.0.0.1:64139/v1/rpc` with method `session.status` only and empty parameters. No SQL text or `sql.query` operation was constructed or sent. |
| Process/listener cleanup | None required. No host, listener, or child process was launched or created; no existing VS Code process or session was touched. |
| Rollback boundary | Revert only this unavailable-evidence section. No production, test, configuration, dependency, source, or task-checkbox change was made. |

### Task 4.2 proof limitations

- The required Code for IBM i `3.0.12` installation and active session were not evidenced, so this is not a live Companion, VS Code, or IBM i proof.
- No substitute host, mock, offline test, retry, remote operation, credential access, raw extension log, host, username, profile, or environment value was used or disclosed.
- Task progress remains **16/17 complete**; task 4.2 is the sole pending task.
