## Exploration: production Step 8 orchestration for mapepire-dual-transport-onboarding

### Current State

This exploration is read-only architecture work. It does not change the authoritative proposal, seven delta specs, design, tasks, apply progress, or failed verification report. The current branch is `fix/mapepire-dual-transport-verification`; its uncommitted remediation is preserved exactly.

The original six slices are checked, but that evidence proves lower-layer seams and injected TUI tests, not the complete production Step 8 route. The partial remediation adds a production managed-daemon `/version` probe, default Step 4 probe factory, independent trust visibility, pinned protocol-fixture validation, and bounded policy/trust/version audit metadata. Those corrections do not compose credentials, authenticated WSS, fallback runtime, or Step 8 into the production wizard. The failed `verify-report.md` remains immutable and correctly records the earlier gaps; it is not acceptance evidence for the proposed work.

#### Exact current production path

`cmd/nexus/main` → `runCommand` → `runConfigure` → `runConfigureTUI` (`tui.RunWithHostIdentityInspector`) → `tea.NewProgram` → `newModelWithIdentityInspector` → Bubble Tea `Model.Update`/`View`. `runConfigure` injects only `profile.Store` and `remote.HostIdentityInspector{}`. It does not inject a credential store, trust persistence service, resolver, WSS client factory, SSH fallback factory, proof service, auditor, or Step 8 command.

Step 3 can run the existing no-auth, cancellable SSH host-key inspection. Its consumer-owned inspector accepts only `(context, host, port)`, and the completed candidate is retained in draft state. On acceptance, navigation reaches `screenProfileMapepire` (Step 4). The current Step 4 has a `mapepireFactory` and `preAuthProbe` seam; the partial remediation supplies a default `ManagedDaemonProbe` factory, which performs bounded unauthenticated HTTPS `/version` against managed `wss://host:port` (default policy target is `8076`). The probe is pre-auth only and returns authentication-pending state. Step 4 has no credentials, session, fallback, query, or runtime lifecycle.

There is no production Step 8 screen or command. `step8Client`/`profileProofClient` and `runProfileStep8Proof` are helper/test-only. The helper only calls `Connect` then `Query`; it does not retrieve credentials, select WSS/SSH, perform consent, create a session, classify failures, audit, cancel, close, or update UI state. Therefore the exact missing production chain is:

```text
Step 8 user action
  -> credential policy/prompt or native keyring Get
  -> resolver using persisted policy/trust and ephemeral Step 4 observation
  -> trusted WSS dial + authenticated Mapepire connect
  -> fixed bounded read-only proof query
  -> typed result/error + metadata-only audit + UI result
  -> close cursor, exit, transport/session, SSH process/client
```

No link in that chain is currently composed by `cmd/nexus` or the wizard.

#### Existing lower-layer capabilities

- `internal/credential`: `CredentialStore` is exact and profile-scoped (`Get`, `Set`, `Delete`); native keyring is fail-closed and returns opaque presence/errors. Secrets are not accepted by `tui.Model` and must be held only in a short-lived orchestration scope.
- `internal/profile`: atomic secret-free JSON storage and schema-v2 trust/policy fields exist. Selected transport, version, readiness, errors, credentials, cache paths, and query results must remain ephemeral.
- `internal/configuration`: `Resolver.Resolve` prefers `DaemonProbe`, classifies availability/policy/unsupported as eligible, blocks identity/protocol/credential/authorization failures, requires SSH trust and consent before `SSHFallback.Start`, and records transport metadata. Its current `SSHFallback` seam is too early/coarse for the full authenticated proof lifecycle.
- `internal/configuration/daemon.go`: `ManagedDaemonProbe` performs bounded `/version` with TLS identity/pin handling, but its current construction uses a `wss` endpoint string for the HTTP URL and is not an authenticated WSS application client.
- `internal/mapepire`: typed envelopes/session, bounded IDs/cursors/frames, one reader, cancellation-terminal closure, and `MessageTransport` exist. The legacy `Session.Execute` remains used by catalog callers; no production Step 8 client factory or fixed proof operation exists.
- `internal/mapepire/wss`: trusted WSS text framing, bounded reads, compression disabled, and terminal identity/cancellation behavior exist. It is an adapter, not a credentialed session/proof service.
- `internal/mapepire/sshstdio`: authenticated process-channel LF framing exists. `internal/remote.Client.StartMapepireTransport` calls fixed `BuildCommand`; `remote.Dial` authenticates with a password and owns SSH/SFTP cleanup.
- `internal/connectors/ibmi/mapepirestdio`: pinned artifact verification, discovery, upload/rollback, Java validation, and fixed `--single` command construction exist. They must be invoked only after fallback classification, trust, credentials, authorization, and explicit consent.
- `internal/configuration/readiness.go`: `RunRemoteDiagnostic` supplies a generic timeout/cancellation wrapper and sanitized classifications, but does not provide the typed dual-transport Step 8 orchestration contract.
- `internal/audit`: transport events are bounded metadata-only records. The remediation adds policy identity, trust outcome, and version bounds; Step 8 still needs proof attempt/result classifications without secrets, endpoint, SQL, raw errors, or result content.
- `cmd/catalogspike`: credentialed live composition is separate from wizard composition and is not evidence that `nexus configure` owns this route.

