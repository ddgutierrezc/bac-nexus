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

Before reservation/copy, the system MUST commit/readback a SQLite row with a stable 128-bit token. The local-only database MUST contain only version/token/validated path/selector/binding digest/creation time—never source, credentials, commands, cursors, model content, or host/user plaintext. Paths MUST remain under validated home in private `0700`; files MUST be random exclusive `0600`.

Admission MUST atomically count/insert in a bounded single-writer transaction. Exact token/row retries SHALL be idempotent; rows MUST be unique by token/path, limited to 64, and listed `LIMIT 65`. No remote work precedes commit/readback. Busy handling MUST be bounded/context-aware; ambiguous commit MUST readback, while mismatch, corruption, or policy/dependency denial MUST fail closed.

Every open MUST verify `application_id=1111573326` (`BACN`), `user_version=1`, exact schema, journal/durability pragmas, path/permissions, and bounded integrity. An existing ledger MUST complete ordered integrity verification before metadata/schema acceptance; a new ledger MUST initialize and then complete it. Every open MUST run `quick_check(1)` under one one-second context that reaches every verification query. When exact quick-check success and overflow-safe `page_count × page_size` establish ≤4 MiB, the system MUST run `integrity_check(1)`. Eligible successful verification MUST terminate as integrity-check passed; quick-check passed MUST be an intermediate ordered observation, not a terminal result. Verification MUST execute as focused independently testable package-internal behavior. Package-internal tests MUST observe only non-sensitive stage/outcome classifications: not-run, passed, corrupt, inconclusive, or bound-exceeded; never diagnostics/source/database content. A package-internal verifier result supplied to `Open` with `passed` MUST allow opening to continue; `not-run`, `corrupt`, `inconclusive`, and `bound-exceeded` MUST return public `source.ErrOwnershipInvalid`. Failed, incomplete, cancelled, inconclusive, corrupt, or bound-exceeded verification MUST return public `source.ErrOwnershipInvalid`. Mismatch, corruption, or policy denial MUST fail closed without rebuild. Recovery MUST revalidate profile, credential, binding, pin, directory, token, and path; it MUST confirm `Remove` plus `Stat`-not-found before row deletion. Remote uncertainty retains the row; tokens/paths MUST NOT be reused. Historical `/tmp` paths MUST NOT be auto-discovered or deleted. Dependency denial MUST prevent acquisition; tests MUST NOT claim power-loss durability.

#### Scenario: Caller cannot direct temporary handling

- GIVEN an MCP caller starts traversal
- WHEN it sends its typed request
- THEN Nexus derives the path and accepts no temporary, list, or delete path

#### Scenario: Atomic admission is confirmed

- GIVEN a new token or exact active row below 64
- WHEN Nexus admits it
- THEN it commits and readbacks before remote reserve or copy

#### Scenario: Ledger admission fails closed

- GIVEN row 65, exhausted contention, corruption, policy/dependency denial, or verification mismatch
- WHEN admission or recovery runs
- THEN no remote work, snapshot, cursor, or content is published

#### Scenario: Quick verification runs every open

- GIVEN a new/existing ledger
- WHEN it opens
- THEN `quick_check(1)` runs before full-check eligibility

#### Scenario: Verification ordering respects ledger state

- GIVEN an existing ledger or a new ledger that requires initialization
- WHEN the ledger opens
- THEN existing-ledger verification precedes metadata/schema acceptance, and new-ledger verification follows initialization

#### Scenario: Eligible verification ends with integrity-check passed

- GIVEN exact quick-check success, metadata establishes ≤4 MiB, and full verification succeeds
- WHEN the ledger opens
- THEN `quick_check(1)` is an intermediate ordered observation and `integrity_check(1)` terminates as passed within one second

#### Scenario: Overflow or an oversized ledger is refused

- GIVEN metadata overflow or a ledger larger than 4 MiB
- WHEN quick-check succeeds
- THEN `integrity_check(1)` is refused and returns `source.ErrOwnershipInvalid`

