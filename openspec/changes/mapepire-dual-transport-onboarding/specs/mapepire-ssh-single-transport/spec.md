# Delta for mapepire-ssh-single-transport

## MODIFIED Requirements

### Requirement: Independent SSH Trust, Consent, and Framing

SSH fallback SHALL require an eligible WSS classification, policy permission, independently stored SSH trust, an opaque credential retrieved at the last responsible moment, and explicit operator consent. V1 TOFU SHALL require exact host/port/fingerprint confirmation; mismatch or rotation SHALL block terminally. Authenticated traffic SHALL use bounded one-JSON-object-per-LF framing and fixed protocol operations only.
(Previously: SSH trust and LF framing were specified without the complete fallback gate.)

#### Scenario: Eligible fallback starts
- GIVEN daemon establishment refusal, unavailable daemon, availability timeout, policy-disabled daemon, or verified unsupported version
- WHEN policy, SSH trust, credential, and consent are valid
- THEN SSH-single starts with no arbitrary command input

#### Scenario: Non-eligible failure does not fall back
- GIVEN an operation timeout after transport/session start, context cancellation, proof/cleanup timeout, resource/bounds limit, identity/trust, protocol/framing/malformed/downgrade, credential, or authorization failure
- WHEN fallback is considered
- THEN SSH is not contacted and the failure remains terminal

#### Scenario: SSH TOFU is independent
- GIVEN a new SSH identity or changed key
- WHEN the operator confirms enrollment or a mismatch is detected
- THEN only SSH evidence is enrolled, or fallback blocks; TLS trust is neither reused nor silently accepted

#### Scenario: SSH resources close
- GIVEN SSH client, channel, process, or transport acquisition has partially succeeded
- WHEN startup, session, proof, or cancellation fails
- THEN every acquired resource is closed and no alternate transport is silently attempted

### Requirement: Verified SSH Protocol Detection

SSH `getversion` SHALL require matching ID and `success=true`; a version field alone SHALL never establish support. Identity, protocol, Java, artifact, session, proof, and authorization failures SHALL remain distinct typed terminal classifications.
(Previously: only ID/success validation and broad error distinction were required.)

#### Scenario: Invalid response is rejected
- GIVEN missing ID, mismatched ID, or `success=false`
- WHEN detection runs
- THEN it returns protocol failure with no readiness claim
