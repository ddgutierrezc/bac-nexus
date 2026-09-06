# Tasks: bounded Code for IBM i Companion proof

## Review Workload Forecast and delivery decision

| Field | Value |
|---|---|
| Execution mode | Interactive |
| Delivery strategy | `exception-ok`: user selected one PR and explicitly approved `size:exception` |
| Session review budget | 2,000 changed lines |
| Total forecast | 2,520–3,070 changed lines; exceeds the session budget under explicit maintainer approval |
| Work-unit status | Eight cohesive implementation and rollback boundaries; not child-PR boundaries |
| Current unit | Slice 7 native evidence: workflow prepared locally; execution deferred pending separate commit/push authorization |
| Slice 1 record | Five tasks complete; 321 changed lines; provider test, vet, and race checks passed; historical 300-line objective reset by maintainer decision |
| Publication | No publication now; retain a protected OIDC route that is disabled by default |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: size-exception
400-line budget risk: High

No commits, pushes, branches, tags, PRs, publication, Windows execution, or IBM i execution are part of this artifact update. Implementation remains one maintainer-approved `size:exception` PR. Keep tests and behavior with their work unit; do not compress code or remove tests/docs to fit a budget.

```text
Single PR (maintainer-approved size:exception; cohesive work-unit boundaries)
  ├─ 1 neutral provider domain (180–280 forecast; 321 recorded) ✓
  ├─ 2 Go fixed-loopback connector (330–400)
  ├─ 3 MCP/CLI mode isolation (330–400)
  ├─ 4 TS package, query, protocol, broker (350–400)
  ├─ 5 public adapter and admission lifecycle (340–400)
  ├─ 6 fixed Windows token setup (350–400)
  ├─ 7 native-Windows consumer/security gates (320–400)
  └─ 8 package, protected OIDC route, docs (300–390)
```

The work units form three milestones: Go provider path (1–3), TypeScript broker path (4–6), and Windows/release evidence (7–8). The honest total forecast is 2,520–3,070 changed lines, exceeding the 2,000-line session budget under explicit maintainer approval.

## Scope guardrails

- Use the neutral port at `internal/provider/provider.go`; do not create `internal/provider/codefori.go` or `internal/companion/`.
- Put the concrete connector only in `internal/connectors/ibmi/codefori/` and the extension only in `companion/vscode-codefori/`.
- Bind only `127.0.0.1:64139` and expose only bounded `POST /v1/rpc`.
- Canonicalize only `SELECT CURRENT_USER FROM SYSIBM.SYSDUMMY1`, exactly 41 ASCII bytes, independently in Go and TypeScript.
- Use immediate admission capacity 16, sufficient for at least ten simultaneous fake calls. Add no waiting/worker queue and no process-wide/global SQL mutex.
- After `runSQL` starts, timeout/cancellation suppresses output but retains capacity until the underlying promise settles.
- Use only the two descriptor-specific fixed PowerShell operations. Add no reusable command runner or caller-controlled executable, script, path, arguments, or environment.
- Keep Native onboarding, profiles, credentials, eligibility, audit, ownership, recovery, SSH/SFTP, Mapepire, configuration, TUI, validation, packaging, and release behavior unchanged.
- Preserve `not_validated_on_ibmi`; passing fakes does not prove shared-job concurrency, cancellation, session events, or raw row labels.

## Milestone 1 — Go provider path

### Slice 1 — Provider-neutral domain contract (180–280 lines)

**Start:** no `internal/provider/` package exists.

**Finish:** a vendor- and transport-neutral `Provider` port owns the proof request/result contract, canonicalization, allowed states, and normalized-result validation.

**Exact edit surface:**

- `internal/provider/provider.go`
- `internal/provider/provider_test.go`

**Verification:** `go test -count=1 ./internal/provider`

**Rollback:** remove only `internal/provider/`; no existing package changes.

