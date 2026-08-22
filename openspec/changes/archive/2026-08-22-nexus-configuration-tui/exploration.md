# Exploration: Nexus Configuration TUI

## Executive Summary

BAC Nexus has no TUI today. The current product has two separate command surfaces: `nexus serve`, which accepts a required profile and starts the stdio MCP server, and the pre-v1 `catalogspike` CLI, which owns a transactional setup wizard plus diagnostic/live flows. The useful TUI opportunity is therefore not “replace the CLI with settings”; it is to provide an optional configuration adapter over reusable profile, credential, trust, artifact, diagnostics, and client-integration services while preserving automation-first commands and strict secret/security boundaries.

The most credible direction is a Bubble Tea-family TUI only after extracting the setup and configuration use cases from `cmd/catalogspike`, with a small first slice focused on profile inventory, non-secret IBM i connection metadata, explicit host-trust enrollment, native credential presence, and a read-only readiness summary. This is a recommendation for proposal/design investigation, not an architecture commitment.

## Current State

### Product and release status

- `README.md` describes `nexus` as a local-first, deployment-neutral, agent-agnostic MCP server with exactly two typed read-only tools: `resolve_catalog_candidates` and `read_selected_source`.
- `cmd/nexus/main.go` accepts `serve`, `version`, and `help`; `serve` requires `-profile <name>`, composes `app.Service`, runs startup recovery, and then runs the official MCP stdio server.
- `internal/mcp/server.go` owns the MCP adapter and exposes exactly two tools. It must remain separate from any interactive UI lifecycle.
- Release identity is represented by `internal/release/manifest.go`, including `ready_for_controlled_ibmi_validation` and `not_validated_on_ibmi`. `docs/IBM_I_VALIDATION.md` remains a manual, read-only external rollout gate.
- The repository is post-v1 archive state. The current code is more advanced than the stale `openspec/config.yaml` context sentence that still says MCP wiring is planned; this exploration treats current source and archived v1 artifacts as authoritative for behavior.

### Implemented configuration inventory

| Concern | Current evidence | Ownership today | Status |
|---|---|---|---|
| Profile identity and persistence | `internal/profile/profile.go`: `Profile`, `Store.Save`, `Load`, `Delete`, `DefaultRoot` | `internal/profile`; setup composition in `cmd/catalogspike` | Implemented |
| IBM i endpoint metadata | `Profile.Host`, `Port`, `Username`; endpoint validation | `internal/profile` | Implemented |
| Host trust/enrollment | `HostKeyFingerprint`, `HostKeyTrust` (`tofu`/`verified`); setup offers manual enrollment or explicitly labeled spike-only inspection | `internal/profile`, `internal/remote`, setup CLI | Implemented, with production verification constraints |
| Credential mode | `CredentialMode` supports `vault` and `prompt` in profile validation; `nexus` uses native credential store in `defaultDeps` | Profile plus credential composition | Implemented but inconsistent across pre-v1 and v1 paths |
| Native credentials | `internal/credential/keyring_store.go`: exact profile-scoped `Get`, `Set`, `Delete`, fixed `BAC Nexus` service and `ibmi/<profile>` account; platform adapters | `internal/credential` | Implemented |
| Legacy encrypted vault | `internal/credential/vault.go`; explicit migration seam in keyring store; setup still writes a vault through the spike path | `internal/credential`, `cmd/catalogspike` | Implemented legacy/transition behavior |
| Java runtime path | `Profile.JavaHome` validates IBM i `/QOpenSys/QIBM/ProdData/JavaVM/...` paths | `internal/profile`, setup CLI | Implemented in profile/setup |
| Mapepire JAR | `Profile.MapepireJAR`; verified auto-discovery under VS Code extensions and manual absolute-path fallback | `internal/mapepire`, profile, `cmd/catalogspike` | Implemented in spike; applicability to current v1 MCP path must be confirmed |
| Policy/allowlist selectors | `internal/security` owns local-principal authorization and typed selectors; audit has `PolicyIDVerifiedReadOnly` | `internal/security`, `internal/audit` | Implemented for v1 service, not user-editable configuration |
| Audit | Sanitized operation/result/count metadata and fixed policy identifier | `internal/audit`, `internal/app` | Implemented; no configurable output sink is exposed by current v1 CLI |
| MCP startup/client integration | `nexus serve -profile`; external MCP clients configure stdio invocation | `cmd/nexus`, `internal/mcp`, client config outside repository | Implemented minimally |
| Diagnostics/readiness | `catalogspike` has diagnostic and live output contracts; v1 has startup recovery and release/validation status, but no unified readiness command | `cmd/catalogspike`, `cmd/nexus`, `internal/release`, `internal/app` | Partly implemented; unified readiness is a candidate capability, not an approved contract |
| Package/version information | `nexus version`, release manifest, binary checksum and VCS identity | `cmd/nexus`, `internal/release`, runbook | Implemented |
| Controlled IBM i validation | Runbook/checklist and sanitized evidence template | `docs/IBM_I_VALIDATION.md`, release package | Implemented as operator documentation; live validation not performed |
| Output/config roots | Profile root uses `os.UserConfigDir()/BAC Nexus/profiles`; old setup accepts test-only config and credential root overrides | `internal/profile`, `cmd/catalogspike` | Implemented, but not presented as a user-facing settings model |

