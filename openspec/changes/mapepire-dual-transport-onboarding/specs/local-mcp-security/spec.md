# Delta for local-mcp-security

## MODIFIED Requirements

### Requirement: Pinned Host Trust Policy

The system MUST support manual verified enrollment and explicit pinned TOFU enrollment for both daemon TLS identity and SSH host identity, with non-secret provenance and independent trust domains. Remote inspection MUST be explicitly requested or policy-authorized, bounded, cancellable, labeled, and exactly confirmed before enrollment. Changed identity, hostname/pin/expiry failure, protocol tampering, and unsafe downgrade MUST fail closed and MUST NOT trigger fallback.
(Previously: the requirement governed only host-key trust and preserved a future `require-verified` SSH mode.)

#### Scenario: Daemon pin or TOFU enrollment
- GIVEN an approved certificate identity and exact operator confirmation
- WHEN pin or explicit TOFU enrollment completes
- THEN only approved trust evidence is persisted

#### Scenario: Explicit SSH TOFU enrollment pins identity
- GIVEN SSH TOFU is authorized and the inspected key is exactly confirmed
- WHEN enrollment completes
- THEN the key and unverified provenance are pinned independently

#### Scenario: TLS mismatch blocks fallback
- GIVEN a daemon presents a mismatched identity or downgrade
- WHEN resolution runs
- THEN it returns terminal failure and does not inspect or use SSH

#### Scenario: SSH mismatch blocks fallback
- GIVEN SSH presents a key different from its approved evidence
- WHEN fallback is attempted
- THEN it returns `host_key_changed` and no remote operation proceeds

#### Scenario: Pinned identity remains verified
- GIVEN an enrolled TLS or SSH identity is presented unchanged
- WHEN a connection is attempted
- THEN trust succeeds without treating the other transport as trusted

## ADDED Requirements

### Requirement: Bounded Dual-Transport Audit Surface

The system MUST audit transport attempt, policy identity, trust result, fallback reason, protocol revision/version, and sanitized result classification. It MUST exclude credentials, certificates, host/path/URL, raw errors, SQL, and result content, and MUST preserve the existing prohibition on arbitrary SSH, SFTP, shell, SQL, and mutation surfaces.

#### Scenario: Audit is sanitized
- GIVEN daemon selection or SSH fallback succeeds or fails
- WHEN the outcome is audited
- THEN only approved bounded classifications and identities are retained

#### Scenario: No live IBM i is required
- GIVEN automated protocol, resolver, framing, artifact, and wizard tests
- WHEN acceptance runs
- THEN fakes or loopback are used and status remains `not_validated_on_ibmi`
