# IBM i Profile Creation Wizard

This is the permanent source of truth for the **Crear perfil IBM i** wizard: what is implemented, what existing lower layers can support, and what still requires approval or composition. It helps product, security, TUI, and connector reviewers avoid treating a local screen or historical plan as a live IBM i capability.

## Executive status

**FACT:** the wizard preserves the nine-step order. Step 3 is trust and policy enrollment only, with independent TLS and SSH trust state; it does not authenticate or use credentials. After explicit TOFU acceptance, focus returns to its valid forward action, so the next explicit `Enter` reaches Step 4 without `Tab`; acceptance itself does not auto-transition. Step 4 performs a bounded pre-auth HTTPS `/version` probe and uses the exact detected state `[OK] Mapepire detected — authentication pending`. The implemented Step 8 service and lifecycle alone own credentials, transport selection/fallback, authenticated `connect`, and the fixed read-only proof; this does not claim a completed end-to-end nine-step journey or live IBM i validation.

**DECISION:** current code, current tests, valid OpenSpec specifications, and current documentation outrank archived OpenSpec artifacts and this document. Historical material is useful context only.

**PROPOSAL:** the requested nine-step sequence is retained exactly as a conceptual architecture. Step 4 is composed only as pre-auth readiness. Steps 5–7 and 9 remain uncomposed; Step 8 has an implemented service/lifecycle boundary but is not evidence of live IBM i validation or a reachable fresh-profile journey.

This document does not replace OpenSpec requirements or Architecture Decision Records (ADRs). It records their current relationship to the wizard and identifies where an ADR or approved specification is still needed.

## Purpose, scope, and evidence rules

**Audience:** BAC Nexus maintainers, security reviewers, product owners, and implementers of the IBM i profile journey.

**Scope:** the local `nexus configure` wizard, the lower-layer capabilities it may compose later, and the conceptual nine-step profile journey. It does not redefine the MCP server, authorize IBM i access, or claim field validation.

| Classification | Meaning |
| --- | --- |
| **FACT** | Proven by current code, tests, valid specifications, or current documentation. |
| **DECISION** | Adopted BAC Nexus decision. |
| **PROPOSAL** | Design or behavior still subject to approval. |
| **FUTURE** | Deliberately deferred capability. |

When sources conflict, use this order: current code and tests; valid OpenSpec; current docs; archived OpenSpec; this explanatory document. In particular, historical verification proves the earlier configuration slice, not implementation of this nine-step wizard.

## Global architecture and journey contract

### What a profile means

**FACT:** a persisted `profile.Profile` is non-secret metadata: name, host, SSH port, username, pinned host-key fingerprint and provenance, optional Java home, optional local Mapepire JAR path, and credential mode. Profile JSON does not contain secret material.

**DECISION:** native operating-system keyring storage is the V1 credential store. It fails closed when unavailable. Profile JSON and the SQLite ownership ledger must never store secrets.

**FACT:** the current wizard keeps accepted name, connection values, and explicit TOFU identity state in Bubble Tea model/draft state. Step 3 moves through authorization, bounded inspection, loading/error, observed review, and completed in-memory states; cancelling a new wizard returns to Home without saving and beginning a new wizard resets later-step draft state.

```mermaid
flowchart LR
  H[Home] --> S1[1 Profile: local]
  S1 --> S2[2 Connection: local]
  S2 --> S3[3 Server Identity: explicit TOFU state machine]
  S3 --> S4[4 Mapepire: pre-auth readiness]
  S4 -. conceptual only .-> S5[5 Java]
  S5 -. conceptual only .-> S6[6 Credentials]
  S6 -. conceptual only .-> S7[7 Review]
  S7 -. optional remote test .-> S8[8 Optional Test]
  S8 --> S9[9 Completion]
  S1 <-- Back --> H
  S2 <-- Back --> S1
  S3 <-- Back --> S2
```

### Local work, remote work, and persistence

**FACT:** Step 3 performs the explicitly authorized SSH host-key inspection but makes no authentication or Mapepire runtime call. Its trust/policy observations are independent: TLS daemon trust is not SSH host trust. Step 4 may perform only a bounded, injected pre-auth daemon readiness probe; its managed implementation requests HTTPS `/version` at the configured host and port, without credentials. It never starts SSH fallback, launches Java, handles a JAR, uploads, or queries Db2. If the daemon is unavailable, the UI reports authentication pending for Step 8 rather than silently downgrading.

**FACT:** the first wizard remote contact is the explicit Step 3 TOFU inspection action, not entry or Continue. It requires consent, a program-lifecycle deadline, cancellation, and sanitized feedback. **PROPOSAL:** a later Step 8 test requires its own approved remote-action contract.

**PROPOSAL:** persist a profile exactly once after secret-free review confirmation in Step 7. Step 8 must test an already saved profile or an explicit temporary in-memory candidate; it must never make “test passed” a hidden prerequisite for saving. Back must return to the relevant prior step with all safe draft values retained.

