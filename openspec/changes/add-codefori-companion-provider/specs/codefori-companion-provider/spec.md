# Code for IBM i Companion Provider Specification

## Purpose

Provide a minimal, optional Code for IBM i proof capability through a 100% TypeScript Companion without changing the profile-selected Native provider.

## Requirements

### Requirement: Native-Windows Companion lifecycle and authenticated loopback

For the first live test, Nexus and VS Code SHALL both run natively on Windows. The Companion SHALL run in the same VS Code extension host as Code for IBM i and SHALL use only the 3.0.12 public `exports`, `getConnection`, `subscribe`, and `runSQL` APIs.

The spike SHALL bind HTTP/JSON only to a compile-time known `127.0.0.1` port. A bind collision MUST fail unavailable; the Companion MUST NOT scan ports, bind wildcard/`localhost`, forward traffic, or fall back to another endpoint. Automated discovery is future evolution.

Each activation MUST generate a new protocol generation and bearer token. A bounded descriptor MUST contain only protocol version, generation, and token. The token MUST NOT enter settings, profiles, flags, environment overrides, MCP, IBM i requests, logs, audit, telemetry, diagnostics, fixtures, errors, or other artifacts.

Windows activation MUST fail closed unless descriptor creation provides approved evidence that another ordinary Windows user cannot read the token. The approved implementation boundary is two fixed descriptor-specific inbox Windows PowerShell publish/cleanup operations; it MUST NOT expose caller-controlled executables, commands, scripts, paths, arguments, or environment. Standard Node `chmod`, `%LOCALAPPDATA%` or `globalStorageUri` placement, local-volume checks, and non-reparse checks alone MUST NOT be treated as DACL evidence. No native helper, native addon, external ACL dependency, or general command runner is approved.

Microsoft documentation verifies that `Directory.CreateDirectory(path, DirectorySecurity)` applies the supplied security during creation and returns an existing directory unchanged. A new descriptor directory MUST therefore use that create-with-security operation, while an existing directory MUST be validated as current-SID-owned, protected, exact-owner-only, normal, and non-reparse before the readiness signal; it MUST NOT be repaired in place. Native-Windows producer, consumer, transient-file, and cross-user evidence MUST still pass before authenticated activation is accepted.

Descriptor location, local volume, loopback, token possession, and generation MUST NOT be represented as proof of a particular VS Code extension-host identity. v0.1 documentation MUST require one active native Windows Companion broker per user and describe multi-window, WSL, SSH, container, forwarding, and cross-host discovery as unsupported.

#### Scenario: Windows descriptor security is not approved

- GIVEN the Companion cannot create the token descriptor with the approved Windows access-control evidence
- WHEN activation is attempted
- THEN the broker does not publish an authenticated endpoint
- AND Companion operations remain deterministically unavailable

#### Scenario: Port or topology is unavailable

- GIVEN the known port is occupied or Nexus cannot reach the active native Windows broker
- WHEN a Companion operation is attempted
- THEN it returns a bounded unavailable state
- AND it does not scan, bridge, or fall back to Native

### Requirement: Public API use makes no disposal or row-label assumption

The adapter MUST treat public `subscribe(context, event, name, callback)` as returning `void`. It MUST NOT store a subscription return value, invoke a purported unsubscribe/dispose method, or document an unevidenced unsubscribe guarantee. Deactivation MUST still close Companion-owned listener and request resources and drop local references.

The adapter MUST call `runSQL` with exactly one canonical statement and `{rows: 1}`. A successful raw result MUST be an array with exactly one row object, exactly one own enumerable column, and one bounded string value. The raw column label MUST be ignored. The protocol SHALL normalize the value to `{"state":"ok","rows":[{"value":"..."}]}` and MUST NOT claim the raw key is `CURRENT_USER` without later live evidence.

#### Scenario: Raw label differs

- GIVEN `runSQL` returns exactly one row and one bounded string column under an arbitrary label
- WHEN the result is normalized
- THEN the successful result contains the stable `value` field
- AND the raw label is neither required nor exposed

### Requirement: Sanitized deterministic session status

`session.status` MUST accept only an empty object and return exactly one `state`: `connected`, `companion_unavailable`, `codefori_extension_unavailable`, or `connection_unavailable`.

Missing, malformed, stale, permission-invalid, unreachable, authentication-rejected, version-mismatched, or generation-mismatched broker state MUST map to `companion_unavailable`. Missing Code for IBM i maps to `codefori_extension_unavailable`; no active connection maps to `connection_unavailable`. Status MUST NOT block Nexus startup or reveal host, user, endpoint, token, library list, connection metadata, or raw errors.

#### Scenario: Companion is absent after startup

- GIVEN Nexus started in Companion mode without a reachable broker
- WHEN `session.status` is invoked
- THEN it returns exactly `{"state":"companion_unavailable"}` within the status deadline

### Requirement: Single proof-query recognizer and dual validation

`sql.query` MUST accept only one `sql` string representing `SELECT CURRENT_USER FROM SYSIBM.SYSDUMMY1` under this bounded normalization:

