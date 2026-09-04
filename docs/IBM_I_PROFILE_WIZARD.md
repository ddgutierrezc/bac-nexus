# IBM i Profile Onboarding

`nexus configure` uses one Spanish-first, four-step onboarding flow:

```text
Create → 1 Name → 2 Connection → 3 Credentials → 4 Review → Connect and Save → completion → Finalize → profiles
```

Name, host, IBM i username, and SSH port are non-secret values. The port defaults
to 22 and remains unchanged through inspection, proof, identity comparison, and
profile persistence. Back preserves those values. Invalid Next or Connect and
Save stays focusable, reports actionable feedback, and moves focus to the first
invalid field.

frames cover all four steps at 120x40, 80x24, and 40x16 in Spanish and English,
with color and `NO_COLOR`. The narrow viewport exposes truthful overflow; text
wraps without losing content, and `NO_COLOR` emits no ANSI escapes.

## Credential boundary and review

Step 3 visibly offers secure password capture through a fixed in-process
`tea.Exec` terminal boundary. Capture reads hidden terminal input directly into
an application-owned, single-use lease. The Bubble Tea model and messages retain
only an opaque lease identity, generation, and secret-free status.

Step 4 reviews only name, host, port, and username. It never displays a password
or lease value. Password bytes exist only at the terminal capture, lease,
credential-store, and proof boundaries; they are revoked and zeroed on retry,
Back, identity edits, cancellation, expiry, stale results, shutdown, and
compensation. There is no eight-step workflow, proof-choice screen, draft, or
proof rerun.

## Connection and trust

After terminal capture, the onboarding service derives the profile defaults and
performs a bounded, authenticated WSS proof before saving. First contact accepts
only an unverified observed host key under the automatic TOFU policy and records
the immutable provenance `automatic-tofu-v1:first-contact-unverified`.

The service durably records `identity_bootstrap_allowed` before proof and
`identity_pin_committed` after profile persistence. If either audit cannot be
recorded, onboarding fails closed. A failed committed audit compensates the new
profile and native-keyring credential; cleanup failures are reported as requiring
cleanup rather than as success.

Existing pins are never silently replaced. Missing, ambiguous, or changed host
identity evidence fails closed. WSS is preferred; SSH fallback is permitted only
after an eligible non-security WSS failure. A policy-bound grant creates internal
`SSHConsent` and a single-use ticket that is consumed immediately. Identity,
trust, protocol, malformed-response, credential, and other security failures
never downgrade. This is internal policy, not a user-facing choice.

## Credential policy

When the native keyring capability is supported, the password is stored only in
the native keyring and profile metadata records keyring mode. Unsupported or
unavailable capability uses prompt-on-use mode and persists no password. A
supported keyring operation that fails is an error, not a fallback.

## Profile management and recovery

Finalize reloads the modern profile list without replaying onboarding feedback.
Existing valid profiles remain compatible and retain open, delete, back, and exit
navigation. Deletion requires the exact confirmation token and preserves the
existing backup/recovery behavior; it does not silently delete credentials.

## Validation boundary

Normal tests use deterministic fakes and temporary directories. They do not
contact IBM i. Live IBM i validation is explicitly opt-in and requires an
approved environment, identity, authority, endpoint, and trust policy. Until
then, Nexus remains `not_validated_on_ibmi`.
