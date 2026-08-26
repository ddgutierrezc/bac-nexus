# mapepire-wss-transport Specification

## Purpose

Connect to a managed Mapepire daemon through a trusted WSS boundary.

## Requirements

### Requirement: Trusted Bounded WSS

The adapter MUST use WSS with bounded handshake, message, deadline, and response limits, hostname verification, and no plaintext or `MP_UNSECURE` production path. One application request/response MUST occupy one JSON text WebSocket message. CA plus hostname validation is preferred; policy-controlled verified pinning or explicit TOFU MAY be used. Pin mismatch and certificate rotation MUST require approved re-enrollment. WebSocket success MUST NOT prove credentials, authorization, Db2, or query readiness.

#### Scenario: CA-trusted daemon is framed correctly
- GIVEN a policy-approved CA and hostname
- WHEN a bounded WSS exchange runs
- THEN each JSON text message is validated and delivered to the protocol client

#### Scenario: Identity failure blocks
- GIVEN hostname, pin, expiry, certificate, or trust validation fails
- WHEN WSS connects
- THEN it returns a sanitized terminal error and does not permit fallback

### Requirement: Supported Daemon Probe

After approved TLS trust, the resolver MAY perform one automatic bounded unauthenticated `/version` probe. It MUST prove only endpoint identity/version and accept only supported Mapepire Server 2.3.5 compatibility; it MUST not use credentials or claim an authenticated session.

#### Scenario: Supported version is selected
- GIVEN CA trust and a supported 2.3.5 `/version`
- WHEN the probe succeeds
- THEN WSS is selected with protocol detected and authentication pending

#### Scenario: Unsupported version permits policy fallback
- GIVEN TLS trust is valid and `/version` proves an unsupported server version
- WHEN policy permits fallback
- THEN resolution may continue to independently trusted SSH with that reason

#### Scenario: WSS cannot prove authorization
- GIVEN WebSocket and `/version` succeed
- WHEN no authenticated `connect` has run
- THEN the result is not an authenticated session or validated query