### Future hypotheses, not current configuration

The following are reasonable TUI categories but are not established current user-configurable contracts: multiple active profiles in `nexus` UX, changing allowlists/policy selectors, selecting audit/output destinations, configuring MCP client registrations, SQL-job/session preferences, dependency traversal limits, connector registry settings, remote/central deployment settings, and a generalized “all connectors” settings model. They require proposal/spec decisions and must not be silently invented by implementation.

## Configuration and Security Flow Map

### Existing setup flow

`catalogspike setup` in `cmd/catalogspike/main.go` currently performs a linear wizard:

1. Resolve profile and credential roots.
2. Collect name, host, port, host-key enrollment mode/fingerprint, username, optional Java home.
3. Discover or manually accept and verify the Mapepire Server 2.3.5 JAR.
4. Build and validate a `profile.Profile`.
5. Prompt for IBM i password and vault master/confirmation using a hidden terminal reader.
6. Require exact confirmation before publishing.
7. Write vault first, then profile; attempt cleanup and surface an orphan-vault error if profile publication fails.

This flow is transactional and injectable through `setupDependencies`, but the orchestration and user interaction are still in `cmd/catalogspike`. It is a wizard, not a reusable application service or TUI model.

### Existing serve flow

`cmd/nexus/main.go` validates a non-empty profile, uses `credential.NewNativeCredentialStore`, `security.NewPolicy`, `audit.NewRecorder`, and constructs `app.Service`. `app.Service.Startup` executes the recovery gate before the MCP server is exposed. The MCP process is intentionally not an interactive configuration process.

### Reusable versus coupled behavior

**Reusable candidates already present:** profile validation and persistence; native credential operations; explicit legacy-vault migration; Mapepire discovery and verification; host-key inspection primitives; app startup/recovery; security authorization; audit recording; release identity/manifest verification.

**Coupling to reduce before a TUI:** `cmd/catalogspike` owns prompt sequencing, secret collection, setup transaction orchestration, diagnostic presentation, and profile selection. `cmd/nexus` owns argument parsing and composition, which is appropriate for startup but not for interactive configuration. `internal/mcp` must not become the TUI's service layer; it is a wire adapter.

**Important boundary:** secrets must remain OS-backed or session-only. A TUI may request a secret through a narrow credential service, but must never put secret bytes into model state, profile JSON, logs, audit, MCP messages, argv, environment, or artifacts. Host trust remains explicit and fail-closed; visual convenience cannot turn current-connection inspection into independent verification.

## Gentle AI Public Repository Research

Research was read-only against the public repository at commit `35deba324046c0687d38b1da66638da952e290ab` (latest commit observed through the GitHub API on 2026-08-21).

### Actual stack and boundaries

- `go.mod` explicitly uses `github.com/charmbracelet/bubbletea v1.3.10`, `github.com/charmbracelet/bubbles v1.0.0`, and `github.com/charmbracelet/lipgloss v1.1.0`, alongside platform/terminal support packages.
- `cmd/gentle-ai/main.go` is a thin entry point delegating to `internal/app.Run`.
- `internal/tui/model.go` is the Bubble Tea model/update/view center. It defines explicit screen states, typed messages, injected operation functions, asynchronous commands, spinner/progress state, and error/result state.
- `internal/tui/router.go` and `internal/tui/screens/` separate navigation and screen rendering from the central model. `internal/tui/styles/` owns presentation styling.
- `internal/pipeline`, `internal/planner`, `internal/backup`, `internal/state`, and component packages own reusable behavior. The TUI invokes these through injected functions rather than becoming the installer engine.

### UX patterns worth learning, not copying