- [x] **RED:** Add table tests for accepted ASCII case and space/tab/CR/LF variations and rejected comments, semicolons, Unicode whitespace, dot whitespace, extra tokens, parameters, arrays, non-ASCII input, and inputs over 128 bytes. <!-- sdd-owner: implementation -->
- [x] **RED:** Add fake-`Provider` tests proving invalid input causes no provider call and accepted input is the exact canonical 41-byte statement. <!-- sdd-owner: implementation -->
- [x] **RED:** Add normalized-result tests requiring one valid UTF-8 string value no larger than 256 bytes for `ok`, no rows for non-success states, and rejection of unknown states, zero/multiple rows, or oversized/invalid values. <!-- sdd-owner: implementation -->
- [x] **GREEN:** Implement `Provider`, `SessionStatusResult`, `QueryRequest`, `QueryResult`, `NormalizedRow`, neutral state types, `CanonicalProofQuery`, `CanonicalizeQuery`, and normalized-result validation in `provider.go` only. <!-- sdd-owner: implementation -->
- [x] **REFACTOR:** Keep the package free of HTTP, endpoint, descriptor, token, profile, credential, PowerShell, VS Code, and Code for IBM i imports or names. <!-- sdd-owner: implementation -->

**Acceptance criteria:** all tests pass; the canonical value is 41 bytes; invalid queries never call the fake; successful normalized results contain exactly one `value`; non-success results carry no rows; only the two exact files above change; Slice 1 recorded 321 changed lines, and its historical 300-line objective was reset by explicit maintainer decision.

### Slice 2 — Fixed-loopback Go connector (330–400 lines)

**Start:** slice 1 defines the domain contract with no transport.

**Finish:** `internal/connectors/ibmi/codefori/` implements `provider.Provider` against one bounded fixed-loopback RPC, using an injected descriptor reader in tests and deterministic unavailable behavior on unsupported platforms.

**Edit surface:** `internal/connectors/ibmi/codefori/{client.go,protocol.go,descriptor.go,descriptor_other.go,client_test.go,protocol_test.go}`.

**Verification:** `go test -count=1 ./internal/provider ./internal/connectors/ibmi/codefori`

- [x] **RED:** Prove no descriptor read or HTTP call occurs for invalid SQL; prove strict version/generation/request-ID correlation, bearer auth, unknown/duplicate/trailing JSON rejection, 512-byte request and 1024-byte response limits, one-second status, six-second-or-greater response-header timeout, seven-second query total, no retry, and token-free errors. <!-- sdd-owner: implementation -->
- [x] **GREEN:** Implement the fixed `127.0.0.1:64139` `POST /v1/rpc` client and protocol adapter without endpoint configuration, discovery, fallback, or generic transport abstraction. <!-- sdd-owner: implementation -->
- [x] **TRIANGULATE:** Validate all returned normalized results through `internal/provider`; map missing, malformed, stale, denied, unreachable, mismatched, and unsupported-platform state to bounded results. <!-- sdd-owner: implementation -->

### Slice 3 — Isolated MCP and CLI selection (330–400 lines)

**Start:** slice 2 supplies a connector, but `nexus serve` still requires a Native profile.

**Finish:** no-profile serve starts a Companion-only MCP surface; profile-selected serve follows the existing Native composition unchanged.

**Edit surface:** `internal/mcp/codefori.go`, `internal/mcp/codefori_test.go`, `cmd/nexus/codefori.go`, and targeted changes in `cmd/nexus/{main.go,main_test.go}`.

**Verification:** `go test -count=1 ./internal/provider ./internal/connectors/ibmi/codefori ./internal/mcp ./cmd/nexus`

- [x] **RED:** Prove Companion exposes exactly `session.status` and `sql.query`, Native retains exactly its existing two tools, contradictory selectors fail before dependency construction, and unavailable calls never switch providers. <!-- sdd-owner: implementation -->
- [x] **GREEN:** Add a separate Companion MCP constructor and composition root; change only deterministic selection/help text in `main.go`; preserve the existing profile-selected `runWithDeps` path. <!-- sdd-owner: implementation -->
- [x] **TRIANGULATE:** Prove Companion mode constructs no profile, credential, eligibility, audit, ownership, recovery, SSH/SFTP, Mapepire, configuration, TUI, or Native validation dependency. <!-- sdd-owner: implementation -->

## Milestone 2 — TypeScript broker path

### Slice 4 — Package, query, protocol, and broker (350–400 lines)

**Start:** Go can call the fixed endpoint; no extension owns it.

**Finish:** `companion/vscode-codefori/` is an independently testable TypeScript extension scaffold with the independent recognizer and fixed bounded broker, using fakes only.