### Affected Areas

- `cmd/nexus/main.go` and `internal/tui/model.go` — production composition root must inject a Step 8 service without putting connector/security logic in Bubble Tea.
- `internal/tui/profile_identity_step.go`, `mapepire_onboarding_step.go`, wizard navigation/render/viewport files and tests — Step 3/4 boundaries must remain pre-auth; a new explicit Step 8 effect needs request identity, cancellation, stale-result rejection, feedback, and cleanup.
- `internal/configuration/` — owns orchestration-facing resolver policy, bounded endpoint/version observations, failure classification, and a consumer-owned authenticated proof boundary.
- `internal/profile/` and `internal/credential/` — provide validated profile metadata and the single credential retrieval boundary; no secret enters model/messages/views.
- `internal/mapepire/`, `wss/`, `sshstdio/` — reuse typed protocol and transport adapters; do not duplicate framing, trust, correlation, or session lifecycle.
- `internal/remote/` and `internal/connectors/ibmi/mapepirestdio/` — provide authenticated SSH, fixed artifact/Java/upload/`--single` lifecycle behind a fallback-only interface.
- `internal/audit/` — extend only with allowlisted proof attempt/outcome metadata and bounded failure classes.
- `docs/IBM_I_PROFILE_WIZARD.md` — update FACT/PROPOSAL status only as each production slice becomes specified and implemented; do not claim the full nine-step journey prematurely.
- Existing OpenSpec proposal/spec/design/tasks/apply-progress/verify-report — require later amendments after this exploration; none are changed here. No new capability spec is needed if Step 8 remains part of `mapepire-transport-onboarding`; a new spec is warranted only if the proof becomes a reusable product capability outside onboarding.

### Approaches

1. **Application-owned Step 8 proof orchestrator (recommended)** — add a small configuration/application service that receives profile metadata, a credential provider, resolver/transport factories, fixed proof operation, consent, and auditor; TUI owns only commands/messages/rendering.
   - Pros: complete production path; consumer-owned narrow interfaces; transport and secrets stay below TUI; easy offline fakes and rollback; preserves WSS-first/no-downgrade policy.
   - Cons: requires explicit composition and a new result contract; must coordinate two session lifecycles.
   - Effort: High, but sliceable.

2. **Put the route in `internal/tui` around the current helper** — construct credentials, WSS/SSH, query, and cleanup from the Step 8 update handler.
   - Pros: fewer initial files.
   - Cons: violates TUI ownership, exposes secret/lifecycle risk, duplicates resolver/session logic, and is difficult to test or audit.
   - Effort: Medium initially, High risk and maintenance cost.

3. **Keep the injected `profileProofClient` seam and call it production** — supply a client from the composition root while leaving orchestration elsewhere implicit.
   - Pros: smallest diff.
   - Cons: does not define credential boundary, WSS-first selection, fallback consent, failure matrix, audit, cleanup, or fixed query; exactly the gap that caused verification to fail.
   - Effort: Low, insufficient.

### Recommendation

