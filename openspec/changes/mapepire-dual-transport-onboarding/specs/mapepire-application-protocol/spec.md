# Delta for mapepire-application-protocol

## MODIFIED Requirements

### Requirement: Pinned Typed Protocol Subset and Fixed Proof

The client SHALL expose only the reviewed typed operations `getversion`, `connect`, `prepare_sql_execute`, `sqlmore`, `sqlclose`, `ping`, and `exit`. Step 8 proof SHALL use a release-owned fixed `VALUES 1` operation through those operations, with fixed projection/parameters and bounds; it MUST NOT accept SQL text, return rows or row content, or expose a generic SQL surface.
(Previously: approved bounded SQL capabilities were permitted without defining the fixed proof.)

#### Scenario: Connect precedes proof
- GIVEN a trusted transport and valid opaque credentials
- WHEN fixed proof runs
- THEN `connect` succeeds before `prepare_sql_execute`, and no proof runs after connect failure

#### Scenario: Fixed proof is redacted
- GIVEN fixed proof succeeds with bounded metadata
- WHEN the product result is returned
- THEN it contains classification and bounded proof metadata only, never SQL, parameters, columns, rows, or bytes

#### Scenario: Invalid proof bounds fail closed
- GIVEN a response or request exceeds cursor, page, row, column, byte, frame, or deadline limits
- WHEN it is validated
- THEN the session terminates with a limit classification and exposes no partial result

#### Scenario: Proof closes resources
- GIVEN a cursor or session was acquired
- WHEN proof succeeds, fails, or is cancelled
- THEN `sqlclose` is attempted as applicable, then `exit` and transport/session cleanup are completed

### Requirement: Correlated Bounded Session

Responses SHALL correlate by echoed ID; wrong, duplicate, or unknown IDs SHALL terminate the session. Release-owned bounds SHALL cover cursors, pages, rows, columns, bytes, pending IDs, frames, deadlines, and session lifetime, and SHALL not be profile-editable.
(Previously: the requirement covered bounded paging but not the fixed proof lifecycle.)

#### Scenario: Paging is bounded
- GIVEN fixed proof requests a cursor/page beyond any release limit
- WHEN validation runs
- THEN it returns a deterministic limit failure without unbounded I/O

### Requirement: Cancellation Semantics

Cancellation SHALL stop local waiting and close every acquired session/transport resource; it SHALL NOT claim remote statement cancellation.
(Previously: cancellation was specified only for the generic client request.)

#### Scenario: Terminal cancellation
- GIVEN proof is waiting on connect, query, close, or exit
- WHEN context cancellation occurs
- THEN the whole session closes and the result is cancelled without a readiness claim