**Edit surface:** package metadata plus `src/{extension.ts,query.ts,query.test.ts,protocol.ts,protocol.test.ts,broker.ts,broker.test.ts}`.

**Verification:** from `companion/vscode-codefori/`, run `npm ci`, `npm run lint`, `npm run typecheck`, `npm test`, and `npm run build`.

- [x] **RED/GREEN:** Pin `@halcyontech/vscode-ibmi-types` to 3.0.12; independently recognize only the proof query; bind only `127.0.0.1:64139`; accept only bounded authenticated `POST /v1/rpc`; reject collision without scan/fallback; and expose no endpoint setting or generic RPC method. <!-- sdd-owner: implementation -->
- [x] **TRIANGULATE:** Prove strict auth/version/generation/ID/body handling and sanitized non-success output without VS Code or IBM i. <!-- sdd-owner: implementation -->

### Slice 5 — Public adapter and immediate admission (340–400 lines)

**Start:** slice 4 has a fake broker boundary.

**Finish:** the broker uses only public Code for IBM i APIs and bounded immediate admission with correlated concurrent responses.

**Edit surface:** `src/{codeforiAdapter.ts,codeforiAdapter.test.ts,admission.ts,admission.test.ts}` plus targeted broker/extension tests.

**Verification:** Companion lint, typecheck, test, and build scripts.

- [x] **RED/GREEN:** Use only `exports.instance`, `getConnection`, `subscribe`, and `runSQL`; treat `subscribe` as `void`; call `runSQL` only with canonical SQL and `{rows: 1}`; normalize one arbitrary raw column label to `value`; suppress raw errors and malformed/late/disconnected results. <!-- sdd-owner: implementation -->
- [x] **RED/GREEN:** Implement immediate capacity 16 with no waiting/worker queue and no global SQL mutex. Admit at least ten barrier-held fakes, complete them in reverse order, and prove exact correlation. <!-- sdd-owner: implementation -->
- [x] **TRIANGULATE:** Prove excess work returns `limit_exceeded`, pre-start cancellation never calls `runSQL`, and a timed-out started promise retains capacity until it settles. Keep evidence `not_validated_on_ibmi`. <!-- sdd-owner: implementation -->

### Slice 6 — Fixed Windows descriptor operations (350–400 lines)

**Start:** slice 5 has a broker that cannot securely publish an activation descriptor.

**Finish:** a private Windows token store invokes only immutable descriptor publish/cleanup operations and keeps intake disabled on uncertainty.

**Edit surface:** `src/windows/{tokenStore.ts,tokenStore.test.ts,publish.ps1,cleanup.ps1}` plus targeted extension lifecycle tests.

**Verification:** Companion lint, typecheck, test, and build scripts with process fakes on normal CI.

- [x] **RED:** Prove absolute inbox PowerShell resolution, `shell: false`, fixed scripts/arguments, minimal environment, bounded stdio/deadlines, readiness-before-RNG, stdin-only secret transfer, fixed output, no retry, sanitized errors, and generation-owned cleanup. <!-- sdd-owner: implementation -->
- [x] **GREEN:** Create/validate only the fixed LocalApplicationData descriptor path. For a new directory use `Directory.CreateDirectory(path, DirectorySecurity)`; for an existing directory validate it unchanged before `READY`. Create the descriptor with a protected current-SID-only DACL before writing, and fail closed if selected file semantics cannot be evidenced. <!-- sdd-owner: implementation -->
- [x] **TRIANGULATE:** Keep the operation private and descriptor-specific; reject any caller-controlled executable, command, script, path, arguments, or environment. <!-- sdd-owner: implementation -->

## Milestone 3 — Windows and release evidence

### Slice 7 — Native-Windows consumer and ACL gates (320–400 lines)

**Start:** fake tests cover the fixed operation, but no Windows behavior is proven.

**Finish:** Windows-scoped producer and Go consumer tests define and exercise the actual ACL gate; authenticated activation remains disabled unless they pass.

**Edit surface:** `internal/connectors/ibmi/codefori/{descriptor_windows.go,descriptor_windows_test.go}` and `companion/vscode-codefori/src/windows/tokenStore.windows.test.ts`, with only minimal package-script wiring.

