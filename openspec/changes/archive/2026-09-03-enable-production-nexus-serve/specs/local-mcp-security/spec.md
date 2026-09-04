# Delta for local-mcp-security

## MODIFIED Requirements

### Requirement: Sanitized Read-Only Surface and Audit
The system MUST expose only the defined catalog-context read operations and MUST NOT provide arbitrary SQL, shell, CL, SSH, SFTP, mutation, infrastructure-execution, generic path, list, or delete tools. Its MCP schemas MUST NOT accept temporary, delete, or listing paths. Serve admission MUST require V3, keyring-only noninteractive credentials, and current proof-bound eligibility for the profile target, policy, host pin, and approved Mapepire artifact identity; missing, stale, mismatched, legacy, prompt, or keyring-unavailable states MUST fail closed before remote work.

It MUST append durable, append-only, redacted audit records to a restrictive local owner-only store and require an explicit retention policy; no retention default is permitted. The ownership DB MUST also be restrictive and owner-only. Records MAY contain only operation class, allowlisted client-policy identifier, result classification, requested/returned line counts, duration, and opaque lifecycle outcome. Audit setup/retention failure MUST block serving, and an audit-write failure MUST fail the affected operation. Records, logs, fixtures, and artifacts MUST exclude source/text, secrets, credential references, paths, hosts, users, commands, SQL, hashes, coordinates, cursors, raw errors, and remote-cleanup details; configuration diagnostics MUST fail closed, be sanitized/audited, not mutate external-client configuration, and claim no live validation. (Previously: audit was in-memory and eligibility/retention persistence was not required.)

#### Scenario: Audit records a successful page
- GIVEN an authorized source page succeeds
- WHEN its audit outcome is appended
- THEN it contains only approved classification and count metadata

#### Scenario: Audit records a denied request
- GIVEN an unauthorized request is rejected
- WHEN its audit outcome is appended
- THEN it records the denial classification without sensitive material

#### Scenario: Ineligible noninteractive serve is rejected
- GIVEN a V3/keyring proof is missing, stale, or mismatched, or keyring is unavailable
- WHEN serving is requested
- THEN it fails closed before remote contact

#### Scenario: Audit policy or write fails
- GIVEN retention is absent/invalid or an append fails
- WHEN startup or an operation requires audit
- THEN startup is blocked or that operation fails without sensitive output

#### Scenario: Remote path control is unavailable
- GIVEN an MCP caller requests catalog context
- WHEN it supplies a temporary, delete, listing, shell, SQL, or path capability
- THEN it is rejected as unsupported and no generic remote operation occurs

#### Scenario: Diagnostic fails safely
- GIVEN a configuration diagnostic fails, times out, or is cancelled
- WHEN its outcome is shown or audited
- THEN it is sanitized, fails closed, changes no external-client file, and claims no live validation