| Area | Current contract |
| --- | --- |
| Back / Continue / Cancel | **FACT:** Steps 1–3 support keyboard Back/Continue/Cancel or Escape according to their implemented screens. Invalid Continue is focusable, blocked, and returns actionable feedback. |
| Retained data | **FACT:** accepted Step 1 and Step 2 drafts survive Back; Step 3 decision survives Back. A newly started wizard resets later drafts. |
| Readiness states | **DECISION:** `READY`, `BLOCKED`, and `DISABLED` are distinct. Blocked controls remain focusable and explain correction; disabled controls are excluded from focus. |
| Feedback and focus | **DECISION:** focus is not selection. The shared semantics use `▸`, `( )`, and `(*)`; feedback precedence is error, explicit feedback, then validation. |
| Validation and responsive layout | **FACT:** current Steps 1–4 use deterministic validation, lossless terminal-cell wrapping, viewport reachability, resize behavior, and `NO_COLOR` rendering tests at 120x40, 80x24, and 40x16. |
| Localization | **FACT:** Spanish is the production default and complete English catalogs exist. **FUTURE:** locale configuration is deferred. |
| Shared TUI composition | **DECISION:** reuse the current shared panel, header, step indicator, actions, inputs, choices, feedback, and footer rather than introducing visual variants per step. |

## The conceptual nine-step sequence

The following order is preserved without renumbering: **Step 1 Profile; Step 2 Connection; Step 3 Server Identity (optionally 3A independent verification and 3B observed/TOFU); Step 4 Mapepire; Step 5 Java; Step 6 Credentials; Step 7 Review; Step 8 Optional Test; Step 9 Completion.** Step 4 is limited to pre-auth readiness; later steps retain ownership of authentication and proof.

### Step 1 — Profile

| Topic | Current evidence and required interpretation |
| --- | --- |
| Purpose / why | **FACT:** collects a unique local profile name before connection details, so later state is associated with a stable operator-facing identifier. |
| Requested information / actions | Name only. Exact rule: 1–64 ASCII characters; first character is an ASCII letter or digit; remaining characters may be ASCII letters, digits, `-`, or `_`. No spaces, dots, accents, or other symbols. |
| Current UX/TUI | **FACT:** `Crear perfil IBM i`, `Paso 1 de 9 — Perfil`, real Bubbles input and block cursor, local loading/duplicate feedback, `< CANCELAR >`, and `[ CONTINUAR ]`. |
| Actual backend and tests | **FACT:** loads local profiles through the injected profile store for duplicate detection; tests cover strict naming, case-insensitive duplicates, loading and load failure, blocked feedback, focus, retained drafts, and responsive rendering. |
| Continue does | **FACT:** validates the loaded local list and emits a local accepted-name message that opens Step 2. |
| Continue does not do | **FACT:** it does not save, authenticate, connect to IBM i, inspect a key, or store a credential. |
| Decisions / proposal / missing work | **DECISION:** duplicate checking fails closed until profile loading completes and is case-insensitive. **PROPOSAL:** retain this rule at Step 7 save time to protect against concurrent creation. Missing work is only later-step composition, not a new naming policy. |
| Status | **FACT:** Partial — visual and local draft behavior exist; it is not a completed persisted-profile step. |

### Step 2 — Connection

| Topic | Current evidence and required interpretation |
| --- | --- |
| Purpose / why | **FACT:** gathers the IBM i endpoint and user separately from trust and credentials. |
| Requested information / actions | Host, user, and SSH port. Port defaults to `22`. Local validation accepts a DNS host or IPv4 address, rejects IPv6 and whitespace; validates the user; and requires a numeric port from 1 through 65535. |
| Current UX/TUI | **FACT:** `Paso 2 de 9 — Conexión`, real inputs, visible block cursor, `< VOLVER >`, `[ CONTINUAR ]`, and copy that Nexus will not yet connect. |
| Actual backend and tests | **FACT:** uses local profile field validators only. Tests prove defaults, focus movement, validation order, Back retention, reset on new wizard, local-only Continue, and responsive rendering. |
| Continue does | **FACT:** emits a local validated `{host, username, port}` draft and enters Step 3. |
| Continue does not do | **FACT:** no DNS probe, TCP/SSH connection, IBM i contact, authentication, credential lookup, or persistence occurs. |
| Decisions / proposal / missing work | **DECISION:** endpoint syntax validation is not reachability validation. **PROPOSAL:** preserve this separation even if a later optional diagnostic is added. Missing work is remote-consented verification, not automatic connection on typing or Continue. |
| Status | **FACT:** Partial — visual and local draft behavior exist; it is not a verified connection. |

### Step 3 — Server Identity