1. **Welcome/launcher shell:** the model has a clear `ScreenWelcome` plus dedicated screens for setup, model configuration, profiles, backups, restore, upgrade, sync, uninstall, and diagnostics-like operations. This supports progressive disclosure instead of one giant form.
2. **Explicit state machine:** screen transitions are named states; `Update` handles typed `tea.Msg` values and `View` delegates rendering to screen functions. This makes keyboard behavior and transition tests inspectable.
3. **Contextual navigation:** the model tracks `PreviousScreen`, cursor state, dimensions, and flow flags such as standalone versus install flow. A Nexus equivalent should preserve “where did I come from?” without leaking lifecycle state across MCP serving.
4. **Safe mutation workflow:** destructive actions use separate confirmation and result screens. The review-store reset test documents a deliberately non-destructive default cursor because reaching a confirmation screen should not make an irreversible action the next keystroke.
5. **Async feedback:** progress and background operations use typed messages (`StepProgressMsg`, `PipelineDoneMsg`, `UpgradeDoneMsg`, etc.), spinners, operation guards, and result screens. The UI does not block while long work runs.
6. **Validation and diagnostics:** forms retain field/error state, previews and conflict warnings are explicit, and failures return to a state where the operator can understand or correct the problem.
7. **Responsive terminal behavior:** `tea.WindowSizeMsg` updates width/height and renderers receive dimensions; long lists have scroll/cursor management. Nexus should define minimum terminal behavior and narrow-layout fallbacks rather than assume a wide terminal.
8. **Accessibility/ergonomics:** the observed structure favors visible key hints, conventional arrows/Enter/Esc, text inputs, and readable hierarchy. Accessibility is primarily a product acceptance concern here; the public code does not establish a formal screen-reader contract.
9. **Testing:** `internal/tui/model_test.go`, `navigation_safety_test.go`, `agent_builder_nav_test.go`, `preset_flow_test.go`, `progress_test.go`, and `restore_test.go` show model/navigation, safety, progress, and flow tests. The suite is broad, but the central model is also very large (about 178 KB at the observed commit), a maintainability warning for Nexus.
10. **Package boundary lesson:** Gentle AI keeps domain operations in app/planner/pipeline/component packages and lets TUI orchestrate presentation. Nexus should borrow this boundary, not its source structure or product semantics.

### Public evidence

- Repository: https://github.com/Gentleman-Programming/gentle-ai
- Observed commit: https://github.com/Gentleman-Programming/gentle-ai/commit/35deba324046c0687d38b1da66638da952e290ab
- Stack: https://raw.githubusercontent.com/Gentleman-Programming/gentle-ai/35deba324046c0687d38b1da66638da952e290ab/go.mod
- Entry point: https://github.com/Gentleman-Programming/gentle-ai/blob/35deba324046c0687d38b1da66638da952e290ab/cmd/gentle-ai/main.go
- TUI model: https://github.com/Gentleman-Programming/gentle-ai/blob/35deba324046c0687d38b1da66638da952e290ab/internal/tui/model.go
- Router: https://github.com/Gentleman-Programming/gentle-ai/blob/35deba324046c0687d38b1da66638da952e290ab/internal/tui/router.go
- TUI tests: https://github.com/Gentleman-Programming/gentle-ai/tree/35deba324046c0687d38b1da66638da952e290ab/internal/tui
- Architecture guide: https://github.com/Gentleman-Programming/gentle-ai/blob/35deba324046c0687d38b1da66638da952e290ab/docs/CODEBASE-GUIDE.md

## Approaches

### 1. Bubble Tea configuration adapter over extracted application services

Use Bubble Tea/Bubbles/Lip Gloss for an optional `nexus configure` or equivalent TUI. Extract setup operations into consumer-owned services/interfaces first; the TUI supplies navigation, forms, validation display, confirmation, progress, and readiness presentation. Keep `nexus serve` and automation commands as separate processes and entry points.

- **Pros:** Closest fit to the researched Gentle AI interaction model; typed event loop and injected commands are testable; good Windows terminal support through the Charm ecosystem; supports staged screens and future connector-specific sections without coupling MCP transport to UI.
- **Cons:** Adds dependency and terminal compatibility surface; Bubble Tea architecture can grow into a large central model if screens are not bounded; credential prompts require careful input-mode management and never-safe-by-default state handling.
- **Effort:** Medium/High.
- **Main risk:** Treating the library choice as the architecture instead of keeping configuration services independent.

### 2. Standard-library terminal adapter over a command/service workflow

Keep dependencies minimal and build a small line-oriented or menu-oriented UI using Go standard library terminal input/output. Reuse extracted services and preserve the current wizard as the baseline.