Use Approach 1. Introduce a configuration/application-owned `Step8ProofService` (name can be finalized during design) with consumer-owned interfaces. The service should receive a validated profile snapshot and explicit consent, retrieve one opaque credential at the last responsible moment, resolve WSS first, authenticate through the existing typed client over `wss.Transport`, and run one fixed bounded read-only proof. Only eligible daemon failures may invoke an SSH fallback factory. The service owns `defer`-style cleanup for cursor/session/transport, zeroes credential buffers, emits one sanitized audit outcome, and returns a typed result suitable for TUI feedback.

The WSS boundary is: TLS trust and endpoint policy are configured once by the WSS adapter; `/version` remains an unauthenticated observation; the proof service supplies credentials only to the typed `connect` request through a credential-aware client operation. It must not reimplement TLS pinning, WebSocket framing, request correlation, or session closure. The SSH boundary is: fallback classification and consent precede `remote.Dial`; the authenticated client then reuses verified artifact/Java/upload/rollback and `StartMapepireTransport`, with no arbitrary command or generic SQL input.

The fixed proof should be a release-owned operation, not user-provided SQL: `connect` followed by one allowlisted, bounded `prepare_sql_execute` against the approved Db2 metadata/health relation, with a fixed projection, parameter policy, row/page/byte/column limits, `sqlclose`, and `exit`. The result should contain only success, transport, protocol version, authenticated/session/query booleans, bounded row count or proof code, classification, and cleanup status. It must never contain SQL text, row content, credentials, endpoint, path, or raw remote errors.

### Risks

- The existing `Resolver` performs pre-auth selection and, for SSH, starts runtime before the authenticated proof service owns the complete lifecycle. The design must split pre-auth observation from post-credential session selection or define a richer application boundary without allowing premature SSH start.
- `ManagedDaemonProbe` currently constructs `wss://...` then derives `https://.../version`; exact endpoint policy, CA/hostname trust, pin/TOFU enrollment, and default port semantics need one authoritative composition path.
- A daemon `/version` or WebSocket handshake is not authentication. Only successful `connect` establishes authenticated session state.
- Credential errors and authorization errors are terminal and must never trigger SSH fallback. TLS/SSH identity, protocol tampering, malformed version, unsafe downgrade, bounds, and cancellation are terminal/no-downgrade as well.
- SSH fallback has more cleanup surfaces: SSH client, SFTP client, process channel, temporary artifact, cursor, session, and secret memory. Partial failure must close every acquired resource and report a sanitized classification.
- Bubble Tea currently uses `context.Background()` in the Step 4 command and has no Step 8 request identity/cancel field. The new effect must use the program lifecycle context, explicit child cancellation, and stale-result protection.
- Existing passing helper tests do not prove production composition. Acceptance must exercise the actual `runConfigure`/model constructor path with counting fakes and a loopback WSS harness.
- No live IBM i, credentials, Java, artifact transfer, or remote SSH process is permitted in normal verification; status remains `not_validated_on_ibmi`.

### Proposed production contract and failure matrix

#### Interfaces and ownership

`internal/configuration` (or a narrowly named application package) owns `Step8ProofService`, `Step8Request`, `Step8Result`, `ProofFailure`, and orchestration. Consumer-owned interfaces should be no broader than:

```go
type CredentialProvider interface { Get(context.Context, string) ([]byte, error) }
type TransportResolver interface { ResolveAuthenticated(context.Context, AuthRequest) (ResolvedSession, error) }
type ProofSession interface { Connect(context.Context, Credential) error; RunFixedProof(context.Context) (ProofEvidence, error); Close(context.Context) error }
type Auditor interface { Record(context.Context, ProofAuditEvent) error }
```

Concrete WSS/SSH factories and credential stores are composed in `cmd/nexus`; TUI sees only `Run(context.Context, Step8Request) tea.Cmd` or an equivalent service call. `internal/mapepire` remains responsible for typed operations, correlation, limits, and session lifecycle; adapters remain responsible for wire framing/trust/process mechanics. No TUI package may import credential, SSH, artifact, Java, or SQL implementation packages.

#### No-downgrade matrix

