# Delta for mapepire-wss-transport

## MODIFIED Requirements

### Requirement: Trusted Authenticated WSS Session

The adapter SHALL perform bounded unauthenticated `/version` observation before authentication, then expose a credential-aware typed session factory. TLS trust SHALL reuse the approved endpoint policy and V1 user-controlled TOFU evidence; mismatch, expiry, hostname failure, or rotation SHALL be terminal. Authentication SHALL occur only in Step 8, with bounded cancellation and cleanup, and SHALL have no SSH dependency.
(Previously: WSS framing and `/version` observation were specified without an authenticated production session factory.)

#### Scenario: Pre-auth observation succeeds
- GIVEN an approved endpoint and trusted TLS identity
- WHEN `/version` returns supported compatibility
- THEN the result is protocol detected/authentication pending and no credential is requested

#### Scenario: Auth failure is terminal
- GIVEN pre-auth `/version` succeeds but typed `connect` fails for credentials or authorization
- WHEN Step 8 classifies the failure
- THEN it returns terminal failure and does not invoke SSH

#### Scenario: WSS success has zero SSH calls
- GIVEN WSS authentication and fixed proof succeed
- WHEN the session closes
- THEN no SSH, artifact, Java, upload, or fallback call occurs

#### Scenario: TLS trust is explicit
- GIVEN first-use TLS identity or a changed/rotated identity
- WHEN the operator confirms enrollment or a mismatch is presented
- THEN exact host/port/fingerprint confirmation enrolls independently, or the mismatch blocks without silent acceptance

#### Scenario: Cancellation cleans up
- GIVEN a WSS dial, connect, proof, or close is pending
- WHEN context is cancelled
- THEN the client closes the WebSocket/session and returns a terminal cancelled classification

## ADDED Requirements

### Requirement: Text Framing and Bounds

Each application request/response SHALL be one bounded JSON text WebSocket message with compression disabled; plaintext and `MP_UNSECURE` production paths MUST NOT exist.

#### Scenario: Binary or oversized input is rejected
- GIVEN a binary frame or message beyond release bounds
- WHEN the adapter receives it
- THEN it terminates with protocol/limit failure and exposes no content
