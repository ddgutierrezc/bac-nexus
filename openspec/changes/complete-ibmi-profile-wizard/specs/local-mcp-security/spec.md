# Delta for local-mcp-security

## MODIFIED Requirements

### Requirement: Native Secret Isolation

The consumer-owned `CredentialStore` MUST expose exact `Get`, `Set`, and `Delete`; its lower-layer status contract MUST report presence, absence, or unavailability without secret bytes. It uses fixed service `BAC Nexus`, account `ibmi/<profile>`, profile `[A-Za-z0-9][A-Za-z0-9._-]{0,63}`, and secrets of 1–4096 bytes. The approved native-keyring adapter SHALL map Windows Credential Manager, macOS Keychain, and Linux Secret Service. SQLite and V3 profiles MUST NOT store secrets. A wizard MUST explicitly select `prompt` (non-persistent transient entry) or `keyring`; keyring unavailability MUST return `credentials_unavailable` and MUST NOT fall back to prompt, vault, plaintext, SQLite, or remote access.

Secrets MUST NOT appear in argv, environment, logs, audit, MCP, SQLite, fixtures, errors, persisted artifacts, TUI commands, messages, model state, views, or previews. Transient-entry services MUST return opaque outcomes. macOS MUST invoke only the fixed keychain executable and use stdin—not argv or environment—for Set secrets; it MUST NOT use a generic shell. Windows native failures and Linux D-Bus failures MUST be deterministic. Corporate policy or dependency approval denial MUST fail closed before remote access.

(Previously: Native credentials allowed only the fixed native keyring path and prohibited prompt fallback.)

#### Scenario: Credential is available
- GIVEN an authorized client and an available approved credential
- WHEN remote authentication needs it
- THEN authentication proceeds without exposing secret material

#### Scenario: Prompt mode remains transient
- GIVEN the operator selected `prompt`
- WHEN a secret is entered for an authorized operation
- THEN the TUI receives only an opaque outcome and no secret is persisted

#### Scenario: Credential is unavailable
- GIVEN required keyring storage is missing, locked, denied, malformed, or ambiguous
- WHEN keyring mode is used
- THEN it returns `credentials_unavailable` with no remote attempt or fallback

#### Scenario: Platform secret transport is constrained
- GIVEN a macOS Set operation or a Windows/Linux native-store failure
- WHEN the adapter handles it
- THEN the secret is absent from argv and environment, or failure is deterministic

#### Scenario: TUI status is secret-free
- GIVEN a configuration view requests credential status
- WHEN the model receives the result
- THEN it contains only an opaque presence classification

#### Scenario: Supported-platform evidence is bounded
- GIVEN a supported windows, darwin, or linux amd64/arm64 target
- WHEN platform acceptance is planned
- THEN it uses available runners only and invents no unavailable-runner coverage

## ADDED Requirements

### Requirement: Explicit Remote-Proof Consent Boundary

Remote proof MUST require an existing pinned-trust decision and explicit WSS consent for each attempt. Only an eligible sanitized WSS failure MAY offer SSH fallback, which MUST require distinct explicit consent. Cancellation, deadline expiry, rejection, or stale completion MUST fail closed, produce sanitized feedback/audit classification, and MUST NOT trigger another transport, persistence, or authentication attempt.

#### Scenario: Trust blocks proof
- GIVEN host trust is absent, changed, or unaccepted
- WHEN proof is requested
- THEN no WSS or SSH attempt occurs

#### Scenario: Failure does not auto-fallback
- GIVEN a consented WSS proof fails with an eligible sanitized classification
- WHEN no SSH consent is provided
- THEN the system pauses for the operator and starts no SSH attempt
