# Delta for mapepire-transport-onboarding

## MODIFIED Requirements

### Requirement: Production Step 8 Proof and Transport Ownership

Steps 3 and 4 SHALL remain credential-free, runtime-free pre-auth observations; Step 7 SHALL persist the profile; only Step 8 SHALL retrieve credentials, select/authenticate transport, perform fallback, and run proof. Step 8 SHALL accept only a saved profile, use policy-owned endpoint/trust, WSS first, and return bounded typed evidence. Fallback eligibility SHALL use distinct typed classifications: `daemon_refused`, `daemon_unavailable`, `daemon_availability_timeout`, `daemon_policy_disabled`, or `unsupported_version`. `operation_timeout` after transport/session start, `cancelled`, `proof_timeout`, `cleanup_timeout`, `limit_exceeded`, `identity_failure`, `trust_mismatch`, `protocol_failure`, `framing_failure`, `malformed_response`, `downgrade_blocked`, `credentials_unavailable`, and `authorization_denied` SHALL be terminal and MUST NOT fallback. Bubble Tea SHALL issue an explicit cancellable command/effect with request identity; the service, not the TUI, owns credentials, sessions, cleanup, and audit. Historical markers SHALL never establish readiness or skip proof.
(Previously: Step 8 behavior was described through injected helper seams and optional proof.)

#### Scenario: Saved profile gate
- GIVEN a saved profile and an unsaved draft
- WHEN Step 8 is requested for either
- THEN only the saved profile proceeds; the draft is rejected without credentials or runtime calls

#### Scenario: Step 3 owns SSH identity observation
- GIVEN Step 3 is active
- WHEN SSH identity inspection or enrollment runs
- THEN it handles only explicit SSH host identity evidence and makes no credential, authentication, or runtime call

#### Scenario: Step 4 owns WSS pre-auth observation
- GIVEN Step 4 is active
- WHEN managed daemon observation runs
- THEN it performs only policy-owned WSS `/version` observation and makes no credential, authentication, query, SSH fallback, or runtime call

#### Scenario: Step 7 and Step 8 ownership
- GIVEN a profile is ready to advance
- WHEN Step 7 saves and Step 8 is requested
- THEN Step 7 persists the profile and only Step 8 retrieves credentials, authenticates, selects/falls back, and proves

#### Scenario: WSS-first proof
- GIVEN policy permits WSS and pre-auth observation is trusted
- WHEN Step 8 runs
- THEN it authenticates WSS, executes fixed proof, and reports no SSH/artifact/Java/upload call

#### Scenario: Fallback is bounded
- GIVEN daemon establishment refusal/unavailability, availability timeout, policy-disabled daemon, or verified unsupported version is returned
- WHEN policy, independent SSH trust, credentials, and explicit consent all pass
- THEN managed SSH fallback may run; operation timeouts, cancellation, proof/cleanup timeouts, limits, identity/trust, protocol/framing/malformed/downgrade, credential, and authorization failures never fall back

#### Scenario: UI request lifecycle
- GIVEN a Step 8 request is loading
- WHEN it is cancelled, retried, navigated back, or an older result arrives
- THEN cleanup occurs, retry uses a new request ID, and stale results are rejected without changing current state

#### Scenario: Cancelled Step 8 is terminal and retryable
- GIVEN Step 8 cancellation has completed, every acquired resource is closed, and a prior result is pending
- WHEN the Bubble Tea UI renders the cancelled request
- THEN it shows explicit sanitized `cancelled` terminal feedback with an actionable retry/back path, renders neither stale success nor failure, rejects stale results by request ID, and persists no readiness

#### Scenario: UI feedback remains actionable
- GIVEN Step 8 is loading, fails, or succeeds
- WHEN the Bubble Tea view updates at supported narrow and wide sizes
- THEN it shows actionable loading/cancel, sanitized failure/retry, or terminal success feedback while preserving focus, reachability, request-ID rejection, and responsive layout

### Requirement: Truthful Readiness and Historical Marker

Readiness SHALL distinguish observation, authentication, and fixed proof. A persisted marker MAY contain only bounded timestamp, outcome classification, and proof revision; it SHALL exclude endpoint, transport, version, user, path, raw error, SQL, rows, and secrets, and SHALL become stale/invalid after endpoint, policy, or trust changes.
(Previously: selected transport, readiness, version, and errors were only described as ephemeral.)

#### Scenario: Marker cannot bypass proof
- GIVEN a prior successful marker or a stale marker
- WHEN Step 8 is requested
- THEN a fresh authenticated proof is required and the marker is not readiness evidence

#### Scenario: Pre-auth copy is exact
- GIVEN trusted pre-auth protocol detection without credentials
- WHEN Step 4 renders its result
- THEN it reports authentication pending and not an authenticated or query-ready state

### Requirement: Production Configure Composition

The real `cmd/nexus` `configure` composition path MUST construct and invoke the Step 8 application service. Injected/helper-only seams, unit fakes, and `cmd/catalogspike` composition MUST NOT count as production proof. Acceptance MUST include deterministic composition-level evidence exercising the real configure wiring with counting fakes or loopback services.

#### Scenario: Real configure invokes Step 8
- GIVEN `cmd/nexus configure` is composed through its production entrypoint
- WHEN the operator invokes Step 8 for a saved profile
- THEN the composed application service is invoked, not a test-only helper

#### Scenario: Composition evidence is deterministic
- GIVEN offline counting fakes or loopback services are available
- WHEN composition-level acceptance runs
- THEN it proves service construction/invocation and daemon zero-SSH behavior without IBM i or live credentials

#### Scenario: Helpers do not satisfy production proof
- GIVEN a unit fake, injected helper seam, or `cmd/catalogspike` path can run proof
- WHEN production composition is evaluated
- THEN that evidence is rejected unless the real `cmd/nexus configure` path invokes the application service