| Topic | Current evidence and required interpretation |
| --- | --- |
| Purpose / why | **FACT:** separates server identity trust from user authentication. A server host key answers “which server answered”; a password or other user credential answers “may this user authenticate.” They must not be conflated. |
| Requested information / actions | **DECISION:** visible V1 is explicit TOFU only. The manual independently verified path is hidden/deferred pending an approved corporate fingerprint source; manual/verified lower-layer capability remains available outside this wizard. |
| Current UX/TUI | **FACT:** one `Paso 3 de 9 — Identidad` has authorization, loading/error, observed-identity review, and completed in-memory substates. It displays the draft host/port, then complete observed algorithm/fingerprint, and accepts through `[ CONFIAR EN ESTA CLAVE ]`. Review Back returns to authorization and clears unaccepted evidence; completed Back returns to Step 2 while retaining accepted evidence for the unchanged endpoint. |
| Actual backend and tests | **FACT:** production `nexus configure` injects an adapter around `remote.InspectHostKey`; its TUI contract accepts only context, host, and port. Inspection is deadline-bound by the program lifecycle context, cancellable by Escape, retryable, and request-identified so stale responses cannot alter state. |
| Continue does | **FACT:** only the focused trust action copies the exact current successful candidate to the draft with algorithm, full fingerprint, and `tofu` provenance. After that acceptance, focus returns to the valid forward action; the next explicit `Enter` advances to Step 4 without a `Tab` workaround. Acceptance does not auto-transition. Step 4 remains a separate pre-auth readiness boundary. |
| Continue does not do | **FACT:** it does not authenticate, read credentials, start Mapepire, call a trust-enrollment service, save/read/update a profile, or issue another remote command. |
| Decisions / proposal / missing work | **DECISION:** observed is not independently verified and changed pinned keys fail closed. **FUTURE:** profile persistence remains the Review/Save boundary; Steps 5–9 remain proposals. |
| Status | **FACT:** Partial — explicit TOFU inspection and in-memory preparation exist; profile persistence and later steps do not. |

#### Step 3A — independently verified fingerprint (conceptual)

**PROPOSAL:** the operator enters a known OpenSSH SHA-256 fingerprint obtained through an approved independent enterprise channel, confirms its source, and enrolls it with provenance `verified`. This is manual verified enrollment; it is not TOFU.

**FACT:** the lower `TrustService.EnrollManual` path supports verified provenance, and profile validation accepts only `tofu|verified`. Known-hosts file integration and the enterprise fingerprint source are open decisions.

#### Step 3B — observed key / TOFU (conceptual)

**PROPOSAL:** after explicit warning and consent, call the bounded `InspectHostKey` operation, show algorithm and fingerprint, require exact confirmation, and persist provenance `tofu` only after confirmation.

**FACT:** lower SSH inspection runs during key exchange before authentication, requires a context deadline, uses secure supported algorithms, and returns an unverified `tofu` candidate. It does not authenticate. Exact pinned mismatch reports `host_key_changed` and prevents remote discovery.

**FACT:** the opt-in local SSH transport harness under `internal/remote/testdata/ssh` exercises this pre-auth observation boundary and an authenticated SSH/SFTP prerequisite against an ephemeral loopback OpenSSH container. It does not provide IBM i, Step 4, Mapepire, SQL, or complete Step 8 evidence.

**PROPOSAL:** V1 may ship 3A only after an approved independent fingerprint source exists. V1 may ship 3B only when policy explicitly authorizes TOFU; it remains unverified. V1 requires at least one approved trust-enrollment path. If neither prerequisite exists, the wizard must remain `BLOCKED` and cannot honestly complete trust enrollment. Pin mismatch remains fail-closed for every permitted path.

### Step 4 — Mapepire

| Topic | Current evidence and required interpretation |
| --- | --- |
| Purpose / why | **DECISION:** report pre-auth Mapepire readiness without claiming authentication or query readiness. |
| Requested information / actions | A bounded daemon readiness observation may report detected/authentication-pending or unavailable/authentication-pending. Transport is not user-selected. |
| Current UX/TUI | **FACT:** uses the shared wizard shell, panel, actions, feedback, viewport, responsive sizes, and `NO_COLOR`; it preserves `Step 4 of 9 — Mapepire`. |
| Actual backend and tests | **FACT:** Step 4 uses the bounded pre-auth readiness seam. Its managed probe requests HTTPS `/version` at the configured host and port and accepts only version `2.3.5`; it does not perform local JAR discovery or acquisition. A local Docker SSH endpoint such as `localhost:2222` cannot satisfy this HTTPS endpoint. Lower-layer artifact and launch capabilities remain separate fallback dependencies and are not wizard behavior. |
| Continue does | **DECISION:** retain only ephemeral readiness and defer authentication to Step 8. |
| Continue does not do | It does not look up credentials, authenticate, select/fallback transports, invoke SSH/JAR/Java/upload/cache, or query IBM i. |
| Decisions / proposal / missing work | WSS `:8076` is preferred by managed policy. SSH fallback is fixed `--single` and is eligible only after later consent, credentials, and independent SSH trust; no silent downgrade is permitted. |
| Status | **FACT:** pre-auth readiness only; no live IBM i validation is claimed. The SSH Docker base mode proves Step 3 host-key observation only; its authenticated override proves SSH/SFTP prerequisites only. Neither proves Step 4 HTTPS `/version`, Mapepire, fallback, or IBM i behavior. |

**Current OpenSpec contract:** Mapepire readiness is pre-auth only. Java, JAR, artifact, upload, and launch work remains an authenticated SSH fallback concern and is not implied by daemon readiness.

### Step 5 — Java

