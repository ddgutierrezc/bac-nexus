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

- [ ] 1.3–1.4 typed application protocol/session core.
- [ ] 2.1–2.4 transports and fallback runtime.
- [ ] 3.1–3.5 resolver, wizard, and final verification.

## Deviations and Risks

- None from the approved design for this slice.
- Schema-v2 profiles require explicit policy and transport-specific trust evidence; legacy v1 profiles remain readable and are not silently upgraded by `Store.Load`.
