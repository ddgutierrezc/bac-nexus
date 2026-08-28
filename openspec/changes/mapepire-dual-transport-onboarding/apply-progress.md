# Apply Progress: Mapepire Dual-Transport Onboarding

## Work Unit

- Slice: 1
- Work unit: `slice-1-schema-policy-trust`
- Delivery: auto-chain, feature-branch-chain; PR #1 standalone profile foundation
- Scope: tasks 1.1 and 1.2 only
- Authored changed lines: 377 (including prior Slice 1 work and this correction; excluding unrelated worktree changes)

## Completed Tasks

- [x] 1.1 RED: added schema-v2 persistence, conservative migration, ephemeral-field rejection, and independent TLS/SSH trust tests.
- [x] 1.2 GREEN: added schema-v2 validation/migration and Nexus-owned resolver release limits.

## TDD Cycle Evidence

| Task | Test layer | Safety net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|
| 1.1 | Unit and local filesystem | `go test -count=1 ./internal/profile ./internal/security`: pre-existing profile/security tests passed | Compile failure on missing schema-v2 API and trust types | Focused tests passed after implementation | Round-trip, malformed/ambiguous trust, deterministic migration, and forbidden ephemeral/secret fields | Empty v2 evidence omitted; legacy serialization preserved |
| 1.2 | Unit | Same focused safety net | Tests required strict schema and migration behavior before production implementation | Focused tests passed | TLS and SSH pin formats are validated by separate transport modes; legacy migration has no automatic trust or fallback | Shared strict key sets and bounded release constants |

## Work Unit Evidence

| Evidence | Result |
|---|---|
| Focused test command and exact result | `go test -count=1 ./internal/profile ./internal/security` — process exit code `0`; observable package result lines: `ok bac-nexus/internal/profile 1.167s`, `ok bac-nexus/internal/security 1.119s`; 2 packages passed (1 profile, 1 security). Normal non-verbose output reports no individual test counts. |
| Runtime harness command/scenario and exact result | N/A: this slice has only local JSON, validation, and in-process policy behavior; no runtime or remote boundary exists |
| Rollback boundary | Revert `internal/profile/profile.go`, `internal/profile/profile_test.go`, `internal/profile/recovery.go`, `internal/configuration/limits.go`, and this slice's task/progress edits; unrelated `.atl`, `tmp`, and prior OpenSpec changes remain untouched |

## Verification

- `go test -count=1 ./...` — passed; all packages green.
- `go vet ./...` — passed.
- `gofmt -d internal/profile/profile.go internal/profile/profile_test.go internal/profile/recovery.go internal/configuration/limits.go` — no output; formatted.
- No IBM i or external network contact.
- No Mapepire transport/session, WSS, SSH, resolver, TUI, dependency, documentation, or artifact-acquisition files were edited.

## Surgical Correction: `slice-1-trust-validation-correction`

- Scope: completed Slice 1 only; `internal/profile` production and regression tests.
- Correction authored changed lines: 45; cumulative Slice 1 authored changed lines: 377.
- RED: `go test -count=1 ./internal/profile -run 'TestSchemaV2AllowsUnenrolledTrustEvidence|TestSchemaV2TrustModesAreTransportSpecific|TestMigrateV1IsConservativeAndDeterministic'` — process exit code `1`; all three new regression scenarios failed for the known defects.
- GREEN: same focused command — process exit code `0`; `ok bac-nexus/internal/profile 1.154s`.

### Correction TDD Cycle Evidence

| Behavior | Test | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|
| Empty trust is valid without trust/fallback | `TestSchemaV2AllowsUnenrolledTrustEvidence` | ✅ exit `1`: invalid trust mode | ✅ exit `0` | ✅ TLS and SSH both zero-valued | ➖ minimal early return |
| Migration validates and round-trips | `TestMigrateV1IsConservativeAndDeterministic` | ✅ exit `1`: migrated profile invalid | ✅ exit `0` | ✅ deterministic output plus Store Save/Load | ➖ none needed |
| SSH CA rejected; TLS CA accepted | `TestSchemaV2TrustModesAreTransportSpecific` | ✅ exit `1`: SSH CA accepted | ✅ exit `0` | ✅ both transport branches | ➖ minimal transport guard |

### Correction Verification Evidence

- `go test -count=1 ./internal/profile ./internal/security` — process exit code `0`; `ok bac-nexus/internal/profile 1.126s`, `ok bac-nexus/internal/security 1.037s`; 2 packages passed.
- `go test -count=1 ./...` — process exit code `0`; 22 test-bearing packages passed, 3 packages reported `[no test files]`.
- `go vet ./...` — process exit code `0`.
- `gofmt -d internal/profile/profile.go internal/profile/profile_test.go` — process exit code `0`; no formatting output.
- `git diff --check` — process exit code `0`.
- Runtime harness: N/A; correction changes only local profile JSON validation/migration and filesystem round-trip, with no runtime or remote boundary.
- Rollback boundary: revert only the correction hunks in `internal/profile/profile.go`, `internal/profile/profile_test.go`, and this progress evidence; retain prior Slice 1 implementation and unrelated worktree changes.

## Remaining Tasks

- [x] 1.3 RED: added typed seven-operation, bounds, ID, correlation, cursor, limit, and cancellation tests.
- [x] 1.4 GREEN: added transport-neutral typed message session with one reader, controlled writes, bounded pending/cursor state, safe protocol errors, and legacy `Execute` compatibility.
- [ ] 2.1–2.4 transports and fallback runtime.
- [ ] 3.1–3.5 resolver, wizard, and final verification.

## Slice 2: `slice-2-typed-protocol-session`

- Delivery decision: `size:exception`, explicitly approved by the maintainer for Slice 2 only; authorized maximum is 600 changed lines.
- Chain preservation: later slices remain `feature-branch-chain`; this exception does not increase their review budget or alter their boundaries.
- Exact current complete intended diff, additions plus deletions: **548 additions + 29 deletions = 577 changed lines**. This includes tracked and untracked Mapepire Go files plus `tasks.md` and this `apply-progress.md`; unrelated `.atl` and `tmp` changes are excluded and untouched. It is within the approved 600-line exception.
- Numstat: `internal/mapepire/protocol.go` 77/2; `internal/mapepire/session.go` 15/21; `internal/mapepire/typed_session.go` 222/0; `internal/mapepire/typed_protocol_test.go` 198/0; `openspec/changes/mapepire-dual-transport-onboarding/tasks.md` 8/2; `openspec/changes/mapepire-dual-transport-onboarding/apply-progress.md` 28/4. Total: 548/29.
- Work-unit boundary: typed protocol/session core only, from task 1.3 RED through task 1.4 GREEN; no WSS, SSH, resolver, TUI, dependency, configuration, profile, or later task implementation.

| Task | RED → GREEN → REFACTOR |
|---|---|
| 1.3 | ✅ typed tests written before production changes and failed (exit `1`) → ✅ focused package passed (exit `0`) → ✅ triangulated valid/invalid operations, caller-ID replacement, strict trailing JSON rejection, session-deadline timeout/wakeup, out-of-order and unknown/duplicate correlation, cursor lifecycle, column/page/cursor/aggregate bounds, and cancellation closure |
| 1.4 | ✅ tests preceded implementation → ✅ focused package passed (exit `0`) → ✅ triangulated one reader, controlled writes, bounded pending state, safe SQL errors, fail-closed cancellation/session wakeup, prepare/sqlmore/sqlclose lifecycle, and legacy `Execute` compatibility |
| Runtime bounds RED/GREEN | ✅ each corrected bound was represented by a failing regression scenario before the correction → ✅ all corresponding scenarios pass: cryptographic client IDs, strict one-object JSON, remaining-session deadline, fail-closed cancellation, columns, page rows, cursor count, aggregate bytes, pending IDs, frame bytes, and field/parameter sizes |
| Evidence | Result |
|---|---|
| Focused test command and exact result | `go test -count=1 ./internal/mapepire` — process exit code `0`; observable result `ok bac-nexus/internal/mapepire 15.977s`; 1 package passed. |
| Full test command and exact result | `go test -count=1 ./...` — process exit code `0`; 22 test-bearing packages passed and 3 packages reported `[no test files]` (25 observable package results). |
| Static validation | `go vet ./...` — process exit code `0`. |
| Formatting | `gofmt -d internal/mapepire/protocol.go internal/mapepire/session.go internal/mapepire/typed_session.go internal/mapepire/typed_protocol_test.go` — process exit code `0`; no output. |
| Diff validation | `git diff --check` — process exit code `0`. |
| Runtime harness command/scenario and exact result | N/A: the work unit has only deterministic in-memory/channel fakes; there is no runtime, network, or IBM i boundary. No network/IBM i access was performed. |
| Rollback boundary | Revert exactly `internal/mapepire/protocol.go`, `internal/mapepire/session.go`, `internal/mapepire/typed_session.go`, `internal/mapepire/typed_protocol_test.go`, and the Slice 2 sections/checkbox updates in `openspec/changes/mapepire-dual-transport-onboarding/tasks.md` and `apply-progress.md`; retain Slice 1, later pending tasks, and unrelated `.atl`/`tmp` worktree changes. |

