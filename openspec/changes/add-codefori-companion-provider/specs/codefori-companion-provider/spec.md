# Code for IBM i Companion Provider Specification

## Purpose

Provide zero-configuration, bounded proof access to an active Code for IBM i session without changing profile-selected Native behavior.

## Requirements

### Requirement: Local Companion availability and credential ownership

The Companion MUST bind only to a fixed `127.0.0.1` endpoint. A collision or unavailable endpoint MUST return `unavailable`; it MUST NOT scan ports, bind wildcard or `localhost`, forward traffic, or select Native mode.

Code for IBM i exclusively owns IBM i credentials. The Companion and Nexus MUST NOT request, receive, log, or persist IBM i credentials. v1 MUST require no descriptor, bearer token, token generation, generation authentication, PowerShell, ACL setup or validation, or credential-persistence configuration.

#### Scenario: Fixed endpoint is unavailable

- GIVEN the fixed loopback endpoint cannot be reached
- WHEN a Companion operation is invoked
- THEN it returns exactly `{"state":"unavailable"}`
- AND it does not scan or fall back to Native mode

#### Scenario: Credentials remain owned by Code for IBM i

- GIVEN Companion mode is started and used
- WHEN Companion and Nexus artifacts are inspected
- THEN IBM i credentials are absent from their inputs, outputs, logs, and persistence

### Requirement: Sanitized deterministic session status

`session.status` MUST accept only an empty object and return exactly one state: `connected`, `companion_unavailable`, `codefori_extension_unavailable`, or `connection_unavailable`. It MUST not disclose endpoint, host, user, connection metadata, or raw errors.

#### Scenario: Companion is absent after startup

- GIVEN Nexus starts in Companion mode without a reachable Companion
- WHEN `session.status` is invoked
- THEN it returns exactly `{"state":"companion_unavailable"}` within one second

### Requirement: Canonical proof query and normalized result

`sql.query` MUST accept only an ASCII `sql` string of at most 128 bytes that normalizes to `SELECT CURRENT_USER FROM SYSIBM.SYSDUMMY1`: leading/trailing ASCII space, tab, CR, or LF are allowed; one-or-more such bytes are required between the four tokens; ASCII case folding is allowed; identifier/dot whitespace and every other byte or SQL form are forbidden. The executed statement MUST be exactly that 41-byte literal.

A successful result MUST contain exactly one row object, one own enumerable column of any label, and one UTF-8 string of at most 256 bytes. It MUST normalize to `{"state":"ok","rows":[{"value":"..."}]}` and discard the raw label. Invalid input MUST return `invalid_query` before execution; non-success results MUST contain no rows and be one of `unavailable`, `invalid_query`, `limit_exceeded`, `timeout`, `cancelled`, or `failed`.

#### Scenario: Accepted variation executes the proof

- GIVEN a permitted ASCII case and whitespace variation
- WHEN `sql.query` is invoked
- THEN the canonical 41-byte statement is executed
- AND the response contains only the normalized `value`

#### Scenario: Broader SQL is rejected

- GIVEN a comment, semicolon, Unicode whitespace, parameter, or extra token
- WHEN `sql.query` is invoked
- THEN it returns exactly `{"state":"invalid_query"}` before execution

### Requirement: Bounded Companion operations

The Companion MUST enforce fixed, non-configurable limits of at most 512 request bytes, 1024 response bytes, one row, one column, a 256-byte value, 16 immediately admitted operations, and 16 active proof queries; capacity MUST be at least ten. Excess work MUST return `limit_exceeded` without a queue. Proof execution MUST wait no longer than five seconds; cancellation or timeout MUST suppress late output, and an already-started operation MUST retain capacity until it settles.

#### Scenario: Capacity is exceeded

- GIVEN all immediate-operation capacity is occupied
- WHEN another proof query arrives
- THEN it returns exactly `{"state":"limit_exceeded"}` without waiting

#### Scenario: Started query times out

- GIVEN an admitted proof query does not settle within five seconds
- WHEN its deadline expires
- THEN it returns exactly `{"state":"timeout"}` with no row data