| Topic | Current evidence and required interpretation |
| --- | --- |
| Purpose / why | **PROPOSAL:** make the Java runtime used to launch Mapepire explicit and safely overridable. |
| Requested information / actions | Default Java home: `/QOpenSys/QIBM/ProdData/JavaVM/jdk80/64bit`; show configured value and optional advanced override. |
| Current UX/TUI | **FACT:** no Step 5 wizard screen exists. |
| Actual backend and tests | **FACT:** the Mapepire launch policy substitutes the exact default when Java home is empty and rejects unsafe paths. This is a local launch-policy default; it does not remotely discover or verify Java. |
| Continue does | **PROPOSAL:** record a configured local value only; remote validation belongs to a separately consented later action. |
| Continue does not do | **FACT:** no current Step 5 exists, and no current code remotely discovers or verifies Java. `Configured` must not be represented as `Checked`. |
| Decisions / proposal / missing work | **PROPOSAL:** present `Configured` for a selected/default value and reserve `Checked` for a remote authenticated verification with deadline/cancellation. Determine whether advanced override is V1-required and whether Java remains only a legacy diagnostic under current OpenSpec. |
| Status | **FACT:** Backend only — a safe default and launch validation exist, not a wizard or remote check. |

### Step 6 — Credentials

| Topic | Current evidence and required interpretation |
| --- | --- |
| Purpose / why | **PROPOSAL:** let an operator choose between asking for a credential for each operation and storing it securely for later approved operations. |
| Requested information / actions | Conceptual choices: **Ask each time** and **Store securely**. Never show, echo, serialize, preview, log, place in argv/environment, or retain the secret in Bubble Tea model/messages/views. |
| Current UX/TUI | **FACT:** no credential Step 6 is composed into this wizard. Legacy profile CRUD and a separate Security TUI exist; neither proves the new wizard is complete. |
| Actual backend and tests | **FACT:** native keyring mechanisms exist for Windows Credential Manager, macOS Keychain, and Linux Secret Service through a profile-scoped `ibmi/<profile>` account. They return opaque outcomes/presence and fail closed when unavailable. Tests prove opaque service outcomes and unavailable-store behavior; keyring platform tests cover constrained behavior. |
| Continue does | **PROPOSAL:** choose a credential policy without exposing a secret, then either store through transient native-keyring entry or record that later operations prompt. |
| Continue does not do | **FACT:** no current Step 6 exists. A profile JSON or SQLite record never stores the secret. |
| Decisions / proposal / missing work | **DECISION:** native keyring is the V1 credential store and unavailable means fail closed before remote work. **PROPOSAL:** converge legacy `vault|prompt` profile semantics with the user-facing `Store securely|Ask each time` language before save. Missing work is wizard composition, secret-input ownership, profile-schema decision, and migration UX. |
| Status | **FACT:** Backend only — security facilities are separate from this wizard. |

### Step 7 — Review

| Topic | Current evidence and required interpretation |
| --- | --- |
| Purpose / why | **PROPOSAL:** show a secret-free final summary so the operator can correct fields before persistence. |
| Requested information / actions | Name, endpoint, user, port, trust provenance/fingerprint status, local Mapepire selection, Java setting, and credential presence/mode—never a secret. Back corrects any earlier draft. |
| Current UX/TUI | **FACT:** no Step 7 wizard screen exists. |
| Actual backend and tests | **FACT:** profile storage can save validated non-secret metadata; separate profile CRUD, trust, credential, readiness, and preview services exist. They are not injected by production `runConfigure`. |
| Continue does | **PROPOSAL:** after explicit confirmation, atomically save the secret-free profile metadata exactly here, then enter Step 8 or Step 9 depending on the chosen optional test. |
| Continue does not do | **PROPOSAL:** saving must not imply IBM i connectivity, successful authentication, Java verification, Mapepire launch, or live validation. |
| Decisions / proposal / missing work | **PROPOSAL:** use these meanings consistently: `Detectado` = local observation/discovery; `Verificado` = independently or remotely checked under a defined policy; `Configurado` = user/default value stored; `Comprobado` = completed explicit test; `Pendiente` = not performed. Save timing and atomic relationship with native keyring metadata remain to be specified. |
| Status | **PROPOSAL:** Proposed — no review workflow is implemented. |

### Step 8 — Optional Test