**Verification:** scoped Go tests and Companion tests on a native Windows runner/host. No unrestricted live IBM i test.

- [x] **PREREQUISITE:** Prepare `.github/workflows/release-companion.yml` with a `windows-latest` validation job that runs the scoped Go consumer tests and opt-in TypeScript token-store Windows tests, uses only `contents: read`, no secrets, publication, or IBM i, bounded logs/artifacts, and leaves `.github/workflows/go-verification.yml` unchanged. <!-- sdd-owner: implementation -->
- [ ] Prove current-process SID owner, protected exact owner-only DACL, regular/non-reparse directory and file, 512-byte descriptor bound, version/generation checks, and no token use before Go validation. <!-- sdd-owner: implementation -->
- [ ] Prove clean creation, existing valid directory handling, no pre-`READY` secret, cross-user read denial, no readable transient file, and generation-owned cleanup. <!-- sdd-owner: implementation -->
- [ ] Prove wrong-owner, inherited, extra-ACE, permissive, reparse, oversized, locked, malformed, and wrong-generation residue keeps the broker unavailable without repair or Native fallback. <!-- sdd-owner: implementation -->
- [ ] Record the evidence honestly as Windows ACL validation only; do not claim extension-host identity or IBM i behavior. <!-- sdd-owner: implementation -->

### Slice 8 — Offline package, protected OIDC route, and operator proof (300–390 lines)

**Start:** slices 1–7 supply the bounded topology and Windows security gates.

**Finish:** Companion CI/package verification and operator documentation exist; publication capability is present but disabled by default and protected.

**Edit surface:** `.github/workflows/release-companion.yml`, Companion package/VSIX metadata and workflow-contract test, and `docs/CODEFORI_COMPANION.md`.

**Verification:** scoped Go tests; Companion install/lint/typecheck/test/build/package/VSIX inspection; workflow contract checks. Existing `.github/workflows/go-verification.yml` is not modified.

- [ ] Add ordinary offline verification for the Companion and validate `companion-v*` SemVer tags, with first intended tag `companion-v0.1.0`. <!-- sdd-owner: implementation -->
- [ ] Add a publish job that runs only for a valid `companion-v*` tag when repository variable `COMPANION_PUBLISH_ENABLED == 'true'`, uses protected environment `companion-marketplace`, grants `id-token: write` only in that job, uses current `vsce` OIDC, and accepts no Marketplace PAT. The unset/default state must be inert. <!-- sdd-owner: implementation -->
- [ ] Add contract tests proving ordinary CI cannot publish, non-Companion tags cannot publish, the enable variable and protected environment are required, no PAT name is present, and existing Go verification remains independent. <!-- sdd-owner: implementation -->
- [ ] Document native-Windows install order, one broker per user, the status/proof-query flow, ACL/cross-user evidence recording without secrets, unsupported multi-window/WSL/SSH/container/forwarding topologies, `not_validated_on_ibmi`, and rollback. <!-- sdd-owner: implementation -->

## Current handoff

**Next action:** await separate commit/push authorization to execute Slice 7's local `.github/workflows/release-companion.yml` Windows validation harness.

**Completed Slices 1–6 and Slice 7 prerequisite:** 20 of 28 task checkboxes are marked `[x]`; their completed edit surfaces remain recorded in the corresponding Slice sections above.

**Completed scope boundary:** the completed work remains limited to the provider domain, fixed-loopback connector, isolated MCP/CLI selection, fake-backed TypeScript query/protocol/broker scaffold, public API adapter seam, immediate admission lifecycle, private fake-backed Windows descriptor operations, and the local Slice 7 workflow harness. The four Slice 7 native-evidence tasks, documentation, Native implementation, and Slice 8 packaging/OIDC/publication work remain unstarted or unchanged.

**Current implementation unit:** Slice 7 native evidence. The local workflow harness is prepared only; no workflow execution, commit, push, PR, release, publication, or IBM i evidence is authorized without separate commit/push authorization.

**Forecast:** Slice 1 recorded 321 changed lines; its historical 300-line objective was reset by explicit maintainer decision after provider test, vet, and race checks passed. Slice 2 is forecast at 330–400. The total 2,520–3,070 forecast exceeds the 2,000-line session budget under the approved `size:exception`; retain cohesive tests and do not compress implementation.