### Slice 2 completion state

- Tasks 1.1–1.4 are checked; tasks 2.1–2.4 and 3.1–3.5 remain unchecked.
- Next recommended action: `apply` (continue with the next assigned feature-branch-chain slice); this slice is not a request to run verification/archive or to implement later tasks.

## Slice 3: `slice-3-trusted-daemon-wss`

- Delivery: `size:exception`, Slice 3 only, explicitly approved by the maintainer; maximum 405 additions plus deletions. Later slices retain feature-branch-chain and the normal 400-line guard.
- Dependency admission: `github.com/coder/websocket v1.8.15`, direct production dependency, ISC license (module `LICENSE.txt`), zero module dependencies; `go list -m -json` reported module sum `h1:6B2JPeOGlpff2Uz6vOEH1Vzpi0iUz20A+lPVhPHtNUA=` and go.mod sum `h1:NX3SzP+inril6yawo5CQXx8+fk145lPDC6pumgx0mVg=`. `go mod verify` passed. `govulncheck` was unavailable and was not installed.
- RED: `go test -count=1 ./internal/mapepire/wss` failed exit 1 before implementation with missing `Dial`/`Options` and missing production package symbols; loopback tests were written first.
- GREEN: focused WSS tests passed exit 0; coverage includes WSS-only scheme and `MP_UNSECURE` rejection, CA/hostname validation, verified pin/explicit TOFU evidence, mismatch/rotation failure, text-only JSON, binary/malformed frames, frame bounds, compression disabled, cancellation terminality, and idempotent close.
- REFACTOR: consolidated TLS policy cloning for injected `http.Transport`, rejected caller `InsecureSkipVerify`, fixed coder limit mapping, and stopped mutating a caller HTTP client with a nil transport; focused tests remained green.

### Slice 3 TDD Cycle Evidence

| Task | Test layer | Safety net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|
| 2.1 | Loopback integration | `go test -count=1 ./internal/mapepire`: exit 0 | ✅ test-first compile failure, exit 1 | ✅ WSS package, exit 0 | ✅ text/binary/malformed, CA/pin/TOFU, bounds/cancel/identity cases | ✅ sanitized terminal errors and disabled compression |
| 2.2 | Loopback integration | N/A (new package) | ✅ tests referenced absent adapter | ✅ focused package, exit 0 | ✅ injected client/TLS, one reader, bounded messages, idempotent close | ✅ transport exposes only `mapepire.MessageTransport` |

### Slice 3 Work Unit Evidence

| Evidence | Result |
|---|---|
| Focused test command and exact result | `go test -count=1 ./internal/mapepire/wss` — exit 0; `ok bac-nexus/internal/mapepire/wss 3.096s`; 1 package passed. |
| Runtime harness command/scenario and exact result | Local `httptest.NewTLSServer` WSS loopback only — exit 0 through the focused package; no external network or IBM i contact. |
| Verification commands and exact result | `go test -count=1 ./...` exit 0 (23 test-bearing packages, 3 no-test packages); `go vet ./...` exit 0; `go mod verify` exit 0; touched Go `gofmt -d` no output; `git diff --check` exit 0. |
| Rollback boundary | Revert `internal/mapepire/wss/wss.go`, `internal/mapepire/wss/wss_test.go`, the coder entries in `go.mod`/`go.sum`, and only the Slice 3 checkbox/progress sections; retain Slices 1–2 and pending later tasks. |

- Exact intended Slice 3 numstat: `internal/mapepire/wss/wss.go` 161/0; `internal/mapepire/wss/wss_test.go` 160/0; `go.mod` 1/0; `go.sum` 2/0; `tasks.md` 2/2. Non-progress entries: 326 additions + 2 deletions. Apply-progress entry: 27 additions + 0 deletions. Complete Slice 3 candidate: 353 additions + 2 deletions = 355 changed lines; within the 400-line budget.
- Tasks 1.1–1.4 and 2.1–2.2 are checked. Tasks 2.3–2.4 and 3.1–3.5 remain pending. Next recommended action: `apply`.

## Slice 3 corrective pass: `slice-3-wss-trust-classification-correction`

- Scope: corrective pass only; tasks 1.1–2.2 not completed by this pass, later tasks pending; authority `proceed`, correction budget 120, no exception.
- TDD Cycle Evidence (`2.1/2.2`): RED ✅ exit 1 before code; GREEN ✅ focused exit 0; TRIANGULATE ✅ CA/pin/self-signed/malformed/transport/endpoint/timeout/framing/cancellation; REFACTOR ✅ cloned authoritative transport and sanitized classification. Verified proof points: canonical raw base64url `sha256/` leaf-pin encoding plus malformed/noncanonical rejection; endpoint URL userinfo rejection; distinct sanitized typed availability/refusal, timeout, TLS identity, invalid endpoint, and invalid configuration errors without raw host/URL/certificate/network text; transport matrix covers cloned caller `http.Client`, cloned nil/default `*http.Transport` with TLS policy installed, cloned supported custom `*http.Transport`, rejected unsupported `RoundTripper`, and rejected caller `InsecureSkipVerify`.
- Work Unit Evidence: focused WSS exit 0 (1 package); loopback TLS/WSS runtime exit 0; rollback boundary `internal/mapepire/wss/` plus this evidence. Pin/TOFU manually enforces exact leaf, hostname and validity; CA keeps chain verification; callback is preserved.
- Exact final numstat: `internal/mapepire/wss/wss.go` 201/0; `internal/mapepire/wss/wss_test.go` 160/0; `go.mod` 1/0; `go.sum` 2/0; `openspec/changes/mapepire-dual-transport-onboarding/tasks.md` 2/2; `openspec/changes/mapepire-dual-transport-onboarding/apply-progress.md` 34/0. Total: **400 additions + 2 deletions = 402 changed lines**, within the approved Slice-3-only maximum.

## Slice 4: `slice-4-ssh-single-fallback`

- Delivery: auto-chain, feature-branch-chain; normal hard limit of 400 changed lines. Scope is tasks 2.3 and 2.4 only; later resolver, audit, readiness, TUI, and documentation tasks remain pending.
- RED: `go test -count=1 ./internal/mapepire/sshstdio` — exit 1 before implementation because the adapter and framing errors were absent.
- GREEN: added bounded LF JSON-object framing, context-terminal channel closure, typed-client transport wiring, terminal unsuccessful-response handling, and consent-gated fixed SSH `--single` startup.
- REFACTOR: retained one typed-client reader and controlled adapter writes; reused existing SSH host identity, artifact verification, upload/rollback, Java validation, and fixed command construction.

### Slice 4 TDD Cycle Evidence

| Task | Test layer | Safety net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|
| 2.3 | Unit/in-process fake channel | Existing mapepire and remote tests passed | ✅ exit 1 | ✅ focused exit 0 | ✅ object, malformed/non-object/oversized/unterminated, newline, correlation, EOF/cancel | ✅ bounded reader/write mutex |
| 2.4 | Unit/fake process and policy | Existing connector/remote tests passed | ✅ symbols absent | ✅ focused exit 0 | ✅ typed wiring, terminal failure, consent and unsafe command rejection | ✅ narrow consumer-owned seams |

### Slice 4 Work Unit Evidence

| Evidence | Result |
|---|---|
| Focused tests | `go test -count=1 ./internal/mapepire/sshstdio ./internal/mapepire ./internal/connectors/ibmi/mapepirestdio ./internal/remote` — exit 0; 4 packages passed. |
| Runtime harness | Same exact runnable command as focused tests: `go test -count=1 ./internal/mapepire/sshstdio ./internal/mapepire ./internal/connectors/ibmi/mapepirestdio ./internal/remote` — exit 0. It exercises only in-process fake SSH channels and pipe-backed process output; no IBM i, network, Java process, artifact download, or credentials. |
| Verification | `go test -count=1 ./...` exit 0; 22 test-bearing packages and 3 no-test packages; `go vet ./...` exit 0. Exact formatting check: `gofmt -d internal/mapepire/sshstdio/sshstdio.go internal/mapepire/sshstdio/sshstdio_test.go internal/mapepire/typed_session.go internal/remote/ssh.go internal/connectors/ibmi/mapepirestdio/policy.go internal/connectors/ibmi/mapepirestdio/policy_test.go` — exit 0, no output. `git diff --check` exit 0. |
| Rollback boundary | Revert `internal/mapepire/sshstdio/`, terminal handling in `internal/mapepire/typed_session.go`, `internal/remote/ssh.go`, consent changes in `internal/connectors/ibmi/mapepirestdio/policy.go` and `policy_test.go`, and only Slice 4 task/progress sections. |

