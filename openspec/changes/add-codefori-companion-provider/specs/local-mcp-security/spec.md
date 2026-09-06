# Delta for local-mcp-security

## ADDED Requirements

### Requirement: Companion local capability fails closed on Windows

Companion mode MUST authenticate its known-port `127.0.0.1` HTTP/JSON endpoint with an ephemeral per-activation bearer token and generation from a bounded local descriptor. The token is a local IPC capability, not an IBM i credential. It MUST NOT appear in profiles, settings, flags, environment overrides, MCP, IBM i traffic, logs, audit, telemetry, diagnostics, fixtures, errors, or persisted artifacts other than the active descriptor.

For the native-Windows first-live topology, activation MUST fail closed unless the descriptor is created with an approved access-control mechanism that prevents read access by another ordinary Windows user. Exactly two fixed descriptor-specific inbox Windows PowerShell publish/cleanup operations are approved for implementation; they MUST expose no caller-controlled executable, command, script, path, arguments, or environment. A new directory MUST use documented create-with-`DirectorySecurity` semantics; because an existing directory is returned unchanged, it MUST be validated before readiness and MUST NOT be repaired in place. Node `chmod`, folder placement, inherited ACL assumptions, local-volume checks, non-reparse checks, and Go-only post-write verification MUST NOT individually be treated as proof of secure creation. No native helper, addon, external ACL dependency, or general command runner is approved.

Same-principal malicious processes and Administrator/SYSTEM remain residual risks. Descriptor path, local volume, loopback reachability, generation, DACL, and token possession MUST NOT be described as proof that Nexus shares a specific VS Code extension host. The implementation MUST NOT scan ports, forward, bridge, or fall back to the Native provider.

#### Scenario: Secure Windows publication is unavailable

- GIVEN no approved TypeScript-compatible Windows ACL mechanism or risk exception exists
- WHEN Companion activation attempts to publish a token
- THEN activation fails closed before exposing the authenticated broker
- AND no Native fallback occurs

#### Scenario: A token is protected from product surfaces

- GIVEN an approved activation has a token descriptor
- WHEN MCP, logs, audit, diagnostics, telemetry, fixtures, settings, and errors are inspected
- THEN neither token nor endpoint appears in those surfaces

### Requirement: Narrow normalized Companion SQL exception

The only SQL exception to the generic-SQL prohibition is `sql.query` in Companion mode. It MUST recognize only `SELECT CURRENT_USER FROM SYSIBM.SYSDUMMY1` with ASCII-only case folding, optional leading/trailing ASCII space/tab/CR/LF, and one-or-more such characters between the four required tokens. Whitespace inside identifiers or around the dot and every other SQL form MUST be rejected.

Nexus MUST recognize and canonicalize before IPC. The Companion MUST independently recognize and canonicalize before invoking `runSQL`. The only executed statement is the canonical 41-byte ASCII literal `SELECT CURRENT_USER FROM SYSIBM.SYSDUMMY1`. This finite recognizer MUST NOT grow into a SQL parser or generic grammar.

The operation MUST enforce fixed request, response, row, column, cell, immediate-admission, timeout, and in-flight limits and MUST return no partial/truncated result. Capacity MUST allow at least ten independently admitted fake operations; excess work MUST fail with `limit_exceeded` without a waiting or worker queue, and no process-wide SQL mutex may be introduced. A timed-out or cancelled started `runSQL` promise MUST retain capacity until it settles. SQL input and returned data MUST be absent from logs, audit, telemetry, diagnostics, fixtures, artifacts, and errors. Bounded row data MAY appear only in the successful MCP result.

A successful raw result MUST contain exactly one row object, one own enumerable column of any label, and one bounded string value. The label MUST be discarded and the value normalized to `rows:[{"value":"..."}]`; security behavior MUST NOT depend on an unevidenced raw `CURRENT_USER` property.

#### Scenario: Permitted normalization executes one canonical statement

- GIVEN an admitted ASCII case/whitespace variation
- WHEN both independent recognizers accept it
- THEN Code for IBM i receives only the canonical statement
- AND only a bounded normalized value may appear in the successful MCP response

#### Scenario: Generic SQL remains prohibited

- GIVEN Native mode or any Companion input outside the finite recognizer
- WHEN SQL behavior is attempted
- THEN no `runSQL`, generic SQL, shell, CL, SSH, SFTP, mutation, or infrastructure-execution capability is reached

## MODIFIED Requirements

### Requirement: Sanitized Read-Only Surface and Audit

The Native provider MUST continue to expose only `resolve_catalog_candidates` and `read_selected_source` and MUST preserve all current profile, V3/keyring credential, eligibility, host-pin, Mapepire identity, ownership, recovery, admission, path, bounded-source, and append-only audit requirements. Companion mode MUST be separately composed and limited to `session.status` and the fixed proof `sql.query`; the modes MUST NOT be combined and MUST NOT fall back to one another.

Native audit setup/retention and per-operation write requirements remain unchanged. Companion observability, if any, MUST contain only fixed classifications and MUST exclude tokens, endpoints, SQL, row values, raw labels, hosts, users, IBM i identifiers, connection details, request IDs, and raw errors. This delta MUST NOT require a change to Native audit implementation.

#### Scenario: Native admission remains unchanged

- GIVEN a profile-selected Native invocation
- WHEN serve admission and operations run
- THEN all existing credential, eligibility, transport, ownership, recovery, and audit controls remain mandatory

#### Scenario: Companion observability is redacted

- GIVEN a Companion operation succeeds, fails, times out, or is cancelled
- WHEN observability is produced
- THEN it contains only an allowed fixed classification and no sensitive input, result, token, endpoint, identity, or raw detail