| Classification | WSS result | SSH fallback | Audit/UI |
|---|---|---|---|
| supported `/version`, trusted WSS, connect success | continue WSS | no | selected + proof result |
| daemon refusal/timeout/unavailable | eligible only if policy permits | require independent SSH trust, credentials, consent | fallback reason |
| verified unsupported version | eligible only if policy permits | same gates | unsupported reason |
| daemon disabled by policy | no WSS attempt | same gates | policy reason |
| TLS hostname/pin/expiry/rotation/TOFU failure | terminal | never | blocked identity |
| WSS protocol/malformed/unsafe downgrade | terminal | never | blocked protocol |
| WSS connect credential/authorization failure | terminal | never | blocked credentials/authorization |
| SSH trust mismatch or missing trust | terminal | no runtime | blocked trust |
| SSH credential/authorization failure | terminal | no further attempt | blocked credentials/authorization |
| artifact/Java/upload/launch failure | terminal for this proof | no alternate silent transport | runtime failure |
| consent absent, cancellation, timeout, limit | terminal for this attempt | no implicit retry/downgrade | cancelled/timeout/blocked |

### Reviewable implementation slices

Each slice is independently coherent, rollback-safe, and must stay under the normal 400 additions plus deletions. Counts are planning estimates, not permission to hide integration work.

1. **Contract and fixed proof domain** (about 180–260 lines). Dependency: current typed protocol/session. Files: new/modified `internal/configuration/step8*`, tests. Add typed request/result/errors, fixed query identifier and bounds, credential-provider and audit interfaces, and pure failure mapping. Focused test: `go test -count=1 ./internal/configuration`. Runtime harness: in-process fake session/credential/auditor. Rollback: remove only new Step 8 domain files. Acceptance: no SQL input, no secret/result-content fields, all classifications table-tested.
2. **Authenticated WSS session factory** (about 260–360 lines). Dependency: slice 1 and existing `wss`/typed client. Files: `internal/mapepire` client integration, `internal/mapepire/wss` factory/credential connect tests, configuration adapter. Focused test: `go test -count=1 ./internal/mapepire ./internal/mapepire/wss ./internal/configuration`. Runtime harness: loopback `httptest` TLS/WSS server exercising `/version`, text frames, connect, fixed proof, close. Rollback: WSS authenticated factory and tests only. Acceptance: TLS logic is reused, one credential boundary, connect-before-query, all cleanup paths.
3. **SSH authenticated fallback runtime adapter** (about 300–390 lines). Dependency: slice 1; reuse existing `remote` and `mapepirestdio`. Files: fallback adapter/orchestration tests in `internal/configuration`, narrowly adjusted `internal/remote` seams only if required. Focused test: `go test -count=1 ./internal/configuration ./internal/mapepire/sshstdio ./internal/remote ./internal/connectors/ibmi/mapepirestdio`. Runtime harness: fake SSH/channel/process and artifact/Java/upload counting fakes; no IBM i. Rollback: new fallback adapter and minimal seams. Acceptance: classification → trust → credential → consent → fixed artifact/runtime → connect; no arbitrary command; complete cleanup.
4. **Production composition root and Step 8 command lifecycle** (about 280–390 lines; do not combine with the TUI slice). Dependency: slices 1–3. Files: `cmd/nexus/main.go`, a composition adapter, production constructor tests. Focused test: `go test -count=1 ./cmd/nexus ./internal/configuration`. Runtime harness: actual configure composition with fake local stores and loopback WSS; prove daemon path makes zero SSH/artifact calls and fallback path uses the same credential reference. Rollback: composition adapter and constructor wiring. Acceptance: production path exists from `runConfigure` to service; no injected-only proof.
5. **Bubble Tea Step 8 effect/UI lifecycle** (about 300–390 lines). Dependency: slice 4. Files: `internal/tui/model.go`, Step 8 files/tests, wizard viewport/localization as needed. Focused test: `go test -count=1 ./internal/tui`. Runtime harness: actual `Update`/`View` at 120x40, 80x24, 40x16 and `NO_COLOR`, loading/cancel/stale success/failure/retry/back. Rollback: Step 8 UI and command wiring only. Acceptance: explicit consent, focusable blocked action, child cancellation, request ID stale protection, terminal feedback, no secret in model/messages/view, responsive navigation.
6. **Audit/documentation and integrated verification evidence** (about 180–280 lines). Dependency: slices 1–5. Files: `internal/audit`, relevant tests, `docs/IBM_I_PROFILE_WIZARD.md`, later authoritative OpenSpec amendments. Focused test: `go test -count=1 ./internal/audit ./internal/configuration ./internal/tui`. Runtime harness: composition-level counting-fake matrix. Rollback: audit additions/docs only. Acceptance: allowlisted metadata, cleanup/cancellation evidence, docs classify current versus proposed behavior, full suite/vet/build/diff checks pass. Keep authoritative artifact changes separate from implementation if their own review exceeds 400 lines.