- Host-key mismatch terminality: the existing `internal/remote/hostidentity_test.go` mismatch proof and the recorded Slice 3 WSS identity-mismatch/rotation proof establish fail-closed identity behavior. A changed SSH host key is terminal (`host_key_changed`) and cannot be classified as availability or become fallback.
- Daemon non-invocation: the recorded structural proof for `internal/mapepire/wss/` shows the WSS adapter imports/calls none of SSH, JAR, Java, upload, or cache capabilities; daemon WSS resolution therefore cannot invoke the Slice 4 fallback runtime.
- Gatekeeper correction: artifact-only evidence clarification; no source/test/task changes, no commands rerun, and the measured Slice 4 total remains **324 additions + 9 deletions = 333 changed lines**.

- Exact Slice 4 numstat (excluding unrelated `.atl`/`tmp`): `internal/mapepire/sshstdio/sshstdio.go` 133/0; `internal/mapepire/sshstdio/sshstdio_test.go` 136/0; `internal/connectors/ibmi/mapepirestdio/policy.go` 4/0; `policy_test.go` 8/2; `internal/mapepire/typed_session.go` 5/5; `internal/remote/ssh.go` 11/0; `tasks.md` 2/2; `apply-progress.md` 25/0. Total: **324 additions + 9 deletions = 333 changed lines**, under the normal 400-line guard. Tasks 1.1–1.4 and 2.1–2.4 are checked; 3.1–3.5 remain pending. Next recommended action: `apply`.

## Slice 5: `slice-5-resolver-readiness`

- Delivery: auto-chain, feature-branch-chain; normal hard limit of 400 changed lines. Scope is tasks 3.1 and 3.2 only; tasks 3.3–3.5 remain pending.
- RED: resolver, version, no-downgrade, trust-gate, consent, terminal credential, sanitized transport-audit, and offline readiness tests were written first; focused compile/test exited `1` before implementation.
- GREEN: added managed WSS-first resolution with bounded version classification, explicit fallback reason, independent SSH trust and consent gates, ephemeral readiness, and metadata-only transport audit validation/storage.
- REFACTOR: kept configuration inward-pointing by using a narrow local audit seam; retained existing WSS/SSH adapters and fallback runtime untouched.

### Slice 5 TDD Cycle Evidence

| Task | Test layer | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|
| 3.1 | Unit/in-process fakes | ✅ exit 1: missing resolver/audit/readiness APIs | ✅ focused exit 0 | ✅ supported, unsupported, availability, identity, trust, consent, sensitive audit | ✅ deterministic typed classifications and bounded audit fields |
| 3.2 | Unit/in-process fakes | ✅ tests preceded production code | ✅ focused exit 0 | ✅ WSS-first, fallback, no downgrade, trust-before-start, offline readiness | ✅ narrow interfaces; no transport-internal edits |

### Slice 5 Work Unit Evidence

| Evidence | Result |
|---|---|
| Focused test command and exact result | `go test -count=1 ./internal/configuration ./internal/audit ./internal/security` — exit 0; 3 packages passed (configuration, audit, security). |
| Runtime harness command/scenario and exact result | N/A: deterministic in-process daemon/SSH counting fakes and local readiness; no live runtime, network, credentials, or IBM i boundary exists. |
| Verification | `go test -count=1 ./...` — exit 0; 22 test-bearing packages passed and 3 reported `[no test files]`; `go vet ./...` — exit 0; recorded formatting command `gofmt -w internal/configuration/resolver.go internal/configuration/resolver_test.go internal/configuration/readiness.go internal/audit/audit.go internal/audit/transport_test.go` — exit 0, no output; `git diff --check` — exit 0. The prior receipt did not record a separate `gofmt -d` invocation, so none is claimed. |
| Privacy/structural proof | Existing tests prove zero SSH fallback calls: `TestResolverWSSVersionAndFallbackTrustGate/supported` selects WSS; `TestResolverClassifiesDaemonAndNeverDowngradesTerminalFailures/identity` and `/credentials` leave trust calls at 0; the protocol/framing terminal path is represented by the default non-`ResolveError` classification and no-downgrade assertion; `TestResolverConsentCredentialTerminalityAndSanitizedAudit` proves consent failure leaves `startCalls` at 0; `TestResolverWSSVersionAndFallbackTrustGate/missing trust` blocks before SSH trust. No SSH fallback call occurs in these daemon-success or terminal cases. Resolver imports no SSH/JAR/Java/upload/cache packages; audit fields remain bounded classifications. |
| Rollback boundary | Revert `internal/configuration/resolver.go`, `resolver_test.go`, readiness fields, `internal/audit/audit.go`, `transport_test.go`, and only Slice 5 task/progress sections; retain Slices 1–4 and unrelated `.atl`/`tmp`. |

- Exact Slice 5 authored numstat: `internal/configuration/resolver.go` 187/0; `resolver_test.go` 112/0; `internal/configuration/readiness.go` 6/4; `internal/audit/audit.go` 37/1; `transport_test.go` 20/0; `tasks.md` 2/2; `apply-progress.md` 27/0. Total: **391 additions + 7 deletions = 398 changed lines**, under the normal 400-line guard. Unrelated `.atl` and `tmp` changes are excluded.
- Tasks 1.1–2.4 and 3.1–3.2 are checked; tasks 3.3–3.5 remain pending. Next recommended action: `apply`.

## Slice 6: `slice-6-tui-docs-final-checks`

- Delivery: auto-chain, feature-branch-chain; normal 400-line guard; tasks 3.3–3.5 only.
- Exact intended source/docs numstat: **231 additions + 18 deletions = 249 changed lines**, excluding pre-existing unrelated `.atl/` and `tmp/` worktree changes. Files: `internal/tui/mapepire_onboarding_step.go` 83/0; `internal/tui/mapepire_onboarding_step_test.go` 102/0; `internal/tui/model.go` 17/1; `internal/tui/profile_identity_step.go` 4/0; `internal/tui/wizard_viewport.go` 6/1; localization catalogs/registry 3/0; `docs/IBM_I_PROFILE_WIZARD.md` 16/16. Task/progress artifact updates add 30/3, for **261 additions + 21 deletions = 282 total**, within the 400-line guard.

### Slice 6 TDD Cycle Evidence

| Task | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|
| 3.3 | `go test -count=1 ./internal/tui` exit 1: missing Step 4/proof seams | Same command exit 0; 1 package passed | Exact pending copy, pre-auth probe count, no client installation, Step 8 connect/query counts, responsive NO_COLOR frames | Reused wizard shell/viewport and narrow injected interfaces |
| 3.4 | Covered by 3.3 RED before composition | TUI composition and approved docs complete | Step numbering, navigation, shared panel/footer/actions, focus, feedback, scrolling and all requested sizes pass existing plus new runtime tests | No resolver/transport/runtime internals changed |
| 3.5 | N/A: verification task; threat matrix is explicitly N/A | All final commands exit 0 | Offline deterministic fakes only; no live IBM i or credentials | Formatting and diff checks clean |

### Slice 6 Work Unit Evidence

| Evidence | Result |
|---|---|
| Focused test command and exact result | `go test -count=1 ./internal/tui` — exit 0; 1 package passed (`ok bac-nexus/internal/tui 8.329s`). Localization was also covered in the focused edit run: 2 packages passed. |
| Runtime harness command/scenario and exact result | Deterministic TUI `Update`/`View` harness with counting pre-auth and proof fakes; Step 4 at 120x40, 80x24, 40x16 with NO_COLOR and Step 8 connect/query proof — exit 0. No live runtime boundary, network, IBM i, credentials, Java, artifact, or SSH process was used. |
| Final commands | Prior final verification recorded `go test -count=1 ./...`, `go vet ./...`, `go build ./cmd/catalogspike`, and `git diff --check` as exit 0. Corrected formatting evidence and its exact command are recorded in the gatekeeper retry below; no command is newly claimed. |
| Zero-runtime proof | Step 4 counting probe is the only pre-auth call; it never installs a proof client. Daemon success and unavailable paths perform zero SSH/JAR/Java/upload/cache calls by construction and tests. Existing `.atl/` modifications and `tmp/ssh-test/compose.yaml` were pre-existing and untouched; no superseded-change path is present in the diff. |
| Rollback boundary | Revert `internal/tui/mapepire_onboarding_step.go`, its test, the Step 4 routing/viewport hunks in `internal/tui/model.go` and `wizard_viewport.go`, the three localization additions, and the documentation/task/progress Slice 6 sections; retain Slices 1–5 and unrelated `.atl`/`tmp` worktree state. |