| Topic | Current evidence and required interpretation |
| --- | --- |
| Purpose / why | **DECISION:** give the operator an explicit opt-in, bounded proof after credentials and review rather than an implicit connection during entry. |
| Requested information / actions | Consent, visible scope, timeout, cancellation, sanitized outcome, and a choice to skip. Candidate checks may include pinned-host verification, authentication, bounded Java check, Mapepire artifact/launch check, and a read-only service probe. |
| Current UX/TUI | **FACT:** an injected Step 8 action screen renders bounded running, success, terminal, and cancelled feedback. It rejects stale request IDs, retry creates a new request, and `View()` performs no I/O. This lifecycle does not compose the missing Step 5–7 review/save journey. The fresh-profile path never creates the saved V3 profile Step 8 requires, and the current new-profile flow has no fallback-consent UX. |
| Actual backend and tests | **FACT:** `Step8Service` accepts a saved profile, clears historical marker data before a fresh attempt, selects trusted WSS first, retrieves opaque credentials only at Step 8, and records bounded audit metadata. It uses the release-owned `VALUES 1` proof revision `values-1-v1`; no generic SQL, proof rows, or proof text is returned. Existing tests use deterministic fakes, in-process counters, and local TLS/WSS loopback only. No live IBM i validation has occurred. |
| Continue does | **DECISION:** after consent, the service owns credentials, WSS-first selection, fixed `--single` SSH fallback only for bounded eligible reasons, authenticated `connect`, and the fixed bounded proof. Success marks only checks actually completed. |
| Continue does not do | **DECISION:** Save and test are distinct. A failed, cancelled, or skipped test must not silently discard an already saved profile, nor convert a save into a tested profile. |
| Decisions / proposal / missing work | **PROPOSAL:** define exact remote test scope, timeout budgets, audit class, and recovery behavior before composition. The valid spec’s “legacy diagnostics” limit applies until changed. |
| Status | **FACT:** Partial — service and action lifecycle contracts exist, but the complete Step 5–8 operator journey and live field validation do not. |

#### Step 8 evidence and live-validation boundary

The current evidence is intentionally offline. It proves deterministic contract behavior; it does **not** prove production network, IBM i, SSH, Java, artifact transfer, upload/download, keyring, credential, or bank-environment behavior.

| Topic | Current evidence | Not established |
| --- | --- | --- |
| WSS proof | In-process fakes and local TLS/WSS loopback cover WSS-first selection, authenticated fixed proof, closure, and zero SSH fallback on WSS success. | A live IBM i daemon, account authorization, certificate deployment, or network route. |
| SSH fallback | Deterministic fakes cover only the allowlisted fallback reasons: `daemon_connection_refused`, `daemon_unavailable`, `daemon_availability_timeout`, `daemon_policy_disabled`, and `daemon_version_verified_unsupported`. TLS and SSH trust remain independent. | SSH reachability, Java availability, pinned-artifact acquisition, upload, process launch, or remote cleanup in an approved environment. |
| Audit and marker | In-memory `Recorder` tests reject prohibited values, retain only bounded taxonomy and explicit cleanup state, and prove a historical marker never establishes readiness. | External audit-sink delivery or any production retention policy. |
| Readiness | Local reports retain `ready_for_controlled_ibmi_validation` and `not_validated_on_ibmi`. | A claim that IBM i was contacted or that a profile is production-ready. |

An approved live verification requires an explicitly approved environment, configuration, identity/authority, endpoint and trust policy, and a separately authorized read-oriented run. Until that happens, `not_validated_on_ibmi` is the required status.

### Step 9 — Completion

| Topic | Current evidence and required interpretation |
| --- | --- |
| Purpose / why | **PROPOSAL:** provide an honest final profile state and safe next actions without claiming operational MCP readiness. |
| Requested information / actions | Secret-free profile state, readiness, diagnostics, OpenCode/Copilot previews, Home, and profile management/recovery actions. |
| Current UX/TUI | **FACT:** no Step 9 completion screen exists. |
| Actual backend and tests | **FACT:** offline readiness, bounded diagnostics, integration previews, profile recovery/CRUD, and MCP foundation exist below the TUI but are mostly not wizard-composed. Previews are copy-only and do not mutate external client configuration. |
| Continue does | **PROPOSAL:** offer Home, profile detail, local readiness, preview, and a separately consented diagnostic. |
| Continue does not do | **FACT:** `nexus serve` is not operationally ready for live IBM i access: current readiness reports recovery, resolver, acquirer, and lease missing from serve composition. `READY` can only mean the explicitly named local/profile condition, never live IBM i validation. |
| Decisions / proposal / missing work | **PROPOSAL:** distinguish “profile saved,” “local configuration ready,” “test completed,” and “ready for controlled IBM i validation.” Missing work includes completion state model and all composition. |
| Status | **FACT:** Backend only — lower facilities exist, but no completion step does. |

## Evaluation of the nine-step UX

The nine-step sequence makes valuable boundaries visible: profile identity, endpoint syntax, server trust, pre-auth readiness, credentials, review, optional remote test, and completion. It must not be renumbered merely because later steps remain future composition.

| Recommendation | Classification |
| --- | --- |
| Keep Step 3 as the implemented explicit TOFU parent state machine; retain hidden manual independently verified enrollment as deferred until an approved corporate source exists. | **DECISION** |
| Keep Mapepire and Java separate conceptually, but group them under an “Local runtime prerequisites” section in V1 if they remain local-only. | **PROPOSAL** |
| Make Step 8 visibly optional and preserve the Step 7 save-versus-test distinction. | **PROPOSAL** |
| Keep Step 4 limited to pre-auth readiness; keep Java and runtime activation separate until separately approved. | **DECISION** |
| Do not move authentication ahead of pinned server identity. | **DECISION** |

## Status matrix

**Controlled status vocabulary:** `Complete`, `Partial`, `Backend only`, `Designed only`, `Proposed`, `Deferred`, and `Not started` are the only values used in this matrix. They describe the named dimension, not an overall product claim; `Not started` is an intentional valid value.

