## Exploration: v1-mcp-foundation

### Current State
The validated `catalogspike` is a local, manual vertical slice. It loads an encrypted vault, verifies a pinned SSH host key, uses a separately supplied and checksum-verified Mapepire JAR to issue one fixed prepared Catalogados query, requires exact candidate selection, copies one IBM i source member with fixed `CPYTOSTMF ... STMFCCSID(1208)`, reads an initial bounded SFTP prefix, removes the remote temporary file, and only displays source through explicit opt-in.

`internal/catalog` bounds a query at 50 candidates plus a sentinel and `Select` requires the five source coordinates to match a returned candidate exactly. `internal/mapepire.Session.Catalog` serializes the fixed query and closes its channel on cancellation. `internal/source.Retriever` validates `1..4 MiB` and `1..50,000` limits before remote work, creates a random `/tmp/bac-nexus-catalog-*.utf8` file, validates UTF-8, avoids splitting a rune for a prefix, counts `\n`, and joins a primary failure with failed deletion. `remote.Client.CopyToUTF8` cancels the fixed command by closing its SSH session; `Dial` closes the client when its context ends. The CLI currently performs one catalog query and immediately calls `Retrieve`; it does not re-query before reading and has no page, revision, cache, or MCP response model.

This is safe enough for a prefix-oriented spike, but it is not a source-pagination implementation. Each current read first copies the complete member to IBM i `/tmp`, then downloads only a prefix. Repeating it for page 2, 3, and later would repeatedly copy the full member, scan from byte zero, and can mix member revisions. On cancellation, the context-owned client may be closed before the deferred SFTP delete, so the current code reports cleanup failure but cannot prove removal of a remote temporary file. No current test covers page identity, line indexing, replay, out-of-order pages, snapshot expiry, or a source change between pages.

The prior v1 exploration remains valid: v1 should use a narrow local application service with typed stdio MCP adapters, retain fixed IBM i operations, revalidate an exact catalog coordinate rather than trust caller-invented paths, use explicit client authorization and native secrets, keep source out of logs/audit/artifacts, and deliver independent sub-400-line slices. The earlier `max-bytes`/`max-lines` controls are a prefix safety mechanism, not a suitable external pagination contract.

### Affected Areas
- `internal/source/retrieve.go` — current byte-prefix reader, UTF-8 validation, random remote temporary file, and cleanup behavior must become a one-time snapshot acquisition seam plus line-page reader; its 4 MiB cap cannot remain both a page cap and the complete-member policy.
- `internal/source/retrieve_test.go` — has useful bounds, UTF-8, and joined-cleanup fakes, but needs deterministic snapshot, line-index, page, expiry, concurrency, cancellation, and cleanup-recovery cases.
- `internal/catalog/catalog.go` and `internal/mapepire/session.go` — exact coordinate matching and the fixed 51-row query must be reused for every page’s mandatory fresh catalog-coordinate re-query.
- `cmd/catalogspike/main.go` — demonstrates the existing one-query/one-prefix flow only; v1 must extract it rather than expose flags or reuse its output model as an MCP contract.
- `internal/remote/ssh.go` — retains ownership of pinned SSH/SFTP and the fixed CPYTOSTMF command. It needs a cleanup-safe lifecycle boundary; it must not become arbitrary command or path execution.
- `internal/remote/ssh_test.go` and `internal/mapepire/session_test.go` — already prove fail-closed host-key and Mapepire cancellation behavior; new tests must prove that page cancellation does not silently abandon an owned remote temporary file.
- `internal/app/`, `internal/mcp/`, `internal/security/`, and `internal/audit/` — proposed small v1 seams for typed pagination, authorization, opaque snapshots, deterministic errors, and sanitized outcomes.
- `openspec/changes/v1-mcp-foundation/proposal.md` and both delta specs — currently say “bounded range” but do not define pagination; they must be corrected in later phases, not by this exploration.

