# mapepire-ssh-single-transport Specification

## Purpose

Provide an independently trusted SSH-single transport adapter.

## Requirements

### Requirement: Independent SSH Trust and Framing

SSH fallback MUST require a separate approved known-host/pin or policy-controlled explicit TOFU identity; a host-key mismatch MUST be terminal. Authenticated stdin/stdout traffic MUST use one JSON object per LF-delimited frame with bounded framing. SSH trust inspection MAY occur without credentials, but protocol availability requires later authenticated runtime startup.

#### Scenario: Availability fallback uses trusted SSH
- GIVEN daemon refusal/timeout and independently approved SSH trust
- WHEN later credentials and consent permit fallback
- THEN SSH-single starts behind the adapter with LF framing

#### Scenario: SSH identity is unsafe
- GIVEN no SSH trust policy or a changed host key
- WHEN fallback is considered
- THEN fallback is blocked with a sanitized terminal classification

### Requirement: Verified SSH Protocol Detection

SSH `getversion` detection MUST require both a matching response ID and `success=true`; a version field alone MUST NOT establish support and Java, artifact, or process errors MUST remain distinct.

#### Scenario: SSH getversion is valid
- GIVEN an authenticated SSH-single process returns matching ID and success
- WHEN detection validates the response
- THEN the supported protocol version is observed

#### Scenario: JS-style unsupported response is rejected
- GIVEN a version response has no matching ID or has `success=false`
- WHEN detection runs
- THEN it returns protocol failure and no readiness claim
