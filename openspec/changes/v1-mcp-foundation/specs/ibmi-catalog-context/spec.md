# ibmi-catalog-context Specification

## Purpose

Provide local MCP clients with bounded, read-only IBM i catalog discovery and coherent exact-member source pagination.

## Requirements

### Requirement: Bounded Catalog Resolution

The system MUST expose typed, read-only `resolve_catalog_candidates`. It MUST return at most 50 stable candidate coordinates or `not_found`, and MUST NOT select an ambiguous candidate.

#### Scenario: Matching candidates are resolved

- GIVEN an authorized client submits a valid catalog query with matching entries
- WHEN it invokes `resolve_catalog_candidates`
- THEN it receives at most 50 candidate coordinates and no source content

#### Scenario: Query is ambiguous or absent

- GIVEN a valid query has multiple or no matching entries
- WHEN the client resolves candidates
- THEN it receives the bounded candidate set or deterministic `not_found`, respectively

### Requirement: Exact Source Page Contract

The system MUST expose typed, read-only `read_selected_source` for an authorized exact selection. A range MUST use positive, one-based inclusive `startLine` and positive `maxLines`; pages MUST contain complete UTF-8 lines without LF delimiters, preserve trailing spaces, and never split a line or Unicode code point. LF is the separator; a final unterminated record is one line. Each response MUST contain `startLine`, `lineCount`, `lines`, `eof`, and, unless EOF, `nextStartLine`.

#### Scenario: First page starts a traversal

- GIVEN an authorized client supplies an exact selection, line 1, and no cursor
- WHEN it invokes `read_selected_source`
- THEN it receives the requested complete lines and an opaque cursor with `nextStartLine` unless EOF

#### Scenario: Empty and final records are deterministic

- GIVEN a member is empty or has a final record without LF
- WHEN its valid first page is read
- THEN the empty member returns EOF with no next line, or the final record is returned as one line

### Requirement: Immutable Snapshot Lease and Freshness

The first valid request MUST acquire one immutable process-local snapshot lease; later pages MUST use that lease. Every request MUST freshly re-query the catalog and require one exact current coordinate match. A cursor MUST be opaque and bound to client policy identity, canonical selection, and process instance. Replay, out-of-order, and concurrent valid pages MUST return their same immutable page. A changed coordinate MUST return `stale_coordinate`; changed source bytes alone MUST continue the captured snapshot.

#### Scenario: Cursor access is coherent

- GIVEN a valid cursor and unchanged current coordinate
- WHEN the client replays, reorders, or concurrently requests valid line pages
- THEN each page is served from the same lease after its fresh coordinate check

#### Scenario: Cursor cannot cross its binding

- GIVEN a cursor is replayed by another policy identity, selection, or process
- WHEN source is requested
- THEN it fails with `snapshot_invalid` or `snapshot_expired` and no content

### Requirement: Bounded Lease Lifecycle

The system MUST acquire one fixed remote copy/download per snapshot, enforce a 4 MiB complete-member ceiling and 16 MiB aggregate snapshot quota, and publish a cursor only after complete UTF-8 validation and confirmed immediate remote-temporary cleanup. CPYTOSTMF MUST write only the exact Nexus-owned IFS target; the original QSYS member MUST remain immutable and MUST NOT be modified or deleted. A lease MUST expire after 10 idle minutes, refreshed only by valid page access. Recovery SHALL follow the Durable Temporary Ownership and Recovery requirement.

#### Scenario: Resource limits and acquisition failures are safe

- GIVEN acquisition exceeds a member, aggregate, line, or response limit, is invalid UTF-8, is cancelled, times out, or cleanup cannot be confirmed
- WHEN the request is processed
- THEN it returns `source_too_large`, `capacity`, `line_too_large`, `response_too_large`, `invalid_source_encoding`, `cancelled`, `deadline_exceeded`, or a sanitized connector error, with no cursor or content

#### Scenario: Expiry restarts coherent traversal

- GIVEN a cursor has expired, been evicted, or the process restarted
- WHEN a later page is requested
- THEN it returns `snapshot_expired`; a full traversal reacquires a snapshot at line 1 and never mixes snapshots

#### Scenario: Acquisition preserves the source member

- GIVEN an authorized exact selection is acquired
- WHEN Nexus copies it to its temporary target
- THEN only that exact target is written and the QSYS member is neither modified nor deleted

### Requirement: Durable Temporary Ownership and Recovery