### Approaches
1. **Stateless line ranges with a fresh remote copy for every page** — accept `startLine` and `maxLines`, fresh-re-query the catalog, run CPYTOSTMF, stream/skip to the requested line, and delete the temporary file for every request.
   - Pros: No local source cache or cursor state; restart is simple; the existing retriever is the closest starting point.
   - Cons: Every page copies the full member, later pages are increasingly expensive, and a source change can make one traversal internally inconsistent. A visible hash requires reading the whole transferred member and becomes an unnecessary correlating identifier. It also magnifies remote temporary-file cleanup risk.
   - Effort: Medium

2. **Immutable in-memory snapshot lease with line pages (recommended)** — on the first page, authorize, fresh-re-query the exact coordinate, make one fixed UTF-8 remote copy, enforce a distinct complete-member ceiling, download the staged file once into a process-local immutable memory snapshot while building a line-offset index and SHA-256 integrity check, delete the remote file, and return an opaque random cursor. Later pages authorize and fresh-re-query again, then read requested complete lines from that immutable snapshot.
   - Pros: One remote copy/download per traversal; pages are consistent even when the IBM i member changes; no source bytes are written to disk; arbitrary ordered, replayed, or out-of-order line pages are cheap; process restart safely invalidates leases rather than pretending to reconstruct an old revision.
   - Cons: Requires bounded process memory, TTL/quota eviction, an opaque server-side cursor store, and an explicit policy for source-size/cache limits. The snapshot may reflect a member that changed after its first page, by design.
   - Effort: High

3. **Disk-backed encrypted snapshot lease** — use the same immutable snapshot model but store source and its line index in a protected local cache, optionally encrypted with an approved OS facility, so larger members survive memory pressure or a server restart.
   - Pros: Supports a larger ceiling and potentially resumable pages; avoids repeated IBM i transfer.
   - Cons: Creates a new sensitive-data persistence system, key-management and deletion obligations, crash recovery complexity, backup/indexing exposure, and a much larger audit/security review. Restart-resumability cannot prove a current source revision without a separate IBM i revision primitive.
   - Effort: High

4. **Generic remote range or hashing commands** — add CL, shell, SQL, or SFTP offset primitives to fetch ranges or calculate revisions remotely.
   - Pros: Could reduce transferred bytes for late pages.
   - Cons: Violates the narrow-tool and fixed-remote-action model, is not validated by the spike, complicates IBM i record/CCSID semantics, and creates an infrastructure-execution surface. It does not by itself solve stable multi-page consistency.
   - Effort: High

### Recommendation
Choose Approach 2 and correct v1 around an immutable, process-local **line-based snapshot lease**. Line ranges are the right public semantic for record-oriented IBM i source: humans and agents navigate source by logical records, not UTF-8 byte offsets. Byte offsets are encoding-dependent, permit split runes and partial records, and become invalid after newline representation changes. Byte offsets may be an internal line-index implementation detail only.

#### Complete proposed contract

`read_selected_source` MUST require an authorized client and this request shape:

```json
{
  "selection": {
    "query": { "item": "PISA061", "productionLibrary": "OPTIONAL" },
    "coordinate": {
      "item": "PISA061",
      "sourceLibrary": "SRCLIB",
      "sourceFileBase": "Q",
      "objectType": "RPGLE",
      "sourceType": "RPGLE"
    }
  },
  "range": { "startLine": 1, "maxLines": 1 },
  "snapshotCursor": "optional opaque cursor from an earlier page"
}
```

`startLine` is a positive, one-based, inclusive logical record number; `maxLines` is positive and no greater than a configured page-line cap. The first request omits `snapshotCursor`; later requests repeat the same canonical selection and provide it. The service MUST authorize and freshly execute the fixed bounded catalog query on **every** request, then require one exact, unique current coordinate match before either creating or using a snapshot. The cursor is bound server-side to the canonical selection, authorized client policy identity, and process instance; it is a random opaque capability with no host, library, path, revision, source, timestamp, or secret encoded in it.

The successful response MUST contain only:

```json
{
  "coordinate": { "item": "PISA061", "sourceLibrary": "SRCLIB", "sourceFileBase": "Q", "objectType": "RPGLE", "sourceType": "RPGLE" },
  "snapshotCursor": "opaque random value",
  "page": { "startLine": 1, "lineCount": 1, "lines": ["source record without its LF separator"], "eof": false, "nextStartLine": 2 }
}
```