| Step | UX designed | TUI implemented | Backend available | Tests | State |
| --- | --- | --- | --- | --- | --- |
| 1 Profile | Complete | Partial | Partial | Complete | Partial |
| 2 Connection | Complete | Partial | Partial | Complete | Partial |
| 3 Server Identity | Partial | Partial | Backend only | Partial | Partial |
| 4 Mapepire | Complete | Partial | Complete | Partial | Partial |
| 5 Java | Proposed | Not started | Backend only | Partial | Backend only |
| 6 Credentials | Proposed | Not started | Backend only | Partial | Backend only |
| 7 Review | Proposed | Not started | Partial | Partial | Proposed |
| 8 Optional Test | Partial | Partial | Partial | Complete | Partial |
| 9 Completion | Proposed | Not started | Backend only | Partial | Backend only |

`Complete` in a cell means only that dimension; no visual-only step is called complete.

## Backend capability map: what Nexus already does without this wizard

| Capability | Step relationship | Current boundary |
| --- | --- | --- |
| Strict profile validation, atomic profile persistence, CRUD, backup/recovery | 1, 7, 9 | Separate profile/configuration services; production wizard receives only profile storage. |
| Host-key inspection, manual enrollment, TOFU enrollment, pin mismatch rejection | 3 | Separate SSH/trust services; inspection is no-auth and bounded. |
| Native keyring presence/set/rotate/delete/migration | 6 | Separate credential/security services; outcomes are opaque. |
| Local Code for IBM i JAR discovery and checksum verification | Legacy lower layer | Not part of Step 4; local/pre-auth readiness performs no JAR discovery or acquisition. |
| Authenticated remote JAR activation/rollback and Mapepire launch policy | 5, 8 | Requires authenticated SSH fallback; never implied by Step 4 daemon readiness. |
| Default Java launch-policy substitution and safe-path validation | 5 | Does not discover or verify remote Java. |
| Local readiness and bounded remote diagnostic wrapper | 8, 9 | Local readiness exposes serve-composition gaps; diagnostic needs a runner. |
| Copilot/OpenCode integration previews | 9 | Preview/copy only; never writes external client configuration. |
| Typed MCP foundation and catalog features | 9 | Existing server foundation; current composition is incomplete for live IBM i use. |

## Remote communication matrix

| Action | Local | Contacts IBM i | Authenticates | Persists |
| --- | --- | --- | --- | --- |
| Type Host/User/port | Yes | No | No | No |
| Choose TOFU branch in current Step 3 | Yes | No | No | No |
| Inspect host key through lower SSH capability | No | Yes | No | No |
| Manually enroll independently verified fingerprint | Yes | No | No | Yes, when profile update is invoked |
| Test profile (conceptual Step 8) | No | Yes | Usually yes after pin verification | **PROPOSAL:** profile already saved; record only sanitized outcome if approved |
| Java remote validation (conceptual) | No | Yes | Yes | No, unless explicit status persistence is approved |
| Step 4 pre-auth Mapepire readiness | No | Potentially, at configured HTTPS endpoint | No | No |
| Remote Mapepire artifact activation / launch | No | Yes | Yes | Remote JAR only; no wizard composition |
| Credential keyring operation | Yes | No | No | Native keyring only |
| Save profile | Yes | No | No | Local profile JSON, secret-free |
| Local readiness | Yes | No | No | No |
| Integration preview | Yes | No | No | No external client file; copy only |

## Security invariants

- **DECISION:** no secrets appear in the UI, logs, audit records, argv, environment, profile JSON, SQLite, previews, test fixtures, or MCP results.
- **DECISION:** observed is not independently verified. TOFU provenance is `tofu`; independent/manual verification is `verified`.
- **DECISION:** a changed pinned key is never silently accepted; exact mismatch fails closed before remote discovery.
- **DECISION:** remote activity requires explicit consent and must honor timeout and cancellation.
- **DECISION:** feedback must be truthful: `Detectado` is not `Verificado`; `Configurado` is not `Comprobado`; `Saved` is not `Tested`.
- **DECISION:** focus is not selection; `BLOCKED` is not `DISABLED`.
- **FACT:** release posture remains `ready_for_controlled_ibmi_validation` and `not_validated_on_ibmi`; no test or local artifact proves live IBM i behavior.

## Concise UX/TUI standards

**DECISION:** `.agents/skills/bac-nexus-tui/SKILL.md` is the current TUI standard and is authoritative for implementation details. Reuse its shared panel/header/indicator/actions/input/choice/feedback/footer model; preserve semantic focus, `READY`/`BLOCKED`/`DISABLED`, lossless wrapping, viewport reachability, responsive matrix, and `NO_COLOR`. Do not duplicate that skill as a second design system here.

## Remaining Work

### Continuation checklist (Step 3→9)