- **Pros:** Lowest dependency approval risk; simpler binary and Windows distribution story; easy to run in constrained environments; minimal MCP lifecycle interaction.
- **Cons:** Harder to achieve the requested navigation quality, responsive layout, rich feedback, and safe multi-screen workflows; more bespoke terminal behavior and test harness work; likely to become a second, less capable wizard.
- **Effort:** Medium.
- **Main risk:** Delivering “a prompt loop” rather than the conceptual Gentle AI-quality configuration experience.

### 3. External/browser or editor-hosted configuration UI

Expose a local HTTP or editor integration and render configuration outside the terminal.

- **Pros:** Better layout, discoverability, and accessibility potential; easier responsive forms.
- **Cons:** Introduces a new local service/security boundary, browser/editor lifecycle, origin/authentication concerns, and a larger deployment story; conflicts with the current small local stdio MCP PoC and has no evidence in the repository.
- **Effort:** High.
- **Main risk:** Scope expansion into a product/control plane before the core configuration ownership is settled.

## Recommendation

For proposal consideration, prefer **Approach 1**, but sequence it as “service extraction and contract inventory first, TUI second.” The TUI should remain an optional configuration adapter as prior architecture #1936 requires. It should never start or host the MCP server in the same interactive lifecycle, and it should call shared application services rather than `cmd/nexus` or `internal/mcp` internals.

The information architecture should be a launcher plus bounded flows, not a single settings screen:

```text
Nexus Configure
├── Profiles
│   ├── List / active selection
│   ├── Create or edit non-secret connection metadata
│   ├── Host trust enrollment
│   ├── Credential status / explicit set or migrate
│   └── Delete with confirmation
├── Integrations
│   ├── MCP client command/config preview
│   └── Copy/export instructions (no secret material)
├── Readiness
│   ├── Profile validity
│   ├── Native credential availability
│   ├── Host trust state
│   ├── Artifact/runtime checks
│   └── Controlled IBM i validation prerequisites
├── Diagnostics
│   ├── Version and release identity
│   ├── Local paths/capabilities, sanitized
│   └── Read-only connectivity/readiness checks, only when approved
└── Help / Quit
```

Screen-level interaction should follow these principles: visible current scope; list-first navigation; `Enter` to inspect/continue; `Esc` to back out without publishing; explicit `Save`/`Apply` confirmation for mutations; separate progress and result states for remote or filesystem work; field-level validation; default focus on safe/non-destructive actions; no secret values in the model or rendered result; and narrow-terminal fallback to one-column screens.

### First vertical slice

The first slice should be deliberately bounded:

1. Launch an optional configuration TUI without starting MCP.
2. List existing profiles and show a sanitized status summary.
3. Create a profile using non-secret fields already represented by `Profile`: name, host, port, username, Java home, and applicable artifact path.
4. Perform explicit host-key enrollment using the existing manual-first trust rules; show TOFU as a warning/state, never as verified identity.
5. Set or confirm native credential presence through a secret-aware service; do not render or persist secret bytes in TUI state.
6. Validate and publish the profile transactionally, with a confirmation and deterministic result screen.
7. Show a read-only readiness summary and how to run `nexus serve -profile <name>`.
8. Test the model/navigation and service boundaries using fakes, including cancellation, malformed input, trust mismatch, native-store unavailable, failed publication, and no-secret-output assertions.

### Staged rollout

- **Stage 0 — inventory/contracts:** decide which existing concerns are genuinely user-configurable and extract setup application services without changing CLI behavior.
- **Stage 1 — profile/readiness adapter:** ship the bounded vertical slice above; keep `catalogspike setup` as a compatibility path.
- **Stage 2 — migration and lifecycle:** add explicit legacy-vault migration, profile edit/delete, and backup/rollback only after custody and persistence rules are approved.
- **Stage 3 — integration assistance:** add MCP client configuration previews or narrowly controlled registration only after client-specific formats and security ownership are approved.
- **Stage 4 — connector-specific configuration:** add IBM i discovery/session options or future connectors as separate screens and contracts, not generic dynamic settings.

### CLI migration strategy

Do not remove or hide automation-friendly commands. Extract a shared command/service layer so:

- `catalogspike setup` remains a scriptable/compatibility wizard during migration;
- a future `nexus configure` TUI invokes the same validation, persistence, trust, credential, and artifact services;
- non-interactive commands accept explicit flags/files and never require TUI state;
- TUI and CLI produce the same typed outcome categories, while presentation differs;
- migration is additive first, deprecation only after an evidence-based compatibility period.

## Explicit Non-Goals for This Exploration

