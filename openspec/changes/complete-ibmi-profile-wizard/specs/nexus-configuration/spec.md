# Delta for nexus-configuration

## ADDED Requirements

### Requirement: Canonical Eight-Step Profile Creation

`nexus configure` MUST present the ordered lifecycle: Profile, Connection, Server Identity, Mapepire Readiness, Credentials, Review & Save, Optional Remote Proof, and Completion. It MUST NOT present a Java screen or a phantom step. The existing bounded Mapepire observation and approved TOFU behavior SHALL remain unchanged by renumbering.

#### Scenario: Credentials are selected safely
- GIVEN a valid draft reaches Credentials
- WHEN the operator selects `prompt` or `keyring`
- THEN the selection is reviewable without displaying or retaining a secret
- AND unavailable keyring storage blocks progression with actionable feedback

#### Scenario: Review saves a fresh profile once
- GIVEN all required non-secret V3 metadata is valid
- WHEN the operator confirms Review & Save
- THEN the system atomically creates exactly one profile before proof is offered
- AND a duplicate or conflicting create leaves existing data unchanged and reports a sanitized conflict

#### Scenario: Keyring failure compensates safely
- GIVEN the selected keyring operation cannot complete before profile creation commits
- WHEN Review & Save is confirmed
- THEN no profile is committed and no remote proof is started

#### Scenario: Fresh profile reaches proof
- GIVEN a V3 profile was saved successfully
- WHEN the operator continues
- THEN Optional Remote Proof receives that saved profile
- AND saving remains successful even when proof is omitted

### Requirement: Optional Proof and Truthful Completion

The proof step MUST be optional, explicit, bounded, cancellable, retryable, and separate from saving. Completion MUST distinguish omitted, cancelled, failed, and successful proof, state only saved/local configuration or `ready_for_controlled_validation` when justified, and MUST NOT claim IBM i or `nexus serve` readiness.

#### Scenario: WSS proof requires consent
- GIVEN a saved profile enters Optional Remote Proof
- WHEN WSS consent has not been explicitly confirmed
- THEN no remote attempt occurs

#### Scenario: WSS lifecycle is controlled
- GIVEN an explicitly consented WSS request is running
- WHEN it is cancelled, times out, or a retry supersedes it
- THEN the operation stops or is superseded and stale results do not change the visible outcome

#### Scenario: SSH fallback is independently consented
- GIVEN WSS returns an eligible sanitized failure
- WHEN the operator has not separately consented to SSH fallback
- THEN SSH MUST NOT start automatically and the operator may retry, omit, or consent

#### Scenario: Terminal interaction remains accessible
- GIVEN any supported size, keyboard-only navigation, resize, or `NO_COLOR`
- WHEN focus moves among fields, blocked actions, consent, retry, and completion
- THEN focus, selection, feedback, and outcome remain discernible and usable without color alone