#### Scenario: Verification respects deadline/cancellation

- GIVEN verification exceeds one second or is cancelled
- WHEN the ledger opens
- THEN it is incomplete and returns `source.ErrOwnershipInvalid`

#### Scenario: Corruption fails closed

- GIVEN quick or full verification reports corruption
- WHEN the ledger opens
- THEN it returns `source.ErrOwnershipInvalid` with no remote work

#### Scenario: Inconclusive verification fails closed

- GIVEN malformed, multiple, absent, or inconclusive verification output
- WHEN the ledger opens
- THEN it returns `source.ErrOwnershipInvalid` with no remote work

#### Scenario: Passed internal verifier result continues opening

- GIVEN a package-internal verifier result of `passed` is supplied to `Open`
- WHEN `Open` maps the result
- THEN opening continues and exposes no diagnostics or content

#### Scenario: Non-success internal verifier results fail closed

- GIVEN a package-internal verifier result of `not-run`, `corrupt`, `inconclusive`, or `bound-exceeded` is supplied to `Open`
- WHEN `Open` maps the result
- THEN it returns `source.ErrOwnershipInvalid` and exposes no diagnostics or content

#### Scenario: Recovery validates ownership and target

- GIVEN a ledger record names an exact private path
- WHEN startup or pre-acquisition recovery runs
- THEN it revalidates ownership, binding, credential, and pin before removal

#### Scenario: Crash recovery is idempotent

- GIVEN a crash before/during copy or after remote removal
- WHEN recovery confirms the exact path absent
- THEN it deletes only that row; otherwise it retains it

#### Scenario: Historical/privileged risks remain bounded

- GIVEN historical shared `/tmp` or privileged remote-file replacement
- WHEN Nexus recovers temporaries
- THEN it never auto-discovers history or claims absolute replacement protection

### Requirement: Deterministic Page Boundaries and Validation

The system MUST cap a response at 200 lines and 128 KiB and reject malformed or non-positive ranges before source work. It MUST return `range_start_out_of_bounds` for a start line after the final line; an empty member accepts only line 1. EOF MUST be deterministic and omit `nextStartLine`. Any error, including catalog `not_found`, `ambiguous`, `stale_coordinate`, or cursor failure, MUST return no partial content.

#### Scenario: Invalid range is rejected

- GIVEN a range is malformed, exceeds caps, or starts after the final line
- WHEN source is requested
- THEN it fails with `invalid_request`, `response_too_large`, or `range_start_out_of_bounds` and no lines

### Requirement: Prefix Compatibility, SDD Completion, and Deferred IBM i Field Validation

The existing `catalogspike` behavior MUST remain prefix-only outside MCP. Automated SDD acceptance MUST use controlled fakes or loopback facilities and MUST prove internal contracts only; it MUST NOT claim IBM i validation or treat automated evidence as a substitute for live IBM i evidence. On SDD completion, the release status MUST be `ready_for_controlled_ibmi_validation` and MUST explicitly state `not_validated_on_ibmi` until approved external field evidence exists.

An authorized operator MUST later perform the deferred, read-only IBM i field-validation rollout gate. That gate MUST validate traversal from line 1 to EOF, newline behavior, cleanup after successful completion and cancellation, and absence of retained source. SDD completion MUST NOT require this external gate and MUST NOT assert that a live IBM i environment exists.

#### Scenario: Automated SDD acceptance completes without a live claim

- GIVEN implementation and automated internal-contract verification have passed
- WHEN SDD completion is recorded
- THEN the release status is `ready_for_controlled_ibmi_validation` and `not_validated_on_ibmi`
- AND no result claims IBM i validation or live-evidence equivalence

#### Scenario: Deferred live field validation succeeds

- GIVEN an approved target, identity, and validation window
- WHEN the operator performs the documented traversal from line 1 to EOF and tests success and cancellation cleanup
- THEN sanitized external evidence confirms newline, EOF, cleanup, and no retained source
