# Delta for production-nexus-serve

## ADDED Requirements

### Requirement: Profile presence selects an isolated provider

`nexus serve` without `-profile` MUST select Companion mode. An existing nonblank `-profile` MUST select the current Native provider and preserve its current admission and behavior. If an explicit provider selector is retained, it MAY only restate the mode implied by profile presence: Companion with no profile or Native with a profile. Any contradiction, unknown provider, explicit Native without a profile, or explicit Companion with a profile MUST fail closed before MCP construction or remote contact.

Provider availability MUST NOT alter selection. Companion mode MUST NOT load, require, create, or modify a Nexus profile, IBM i credential, eligibility record, ownership ledger, direct recovery dependency, SSH/SFTP transport, Mapepire transport, Native configuration, TUI state, or Native validation evidence. Missing Companion state MUST NOT block MCP startup and MUST NOT trigger Native fallback.

#### Scenario: No profile defaults to Companion

- GIVEN `nexus serve` has no profile and no contradictory explicit flag
- WHEN Nexus starts without a Companion broker
- THEN Companion-mode MCP serving starts
- AND later tool calls return bounded unavailable states

#### Scenario: Profile preserves Native selection

- GIVEN `nexus serve -profile <name>`
- WHEN Nexus starts
- THEN the existing Native composition and admission path is selected
- AND Companion dependencies are not constructed

#### Scenario: Explicit flags contradict profile presence

- GIVEN Companion is explicitly requested with a profile, or Native is explicitly requested without one
- WHEN `serve` is invoked
- THEN it fails closed before MCP serving, descriptor access, or remote contact

### Requirement: Companion evidence remains offline and qualified

Normal Companion verification MUST use Go/TypeScript fakes and host-local loopback only. It MUST NOT require or contact VS Code or IBM i. At least ten fake-backed calls MUST prove concurrent immediate admission and reordered response correlation without a waiting/worker queue or process-wide/global mutex. Excess work MUST fail boundedly, and any timed-out or cancelled started `runSQL` promise MUST retain capacity until its underlying promise settles. This evidence MUST NOT be represented as Code for IBM i shared-job parallel-safety evidence.

The first authorized live topology, when separately approved, is native Windows GoLand/Nexus plus native Windows VS Code. Release evidence and documentation MUST retain `not_validated_on_ibmi` until that controlled validation occurs. Folder/local-volume checks MUST NOT be used as proof of extension-host identity.

#### Scenario: Offline verification runs

- GIVEN normal CI
- WHEN Companion status, query, timeout, cancellation, and correlation cases run
- THEN they use fakes or local loopback only
- AND their evidence remains `not_validated_on_ibmi`

## MODIFIED Requirements

### Requirement: Bounded Operational MCP Lifecycle

When a profile selects Native mode, the service MUST continue to expose exactly `resolve_catalog_candidates` and `read_selected_source`. All existing catalog/source bounds, cancellation, cursor, lease, SSH/SFTP, owned temporary path, recovery, cleanup, eligibility, credential, Mapepire, and sanitized failure behavior remain unchanged.

When no profile selects Companion mode, the service MUST expose exactly `session.status` and `sql.query`. It MUST NOT register either Native tool, construct Native operations, or fall back to Native. Companion availability is checked lazily by tool operations, not during startup.

Companion `sql.query` MUST use only the separately specified finite proof recognizer and bounded normalized single-column result. A five-second Companion SQL wait MUST be paired with a Go response-header timeout of at least six seconds and a total query deadline of at least seven seconds. Admission MUST be immediate and bounded with capacity for at least ten fake calls; no waiting/worker queue or global default SQL serialization SHALL be introduced. A started promise retains capacity through timeout/cancellation until settlement; fake concurrency evidence remains distinct from live IBM i behavior.

#### Scenario: Native tool surface remains exact

- GIVEN a profile-selected Native invocation completes admission
- WHEN MCP registration completes
- THEN exactly `resolve_catalog_candidates` and `read_selected_source` are exposed
- AND no Companion tool is registered

#### Scenario: Companion tool surface remains exact

- GIVEN a no-profile Companion invocation starts
- WHEN MCP registration completes
- THEN exactly `session.status` and `sql.query` are exposed
- AND no Native tool is registered

#### Scenario: Provider becomes unavailable

- GIVEN either selected provider later becomes unavailable
- WHEN an operation runs
- THEN it returns that provider's existing bounded sanitized outcome
- AND Nexus does not switch providers