- Tasks 1.1–3.5 are checked: **13/13 complete**.
- No threat-matrix RED tests apply: every listed row is explicitly N/A.
- Next recommended action: `verify`.

## Slice 6 gatekeeper corrective retry

- Scope: artifact-only correction; no source, tests, `tasks.md`, or runtime commands were rerun.
- Exact already-run formatting command: `gofmt -d internal/tui/model.go internal/tui/wizard_viewport.go internal/tui/mapepire_onboarding_step.go internal/tui/mapepire_onboarding_step_test.go internal/localization/localization.go` — exit `0`, no output.
- Complete touched Go-file list for Slice 6: `internal/tui/model.go`, `internal/tui/wizard_viewport.go`, `internal/tui/mapepire_onboarding_step.go`, `internal/tui/mapepire_onboarding_step_test.go`, `internal/tui/profile_identity_step.go`, and `internal/localization/localization.go`. `internal/tui/profile_identity_step.go` was formatted by the separately already-run command `gofmt -w internal/tui/profile_identity_step.go internal/tui/mapepire_onboarding_step_test.go` — no new formatting result is claimed here.
- Step 4 guide correction: it is local/pre-auth readiness only. It performs no JAR discovery/acquisition, Java, SSH process, artifact upload/cache, credentials, authentication, query, or fallback runtime. Daemon unavailability with policy-permitted fallback shows exactly `[OK] Mapepire detected — authentication pending` and defers all SSH fallback execution to Step 8.
- Preserved behavior: managed WSS `:8076` is preferred; fallback is fixed SSH `--single`; TLS and SSH trust are independent; no silent downgrade; Step 8 owns credentials, transport selection/fallback, authenticated `connect`, and optional bounded read-only query proof.
- No semantic tests, builds, vet, or runtime harnesses were rerun during this correction. Structural readback was limited to this artifact and `docs/IBM_I_PROFILE_WIZARD.md`.
- Corrected Slice 6 intended numstat, including this correction: **286 additions + 34 deletions = 320 changed lines**, excluding pre-existing unrelated `.atl/` and `tmp/` worktree changes; the 400-line guard remains satisfied.
- Next recommended action remains `verify` after gatekeeper acceptance.

## Bounded unmanaged remediation: `mapepire-dual-transport-verification`

- Scope: one native-authorized correction for failed evidence revision `sha256:6f4da1cf3a97fc9898530b894112d9cbfab54d275e0f82badf3a455e7aff7004`; no task checkboxes changed and the failed report remains immutable.
- RED: `go test -count=1 ./internal/configuration ./internal/audit ./internal/mapepire` — exit 1: missing managed probe and audit contract fields.
- GREEN: same focused command plus `./internal/tui` — exit 0; 4 packages passed, including loopback TLS `/version`, pinned fixture validation, audit allowlists, and production TUI default probe composition.
- REFACTOR: retained narrow probe/factory seams, bounded HTTPS body/deadline, TLS hostname/pin checks, typed resolution, and metadata-only audit fields; no fallback/runtime imports were added to the daemon probe.
- Corrections: concrete managed WSS `:8076` endpoint and HTTPS `/version` probe; TUI production default pre-auth probe factory; pinned fixture `2ef44166fcb515744fb922b49ed3673b2dac6b26`; audit `PolicyID`, `TrustOutcome`, and `Version` bounds.
- Work Unit Evidence: focused command exit 0 (4 packages); runtime harness exit 0 through `httptest.NewTLSServer` `/version` and deterministic TUI tests; rollback boundary is the remediation hunks in `internal/configuration/daemon.go`, `internal/configuration/{resolver.go,resolver_test.go}`, `internal/mapepire/{testdata,typed_protocol_test.go}`, `internal/mapepire/wss/http.go`, `internal/audit/{audit.go,transport_test.go}`, and `internal/tui/{model.go,mapepire_onboarding_step.go}`.
- Final checks: `go test -count=1 ./...`, `go vet ./...`, touched-file `gofmt -d`, external-temp `go build ./cmd/catalogspike` (removed), and `git diff --check` all exited 0. No IBM i, corporate network, credentials, Java, artifact transfer, or remote SSH process was used.

### Gatekeeper corrective retry evidence

- Provider bindings are intentionally distinct: failed report artifact hash `sha256:6f4da1cf3a97fc9898530b894112d9cbfab54d275e0f82badf3a455e7aff7004`; native remediation-state reference `sha256:34150b0df3ba4972ff4830654be86731e2fafccb8542db04e078a8a0e2924e4e`. The provider acquire/settle binding uses the former. Native status has empty lineage and RDD is disabled; no lineage, generation, or fix-batch envelope is invented.
- Production composition cases: `TestProductionModelComposesIndependentTLSAndSSHReadiness` proves independent Step 3 seams and render purity; `TestMapepireStep4IsPreAuthAndUsesExactPendingCopy` and `TestMapepireStep4UnavailableDoesNotInvokeFallbackRuntime` prove explicit Step 4 effect/no fallback; `TestStep8ProofIsExplicitlyOwnedAndCredentialFreeBeforeInvocation` and `TestStep8AloneProvesConnectAndQuery` prove the owned Step 8 connect/query seam.
- Daemon/resolver cases: `TestManagedDaemonProbeReadsVersionWithoutCredentials`, `TestManagedDaemonProbeUsesBoundedHTTPSVersionEndpoint`, `TestResolverWSSVersionAndFallbackTrustGate`, and `TestResolverClassifiesDaemonAndNeverDowngradesTerminalFailures` cover managed `:8076`, `/version`, trust gates, terminality, and zero fallback calls.
- Protocol/audit cases: `TestPinnedProtocolFixtureRevisionIsValid` decodes fixture `protocol-2ef44166fcb515744fb922b49ed3673b2dac6b26.json`; `TestTransportAuditCarriesBoundedPolicyAndTrustMetadata` covers allowlisted `PolicyID=verified-readonly`, `TrustOutcome` values `verified|untrusted|blocked`, and `Version` length bounds. Sensitive endpoint, host, path, URL, credentials, raw errors, certificates, SQL, source, payload, and user fields are excluded.
- Exact touched Go format check: `gofmt -d internal/audit/audit.go internal/audit/transport_test.go internal/configuration/daemon.go internal/configuration/resolver.go internal/configuration/resolver_test.go internal/mapepire/typed_protocol_test.go internal/mapepire/wss/http.go internal/tui/model.go internal/tui/mapepire_onboarding_step.go internal/tui/mapepire_onboarding_step_test.go internal/tui/profile_identity_step.go` — exit 0, no output. Build used external temporary output `C:\Users\David\AppData\Local\Temp\opencode\catalogspike-remediation.exe`, then removed.
- Exact final remediation numstat: **282 additions + 5 deletions = 287 changed lines**, excluding pre-existing `.atl/`, `tmp/`, and immutable failed report. The 400-line limit remains satisfied.
- Failed `verify-report.md` remains unchanged. Next action exactly: settle this same attempt with provider-required `--remediates-evidence-revision sha256:6f4da1cf3a97fc9898530b894112d9cbfab54d275e0f82badf3a455e7aff7004`, then launch FRESH independent `sdd-verify`; never archive directly.