`lines` preserves each complete UTF-8 record’s bytes other than the LF delimiter, including significant trailing spaces. A page never splits a Unicode code point or a source record. The reader treats LF as the logical separator; a preceding CR is source data unless IBM i manual validation proves and the contract later explicitly adopts a different CPYTOSTMF newline representation. An empty member accepts only `startLine: 1` and returns `lineCount: 0`, `lines: []`, and `eof: true` with no `nextStartLine`. A final non-LF-terminated record is one line. `startLine` beyond the valid empty-member case or past the final record returns `range_start_out_of_bounds`; callers use `nextStartLine`, not trial-and-error. `eof: true` omits `nextStartLine`.

Before remote work, reject malformed fields, non-positive values, `maxLines` above the page cap, and a client not permitted to reconstruct source. While acquiring a first snapshot, reject `source_too_large` when the remote staged regular file exceeds a separately configured absolute **complete-member** ceiling; reject `invalid_source_encoding`, `line_too_large`, and `response_too_large` without returning partial content. The per-page line and encoded-response-byte caps bound MCP output; the complete-member ceiling, maximum simultaneous snapshots, aggregate snapshot-memory quota, and short idle TTL bound server resources. These are separate controls: a small page limit must not prevent complete authorized traversal, and a page limit must not be misrepresented as a whole-source ceiling.

Deterministic public error classifications should be `unauthorized`, `invalid_request`, `not_found`, `ambiguous`, `stale_coordinate`, `snapshot_invalid`, `snapshot_expired`, `range_start_out_of_bounds`, `source_too_large`, `line_too_large`, `response_too_large`, `invalid_source_encoding`, `cancelled`, `deadline_exceeded`, `credentials_unavailable`, `host_key_changed`, and sanitized `connector_unavailable`. They must not return source fragments, raw server text, SQL, paths, host names, credential references, cursor bindings, hashes, or cleanup internals. `snapshot_invalid` covers a cursor whose bound selection/client does not match; `snapshot_expired` covers TTL/quota eviction or restart. A restart intentionally invalidates all cursors, so a client restarts at page one rather than mixing snapshots.

The acquisition sequence is: authorize; fresh catalog re-query; exact selection; create a random Nexus-owned remote temporary name; fixed CPYTOSTMF; stat a regular staged file and enforce the absolute ceiling; stream it once into bounded memory while validating UTF-8 and building offsets; atomically publish the immutable snapshot only after full validation; remove the remote temporary file; then emit the first page. The internal SHA-256 is for implementation integrity/debug correlation only and is neither exposed nor logged. If acquisition, cancellation, or cleanup fails, publish no cursor and return no content. A cleanup design must outlive request cancellation: track only random Nexus-owned temporary names, attempt deletion with a bounded cleanup context/connection that is not prematurely closed by the request context, and return a sanitized failure if removal cannot be confirmed. Crash cleanup requires an approved bounded recovery policy for the owned prefix; the current spike does not provide it and cannot claim guaranteed cleanup after process death.

Snapshot files are rejected for v1. Source remains in bounded process memory only, is scoped to the client policy identity, is evicted on TTL/quota/process exit, and is never written to logs, audit, fixtures, artifacts, or a durable cache. Readers take an immutable snapshot reference: duplicate/replayed and out-of-order pages return the same page, concurrent readers cannot mutate it, and eviction waits for active references. Cancellation and timeout stop catalog/remote work, close its channel/session, discard any incomplete buffer/index, and perform owned-temp cleanup; a local cached-page cancellation returns no partial response. A later page still re-authorizes and revalidates the current catalog coordinate; if coordinates changed it returns `stale_coordinate`, even though a matching snapshot exists. If only source bytes changed, the lease deliberately continues the captured snapshot and never claims it is the current member.

Audit only operation class, allowlisted client-policy identifier, result classification, page line count, requested line count, elapsed duration, and an opaque snapshot lifecycle outcome. It MUST exclude source bytes, line text, hashes/revisions, cursors, raw coordinates unless classification policy explicitly permits them, profile/host/user data, paths, commands, SQL, secrets, and remote cleanup details. Authorization MUST explicitly recognize that pagination is equivalent to permitting full source reconstruction; it is not a safer “partial source” permission.

