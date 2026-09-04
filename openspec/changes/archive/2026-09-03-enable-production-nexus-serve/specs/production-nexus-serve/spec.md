# production-nexus-serve Specification

## Purpose

Run one approved Nexus profile as a bounded, read-only stdio MCP service.

## Requirements

### Requirement: Fail-Closed Serve Admission and Composition
`nexus serve -profile <name>` MUST load exactly one V3 keyring profile and validate its proof-bound eligibility before any remote contact. Eligibility SHALL bind the approved target, policy, host pin, and Mapepire artifact identity. It MUST open only a restrictive owner-only ownership DB and reject legacy, prompt, missing, stale, mismatched, unavailable-keyring, unsafe/unavailable ownership-ledger, and unavailable/invalid audit-retention states with sanitized stderr diagnostics. It MUST compose the Resolver, Acquirer, Recovery, Leases, persistent Auditor, and MCP server; recovery MUST succeed before the server starts.

#### Scenario: Valid startup
- GIVEN an approved V3/keyring profile, eligibility, ledger, retention, and dependencies
- WHEN `serve` starts
- THEN recovery completes before stdio MCP serves both tools

#### Scenario: Admission rejection has no remote contact
- GIVEN a legacy, prompt, missing, stale, mismatched, or keyring-unavailable profile
- WHEN `serve` starts
- THEN it fails closed before SSH, SFTP, or Mapepire contact

#### Scenario: Ledger, audit, or recovery fails
- GIVEN ownership validation, audit setup/retention, or recovery fails
- WHEN `serve` starts
- THEN the server does not start and reports only a sanitized classification

### Requirement: Bounded Operational MCP Lifecycle
The service MUST expose only `resolve_catalog_candidates` and `read_selected_source`. Catalog resolution SHALL use the fixed prepared Catalogados operation with at most 50 candidates and context cancellation/error mapping. Source acquisition MUST use request-scoped SSH/SFTP, fixed `CPYTOSTMF`, an owned temporary path, recovery, and exact owned cleanup; it MUST acquire at most 4 MiB, page at most 200 lines and 128 KiB marshaled, and bind cursors to selection, profile, and the 10-minute process lease. Stale, expired, invalid, cancelled, timed-out, and remote failures MUST return deterministic sanitized results without partial source.

### Requirement: Fixed Mapepire SSH Stdio Launch
When the approved SSH stdio fallback is selected, successful `EnsureServerJAR` activation from exactly `https://github.com/Mapepire-IBMi/mapepire-server/releases/download/v2.3.6/mapepire-server.jar` and remote SHA-256 verification MUST issue the only launch input: an immutable receipt bound to the exact authenticated remote-files capability, authenticated host identity, safe absolute Mapepire 2.3.6 path, fixed SHA-256, and fixed launch-policy revision. Before `NewSession`, the receipt-owned admission boundary MUST rehash the receipt path through that same capability; zero, mutated, capability, host, path, policy, SHA, or rehash-mismatched receipts MUST fail closed before its fixed-start seam is invoked. It SHALL then execute one deterministic shell-safe SSH exec command containing exactly `QIBM_JAVA_STDIO_CONVERT=N`, `QIBM_PASE_DESCRIPTOR_STDIO=B`, `QIBM_USE_DESCRIPTOR_STDIO=Y`, `QIBM_MULTI_THREADED=Y`, fixed Java `/QOpenSys/QIBM/ProdData/JavaVM/jdk80/64bit/bin/java`, `-jar`, the receipt path, and `--single`. No raw RemoteJAR/SHA/Java/environment/argv launch API or caller fragments remain. Receipt plus immediate rehash narrows, but does not eliminate, same-account post-hash/pre-exec TOCTOU; the public digest is integrity evidence, not signed end-to-end provenance.

#### Scenario: Receipt is rehashed before session creation
- GIVEN an issued receipt and its authenticated remote capability
- WHEN the artifact, capability, host identity, policy, path, or SHA no longer matches
- THEN launch fails before `NewSession` and no process starts

#### Scenario: Catalog request is bounded
- GIVEN an admitted server and allowed catalog request
- WHEN the prepared Catalogados operation returns, times out, is cancelled, or errors
- THEN it returns at most 50 candidates or the mapped deterministic outcome

#### Scenario: Exact source selection is acquired and paged
- GIVEN an exact resolved selection and a valid request
- WHEN source is acquired through fixed SSH/SFTP operations
- THEN owned cleanup occurs and pages report EOF/next start without exposing a generic path

#### Scenario: Cursor or acquisition is invalid
- GIVEN a stale coordinate, expired/invalid cursor, cancellation, or acquisition failure
- WHEN a source page is requested
- THEN no partial page is returned and owned recovery remains available

### Requirement: Protocol and Shutdown Determinism
Stdout MUST contain MCP JSON-RPC only; lifecycle and error diagnostics MUST be sanitized stderr output. On cancellation or transport completion, the service MUST stop accepting work, close the MCP server, request-scoped connections, leases, ownership ledger, and audit sink in deterministic order, and perform only ledger-owned cleanup.

#### Scenario: Protocol output is uncontaminated
- GIVEN startup, lifecycle, or operational diagnostics occur
- WHEN stdio is observed
- THEN stdout contains only protocol frames and diagnostics appear only on stderr

#### Scenario: Graceful shutdown
- GIVEN a running server and cancellation or client disconnect
- WHEN shutdown completes
- THEN resources close deterministically and only owned paths are cleaned

### Requirement: Controlled Validation Evidence
Normal automated tests MUST use fake profile, credential, catalog, SSH/SFTP, Mapepire, ledger, audit, and MCP/composition seams and MUST NOT contact IBM i. Live IBM i/client validation SHALL require an explicit controlled gate and approved prerequisites; until it passes, release status MUST remain `not_validated_on_ibmi`.

#### Scenario: Automated composition test
- GIVEN normal CI
- WHEN startup, rejection, tool, cancellation, and shutdown tests run
- THEN fakes prove the contracts without IBM i contact

#### Scenario: Live gate is absent
- GIVEN controlled live prerequisites are absent
- WHEN release evidence is assembled
- THEN it states `not_validated_on_ibmi` and no live attempt occurs