## Slice 1 Foundation: `step8-foundation`
- Scope: production tasks 4.1–4.3 only; strict TDD; all three complete; remaining 15 production tasks pending.
- RED: `go test -count=1 ./internal/configuration ./internal/profile ./internal/credential` exit 1 before implementation: missing Step8 symbols.
- GREEN: same command exit 0; 3 packages passed. REFACTOR: typed mappings, narrow contracts, v3 prompt/keyring migration, fixed proof metadata, bounded marker.
- Tests: `TestStep8DecisionReasonsAreExhaustiveAndFailClosed` covers all decisions/reasons and unknowns; `TestStep8ResultClassesMapOnlyKnownValues` covers every public terminal class and distinct trust/credentials/downgrade classes; `TestStep8ResultInvariantsFailClosed` covers eligible/terminal invariants.
- Security tests: `TestStep8SavedProfileAndCredentialContract` covers saved-profile gate, prompt/keyring, invalid mode, `MigrateToV3`, and exact `ibmi/<profile>`; `TestStep8CredentialFailuresAndProofMetadataFailClosed` covers prompt/keyring unavailable/denied/not-found, no string guessing, fixed `values-1-v1`, one-row metadata, and no SQL/text/row fields.
- Marker: `TestStep8MarkerIsBoundedAndInvalidated` proves bounded `{schemaVersion,atUnixMs,outcome,proofRevision}`, unchanged validity, endpoint/policy/trust invalidation, and never-readiness behavior.
- Work Unit Evidence: focused command exit 0, 3 packages; runtime harness N/A because this foundation has no runtime boundary; no IBM i/network/secret/process was used.
- Full `go test -count=1 ./...` exit 0: 23 test-bearing packages, 3 no-test; `go vet ./...` exit 0; `gofmt -d internal/configuration/step8.go internal/configuration/step8_test.go internal/profile/profile.go internal/credential/key.go` exit 0/no output; intended-path `git diff --check` exit 0.
- Structural readback: `internal/configuration/{step8.go,step8_test.go}`, `internal/profile/profile.go`, and `internal/credential/key.go` import no WSS/SSH/remote/JAR/Java/upload/runtime/Bubble Tea/TUI/audit/generic SQL/secret persistence.
- Security: schema-v3 saved profile only; prompt/keyring only; legacy vault requires explicit migration; unknowns fail closed; marker never readiness/bypass; fixed proof exposes metadata only.
- Exact intended candidate: **371 additions + 13 deletions = 384 changed lines**, excluding unrelated `.atl`/`tmp`; strictly below 400 after this receipt edit. Rollback: revert the four foundation files and this Slice 1 receipt/task evidence.
- Immutable verify hash remains `sha256:6f4da1cf3a97fc9898530b894112d9cbfab54d275e0f82badf3a455e7aff7004`; next: `apply` Slice 2 authenticated WSS; no-code-change confirmation.

## Slice 2: `slice-2-authenticated-wss`

- Scope: production tasks 5.1–5.3 only; strict TDD; auto-chain, feature-branch-chain; normal 400-line ceiling. SSH fallback, production composition, TUI, audit, and final verification remain out of scope.
- RED: `go test -count=1 ./internal/mapepire -run 'TestFixedProof'` — exit 1 before implementation: missing authenticated proof API, credential fields, and fixed-proof constants. `go test -count=1 ./internal/mapepire/wss -run 'TestAuthenticatedFactory'` — exit 1 before implementation: missing WSS factory and session API.
- GREEN: `go test -count=1 ./internal/mapepire -run 'TestFixedProof'` — exit 0; fixed proof authenticates first, sends exactly `VALUES 1`, returns metadata only, and cancellation closes the transport. `go test -count=1 ./internal/mapepire/wss -run 'TestAuthenticatedFactory'` — exit 0; loopback TLS/WSS factory/session proves credential-only connect and close lifecycle.
- REFACTOR: application and password fields are valid only on `connect`; the WSS factory opens without credentials; fixed proof uses the release-owned SQL/revision and never returns SQL, parameters, rows, or bytes. Existing trusted WSS framing, TLS identity, bounds, cancellation, and idempotent close remain reused.

### Slice 2 TDD Cycle Evidence

| Task | Test layer | Safety net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|
| 5.1 | Loopback WSS and typed-session tests | Existing `configuration`, `mapepire`, and `wss` tests passed | ✅ focused compile/test exit 1 before new APIs | ✅ focused tests exit 0 | ✅ supported proof and cancelled proof paths | ✅ explicit factory-open/authenticate/prove separation |
| 5.2 | Typed protocol plus WSS loopback | N/A for new session file; existing package safety net passed | ✅ tests referenced absent factory/session and auth request | ✅ focused tests exit 0 | ✅ custom application, credential-only connect, fixed proof, close | ✅ narrow `Factory`/`Session` boundary |
| 5.3 | Loopback runtime and structural package scan | Existing WSS bounds/cancellation tests passed | ✅ cancellation and terminal behavior were represented before implementation | ✅ focused and full suites exit 0 | ✅ cancellation, frame/limit terminality, idempotent close, metadata-only result | ✅ no SSH/remote/artifact imports or calls in WSS package |

### Slice 2 Work Unit Evidence

| Evidence | Result |
|---|---|
| Focused test command and exact result | `go test -count=1 ./internal/configuration ./internal/mapepire ./internal/mapepire/wss` — exit 0; 3 packages passed. |
| Runtime harness command/scenario and exact result | Same focused command exercises `httptest.NewTLSServer` WSS loopback through `Factory.Open` and `Session.Prove` — exit 0; no IBM i, corporate network, Java, SSH process, artifact, or real credential was used. |
| Full validation | `go test -count=1 ./...` — exit 0; 23 test-bearing packages passed and 3 reported `[no test files]`; `go vet ./...` — exit 0. |
| Formatting and diff | `gofmt -d internal/mapepire/protocol.go internal/mapepire/typed_session.go internal/mapepire/typed_protocol_test.go internal/mapepire/wss/session.go internal/mapepire/wss/wss_test.go internal/mapepire/wss/wss.go internal/mapepire/wss/http.go` — exit 0, no output; `git diff --check` — exit 0. |
| Security / cleanup proof | Auth fields are rejected on every non-connect operation; proof request has no credentials; successful proof sends `sqlclose` then `exit`; cancellation and all transport terminal paths close the WSS session. Structural scan found no SSH fallback, remote, artifact, Java, or upload imports/calls in `internal/mapepire/wss`. |
| Changed-line budget | Intended Slice 2 files plus task/progress evidence: **under 400 additions+deletions**; unrelated pre-existing `.atl/`, `tmp/`, and `internal/credential/keyring_store.go` changes excluded and untouched. |
| Rollback boundary | Revert `internal/mapepire/protocol.go`, `internal/mapepire/typed_session.go`, `internal/mapepire/typed_protocol_test.go`, `internal/mapepire/wss/session.go`, `internal/mapepire/wss/wss_test.go`, and only the Slice 2 task/progress sections; retain Slice 1 and unrelated worktree changes. |

- Exact intended Slice 2 candidate: **267 additions + 18 deletions = 285 changed lines**, including source, tests, and task/progress evidence; unrelated pre-existing `.atl/`, `tmp/`, and `internal/credential/keyring_store.go` changes are excluded.

### Slice 2 completion state

- Tasks 1.1–1.4, 5.1–5.3 are checked for this apply history; later SSH, composition, TUI, audit, and final verification tasks remain pending in the current OpenSpec plan.
- No native attempt authority was acquired or settled by this executor. No commit, push, PR, staging, `.atl/`, or `tmp/` edit was performed.
- Next recommended action: `apply` the next assigned slice; this receipt does not claim final verification or archive readiness.

## Slice 6A: `step8-ssh-gates`

- Scope: production tasks 6.1–6.2 only; strict TDD; auto-chain, feature-branch-chain. No SSH client/runtime, artifact, Java, upload, launch, proof, composition, TUI, audit, or documentation behavior was added.
- Safety net: `go test -count=1 ./internal/configuration ./internal/security` — exit `0`; 2 packages passed before production edits.
- RED: `go test -count=1 ./internal/configuration -run 'TestPostObservationGate'` — exit `1` before production implementation; compiler reported missing `PostObservationGate` and `Observation` symbols.
- Additional RED: `go test -count=1 ./internal/configuration -run 'TestPostObservationGateZeroesCredentialOnTerminalRetrievalFailure'` — exit `1`; a returned credential remained non-zero after terminal retrieval failure.
- GREEN: both focused named commands exited `0` after adding the post-observation gate and credential zeroization.
- REFACTOR: extracted exact observation-terminal mapping and one credential-zeroization helper; focused named tests remained green.

### Slice 6A TDD Cycle Evidence

| Task | Test layer | Safety net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|
| 6.1 | Unit/counting fakes | `./internal/configuration ./internal/security` exit `0` | ✅ missing gate/observation symbols, exit `1` | ✅ named gate tests exit `0` | ✅ eligible, WSS, every terminal/unknown observation, invalid profile, policy, trust, consent, credential paths and zero calls | ✅ shared deterministic fakes |
| 6.2 | Unit/counting fakes | Same safety net | ✅ credential-error zeroization failure, exit `1` | ✅ named gate tests exit `0` | ✅ ordered policy → trust → consent → credential, terminal classes, credential zeroization | ✅ terminal mapper and zeroization helper |

### Slice 6A Work Unit Evidence