- input is at most 128 UTF-8 bytes and contains ASCII only;
- ASCII space, tab, CR, and LF are the only whitespace;
- leading and trailing whitespace are permitted;
- one or more permitted whitespace characters are required between `SELECT`, `CURRENT_USER`, `FROM`, and `SYSIBM.SYSDUMMY1`;
- token matching uses ASCII-only case folding;
- whitespace inside identifiers or around the dot is forbidden; and
- every other byte, token, comment, semicolon, array, parameter, `@`, CL, command, transaction, CTE, call, DDL, DML, or additional statement is forbidden.

Go MUST validate and canonicalize before IPC. TypeScript MUST independently validate and canonicalize before `runSQL`. The executed canonical value MUST be exactly `SELECT CURRENT_USER FROM SYSIBM.SYSDUMMY1`, which is 41 ASCII bytes (`6+1+12+1+4+1+6+1+9`). No parser or generic SQL grammar SHALL be introduced.

A validation failure MUST prevent execution and return `{"state":"invalid_query"}`. The complete non-success states are `unavailable`, `invalid_query`, `limit_exceeded`, `timeout`, `cancelled`, and `failed`, each with no row data or additional property.

#### Scenario: Case and whitespace normalize to the proof

- GIVEN an ASCII case variation with only the permitted token-separating whitespace
- WHEN both recognizers admit it
- THEN `runSQL` receives only the canonical 41-byte statement

#### Scenario: Any broader SQL is rejected

- GIVEN comments, semicolons, Unicode whitespace, extra tokens, multiple statements, arrays, parameters, commands, or other SQL
- WHEN `sql.query` is invoked
- THEN it returns exactly `{"state":"invalid_query"}` before execution

### Requirement: Bounds, timeout, cancellation, and correlation

The implementation MUST use fixed non-user-configurable bounds no greater than: 512 request bytes, 1024 response bytes, one result row, one result column, a 256-byte UTF-8 value, 16 immediately admitted broker operations, and 16 active `runSQL` promises. Capacity MUST be at least ten. Excess work MUST return `limit_exceeded` without a waiting or worker queue. Companion SQL wait MUST be 5 seconds. Go response-header timeout MUST be at least 6 seconds and MUST NOT preempt a valid five-second SQL wait; Go total query time MUST be at least 7 seconds. Status retains a one-second total deadline. Retries are forbidden.

At least ten fake-backed protocol requests MUST be admitted independently and complete in deliberately reordered sequences with correct request correlation. No process-wide/global mutex may serialize all admission, response handling, or `runSQL` by default. Cancellation/deadline before `runSQL` starts MUST prevent it from starting; cancellation/deadline after start MUST suppress late output. Because public `runSQL` has no evidenced cancellation handle, a timed-out or cancelled active promise MUST retain capacity until it settles; remote cancellation MUST NOT be claimed.

Offline evidence MUST retain `not_validated_on_ibmi` and MUST NOT claim shared-job parallel safety.

#### Scenario: Ten responses complete out of order

- GIVEN ten fake-backed calls cross an execution barrier
- WHEN their responses complete in reverse order
- THEN every caller receives only its correlated value
- AND no global serialization prevented all ten from entering

#### Scenario: Five-second SQL remains possible

- GIVEN an admitted fake SQL operation completes within five seconds
- WHEN the Go client waits for HTTP response headers
- THEN a shorter response-header timeout does not terminate it

### Requirement: Package and release namespace

The Companion MUST be independently packaged and remain 100% TypeScript. It MUST pin `@halcyontech/vscode-ibmi-types` to 3.0.12. Offline CI MUST preserve Go verification and add Node install, lint, typecheck, test, build, and VSIX inspection without VS Code or IBM i.

The proposed workflow path MUST be `.github/workflows/release-companion.yml`. Its release namespace MUST be `companion-v*`, with the first intended tag exactly `companion-v0.1.0`. Publication MUST use current `vsce` OIDC with workflow/default `contents: read`, publish-job-only `id-token: write`, and no long-lived Marketplace PAT. The publish job MUST be inert by default and reachable only for a valid `companion-v*` tag when repository variable `COMPANION_PUBLISH_ENABLED` is exactly `true` and the protected `companion-marketplace` environment approves it. The enable variable MUST remain disabled until extension identity, publisher ownership, OIDC registration, and Windows token security are approved.

#### Scenario: Ordinary CI verifies without publishing

- GIVEN a non-release CI event or an unset/false publication variable
- WHEN Companion verification runs
- THEN offline package checks run
- AND Marketplace publication does not occur

#### Scenario: Protected OIDC route is explicitly enabled

- GIVEN a valid `companion-v*` tag, approved extension/publisher registration, repository variable `COMPANION_PUBLISH_ENABLED` equal to `true`, and protected-environment approval
- WHEN the publish job starts
- THEN only that job receives `id-token: write`
- AND current `vsce` OIDC is used without a Marketplace PAT
