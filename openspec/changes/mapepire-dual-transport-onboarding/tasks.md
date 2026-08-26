# Tasks: Mapepire Dual-Transport Onboarding

## Review Workload Forecast

| Field | Value |
|---|---|
| Estimated changed lines | 1,200–1,500 authored lines |
| 400-line budget risk | High; 400-line guard exceeded |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 → PR 2 → PR 3 → PR 4 → PR 5 → PR 6 |
| Delivery strategy | auto-chain |
| Chain strategy | feature-branch-chain |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: feature-branch-chain
400-line budget risk: High

### Slice 2 Delivery Decision

- Maintainer-approved exception: `size:exception` applies only to Slice 2 (`slice-2-typed-protocol-session`).
- Slice 2 maximum changed-line allowance: 600; current complete intended diff is recorded in `apply-progress.md`.
- This exception does not change the later-slice delivery strategy: remaining work stays `feature-branch-chain`.

### Suggested Work Units

| Unit | Goal | Likely PR / dependency / boundary | Focused test command | Runtime harness | Rollback boundary |
|---|---|---|---|---|---|
| 1 | Schema-v2 policy/trust compatibility | PR #1 base=feature/tracker; standalone profile foundation | `go test ./internal/profile ./internal/security` | N/A: local JSON/policy fakes only | `internal/profile`, policy changes |
| 2 | Typed protocol/session core | PR #2 base=PR #1; depends on policy constants | `go test ./internal/mapepire` | N/A: deterministic channel fakes | `internal/mapepire` core files/tests |
| 3 | Trusted daemon WSS | PR #3 base=PR #2; includes dependency admission | `go test ./internal/mapepire/wss` | Loopback `httptest` TLS/WSS only | `internal/mapepire/wss`, `go.mod`, `go.sum` |
| 4 | SSH-single adapter and fallback runtime | PR #4 base=PR #3; depends on protocol | `go test ./internal/mapepire/sshstdio ./internal/connectors/ibmi/mapepirestdio ./internal/remote` | Fake SSH process; no IBM i | SSH adapter/runtime changes only |
| 5 | Resolver, readiness, audit, security wiring | PR #5 base=PR #4; depends on both adapters | `go test ./internal/configuration ./internal/audit ./internal/security` | Counting fakes; no network | Resolver/config/audit/security files |
| 6 | Step 3/4 composition and documentation | PR #6 base=PR #5; final user-visible slice | `go test ./internal/tui` | `View()` at 120x40, 80x24, 40x16 and NO_COLOR | TUI files plus `docs/IBM_I_PROFILE_WIZARD.md` |

## Phase 1: Foundation and Protocol (PRs 1–2)

- [x] 1.1 RED: test strict schema-v2 read/write, v1 conservative migration, secret-free ephemeral observations, and independent TLS/SSH evidence in `internal/profile/*_test.go`.
- [x] 1.2 GREEN: implement profile policy/trust schema-v2 validation and migration in `internal/profile/`; add resolver limits as release constants.
- [x] 1.3 RED: test typed seven-operation validation, bounded fields, random IDs, out-of-order correlation, duplicate/unknown IDs, cursors, limits, and cancellation closure in `internal/mapepire/*_test.go`.
- [x] 1.4 GREEN: replace serialized `internal/mapepire` framing/session with typed envelopes, one reader, controlled writer, bounded pending map, safe errors, and compatibility `Execute`.

## Phase 2: Transports and Fallback (PRs 3–4)

- [x] 2.1 RED: add dependency/provenance/license/checksum/vulnerability review evidence and loopback tests for WSS text frames, CA/hostname, pin/TOFU, rotation, bounds, compression-off, and terminal identity failure.
- [x] 2.2 GREEN: add `github.com/coder/websocket` v1.8.15, module `h1:6B2JPeOGlpff2Uz6vOEH1Vzpi0iUz20A+lPVhPHtNUA=` and go.mod `h1:NX3SzP+inril6yawo5CQXx8+fk145lPDC6pumgx0mVg=`; implement `internal/mapepire/wss/` with TLS policy, bounded reads, deadlines, compression off, and cancellation-terminal sessions.
- [ ] 2.3 RED: test SSH LF framing, matching-ID plus `success=true` detection, EOF/exit, host-key mismatch, cancellation, and rejection of arbitrary command input.
- [ ] 2.4 GREEN: implement `internal/mapepire/sshstdio/` and wire `internal/remote/` plus `internal/connectors/ibmi/mapepirestdio/` so artifact/cache/upload/Java/`--single` exist only behind consented SSH fallback.

## Phase 3: Resolver, Wizard, and Verification (PRs 5–6)

- [ ] 3.1 RED: test resolver classification/no-downgrade, WSS `/version`, fallback trust gate, credential terminality, sanitized audit, and local readiness in `internal/configuration/*_test.go` and `internal/audit/*_test.go`.
- [ ] 3.2 GREEN: implement managed WSS-first resolver, trust enrollment, readiness/audit/security wiring; keep observations ephemeral and daemon independent of SSH/JAR/Java/upload.
- [ ] 3.3 RED: test Step 3/4 no-credential/no-runtime behavior, exact authentication-pending copy, Step 8 connect/query proof, focus/feedback/reachability, and daemon zero-fallback calls in `internal/tui/*_test.go`.
- [ ] 3.4 GREEN: compose Steps 3–4 without renumbering, preserving shared TUI primitives; update `docs/IBM_I_PROFILE_WIZARD.md` to the approved dual-transport behavior.
- [ ] 3.5 Run `gofmt`, `go test -count=1 ./...`, `go vet ./...`, and `go build ./cmd/catalogspike`; confirm no IBM i contact, no edits under `mapepire-artifact-acquisition/`, `.atl/`, or `tmp/`. Threat-matrix rows are all explicitly N/A, so no threat RED tests apply.
