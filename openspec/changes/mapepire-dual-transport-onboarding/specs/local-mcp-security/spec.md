# Delta for local-mcp-security

## MODIFIED Requirements

### Requirement: Native Secret Isolation and Step 8 Credential Boundary

Credentials SHALL support policy-governed `Ask each time` and native-keyring `Store securely`. Retrieval SHALL occur only in Step 8, remain within orchestration lifetime, and be zeroized where practical. Prompt/keyring denial, unavailable, malformed, locked, or policy-denied outcomes SHALL fail closed; no plaintext, vault, or mode downgrade is permitted. Secrets MUST NOT enter TUI state/messages, logs, audit, argv, environment, profiles, or results.
(Previously: native keyring was the only permitted authentication path and Step 8 timing was not normative.)

#### Scenario: Prompt and keyring success
- GIVEN the selected policy mode is `Ask each time` or `Store securely`
- WHEN Step 8 retrieves credentials successfully
- THEN the opaque credential is used only for authentication and is released after orchestration

#### Scenario: Prompt denial is terminal
- GIVEN `Ask each time` is selected and the operator denies or abandons the prompt
- WHEN Step 8 requests credentials
- THEN it returns `credentials_unavailable` without a mode downgrade or remote call

#### Scenario: Keyring unavailable is terminal
- GIVEN `Store securely` is selected and the native keyring is unavailable, locked, or denied
- WHEN Step 8 requests credentials
- THEN it returns `credentials_unavailable` without plaintext fallback, mode switch, SSH fallback, or remote call

#### Scenario: Credential failure never downgrades
- GIVEN either mode returns malformed, empty, or retrieval failure
- WHEN Step 8 requests credentials
- THEN it fails closed without plaintext, vault, alternate mode, SSH fallback, or remote call

### Requirement: Independent Trust and Sanitized Audit

TLS and SSH TOFU SHALL be user-controlled V1 enrollments with explicit host/port/fingerprint confirmation and independent storage. Mismatch or rotation SHALL block terminally. Audit SHALL use an allowlist of bounded policy identity, transport attempt, trust outcome, fallback reason, proof revision/version, result classification, duration, and lifecycle outcome; it MUST exclude credentials, endpoints, hosts, paths, users, SQL, rows, raw errors, and secrets.
(Previously: host trust and audit were bounded but did not define dual-transport independence or proof metadata.)

#### Scenario: Trust enrollment and mismatch
- GIVEN first-use TLS or SSH identity, or a changed identity
- WHEN exact confirmation is supplied or the mismatch is presented
- THEN only that transport is enrolled, or the operation blocks without silent acceptance

#### Scenario: Audit redaction
- GIVEN any success, denial, fallback, cancellation, cleanup, or failure outcome
- WHEN it is audited or shown
- THEN only allowlisted bounded metadata is retained and no sensitive field appears

### Requirement: Controlled Remote Mutation Surface

Remote artifact upload and fixed process launch SHALL occur only inside the eligible, consented, policy-authorized SSH runtime; arbitrary shell, SFTP, SQL, command, mutation, and generic execution surfaces MUST NOT exist.

#### Scenario: Step 3/4 security boundary
- GIVEN the wizard performs pre-auth Steps 3 or 4
- WHEN they complete
- THEN zero credentials, runtime, artifact, Java, upload, SQL, or remote mutation calls occur

## ADDED Requirements

### Requirement: Historical Marker Privacy

Any persisted proof marker SHALL contain only bounded timestamp, outcome classification, and proof revision, SHALL never gate or skip proof, and SHALL be invalidated by endpoint, policy, or trust changes.

#### Scenario: Marker is not sensitive
- GIVEN a successful or failed proof marker
- WHEN it is persisted and later read
- THEN prohibited transport/version/endpoint/user/path/error/SQL/result/secret data is absent
