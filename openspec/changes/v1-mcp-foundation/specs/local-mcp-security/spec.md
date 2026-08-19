# local-mcp-security Specification

## Purpose

Protect local MCP access to IBM i catalog context through fail-closed client, credential, trust, and audit policies.

## Requirements

### Requirement: Explicit Client Authorization

The system MUST authorize each local client against an explicit policy before catalog or source access. Source-page authorization MUST explicitly authorize complete source reconstruction, not merely partial reads. Unauthorized, unknown, or malformed identity MUST fail closed with `unauthorized` and MUST NOT contact the remote system.

#### Scenario: Authorized client proceeds

- GIVEN a client identity explicitly permits the requested read-only operation
- WHEN it invokes a catalog-context tool
- THEN authorization permits evaluation of the request

#### Scenario: Unauthorized client is rejected

- GIVEN an unknown client or a client without source-access permission
- WHEN it invokes a catalog-context tool
- THEN it receives `unauthorized` and no remote operation occurs

### Requirement: Native Secret Isolation

The system MUST obtain required credentials only through the approved native credential facility. Secrets MUST NOT appear in MCP requests or responses, command arguments, environment variables, logs, audit events, fixtures, or persisted SDD artifacts. Missing or unavailable credentials MUST fail closed with deterministic `credentials_unavailable` and no remote attempt.

#### Scenario: Credential is available

- GIVEN an authorized client and an available approved credential
- WHEN an operation needs remote authentication
- THEN authentication proceeds without exposing secret material

#### Scenario: Credential is unavailable

- GIVEN required credentials cannot be obtained
- WHEN the client invokes an operation
- THEN it receives `credentials_unavailable` and no source is returned

### Requirement: Pinned Host Trust Policy

The system MUST support explicit pinned TOFU enrollment that records non-secret provenance. A changed host key MUST be rejected deterministically until explicitly re-enrolled under authorized policy. The policy model MUST preserve a future `require-verified` mode that rejects unverified hosts rather than enrolling them.

#### Scenario: Explicit TOFU enrollment pins a host

- GIVEN TOFU enrollment is authorized and host provenance is supplied
- WHEN the host is first accepted
- THEN its key and provenance are pinned for later verification

#### Scenario: Pinned key changes

- GIVEN a host presents a key different from its pinned key
- WHEN a connection is attempted
- THEN it fails with deterministic `host_key_changed` and no catalog or source access

### Requirement: Sanitized Read-Only Surface and Audit

The system MUST expose only the defined catalog-context read operations and MUST NOT provide arbitrary SQL, shell, CL, SSH, SFTP, mutation, or infrastructure-execution tools. It MUST audit only operation class, allowlisted client-policy identifier, result classification, requested and returned line counts, duration, and opaque lifecycle outcome. Audit, logs, fixtures, and artifacts MUST exclude source or line text, hashes, cursors, coordinates, paths, hosts, users, commands, SQL, credential references, secrets, and remote-cleanup details.

#### Scenario: Audit records a successful page

- GIVEN an authorized source page succeeds
- WHEN its audit outcome is recorded
- THEN it contains only approved classification and count metadata, with no source or sensitive identifiers

#### Scenario: Audit records a denied request

- GIVEN an unauthorized request is rejected
- WHEN its audit outcome is recorded
- THEN the record identifies the denial classification without sensitive material
