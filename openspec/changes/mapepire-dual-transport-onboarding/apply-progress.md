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