1. Preserve the Step 3 fix: TOFU acceptance remains explicit, focuses the forward action, and requires the next `Enter` to enter Step 4.
2. Validate Step 4 against a real compatible Mapepire HTTPS `/version` endpoint; do not use the Docker SSH service on `localhost:2222` as Step 4 evidence.
3. Specify and compose Steps 5–7, including Java/credential ownership, secret-safe UX, review, and one secret-free atomic saved-profile boundary.
4. Make Step 8 reachable from a newly created profile by supplying that saved profile and implementing explicit fallback-consent UX; retain save-versus-test separation.
5. Implement Step 9 completion/readiness/preview navigation without claiming live IBM i or `nexus serve` readiness.
6. Define V1 server-trust policy: independent fingerprint source, TOFU eligibility, `known_hosts` relationship, and whether both 3A/3B ship.
7. Perform separately approved, read-only validation against real Mapepire and IBM i environments; Docker base/authenticated override evidence remains limited to SSH boundaries.

## Risks and open decisions

| Area | Current fact | Open decision / risk |
| --- | --- | --- |
| Trust provenance | `tofu|verified` is validated and pinned mismatch fails closed. | What approved independent source produces verified fingerprints? |
| TOFU | Inspection is no-auth, bounded, warned by service contract, and unverified. | Is TOFU permissible for V1, and for which environments? |
| `known_hosts` | No current wizard integration is established. | Import, reconcile, or deliberately exclude it? |
| Credential store/schema | Native keyring is adopted; profile JSON contains non-secret mode only. | Converge legacy `vault|prompt` semantics and migration behavior. |
| Remote validation | No live IBM i validation occurred. | Exact V1 checks, authorization, timeout, audit, and partial-result policy. |
| Persistence timing | Current Steps 1–3 do not save. | Approve Step 7 atomic boundary and keyring/profile ordering. |
| Mapepire / Java scope | Valid OpenSpec calls them legacy diagnostics. | Approve or reject canonical wizard Steps 4–5. |
| Readiness semantics | Local readiness exposes missing serve composition. | Define each completion label without overstating operational readiness. |

## V1 recommendation

### V1 REQUIRED

- **DECISION:** strict local Steps 1–2 validation and retention; no implicit network activity.
- **PROPOSAL:** at least one approved trust-enrollment path: 3A only with an approved independent fingerprint source, or 3B only with explicitly authorized TOFU. If neither exists, keep trust enrollment `BLOCKED`; do not complete the wizard dishonestly. Pin mismatch remains fail-closed.
- **DECISION:** native-keyring-only secret handling with fail-closed unavailable behavior.
- **PROPOSAL:** truthful completion states and local readiness that preserves `not_validated_on_ibmi`.

### V1 OPTIONAL

- **PROPOSAL:** Step 3B TOFU when approved policy and explicit consent are available.
- **PROPOSAL:** Step 8 bounded optional test after save.
- **PROPOSAL:** local Mapepire discovery and Java configuration UI if the OpenSpec scope changes.

### DEFERRED

- **FUTURE:** automatic remote Java discovery/verification during entry.
- **FUTURE:** automatic remote Mapepire artifact activation or launch from a local discovery result.
- **FUTURE:** locale selection (Spanish remains default; English catalog remains complete).
- **FUTURE:** turning local readiness into a claim of live IBM i readiness.

### FUTURE ENTERPRISE

- **FUTURE:** enterprise fingerprint-source integration and approved `known_hosts` strategy.
- **FUTURE:** centrally managed policy/audit integration beyond current sanitized local contracts.
- **FUTURE:** enterprise-managed remote diagnostics, distribution controls, and field-validation evidence workflow.

## Minimal ADR recommendations

Do not create ADRs for this documentation-only change. Reuse current security and OpenSpec material where equivalent; create only these decision records if approved work needs them:

1. **Server trust enrollment policy:** independent fingerprint source, TOFU eligibility, `known_hosts`, provenance, and changed-key handling. Existing equivalent evidence: `docs/SECURITY.md` and `openspec/specs/nexus-configuration/spec.md`.
2. **Credential policy and persistence ordering:** native keyring semantics, legacy `vault|prompt` convergence, migration, and Step 7 save/keyring ordering. Existing equivalent evidence: current configuration specification and security documentation.
3. **Mapepire/Java wizard scope:** whether the valid legacy-diagnostics classification changes to canonical Steps 4–5.
4. **Readiness and completion semantics:** definitions of local ready, saved, tested, and controlled-validation readiness; do not duplicate the current rollout documentation unless behavior changes.

## Source and evidence index

