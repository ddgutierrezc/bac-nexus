# mapepire-application-protocol Specification

## Purpose

Provide a small, typed, transport-neutral Mapepire client.

## Requirements

### Requirement: Pinned Typed Protocol Subset

The client MUST be custom Go code with no runtime `mapepire-go` dependency. It MUST be pinned to the reviewed protocol revision and compatible with Mapepire Server 2.3.5, never `latest`. It SHALL expose only `getversion`, `connect`, `prepare_sql_execute`, `sqlmore`, `sqlclose`, `ping`, and `exit`, with typed inputs, safe error categories, validation, and no arbitrary operation or generic SQL surface beyond approved bounded capabilities.

#### Scenario: Valid requests are bounded
- GIVEN a supported operation and valid bounded fields
- WHEN the client sends it
- THEN it emits a typed request with a unique ID

#### Scenario: Unsupported or malformed operation is rejected
- GIVEN an unknown operation, invalid field, or unbounded request
- WHEN validation runs
- THEN it returns a safe deterministic error without transport I/O

### Requirement: Correlated Bounded Session

Responses MUST correlate by echoed ID independently of response order; wrong, duplicate, or unknown IDs MUST fail the session safely. V1 MUST use one session with no pooling. Nexus-owned release-versioned limits MUST bound frames/messages, rows, bytes, columns, cursors, pending requests, deadlines, and session lifetime, and MUST NOT be profile-editable.

#### Scenario: Responses arrive out of order
- GIVEN two valid pending requests
- WHEN responses arrive in reverse order
- THEN each caller receives its matching typed result

#### Scenario: Correlation violation fails closed
- GIVEN a wrong, duplicate, or unknown response ID
- WHEN it is received
- THEN the session rejects it and exposes no partial result

#### Scenario: Paging remains bounded
- GIVEN `sqlmore` requests cursors, rows, columns, or bytes beyond policy
- WHEN validation runs
- THEN it rejects the request or response with a safe limit error

#### Scenario: Fixture framing is conformant
- GIVEN an official protocol fixture for the pinned revision
- WHEN WSS text or SSH LF framing is decoded
- THEN validation accepts only the documented JSON shape and bounds

### Requirement: Cancellation Semantics

The client MUST NOT claim per-query cancellation. Context cancellation MUST stop local waiting and close the whole session/transport, reporting that remote statement cancellation is not guaranteed.

#### Scenario: Query cancellation closes the session
- GIVEN a request is pending
- WHEN its context is cancelled
- THEN the session closes and reports no remote-cancel guarantee