| Evidence | Result |
|---|---|
| Focused test command and exact result | `go test -count=1 ./internal/configuration ./internal/security` — exit `0`; `ok bac-nexus/internal/configuration 2.570s`, `ok bac-nexus/internal/security 1.090s`; 2 packages passed. |
| Runtime harness command/scenario and exact result | The focused command exercised deterministic in-process counting policy/trust/credential fakes — exit `0`; no remote/runtime boundary exists in 6A, and no IBM i, network, SSH, process, Java, artifact, upload, or credential store was contacted. |
| Full/static/format/diff validation | `go test -count=1 ./...` exit `0`: 22 test-bearing packages passed and 3 reported `[no test files]`; `go vet ./...` exit `0`; `gofmt -w` then `gofmt -d internal/configuration/step8.go internal/configuration/step8_test.go` exit `0` with no output; `git diff --check` exit `0`. |
| Order and zero-call/security proof | Tests prove saved request/profile and observation validation before all gates; only the five eligible reasons can reach policy; policy and SSH trust precede consent, which precedes `CredentialProvider.Get`; invalid/WSS/terminal/unknown observations make zero gate calls; policy/trust/consent blocks prevent credential retrieval; credential terminality ends at the gate. Credential buffers are zeroed on success and retrieval failure. |
| Structural proof | `step8.go` and `step8_test.go` import no `remote`, SSH framing, artifact, Java, upload, launch, process, shell, SQL, or download package and contain no remote/runtime calls. The only artifact/Java/upload/launch occurrences are pre-existing typed `ResultClass` constants from Foundation. |
| Cleanup/process evidence | No runtime resource or process can be acquired in this slice; credential bytes are zeroed before `Apply` returns on both success and retrieval failure. |
| Rollback boundary | Revert only `internal/configuration/step8.go`, `internal/configuration/step8_test.go`, and the 6A checkbox/progress edits; retain all earlier history, pending 6B–9 work, and unrelated `.atl`, `tmp`, and credential-store worktree changes. |

- Exact Slice 6A authored numstat: `internal/configuration/step8.go` 118/0; `internal/configuration/step8_test.go` 169/0; `tasks.md` 3/3; `apply-progress.md` 32/0. Total: **322 additions + 3 deletions = 325 changed lines**, under the 400-line hard budget.
- Completion state: tasks 6.1–6.2 are checked; cumulative task total is **31 total = 16 completed + 15 pending**. No native runtime authority was acquired, settled, reset, or changed; no commit, stage, push, PR, `.atl`, or `tmp` edit occurred.
- Next recommended action: apply Slice 6B runtime only on `feature/step8-ssh-runtime`; it must add the runtime seam separately and preserve this gate's ordering/terminal contract.

## Slice 6B: `step8-ssh-runtime`

- Scope: production tasks 6.3–6.4 only; strict TDD; auto-chain, feature-branch-chain. It consumes only a successful 6A `ssh_eligible` admission and does not repeat eligibility, policy, trust, consent, or credential ordering.
- RED: `go test -count=1 ./internal/configuration -run 'TestSSHRuntimeFactory'` — exit `1` before production implementation: `SSHRuntimeFactory`, `SSHRuntimeClient`, and `SSHRuntimeOperationTimeout` were undefined.
- GREEN: the same focused command — exit `0`; artifact rejection occurs before the injected production-owned `remote.Dial` seam, the runtime uses a bounded 15-second operation context, Java and upload failures map to exact terminal classes, and an acquired client closes once.
- REFACTOR: extracted `mapepirestdio.ValidateJavaHome` from the existing fixed command policy and exposed only `remote.Client.RemoteFiles()` to the artifact uploader; no launch, process, proof, SQL, shell, or download API was added.

### Slice 6B TDD Cycle Evidence

| Task | Test layer | Safety net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|
| 6.3 | Unit/counting SSH/artifact/Java/upload fakes | `go test -count=1 ./internal/configuration ./internal/remote ./internal/connectors/ibmi/mapepirestdio` — exit `0`, 3 packages | ✅ focused compile failure, exit `1` | ✅ focused command, exit `0` | ✅ six unsafe artifact states; Java and upload failures; dial deadline/timeout; close count | ✅ narrow fakes over production runtime seam |
| 6.4 | Unit/counting fakes | Same safety net | ✅ factory, client, timeout, and seams absent | ✅ focused command, exit `0` | ✅ local verification before dial, bounded context propagation, typed artifact/java/upload/timeout classes, deterministic rollback | ✅ reused existing `VerifyServerJAR`, `EnsureServerJAR`, and remote close ownership |

### Slice 6B Work Unit Evidence

| Evidence | Result |
|---|---|
| Focused test command and exact result | `go test -count=1 ./internal/configuration ./internal/remote ./internal/connectors/ibmi/mapepirestdio` — exit `0`; 3 packages passed. |
| Runtime harness command/scenario and exact result | `go test -count=1 ./internal/configuration -run 'TestSSHRuntimeFactory'` — exit `0`; deterministic counting SSH/artifact/Java/upload fakes prove unpinned/corrupt/partial/changed/latest/unverified local artifact rejection before dial, bounded operation contexts, Java/upload terminal classes, and close-on-partial-failure. No IBM i, corporate network, actual SSH/Java process, credential store, or download. |
| Full/static/format/diff validation | `go test -count=1 ./...` — exit `0`: 22 test-bearing packages passed and 4 reported `[no test files]`; `go vet ./...` — exit `0`; final `gofmt -w` then `gofmt -d internal/configuration/ssh_runtime.go internal/configuration/ssh_runtime_test.go internal/remote/ssh.go internal/connectors/ibmi/mapepirestdio/policy.go` — exit `0`, no output; `git diff --check` — exit `0`. |
| Artifact-before-mutation / typed classifications | `VerifyServerJAR` runs before `Dial`; rejected unsafe artifacts return `artifact_failure` with zero dials. Dial timeout maps to `operation_timeout`; Java failure maps to `java_failure`; bounded uploader failure maps to `upload_failure`; close failure is `cleanup_failure`. Existing `EnsureServerJAR` retains checksum, 64 MiB, exclusive temporary upload, remote verification, and rollback semantics. |
| Cleanup/process evidence | Any client acquired after local verification is closed exactly once on Java/upload failure; credentials are zeroed on return. This slice starts no process/channel, launches no `--single`, runs no proof/SQL, and exposes no generic shell/SFTP/download surface. |
| Rollback boundary | Revert `internal/configuration/ssh_runtime.go`, `internal/configuration/ssh_runtime_test.go`, the narrow `RemoteFiles` adapter in `internal/remote/ssh.go`, `ValidateJavaHome` extraction in `internal/connectors/ibmi/mapepirestdio/policy.go`, and only the 6B task/progress edits. Retain 6A gates, all prior history, pending 6C–9 work, and unrelated `.atl`, `tmp`, and credential-store worktree changes. |

- Exact Slice 6B authored numstat: `internal/configuration/ssh_runtime.go` 126/0; `internal/configuration/ssh_runtime_test.go` 101/0; `internal/connectors/ibmi/mapepirestdio/policy.go` 13/3; `internal/remote/ssh.go` 3/0; `tasks.md` 3/3; `apply-progress.md` 29/0. Total: **275 additions + 6 deletions = 281 changed lines**, under the 400-line hard budget. It excludes unrelated `.atl`, `tmp`, and `internal/credential/keyring_store.go` changes.
- Completion state: tasks 6.3–6.4 are checked; cumulative task total is **31 total = 18 completed + 13 pending**. No native runtime authority was acquired, settled, reset, or changed; no commit, stage, push, PR, `.atl`, or `tmp` edit occurred.
- Next recommended action: apply Slice 6C proof only on `feature/step8-ssh-proof`; keep this runtime factory free of launch, process channel, typed SSH session, `VALUES 1`, SQL, proof cleanup, or retry behavior.

## Slice 6C: `step8-ssh-proof`

- Scope: production tasks 6.5–6.6 only; strict TDD; auto-chain, feature-branch-chain. It consumes the admitted 6A result and acquired 6B runtime without repeating gates or changing acquisition/rollback behavior.
- RED: `go test -count=1 ./internal/configuration -run 'TestSSHRuntimeProve'` — exit `1` before production implementation: `SSHRuntime.Prove` was undefined.
- GREEN: the same named command — exit `0`; the admitted runtime can invoke only the private verified upload handle through `remote.FixedMapepireProof`, which calls the existing fixed `StartMapepireTransport` → `StartMapepire` → `BuildCommand` path.
- REFACTOR: retained a configuration-facing remote proof result and typed remote failure stage rather than importing Mapepire into configuration or exposing a channel, command, path, SQL, shell, SFTP, download, or retry API.

### Slice 6C TDD Cycle Evidence

| Task | Test layer | Safety net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|
| 6.5 | Unit / deterministic typed-session fake | `go test -count=1 ./internal/configuration ./internal/mapepire ./internal/mapepire/sshstdio` — exit `0`; 3 packages passed before edits | ✅ named test compile failure, exit `1` | ✅ named test exit `0` | ✅ connect precedes fixed `VALUES 1`; credentials occur only on connect; metadata contains only row count/revision; artifact, launch, session, proof, cancellation, limit, protocol, and unknown classifications are asserted | ✅ kept fixed request and error contracts narrow |
| 6.6 | Unit / deterministic typed-session fake | Same safety net | ✅ absent `SSHRuntime.Prove` API before implementation | ✅ focused 3-package command exit `0` | ✅ remote fixed proof routes through fixed single-mode launcher and typed session; no caller command/path/SQL input | ✅ private verified remote-artifact handle and typed remote stage |