#### Migration, verification, and delivery slices

Do not reinterpret the spike CLI’s `-max-bytes`/`-max-lines` as page fields. Preserve its prefix behavior during extraction, deprecate it only in v1 MCP documentation, and introduce the new line-page contract behind `read_selected_source`. The service may reuse the fixed CPYTOSTMF/SFTP path but must not expose old `remoteSize`, `bytes`, `truncated`, or `cleanup` fields as MCP source-page output: those fields reveal a prefix implementation rather than a stable page contract.

Use deterministic consumer-owned fakes for catalog freshness, staged-file size/contents, exact deletion ordering, blocked reads, cancellation, timeout, and clock/TTL/quota. Add table and property-style cases over arbitrary valid UTF-8 records, trailing spaces, empty members, final unterminated records, multibyte runes on page boundaries, invalid UTF-8, oversized single records, all start-line boundaries, replay/out-of-order pages, cursor/client/selection mismatch, source change after snapshot creation, catalog-coordinate change after snapshot creation, concurrent page reads, restart/eviction, and joined cleanup failures. Retain loopback SSH/Mapepire cancellation tests; add no automated live IBM i harness. The approved manual validation must traverse a known large member from first page to EOF, compare only a locally approved count/check result without retaining source, prove changed-coordinate failure, observe no owned remote temporary file after success/cancellation, and confirm CPYTOSTMF newline behavior.

Keep implementation reviewable and rollbackable: (1) pure line-index/range model plus properties, under 400 lines; (2) bounded immutable in-memory store and cursor lifecycle, under 400 lines; (3) remote staged-snapshot acquisition with cancellation-safe cleanup and fakes, under 400 lines; (4) application freshness/authorization integration, under 400 lines; (5) MCP schemas, error mapping, audit redaction, and documentation, under 400 lines; then manual approval validation. Each slice is independently revertible; do not add durable cache/restart persistence in v1.

The rejected alternatives either produce O(number-of-pages × member-size) remote work and inconsistent traversals (Approach 1), create a durable sensitive-data subsystem before it is justified (Approach 3), or weaken the fixed, least-privilege remote boundary (Approach 4).

One product decision remains genuinely required before proposal/spec correction: approve the data-classification and operational values for complete-member ceiling, aggregate in-memory cache quota, lease TTL, page response cap, and recovery treatment for an unconfirmed remote temporary file. The architecture can provide safe defaults, but those values determine whether authorized full-source reconstruction and transient local memory exposure are acceptable for BAC’s approved IBM i source classification. The existing credential-execution, source-exposure, audit-retention, verified-host, and SDK-approval gates also remain unresolved from the prior exploration.

### Risks
- **Source reconstruction is intentional disclosure:** authorized pagination can reconstruct an entire member. Authorization, client identity, model/data-handling approval, and audit policy must be designed for full-source access, not a misleading per-page privilege.
- **Cleanup is not currently provable on cancellation or crash:** current context-owned connection closure can make the deferred remote delete fail. A bounded independent cleanup/recovery policy is a required implementation safeguard, not a cosmetic warning.
- **Snapshot resource and exposure risk:** memory snapshots eliminate disk persistence but need hard member/quota/TTL limits and still exist in process memory until eviction.
- **IBM i serialization details need live confirmation:** CPYTOSTMF UTF-8 conversion and newline behavior are assumed from the spike’s LF tests, not yet validated as a pagination contract on an approved IBM i member.
- **Fresh coordinates do not prove fresh bytes:** re-querying catalog coordinates prevents stale or invented targets, while snapshot semantics intentionally serve the initial member image if bytes change later; this must be documented rather than hidden.
- **Credential and transport gates remain:** unattended MCP secret access, official SDK approval, verified-host policy, and source/audit classification must fail closed until approved.

### Ready for Proposal
No, conditionally. The user’s arbitrary-range decision is now reconciled into a complete recommended contract, but proposal/spec correction should wait for the explicit source-classification/resource/remote-orphan-cleanup decision above. Tell the user that v1 can support bounded start-to-finish traversal without byte slicing or repeated full copies, provided it adopts process-local immutable snapshot leases and treats pagination as full-source authorization.