- No product code, dependencies, issues, PRs, or TUI implementation.
- No commitment to Bubble Tea, a command name, a package layout, or a settings schema.
- No replacement of the MCP server lifecycle with an interactive process.
- No arbitrary SSH, SQL, shell, CL, remote listing, deletion, or mutation UI.
- No secret viewer, plaintext export, secret-in-model state, or environment/argv credential flow.
- No generalized policy editor until authorization ownership and administrative governance are approved.
- No central deployment, multi-user administration, web dashboard, graph database, or future connector framework.
- No claim that Mapepire/JAR configuration remains required for the current v1 MCP path; applicability must be verified before exposing it in a TUI.
- No live IBM i validation; current status remains `ready_for_controlled_ibmi_validation` / `not_validated_on_ibmi`.

## Unresolved Product Decisions

These are genuine later decisions, not harness questions:

1. Is the intended user-facing command `nexus configure`, a separate `nexus tui`, or another name, and must it coexist with `catalogspike` indefinitely?
2. Which configuration concerns are in the first supported product slice versus diagnostic-only or operator-only information?
3. Is Mapepire/JAR/Java configuration still part of Nexus v1 runtime, or only legacy spike support?
4. Should users be able to create/edit/delete profiles in the TUI, or only inspect and invoke existing approved profiles?
5. What is the approved native credential UX on Windows, macOS, and Linux, including explicit migration from the old vault?
6. Who may change policy/allowlist selectors, and are those application settings or centrally governed deployment policy?
7. Should the TUI offer an MCP client configuration preview only, or write client configuration files? Which clients and file locations are approved?
8. What readiness checks may contact IBM i, under which explicit operator action, timeout, and audit rules?
9. Is audit output intentionally fixed in v1, or may an operator select a local sink/retention policy?
10. What minimum Windows terminal versions, color/accessibility expectations, and non-interactive fallback behavior are required?
11. Should TUI changes support rollback/backups of profiles, and what exact persisted files are safe to back up?
12. Does future connector extensibility require a screen registry now, or should each connector add a separately reviewed flow later?

## Risks

- **Secret custody regression:** a convenient form model can accidentally retain passwords, render them, or include them in diagnostics. The model must be structurally unable to carry secret values where possible.
- **Lifecycle coupling:** starting MCP from the TUI could mix interactive terminal control with stdio JSON-RPC and break MCP clients. Keep processes separate.
- **Boundary leakage:** moving setup code out of `cmd/catalogspike` may accidentally make TUI code depend on command flags or MCP adapters instead of application services.
- **Stale applicability:** Mapepire/JAR and Java fields are implemented in the spike but may not be required by the v1 MCP runtime; exposing them as first-class settings without verification would encode legacy assumptions.
- **Policy overreach:** making allowlists or audit sinks user-editable could weaken fail-closed authorization or enterprise governance.
- **Windows terminal variance:** input modes, hidden prompts, Unicode width, resize behavior, and terminal capability differences require CI/manual evidence; do not infer support from Unix behavior.
- **Dependency approval:** Bubble Tea-family packages are a new dependency surface for BAC Nexus and need approved versions and licensing/security review.
- **Central-model growth:** copying Gentle AI's large all-in-one model would create a maintenance bottleneck. Keep screen flows and services bounded.
- **Migration drift:** if CLI and TUI implement separate validation/persistence logic, profiles can diverge and automation becomes unreliable.
- **Overbroad scope:** “everything configurable” can turn into a control plane. Roll out by bounded concern inventory and separate proposals.

## Ready for Proposal

**Yes, with a bounded proposal.** The next proposal should authorize investigation and extraction planning for one configuration adapter slice, not “build the complete Nexus TUI.” It should explicitly preserve `nexus serve`, the automation-friendly CLI, native secret custody, fail-closed trust/policy, current release-status claims, and the separation between TUI, application services, and MCP transport. Before design, the proposal should resolve the command name, first-slice scope, Mapepire applicability, credential migration ownership, and whether MCP client configuration is preview-only or writable.

## Key Learnings

1. BAC Nexus currently has a wizard and MCP server, but no reusable configuration application service or TUI.
2. Current profiles combine IBM i metadata, trust state, runtime paths, artifact paths, and credential mode, while native secrets are intentionally outside profile persistence.
3. Gentle AI uses Bubble Tea, Bubbles, and Lip Gloss with explicit screens, typed messages, injected operations, progress states, and extensive navigation safety tests.
4. The safest Nexus TUI direction is an optional adapter that never owns MCP serving or secret custody.
5. “Everything configurable” must be delivered as a staged inventory of bounded flows rather than one universal settings screen.
