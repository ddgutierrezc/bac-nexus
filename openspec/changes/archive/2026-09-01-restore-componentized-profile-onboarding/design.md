# Design: Restore Componentized Profile Onboarding

## Technical Approach

Replace direct Create with a four-step controller while protecting dirty recovery. Restore `d027210`'s byte-identical wizard render/viewport/progress primitives; adapt its identity/connection components. Create uses the wizard shell; metadata Edit retains `profileScreen`, `profileValidation`, Save/Cancel, viewport, and result/discard behavior.

## Architecture Decisions

| Decision | Alternatives / tradeoff | Choice and rationale |
|---|---|---|
| State ownership | Eight screens; enlarged direct form | One `onboardingStep` enum with independent focus, computed guard, scoped feedback, and `OperationIdentity`. |
| Secret handoff | Boolean status loses credential; TUI-held bytes violate policy | Application-owned, single-use secret lease; TUI holds only opaque session/generation and status. |
| Persistence gate | Early commit; best-effort cleanup | Inspection/proof, required sanitized audit, and credential handling precede profile commit; explicit compensation fails closed. |
| Historical recovery | Checkout revision; rewrite UI | Restore exact primitives symbol-by-symbol, preserving recovered Edit, validation, localization, viewport, and lifecycle work. |

## Data Flow

    Step 3 tea.Exec ──→ service.Capture(terminal, session)
         │                    │ hidden read; explicit prompt
         │                    └─ bounded expiring lease (bytes)
         └─ CaptureMsg(status, session/generation only) ──→ Step 4
    Connect and Save ──→ Consume(session, request{Name,Host,Port,Username})
      ──→ Inspect(host,port) ──→ Proof(profile-with-port)
      ──→ Sanitized required audit ──→ Credential handling ──→ Commit(profile)

`Capture` prompts on real stderr, reads 1–1024 hidden bytes, and creates one two-minute lease. Retry/replacement, Back, post-capture identity edits, cancel, expiry, stale generation, and shutdown revoke and zero it; consume is atomic and once-only. Running cancellation zeroes after workers stop.

Commit cannot start until inspection/proof, required audit writes, and credential handling succeed in that order. Audit failure zeroes/revokes and leaves no profile/credential. Credential failure removes partial credential; failure to remove sets `CleanupRequired`. Commit failure rolls back partial profile/pin and credential in reverse order; incomplete compensation retains the recovery journal and returns sanitized not-saved/cleanup guidance. Audit records contain only allowlisted proof/decision metadata.

## File Changes

| Files | Action | Ownership / dependency order |
|---|---|---|
| `internal/tui/wizard_{render,viewport,progress}.go`, tests | Create | Slice 1: exact primitives/contracts; relocate `wizardOverflowIndicator`. |
| TUI step/controller/model files and tests | Create/modify | Slice 2: transitions, guards, lifecycle; preserve Edit files/behavior. |
| `internal/configuration/onboarding.go`, tests; `internal/remote/ssh.go`; `cmd/nexus/main.go` | Modify | Slice 3: lease, prompt, selected port, compensation. |
| catalogs/tests, `render_matrix_test.go`, wizard docs, `DESIGN.md` | Modify | Slice 4: locale/color evidence/docs. Each stacked slice MUST remain at or below 800 total changed lines, forecast and verified by the tasks phase. |

Before Slice 1, capture `git diff` and baseline dirty tests. Never checkout paths wholesale. Apply historical symbols selectively; explicitly reconcile Edit/lifecycle files, tests, and additive catalog IDs.

## Interfaces / Contracts

`OnboardingRequest` becomes `{Name, Host, Username string; Port int}`. `Capture`, `Revoke`, and `StartCaptured` expose identities/status only and revalidate inputs. `Port` flows unchanged through inspection, profile construction, identity comparison, proof, result, commit, and JSON; only input initialization defaults it.

Password bytes may exist only in capture, lease, credential store, and proof calls. They MUST NOT enter audit, logs, files, errors/wrapped causes, profile/recovery persistence, TUI `Model`/`tea.Msg`/`View`, or JSON/text/gob/custom serialization, formatting, reflection, or debug dumps. Secret-owning types expose no stringer/marshaler and are unreachable from TUI/persisted structs; outward errors/audits use allowlisted codes/metadata. Buffers zero on every success, failure, cancel, timeout, stale result, replacement, and compensation path.

Precedence is operation error > explicit status > validation. Related edits clear matching validation. Ready executes; blocked remains focusable and reveals name, host, username, then port; disabled is skipped only when unavailable/running. Structured ranges drive reveal; historical geometry, rhythm, cursor, wrapping, viewport, and markers remain authoritative.

## Testing Strategy

Strict RED-first: table-driven controller/guard tests; fake-clock lease lifecycle/zeroization tests; canary-negative audit/log/file/error/persistence/TUI/serialization/reflection tests; ordered spies for inspect/proof → sanitized audit → credential → commit and every compensation; non-default-port spies end-to-end. Before shared-screen edits, extend `profile_screen_test.go` with failing `Update`/`View` evidence preserving Edit Save/Cancel reachability, validation/focus, narrow viewport reveal, successful result, and cancel-discard. Add Create runtime traversal at 120x40, 80x24, and 40x16 across both locales and color/`NO_COLOR`. Run focused/full/race tests, vet, build, and diff-check.

## Threat Matrix

| Boundary | Applicability | Safe/failure behavior | Planned RED test |
|---|---|---|---|
| Documentation-like paths | N/A: no classification/execution | No path execution | None |
| Git repository selection | N/A: no VCS automation | No repository selection | None |
| Commit state | N/A: no commits | No index mutation | None |
| Push state | N/A: no push | No ref resolution | None |
| PR commands | N/A: no PR automation | No command composition | None |
| Terminal process integration | Applicable: fixed in-process `tea.Exec` | Accept matching real stdin/stderr terminals only; no executable/shell/args; fail closed and retain no lease | mismatched/non-terminal handles, cancel/EOF/error, explicit prompt, zeroized rejection, no external process |

## Migration / Rollout

No data migration. Roll back stacked slices in reverse order; port UI and backend propagation remain atomic.

## Open Questions

None.
