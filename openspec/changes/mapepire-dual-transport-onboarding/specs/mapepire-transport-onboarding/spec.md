# mapepire-transport-onboarding Specification

## Purpose

Resolve trusted Mapepire transport and provide truthful nine-step onboarding.

## Requirements

### Requirement: Managed Resolver and Trust Policy

The resolver MUST prefer a managed WSS daemon at policy endpoint default port `8076`; users MUST NOT select transport or URL. It MUST allow automatic SSH fallback only for bounded timeout/refusal/unavailability, daemon-disabled policy, or verified unsupported version. TLS identity mismatch, downgrade/tampering, credential or authorization failure MUST be terminal. Fallback additionally requires independent SSH trust. One credential reference MUST serve both transports.

#### Scenario: Daemon unavailable falls back safely
- GIVEN trusted SSH policy and daemon refusal within timeout
- WHEN resolution runs after fallback authorization
- THEN SSH is selected and the reason is classified as availability

#### Scenario: Credential failure never falls back
- GIVEN daemon `connect` fails for credentials or authorization after Step 6
- WHEN resolution classifies the failure
- THEN it stops without SSH fallback

#### Scenario: Fallback lacks trust
- GIVEN daemon availability is fallback-eligible but no SSH trust policy exists
- WHEN resolution considers SSH
- THEN it blocks fallback without contacting SSH

### Requirement: Step 3/4 Truthful Readiness

Steps 3 and 4 MUST be redesigned together without changing nine-step order. Step 3 MUST inspect daemon TLS first and SSH host key only when eligible and permitted; it MUST use no credentials, authentication, Db2, JAR, Java, upload, or launch. Step 4 MAY perform only pre-auth transport/protocol detection and MUST say exactly `[OK] Mapepire detected — authentication pending` or a localized equivalent when applicable. Credentials remain Step 6; authenticated `connect` and optional bounded query proof begin at Step 8. Readiness MUST distinguish trusted identity, reachability, detected protocol, authentication pending, authenticated session, and validated query.

#### Scenario: Step 3 and Step 4 do not overclaim
- GIVEN a trusted daemon or eligible fallback
- WHEN Steps 3–4 complete
- THEN no credential/auth/SSH runtime action occurs and no authenticated or query-ready claim appears

#### Scenario: Step 8 proves the session
- GIVEN credentials are available at Step 6
- WHEN Step 8 performs `connect` and an optional bounded read-only query
- THEN only then may authenticated session or validated query be reported

#### Scenario: Step 3 uses no credentials
- GIVEN Step 3 inspects daemon TLS and an eligible SSH identity
- WHEN inspection completes
- THEN it performs no credential lookup, authentication, Db2, JAR, Java, upload, or launch

### Requirement: Secret-Free Persistence and Audit

Profiles MUST persist only endpoint policy reference, fallback permission, trust modes, and approved fingerprints/pins. Selected transport, readiness, version, and errors MUST be ephemeral and recomputed. Audit MUST record policy identity, attempts/classifications, trust outcome, fallback reason, protocol revision/version, and sanitized result; it MUST exclude secrets, certificates, hosts, paths, URLs, raw errors, SQL, and result content.

#### Scenario: Legacy profile migrates conservatively
- GIVEN legacy SSH fingerprint, `MapepireJAR`, and credential-mode fields
- WHEN the profile is loaded or migrated
- THEN compatibility is retained only with revalidation; old observations are never trusted automatically
