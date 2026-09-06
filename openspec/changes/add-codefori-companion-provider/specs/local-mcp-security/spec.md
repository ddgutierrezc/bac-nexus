# Delta for local-mcp-security

## ADDED Requirements

### Requirement: Companion v1 local-machine trust boundary

Companion v1 MUST bind only to `127.0.0.1`, use no peer authentication, and require zero additional user configuration. It MUST reject every browser-origin request with `browser_origin_rejected`; it MUST NOT bind wildcard or `localhost`, expose a remote endpoint, scan ports, forward traffic, or fall back to Native mode.

The Companion MUST allow only `session.status` and the fixed proof query. It MUST NOT provide arbitrary SQL or execution capabilities. It MUST enforce 512-byte bodies, 1024-byte results, a five-second wait, 16 immediate operations/active queries, and no queue. v1 establishes machine-local reachability only, not Windows user, session, process, or extension-host identity. Any local process may attempt the narrow calls; that accepted risk and deferred hardening MUST be documented.

#### Scenario: Local unauthenticated call is allowed

- GIVEN a non-browser local process reaches the loopback endpoint
- WHEN it invokes an allowlisted operation with valid input
- THEN the Companion evaluates it without caller authentication

#### Scenario: Browser-origin call is rejected

- GIVEN a request includes a browser origin
- WHEN it targets the Companion endpoint
- THEN it returns exactly `{"state":"browser_origin_rejected"}` without execution

#### Scenario: Remote reachability is attempted

- GIVEN a caller is not using the loopback endpoint
- WHEN it attempts a Companion operation
- THEN no Companion endpoint is reachable

## MODIFIED Requirements

### Requirement: Local-Principal Authorization

The current local OS principal MUST remain the Native catalog-context trust boundary. Companion v1 instead uses the accepted unauthenticated local-machine boundary. Advisory selectors and `clientInfo` MUST NOT authenticate a product or principal. Unauthorized, unknown, or malformed Native selectors MUST fail closed with `unauthorized` before remote contact.
(Previously: the local OS principal was the system-wide trust boundary.)

#### Scenario: Authorized selector proceeds

- GIVEN an advisory selector permits a Native read-only operation
- WHEN it invokes a catalog-context tool
- THEN authorization permits evaluation

#### Scenario: Unauthorized selector is rejected

- GIVEN an unknown selector or one without source permission
- WHEN it invokes a Native catalog-context tool
- THEN it receives `unauthorized` and no remote operation occurs

### Requirement: Sanitized Read-Only Surface and Audit

Native mode MUST preserve its existing read-only tool, profile, credential, eligibility, host-pin, ownership, audit, and fail-closed requirements. Companion mode MUST expose exactly `session.status` and fixed `sql.query`; it MUST not construct Native operations, credentials, profiles, transports, audit state, or fallback. Its observability MAY contain only fixed classifications and MUST exclude IBM i credentials, SQL, values, labels, endpoints, identities, and raw errors.
(Previously: serve admission and the read-only surface applied only to profile-selected Native mode.)

#### Scenario: Audit records a successful page
- GIVEN an authorized Native source page succeeds
- WHEN its audit outcome is recorded
- THEN it contains only approved classification and count metadata

#### Scenario: Audit records a denied request
- GIVEN an unauthorized Native request is rejected
- WHEN its audit outcome is recorded
- THEN it records no sensitive material

#### Scenario: Ineligible noninteractive serve is rejected
- GIVEN required Native admission evidence is unavailable
- WHEN Native serving is requested
- THEN it fails closed before remote contact

#### Scenario: Audit policy or write fails
- GIVEN Native retention is invalid or an append fails
- WHEN startup or an operation requires audit
- THEN startup is blocked or that operation fails safely

#### Scenario: Remote path control is unavailable
- GIVEN a Native caller supplies a generic remote capability
- WHEN it requests catalog context
- THEN it is rejected as unsupported

#### Scenario: Diagnostic fails safely
- GIVEN a Native configuration diagnostic fails
- WHEN its outcome is shown or audited
- THEN it is sanitized and claims no live validation