### Slice 6C Work Unit Evidence

| Evidence | Result |
|---|---|
| Focused test command and exact result | `go test -count=1 ./internal/configuration ./internal/mapepire ./internal/mapepire/sshstdio` — exit `0`; `ok bac-nexus/internal/configuration 2.618s`, `ok bac-nexus/internal/mapepire 15.981s`, `ok bac-nexus/internal/mapepire/sshstdio 0.859s`; 3 packages passed. |
| Runtime harness command/scenario and exact result | The focused command exercises `TestSSHRuntimeProveUsesOnlyFixedSingleSessionAndMetadata` with a deterministic typed-session fake: connect → `VALUES 1` → `sqlclose` → `exit`; exit `0`. No IBM i, corporate network, actual SSH/Java process, credential store, or download was used. |
| Full/static/format/diff validation | `go test -count=1 ./...` — exit `0`: 22 test-bearing packages passed and 4 reported `[no test files]`; `go vet ./...` — exit `0`; final `gofmt -w` then `gofmt -d internal/configuration/ssh_runtime.go internal/configuration/ssh_runtime_test.go internal/configuration/ssh_proof_test.go internal/remote/ssh.go` — exit `0`, no output; `git diff --check` — exit `0`. |
| Fixed-proof and terminal proof | The test asserts launch policy is constructed from the runtime's private verified remote handle with consent; requests are exactly connect, `prepare_sql_execute` with `VALUES 1`/one row, `sqlclose`, and exit. It rejects credential fields after connect and returns only `{Rows, ProofRevision}`. Existing 6B unsafe-artifact test covers `artifact_failure`; 6C covers exact launch/session/proof and typed cancellation/limit/protocol/unknown terminal mapping. |
| Structural proof | `SSHRuntime` keeps the uploaded path private; `Prove` has no command, path, endpoint, SQL, shell, SFTP, download, or retry parameter. `remote.FixedMapepireProof` reaches only `StartMapepireTransport` → `StartMapepire` → existing `BuildCommand`, whose sole command is the closed allowlisted `--single` form. No alternate transport is invoked. |
| Cleanup/process evidence | The typed client's fixed proof continues to close cursor then exit and closes its transport. This slice introduces no new process cleanup choreography or credential-zero-after-settlement integration; those exhaustive traces remain 6D. |
| Rollback boundary | Revert `internal/configuration/ssh_runtime.go`, `ssh_runtime_test.go`, `ssh_proof_test.go`, the `internal/remote/ssh.go` fixed-proof adapter, and only the 6C task/progress edits; retain 6A gates, 6B acquisition/rollback, later 6D–9 work, and unrelated `.atl`, `tmp`, and credential-store state. |

- Exact Slice 6C authored numstat: `internal/configuration/ssh_runtime.go` 55/3; `ssh_runtime_test.go` 4/0; `ssh_proof_test.go` 168/0; `internal/remote/ssh.go` 72/0; `tasks.md` 3/3; `apply-progress.md` 30/0. Total: **332 additions + 6 deletions = 338 changed lines**, under the 400-line hard budget. Unrelated `.atl`, `tmp`, and `internal/credential/keyring_store.go` changes are excluded and untouched.
- Completion state: tasks 6.5–6.6 are checked; cumulative task total is **31 total = 20 completed + 11 pending**. No native runtime authority was acquired, settled, reset, or changed; no commit, stage, push, PR, `.atl`, or `tmp` edit occurred.
- Next recommended action: apply Slice 6D cleanup only on `feature/step8-ssh-cleanup`; do not add composition, TUI, audit/docs, or final verification.

## Slice 6D: `step8-ssh-cleanup`

- Scope: production tasks 6.7–6.8 only; strict TDD; auto-chain, feature-branch-chain. Preserves the committed 6A admission gates, 6B acquisition/rollback, and 6C fixed proof. No composition, TUI, audit/docs, final verify, artifact capability, generic remote API, native runtime authority, `.atl`, `tmp`, or credential-store changes.
- RED: `go test -count=1 ./internal/configuration -run '^TestSSHRuntimeProveSettlesClientBeforeZeroingCredentials$'` — exit `1`; success, proof-failure, and cancellation paths returned `Cleanup:false`, and the cleanup-failure path settled no client. `go test -count=1 ./internal/configuration -run '^TestSSHRuntimeFactoryKeepsPrimaryFailureAndAssignsUniqueTraceIDs$'` — exit `1`; cleanup failure incorrectly replaced the Java primary classification with `cleanup_failure`.
- GREEN: `go test -count=1 ./internal/configuration -run '^(TestSSHRuntimeProveSettlesClientBeforeZeroingCredentials|TestSSHRuntimeFactoryKeepsPrimaryFailureAndAssignsUniqueTraceIDs)$'` — exit `0`; every proven runtime client settles exactly once, credentials are zeroed after settlement on success/failure/cancellation, cleanup status remains sanitized, primary failure remains typed, and factory-acquired trace IDs are distinct.
- REFACTOR: `SSHRuntime` owns a private atomic trace ID and mutex-guarded settlement. `Prove` introduces a bounded 60-second proof context and uses deferred settlement before credential zeroization; it exposes neither trace IDs nor cleanup errors in `Step8Result`.

### Slice 6D TDD Cycle Evidence

| Task | Test layer | Safety net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|
| 6.7 | Unit / deterministic acquire-settle fakes | `go test -count=1 ./internal/configuration ./internal/remote ./internal/mapepire/sshstdio ./internal/connectors/ibmi/mapepirestdio` — exit `0`; 4 packages | ✅ both named lifecycle/mapping tests failed before production changes (exit `1`) | ✅ named lifecycle tests exit `0` | ✅ success, proof failure, cancellation, cleanup failure, and distinct acquired-runtime traces | ✅ closed consumer-facing interface retained; no generic primitive added |
| 6.8 | Unit / bounded fake typed channel | Same safety net | ✅ cleanup/zeroization behavior failed before implementation | ✅ named lifecycle tests exit `0` | ✅ client settles once even after a second `Close`; cleanup failure preserves the primary class and exposes only sanitized cleanup state | ✅ deferred LIFO order is proof/session cleanup → SSH client settlement → zero credential buffer |

### Slice 6D Work Unit Evidence

| Evidence | Result |
|---|---|
| Focused test command and exact result | `go test -count=1 ./internal/configuration ./internal/remote ./internal/mapepire/sshstdio ./internal/connectors/ibmi/mapepirestdio` — exit `0`; 4 packages passed. |
| Runtime harness command/scenario and exact result | `go test -count=1 ./internal/configuration -run '^(TestSSHRuntimeProveSettlesClientBeforeZeroingCredentials|TestSSHRuntimeFactoryKeepsPrimaryFailureAndAssignsUniqueTraceIDs)$'` — exit `0`; deterministic counting client plus bounded fake typed channel prove proof/session cleanup precedes client settlement, exact-once settlement, cancellation, typed primary result preservation, and credential zeroization. No IBM i, corporate network, actual SSH/Java process, real credential store, or download. |
| Full/static/format/diff validation | `go test -count=1 ./...` — exit `0`: 22 test-bearing packages passed and 4 reported `[no test files]`; `go vet ./...` — exit `0`; final `gofmt -w` then `gofmt -d internal/configuration/ssh_runtime.go internal/configuration/ssh_runtime_test.go internal/configuration/ssh_proof_test.go` — exit `0`, no output; `git diff --check` — exit `0`. |
| Terminal, rejection, and no-downgrade proof | Existing exhaustive `IsTerminalResult`/Step 8 mapping tests cover every defined public terminal class and unknown fail-closed behavior. The 6D lifecycle matrix covers proof timeout/cancellation/limit/protocol/framing/launch/session/proof paths through the fixed proof boundary. `SSHRuntimeClient` remains exactly `Close`, bounded `RemoteFiles`, and `FixedMapepireProof`; it accepts no shell, command, SQL, download, retry, or alternate-transport input. |
| Rollback boundary | Revert only `internal/configuration/ssh_runtime.go`, `internal/configuration/ssh_runtime_test.go`, `internal/configuration/ssh_proof_test.go`, and the 6D checkbox/progress additions; retain committed 6A–6C behavior, Phase 7+ pending work, and unrelated `.atl`, `tmp`, and `internal/credential/keyring_store.go` state. |

