# IBM i Profile Onboarding

`nexus configure` uses one direct, Spanish-first onboarding flow:

```text
Create → host + IBM i username → Connect and Save → completion → Finalize → profiles
```

The persistent Bubble Tea model holds only the host and IBM i username. Selecting
**CONECTAR Y GUARDAR** captures the password transiently through a fixed,
in-process `tea.Exec` terminal boundary. Password bytes never enter the model,
Bubble Tea messages, views, logs, audit records, profile JSON, or files.

English catalog parity is maintained for explicit English composition. Runtime
frames are covered at 120x40, 80x24 with `NO_COLOR`, and 40x16.

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