| Source | Role / conflict note |
| --- | --- |
| [Current wizard model](../internal/tui/model.go) | Current local state, message seams, screens, and `Run` boundary. |
| [Step 1 implementation and tests](../internal/tui/profile_step.go) / [tests](../internal/tui/profile_step_test.go) | Naming, local profile loading, transition seam, no-save proof, feedback, responsive behavior. |
| [Step 2 implementation and tests](../internal/tui/profile_connection_step.go) / [tests](../internal/tui/profile_connection_step_test.go) | Host/user/port default and local-only transition. |
| [Step 3 implementation and tests](../internal/tui/profile_identity_step.go) / [tests](../internal/tui/profile_identity_step_test.go) | Explicit TOFU state machine, trust boundary into Step 4, responsive proof. |
| [Production configure composition](../cmd/nexus/main.go) / [test](../cmd/nexus/configure_test.go) | `runConfigure` injects profile storage and the concrete `remote.HostIdentityInspector` through the host+port-only identity boundary. It does not compose credential, trust persistence, readiness, diagnostics, previews, Mapepire, or later remote services. |
| [Design system](../DESIGN.md) and [TUI implementation standard](../.agents/skills/bac-nexus-tui/SKILL.md) | Current terminal visual language plus shared focus, feedback, viewport, responsive, and `NO_COLOR` implementation rules. |
| [Profile contracts](../internal/profile/profile.go) / [tests](../internal/profile/profile_test.go) / [recovery](../internal/profile/recovery.go) / [recovery tests](../internal/profile/recovery_test.go) | Secret-free profile shape, `tofu|verified`, legacy `vault|prompt`, validation, atomic persistence, backup, restore, and recovery. |
| [Credential keyring](../internal/credential/keyring_store.go) / [keyring tests](../internal/credential/keyring_store_red_test.go) / [legacy vault tests](../internal/credential/vault_test.go) | Native keyring fail-closed behavior, platform-bound secret lifecycle, and explicit legacy-vault migration evidence. |
| [Configuration security](../internal/configuration/security.go) / [tests](../internal/configuration/security_test.go) / [configuration service](../internal/configuration/service.go) / [service tests](../internal/configuration/service_test.go) / [recovery tests](../internal/configuration/recovery_test.go) | Opaque credential outcomes, manual/TOFU trust enrollment, exact confirmations, profile restoration coordination, and configuration-service boundaries. |
| [SSH trust](../internal/remote/ssh.go) / [tests](../internal/remote/ssh_test.go) | No-auth host-key inspection, deadline, secure algorithms, and mismatch behavior. |
| [Security policy](../internal/security/policy.go) / [tests](../internal/security/policy_test.go) | Local-principal authorization, pinned-trust policy, and fail-closed selector behavior used below the wizard. |
| [Mapepire policy](../internal/connectors/ibmi/mapepirestdio/policy.go) / [policy tests](../internal/connectors/ibmi/mapepirestdio/policy_test.go) / [discovery](../internal/connectors/ibmi/mapepirestdio/discovery.go) / [discovery tests](../internal/connectors/ibmi/mapepirestdio/discovery_test.go) / [artifact](../internal/connectors/ibmi/mapepirestdio/artifact.go) / [artifact tests](../internal/connectors/ibmi/mapepirestdio/artifact_test.go) | Lower-layer exact version/checksum and authenticated fallback artifact lifecycle; not Step 4 behavior. |
| [Mapepire session](../internal/mapepire/session.go) / [session tests](../internal/mapepire/session_test.go) / [protocol](../internal/mapepire/protocol.go) / [protocol tests](../internal/mapepire/protocol_test.go) | Generic bounded `mapepire-server --single` session and protocol behavior, distinct from IBM i launch policy. |
| [Readiness implementation](../internal/configuration/readiness.go) / [tests](../internal/configuration/readiness_test.go) | Offline status and bounded diagnostics; serve composition gap. |
| [Integration preview core](../internal/integrationpreview/preview.go) / [tests](../internal/integrationpreview/preview_test.go) / [OpenCode adapter](../internal/integrationpreview/opencode/preview.go) / [Copilot adapter](../internal/integrationpreview/copilot/preview.go) | Schema-validated, copy-only client previews; no external client configuration mutation. |
| [MCP server](../internal/mcp/server.go) / [tests](../internal/mcp/server_test.go) / [application service](../internal/app/service.go) / [tests](../internal/app/service_test.go) | Direct typed MCP facade, its two-tool boundary, and the lower application service; neither makes the wizard or live serve composition complete. |
| [Current configuration spec](../openspec/specs/nexus-configuration/spec.md) / [local MCP security spec](../openspec/specs/local-mcp-security/spec.md) / [IBM i catalog-context spec](../openspec/specs/ibmi-catalog-context/spec.md) | Valid requirements for configuration, security/credential/trust/diagnostics, and bounded MCP/catalog behavior. **Conflict:** the configuration spec keeps Java/Mapepire/JAR as legacy diagnostics, not current canonical wizard Steps 4–5. |
| [README](../README.md), [architecture](ARCHITECTURE.md), [security](SECURITY.md), [IBM i validation runbook](IBM_I_VALIDATION.md) | Current product, composition, security, and rollout truth. |
| [Archived configuration design](../openspec/changes/archive/2026-08-22-nexus-configuration-tui/design.md) and [verify report](../openspec/changes/archive/2026-08-22-nexus-configuration-tui/verify-report.md) | Historical evidence only. It records a prior completed configuration slice and explicitly reports no IBM i contact; it cannot promote conceptual Steps 4–9 to facts. |

## Review checklist

- [ ] Claims about wizard behavior are classified and trace to current evidence.
- [ ] Steps 4–9 remain conceptual unless an approved specification and implementation change them.
- [ ] Remote activity, authentication, persistence, and local discovery are not conflated.
- [ ] Completion/readiness text does not claim live IBM i validation or operational serve readiness.
- [ ] This document is read with, not instead of, applicable OpenSpec requirements and ADRs.
