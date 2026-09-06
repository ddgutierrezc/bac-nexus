---
schema: gentle-ai.sdd-preproposal/v1
revision: 5
change: add-codefori-companion-provider
proposal_ready: bounded_offline_implementation
artifact_store: openspec
---

# Pre-proposal state: add-codefori-companion-provider

## Input artifacts

```yaml
exploration:
  path: openspec/changes/add-codefori-companion-provider/exploration.md
  observed: true
research:
  selected: false
  blocked_attempt_retained: openspec/changes/add-codefori-companion-provider/research.md
  user_resolution: continue_without_formal_research
```

The formal research lane remains explicitly deselected. The unchanged blocked `research.md` is a historical blocked-attempt record and is not treated as validated evidence.

## Confirmed product decisions

```yaml
first_live_topology:
  status: confirmed
  decision: GoLand/Nexus and VS Code both run natively on Windows.
serve_selection:
  status: confirmed
  decision: No profile defaults to Companion; an existing -profile selects Native; contradictory explicit flags fail closed; no availability fallback exists.
companion_implementation:
  status: confirmed
  decision: The Companion runtime is 100 percent TypeScript; the only allowed system-utility exception is the fixed inbox Windows PowerShell descriptor ACL operation, with no custom helper, addon, third-party ACL dependency, or general command runner.
windows_descriptor_acl:
  status: mechanism_authorized_native_evidence_pending
  decision: Implement exactly two descriptor-specific fixed PowerShell operations for secure publication and generation-owned cleanup; documented Directory.CreateDirectory with DirectorySecurity secures new-directory creation, existing directories are returned unchanged and require validation, and no caller-provided executable, script, path, arguments, or inherited-ACL downgrade is permitted.
transport:
  status: confirmed
  decision: HTTP/JSON on 127.0.0.1 and a known port are acceptable for the spike; automated discovery is deferred evolution.
sql_policy:
  status: confirmed
  decision: Only SELECT CURRENT_USER FROM SYSIBM.SYSDUMMY1 is recognized with limited ASCII whitespace/case normalization; no arbitrary SQL or parser project is permitted.
query_length:
  status: settled
  decision: SELECT CURRENT_USER FROM SYSIBM.SYSDUMMY1 is exactly 41 ASCII bytes and is not a blocker.
concurrency:
  status: confirmed
  decision: Immediate bounded admission allows at least ten calls with no waiting/worker queue or global SQL serialization; timed-out started promises retain capacity until settlement, and live shared-job behavior remains unvalidated.
release:
  status: confirmed
  decision: Initial tag companion-v0.1.0 and workflow .github/workflows/release-companion.yml; publication identity is not invented or approved here.
native_boundary:
  status: confirmed
  decision: Native onboarding, profiles, credentials, SSH, Mapepire, configuration, TUI, validation, audit, recovery, and ownership implementation remain untouched.
```

## Corrected evidence limits

- Public Code for IBM i 3.0.12 `subscribe(...)` returns `void`; no unsubscribe/disposable is evidenced.
- Descriptor location, loopback, SID, DACL, and local-volume checks do not prove VS Code extension-host identity.
- The raw `runSQL` column label is not live-evidenced; normalize an exactly-one-row/exactly-one-column result without depending on its key.
- A Go response-header timeout must exceed the five-second Companion SQL wait.
- The canonical query is settled at 41 ASCII bytes; no further confirmation is required.
- Microsoft documentation confirms that `Directory.CreateDirectory(path, DirectorySecurity)` applies security during new-directory creation and returns an existing directory unchanged; an existing directory therefore requires exact validation before readiness.
- Protected file create-new/open-handle semantics, fixed executable/spawn behavior, and equivalent Go consumer checks still require source confirmation and native evidence.
- No Windows ACL command or test was executed in this design pass; documentation alone cannot establish Windows behavior.

## Conditional feasibility

The fixed operation is bounded to descriptor publication and cleanup. It resolves the inbox Windows PowerShell executable without `PATH`, runs with immutable no-profile/noninteractive arguments and `shell: false`, derives its fixed local-application-data path internally, and accepts no model-, MCP-, setting-, or caller-controlled script/path interpolation.

Publication must secure and validate a current-process-SID-owned, protected, current-user-only directory before emitting `READY`. TypeScript generates the token only after `READY` and sends the bounded descriptor through stdin. The operation then creates the file with an explicit protected DACL as part of create-new, validates it through the open handle, and emits only fixed bounded status text. The secret is never placed in argv, environment, stdout, stderr, logs, fixtures, or a temporary file. Cleanup retains the bound socket and removes only an exact descriptor whose validated generation belongs to that activation; it is never recursive.

This is not a general local execution API and is not permission to repair a descriptor after writing it. `chmod`, inherited `%LOCALAPPDATA%` permissions, directory placement, and Go-only post-write checks remain rejected as confidentiality evidence.

## Readiness and blockers

```yaml
proposal_ready: bounded_offline_implementation
live_activation_blocking_conditions:
  - Official sources must still verify protected file create-new/open-handle validation, trusted inbox PowerShell resolution, noninteractive spawn behavior, known-folder/SID semantics, and equivalent Go consumer checks; new-directory create-with-security semantics are verified.
  - Separately authorized native-Windows tests must prove exact current-user SID ownership, protected current-user-only DACLs, no readable transient descriptor, no secret in argv/stdout, cross-user read denial, fail-closed adversarial cases, bounded timeout behavior, and generation-owned cleanup.
non_release_blockers:
  - Marketplace extension identity and OIDC trusted-publisher ownership remain unapproved.
  - Live Code for IBM i row shape, session events, cancellation, and shared-job concurrency remain not_validated_on_ibmi.
resolved_conditions:
  - The fixed local PowerShell mechanism is authorized for bounded evaluation; chmod and inherited ACLs remain rejected.
  - The canonical query length is 41 bytes.
```

The option is ready for bounded fake-first implementation behind fail-closed activation and descriptor-specific publish/cleanup ports, but native-Windows operation is not yet proven. The first unit is only `internal/provider/provider.go` plus `internal/provider/provider_test.go`, covering the neutral port, canonical proof query, and normalized result validation within 300 changed lines. Later slices complete remaining source verification and a separately authorized real-Windows ACL run. Until those gates pass, authenticated Companion activation and publication remain disabled. Do not substitute post-write ACL repair, a custom helper, arbitrary PowerShell, a broad parser, a waiting/worker queue, global SQL serialization, or any Native-path refactor.