### Precisely required planning amendments

After maintainer review of this exploration, amend rather than silently reinterpret:

- **Proposal:** replace “Step 8 proves `connect`/optional bounded query” with the explicit production service boundary, credential retrieval timing, WSS-first authenticated flow, SSH fallback gates, fixed proof, cleanup, audit, and UI ownership. Clarify that current partial remediation is not production composition.
- **Design:** replace the current pre-auth-only `Resolver`/`SSHFallback` description with separate pre-auth observation and post-credential proof orchestration; define consumer-owned interfaces, typed result/error types, composition root, fixed proof operation, lifecycle, and exact dependency direction. Correct the claim that wizard composition is covered by the existing six slices.
- **Tasks:** retain completed 1.x/2.x work and split 3.x into the six slices above. Uncheck or supersede any task whose wording claims Step 8 production composition is complete. Add per-slice dependency, focused command, runtime harness, rollback boundary, and measured <=400 forecast. Do not put all integration in one “TUI” task.
- **Specs:** expand `mapepire-transport-onboarding` with a requirement for production Step 8 authenticated proof orchestration and scenarios for credential boundary, WSS-first, no-downgrade, fallback consent/runtime, fixed query, cancellation/cleanup, stale UI result, and sanitized audit. Expand `mapepire-application-protocol` only if the fixed proof requires a missing typed operation; do not add generic SQL. Expand `nexus-configuration` for explicit Step 8 result/readiness semantics. Expand `local-mcp-security` for proof audit and secret lifetime if not already covered. Existing WSS, SSH, and fallback-runtime specs need scenario cross-links, not a new capability.
- **New capability spec:** not required; Step 8 is an orchestration requirement of `mapepire-transport-onboarding`. Create a new capability only if the service will be reused by MCP/serve or other workflows independently of onboarding.
- **Wizard guide:** update only after each slice is implemented and verified. Preserve the distinction: current Step 4 is pre-auth, current Steps 5–9 are not fully composed, and live IBM i remains unvalidated.

### Treatment of current remediation and failed verification

Preserve all uncommitted remediation files and `.atl/`/`tmp/` state exactly; do not edit or include them in planning boundaries. Do not relaunch the remaining native Step 8 attempt before authoritative planning is amended. Do not settle/acquire/reset attempts, archive, or alter the failed report. The failed report is a historical blocker record: its production-composition finding remains valid as the reason for this re-plan, while its earlier missing-probe/audit/fixture findings are addressed by the uncommitted remediation and must be re-evaluated only by a fresh independent verification after the new slices are complete.

### Unresolved Decisions

- Whether the fixed proof relation/projection is approved for all target IBM i environments, and what non-sensitive proof evidence may be shown to the operator.
- Whether Step 8 runs against an already saved profile only, or may use a temporary secret-free candidate before Step 7 persistence; the guide currently proposes saved-profile/explicit temporary behavior but does not decide it.
- Whether `Ask each time` and `Store securely` are both V1 options now, including the exact keyring prompt UX and legacy `vault|prompt` migration semantics.
- Whether the managed daemon endpoint is always policy-owned `8076` or an approved deployment policy may supply another endpoint without making it a user transport choice.
- Which TLS and SSH trust modes are permitted in target environments (CA/pin/TOFU and independent SSH verification), including rotation/re-enrollment policy.
- Whether SSH artifact acquisition/upload/Java validation is approved as part of this change’s Step 8 proof or must remain a separately approved dependent capability.
- Whether Step 8 failure/success status is ephemeral only, or whether any sanitized “tested” marker may be persisted; the safe default is ephemeral.

### Ready for Proposal

No for immediate proposal amendment until the maintainer resolves the seven decisions above. Yes for design preparation: the production gap is evidenced, the recommended boundary is clear, and the work can be sliced without downgrading Step 8 to a helper seam. The next action should be an interactive proposal/design amendment review, followed by task replanning; no native attempt should be launched before that.