- Exact Slice 6D authored numstat: `internal/configuration/ssh_runtime.go` 47/16; `ssh_runtime_test.go` 39/2; `ssh_proof_test.go` 51/1; `tasks.md` 3/3; `apply-progress.md` 28/0. Total: **168 additions + 22 deletions = 190 changed lines**, under the 400-line hard budget and excluding unrelated `.atl`, `tmp`, and `internal/credential/keyring_store.go` changes.
- Completion state: tasks 6.7–6.8 are checked; cumulative task total is **31 total = 22 completed + 9 pending**. No native runtime authority was acquired, settled, reset, or changed; no commit, stage, push, PR, `.atl`, or `tmp` edit occurred.
- Next recommended action: apply Phase 7 composition only on `feature/step8-compose`.

## Phase 7A: `step8-orchestrator-wss`

- Scope: task 7.1 only; strict TDD; auto-chain, feature-branch-chain. The service owns saved-profile validation, marker clearing, credential-free pre-auth observation, WSS proof, cleanup, zeroization, sanitized audit/marker; it emits SSH eligibility only and never invokes fallback.
- RED: `go test -count=1 ./internal/configuration -run '^TestStep8Service'` — exit `1` before production code: missing `Step8WSSSession`, `Step8AuditEvent`, and service contracts.
- GREEN: same command — exit `0`; 1 package passed. The deterministic application-service harness proves `clear → observe → credential → open → prove → close → zero → audit → marker` on valid WSS success.
- REFACTOR: narrow configuration-owned WSS, marker, and audit interfaces retain adapter trust/wire ownership and expose no endpoint, SQL, rows, secret, raw error, SSH, artifact, Java, upload, process, or fallback surface.

### Phase 7A TDD Cycle Evidence

| Task | Test layer | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|
| 7.1 | Deterministic application-service fake session | ✅ exit `1`, missing service symbols | ✅ exit `0` | ✅ valid WSS, terminal historical-marker, invalid profile, proof-failure/marker suppression | ✅ narrow typed interfaces |

### Phase 7A Work Unit Evidence

| Evidence | Result |
|---|---|
| Focused test command and exact result | `go test -count=1 ./internal/configuration ./internal/mapepire ./internal/mapepire/wss` — exit `0`; 3 packages passed. |
| Runtime harness command/scenario and exact result | `go test -count=1 ./internal/configuration -run '^TestStep8Service'` — exit `0`; counting service fakes prove WSS order, cleanup-before-zeroization-before-audit, marker write only after valid proof, and terminal marker non-readiness. No IBM i/network/SSH/process/credential store. |
| Rollback boundary | Revert `internal/configuration/step8_service.go`, `internal/configuration/step8_service_test.go`, and only this task/progress evidence; retain 6A–6D and unrelated `.atl`, `tmp`, and keyring-store state. |

- Final checks: `go test -count=1 ./...` exited `0` with 22 test-bearing packages passed and 4 packages reporting `[no test files]`; `go vet ./...` exited `0`. Source normalization used `gofmt -w "internal/configuration/step8_service.go" "internal/configuration/step8_service_test.go"`; check-only validation used `gofmt -d "internal/configuration/step8_service.go" "internal/configuration/step8_service_test.go"` and exited `0` with no output. `git diff --check` exited `0`.
- Zero-fallback proof: the service has no SSH/artifact/Java/upload/process/fallback dependency or interface; the WSS-success trace contains only WSS operations. Terminal/unknown observations return before credential retrieval; eligible observations return the typed `ssh_eligible` continuation without execution.
- Exact authored numstat: **364 additions + 2 deletions = 366 changed lines**, excluding unrelated `.atl`, `tmp`, and `internal/credential/keyring_store.go` state. Task aggregate: **31 total = 23 completed + 8 pending**.
- No native authority was acquired, settled, reset, or changed; no commit, stage, push, or PR occurred. Next recommended action: apply 7B only on `feature/step8-orchestrator-ssh`.

## Phase 7B: `step8-orchestrator-ssh`

- Scope: task 7.2 only; strict TDD; auto-chain, feature-branch-chain. Extends the existing application-owned WSS-first service through the existing 6A admission gate and 6B–6D runtime/proof/cleanup boundary. No `cmd/nexus` wiring, TUI, audit schema/docs, live IBM i, generic remote surface, native authority, `.atl`, `tmp`, or credential-store change.
- RED: `go test -count=1 ./internal/configuration -run '^(TestStep8ServiceFallsBackForExactlyFiveEligibleReasonsWithGateCredential|TestStep8ServiceNeverFallsBackForWSSOrTerminalObservations)$'` — exit `1` before production changes because `Step8Service` lacked `Gate` and `SSH` fields. `go test -count=1 ./internal/configuration -run '^TestSSHRuntimeFactoryRetainsGateCredentialUntilProofSettlement$'` then exited `1` because runtime acquisition zeroized the credential before fixed proof.
- GREEN: the same named command — exit `0`; the five eligible observations enter the SSH path and configured WSS/terminal/unknown observations make zero gate/runtime calls.
- REFACTOR: `PostObservationGate.ApplyWithCredential` retains the one opaque credential reference inside configuration through SSH runtime settlement. The service owns no command/path/SQL/download input and invokes only the pre-existing fixed runtime proof.

### Phase 7B TDD Cycle Evidence

| Task | Test layer | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|
| 7.2 | Deterministic application-service SSH/process/artifact/Java/upload boundary fakes | ✅ exit `1`: missing `Step8Service.Gate` and `.SSH`; runtime initially zeroized before proof | ✅ named commands exit `0` | ✅ all five exact eligible reasons; WSS/identity/protocol/malformed/downgrade/cancel/operation/limit/credential/authentication/authorization/framing/cleanup/proof timeout and unknown terminal observations make zero fallback calls; primary proof failure survives cleanup failure | ✅ one gate callback keeps a single credential reference through `Open` and fixed proof; existing 6B–6D runtime stays closed-surface |

### Phase 7B Work Unit Evidence

| Evidence | Result |
|---|---|
| Focused test command and exact result | `go test -count=1 ./internal/configuration ./internal/remote ./internal/connectors/ibmi/mapepirestdio` — exit `0`; 3 packages passed: configuration, remote, and mapepirestdio. |
| Runtime harness command/scenario and exact result | `go test -count=1 ./internal/configuration -run '^(TestStep8ServiceFallsBackForExactlyFiveEligibleReasonsWithGateCredential|TestStep8ServiceNeverFallsBackForWSSOrTerminalObservations|TestStep8ServiceFallbackKeepsPrimaryFailureAndSuppressesMarker)$'` — exit `0`; deterministic counting gate/SSH runtime/client fakes prove the exact five-class fallback set, policy → SSH trust → consent → one credential → runtime → fixed proof order, pointer-identical credential use for dial/proof, LIFO settlement before zeroization, marker only after proof plus cleanup, and zero fallback calls otherwise. No IBM i, corporate network, actual SSH/Java process, credential store, or download was used. |
| Rollback boundary | Revert only `internal/configuration/step8.go`, `internal/configuration/step8_service.go`, `internal/configuration/step8_service_test.go`, and this 7.2 task/progress evidence; retain 6A–6D, 7.1, pending 7.3–9, and unrelated `.atl`, `tmp`, and `internal/credential/keyring_store.go` state. |

- Validation: `go test -count=1 ./...` exit `0` (22 test-bearing packages passed; 4 reported `[no test files]`); `go vet ./...` exit `0`; final `gofmt -w` then `gofmt -d internal/configuration/step8.go internal/configuration/step8_service.go internal/configuration/step8_service_test.go` exit `0` with no output; `git diff --check` exit `0`.
- Structural/call-order proof: `DecisionForReason` has exactly five SSH-eligible constants; unknown/mismatched observations fail closed. The service reaches SSH only from `DecisionSSHEligible`, then delegates policy/trust/consent/credential to 6A, `SSHRuntimeFactory.Open` to 6B, and fixed `SSHRuntime.Prove` to 6C/6D. The service accepts no shell, command, path, SQL, download, retry, alternate transport, source, row, raw error, or secret result. Success writes the marker only after valid metadata and cleanup; failures audit only allowlisted class/revision/cleanup metadata and do not establish a marker. Cleanup failure preserves the primary result class with `Cleanup:false`.
- Exact Phase 7B authored numstat: **301 additions + 6 deletions = 307 changed lines**. The complete current intended diff is **302 additions + 7 deletions = 309 changed lines** because it also retains the pre-authorized tasks-only correction; both exclude unrelated `.atl`, `tmp`, and `internal/credential/keyring_store.go` state. Task aggregate: **31 total = 24 completed + 7 pending**.
- No native authority was acquired, settled, reset, or changed; no commit, stage, push, or PR occurred. Next recommended action: apply 7C only on `feature/step8-compose`.