Before remote reservation or copy, the system MUST commit and exactly reopen/read back one SQLite ownership row for the stable 128-bit token. The local-only database MUST be outside network filesystems and store only version, token, exact validated remote path, approved bounded selector, target-binding digest, and creation time; it MUST NOT store source, credentials, commands, cursors, model content, or host/user plaintext. Paths MUST remain beneath the validated home in a Nexus-private `0700` directory, and temporaries MUST be exclusive random `0600` files.

Admission MUST use a single-writer, bounded transaction that atomically counts and inserts. The same token and exact row SHALL be idempotent; active rows MUST be unique by token and path, limited to 64, and listed with `LIMIT 65`. Remote work is forbidden until commit and exact readback succeed. Busy handling MUST use a bounded timeout and context-aware bounded retries; ambiguous commit MUST resolve by exact-token readback, while mismatch, corruption, or policy/dependency denial MUST fail closed.

Every open MUST verify `application_id=1111573326` (`0x4241434E`, ASCII `BACN`), `user_version=1`, the exact schema, journal/durability pragmas, path/permissions, and bounded integrity checks. Mismatch, corruption, or policy denial MUST fail closed without destructive rebuild. Recovery MUST revalidate the profile, credential, binding, pin, directory, token, and exact path; it MUST complete exact `Remove` plus `Stat`-not-found before transactional row deletion. Remote uncertainty retains the row. Stale-row resurrection is safe because tokens and paths MUST NOT be reused. Historical `/tmp` paths MUST NOT be discovered or deleted automatically. Dependency approval unavailability MUST prevent acquisition deterministically; tests MUST NOT claim to prove power-loss durability.

#### Scenario: Caller cannot direct temporary handling

- GIVEN an MCP caller starts a source traversal
- WHEN it submits its typed request
- THEN Nexus derives the temporary path itself and accepts no temporary, list, or delete path

#### Scenario: Atomic SQLite admission is confirmed

- GIVEN a stable token is new or exactly matches an active row
- WHEN Nexus admits it below the 64-row limit
- THEN it commits and exact-readbacks the row before any remote reserve or copy

#### Scenario: Ledger admission fails closed

- GIVEN SQLite reports row 65, contention beyond bounded retries, corruption, a policy/dependency denial, or a verification mismatch
- WHEN Nexus attempts admission or recovery
- THEN it performs no remote work and publishes no snapshot, cursor, or content

#### Scenario: Recovery validates ownership and target

- GIVEN a ledger record names an exact private temporary path
- WHEN startup or pre-acquisition recovery runs
- THEN Nexus revalidates ownership, binding, credential, and pinned host before exact-path removal

#### Scenario: Crash recovery is idempotent

- GIVEN a crash occurred before copy, during copy, or after remote removal before row deletion
- WHEN recovery confirms the recorded exact path is absent
- THEN it deletes only that row; otherwise it retains the row for later recovery

#### Scenario: Historical and privileged risks remain bounded

- GIVEN a historical shared `/tmp` file or a privileged IBM i administrator changes a remote file
- WHEN Nexus recovers temporaries
- THEN it never auto-discovers the historical file and reports no absolute replacement protection

### Requirement: Deterministic Page Boundaries and Validation

The system MUST cap a response at 200 lines and 128 KiB and reject malformed or non-positive ranges before source work. It MUST return `range_start_out_of_bounds` for a start line after the final line; an empty member accepts only line 1. EOF MUST be deterministic and omit `nextStartLine`. Any error, including catalog `not_found`, `ambiguous`, `stale_coordinate`, or cursor failure, MUST return no partial content.

#### Scenario: Invalid range is rejected

- GIVEN a range is malformed, exceeds caps, or starts after the final line
- WHEN source is requested
- THEN it fails with `invalid_request`, `response_too_large`, or `range_start_out_of_bounds` and no lines

### Requirement: Prefix Compatibility and Approved Acceptance

The existing `catalogspike` behavior MUST remain prefix-only outside MCP. Automated tests MUST use controlled fakes or loopback facilities. One approved read-only IBM i acceptance MUST validate newline behavior, traversal from first page to EOF, and cleanup after success and cancellation without retaining source.

#### Scenario: Approved live acceptance succeeds

- GIVEN an approved target, identity, and validation window
- WHEN the documented traversal is performed
- THEN sanitized evidence confirms newline, EOF, and cleanup outcomes without stored source
