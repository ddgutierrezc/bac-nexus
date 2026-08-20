# local-mcp-security Specification

## Purpose

Protect local MCP access to IBM i catalog context through fail-closed client, credential, trust, and audit policies.

## Requirements

### Requirement: Local-Principal Authorization

The current local OS principal MUST be the trust boundary on Windows, macOS, and Linux. The system MUST authorize each advisory selector against explicit policy before catalog or source access; `clientInfo` and profile names MUST NOT authenticate a product or principal. Source-page authorization MUST explicitly authorize complete source reconstruction. Unauthorized, unknown, or malformed selectors MUST fail closed with `unauthorized` and MUST NOT contact the remote system. Same-principal malicious processes and privileged OS users remain residual risks.

#### Scenario: Authorized selector proceeds

- GIVEN an advisory selector explicitly permits the requested read-only operation
- WHEN it invokes a catalog-context tool
- THEN authorization permits evaluation of the request

#### Scenario: Unauthorized selector is rejected

- GIVEN an unknown selector or a selector without source-access permission
- WHEN it invokes a catalog-context tool
- THEN it receives `unauthorized` and no remote operation occurs

### Requirement: Native Secret Isolation

The consumer-owned `CredentialStore` MUST expose only exact `Get`, `Set`, and `Delete`, using fixed service `BAC Nexus`, account `ibmi/<profile>`, profile `[A-Za-z0-9][A-Za-z0-9._-]{0,63}`, and secrets of 1–4096 bytes. Its approved native-keyring adapter SHALL map Windows Credential Manager, macOS Keychain, and Linux Secret Service. SQLite MUST NOT store secrets. Missing, locked, unavailable, policy-denied, malformed, or ambiguous native-store results MUST return `credentials_unavailable`; no prompt, vault, plaintext, SQLite, or remote fallback is permitted.

Secrets MUST NOT appear in argv, environment, logs, audit, MCP, SQLite, fixtures, errors, or persisted artifacts. macOS MUST invoke only the fixed keychain executable, use stdin—not argv or environment—for Set secrets, and MUST NOT use a generic shell. Windows native failures and Linux D-Bus failures MUST be deterministic. Corporate policy or dependency approval denial MUST fail closed before remote access.

#### Scenario: Credential is available

- GIVEN an authorized client and an available approved credential
- WHEN an operation needs remote authentication
- THEN authentication proceeds without exposing secret material

#### Scenario: Credential is unavailable

- GIVEN the required native store is missing, locked, unavailable, denied, malformed, or ambiguous
- WHEN the client invokes an operation
- THEN it receives `credentials_unavailable` and no remote attempt or fallback occurs

#### Scenario: Platform secret transport is constrained

- GIVEN a macOS Set operation or a Windows/Linux native-store failure
- WHEN the adapter handles the operation
- THEN the secret is absent from argv and environment, or the failure is deterministic

#### Scenario: Supported-platform evidence is bounded

- GIVEN a supported windows, darwin, or linux amd64/arm64 target
- WHEN platform acceptance is planned
- THEN it uses available runners only and does not invent unavailable-runner coverage

### Requirement: Explicit Native Credential Migration

The system MUST migrate an old vault only through an explicit operation. It SHALL read the old secret once, write it to the active native store, read back the exact secret, zeroize temporary buffers where possible, and delete the old vault only after confirmation. Native-store unavailability MUST retain the vault and fail closed. Automatic migration is prohibited.

#### Scenario: Migration succeeds only after confirmation

- GIVEN an operator explicitly requests migration and the active native store is available
- WHEN the stored secret is read back exactly after Set
- THEN Nexus zeroizes temporary buffers and deletes the old vault

#### Scenario: Migration cannot confirm native storage

- GIVEN an explicit migration cannot access or confirm the native store
- WHEN migration is attempted
- THEN it retains the old vault and returns `credentials_unavailable`

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

The system MUST expose only the defined catalog-context read operations and MUST NOT provide arbitrary SQL, shell, CL, SSH, SFTP, mutation, or infrastructure-execution tools. Its MCP schemas MUST NOT accept a temporary, delete, or listing path, and no generic remote listing or deletion capability SHALL exist. It MUST audit only operation class, allowlisted client-policy identifier, result classification, requested and returned line counts, duration, and opaque lifecycle outcome. Audit, logs, fixtures, and artifacts MUST exclude source or line text, hashes, cursors, coordinates, paths, hosts, users, commands, SQL, credential references, secrets, and remote-cleanup details.

#### Scenario: Audit records a successful page

- GIVEN an authorized source page succeeds
- WHEN its audit outcome is recorded
- THEN it contains only approved classification and count metadata, with no source or sensitive identifiers

#### Scenario: Audit records a denied request

- GIVEN an unauthorized request is rejected
- WHEN its audit outcome is recorded
- THEN the record identifies the denial classification without sensitive material

#### Scenario: Remote path control is unavailable

- GIVEN an MCP caller requests catalog context
- WHEN it attempts to specify a temporary, delete, or listing path
- THEN the request is rejected as unsupported and no generic remote operation occurs
