# Delta for production-nexus-serve

## MODIFIED Requirements

### Requirement: Fail-Closed Serve Admission and Composition

`nexus serve -profile <name>` MUST load exactly one V3 keyring profile, validate proof-bound eligibility before remote contact, open only a restrictive ownership DB, and preserve its existing Native Resolver, Acquirer, Recovery, Leases, Auditor, and MCP composition. Legacy, prompt, missing, stale, mismatched, unavailable-keyring, unsafe ledger, audit, or recovery states MUST fail closed with sanitized diagnostics.

`nexus serve` without `-profile` MUST select Companion mode without creating or requiring a profile, IBM i credential, eligibility record, ownership ledger, Native transport, Native audit state, or remote contact. Selection MUST NOT depend on provider availability. A profile MUST always select Native; Companion unavailability MUST NOT cause Native fallback.
(Previously: every `serve` invocation required a profile and composed only Native dependencies.)

#### Scenario: Valid Native startup
- GIVEN approved Native profile, eligibility, ledger, retention, and dependencies
- WHEN `serve -profile <name>` starts
- THEN recovery completes before Native MCP serving

#### Scenario: Native admission rejection has no remote contact
- GIVEN Native profile admission is invalid or unavailable
- WHEN `serve -profile <name>` starts
- THEN it fails closed before SSH, SFTP, or Mapepire contact

#### Scenario: Native ledger, audit, or recovery fails
- GIVEN a Native ownership, audit, or recovery dependency fails
- WHEN `serve -profile <name>` starts
- THEN Native MCP does not start and emits a sanitized classification

#### Scenario: No profile selects Companion
- GIVEN `serve` has no profile
- WHEN Companion is unavailable
- THEN Companion MCP starts and later operations return bounded unavailable states

### Requirement: Bounded Operational MCP Lifecycle

Native mode MUST expose only `resolve_catalog_candidates` and `read_selected_source`, preserving its fixed catalog operation, 50-candidate maximum, request-scoped SSH/SFTP, fixed copy operation, owned temporary path, recovery, exact cleanup, 4 MiB acquisition maximum, 200-line/128 KiB page maximum, selection/profile/10-minute lease cursor binding, and deterministic sanitized no-partial outcomes.

Companion mode MUST expose exactly `session.status` and `sql.query`, MUST NOT register Native tools, and MUST NOT switch providers. Its query MUST use only the fixed proof recognizer and bounded normalized one-column result. It MUST allow at least ten immediate fake-backed calls while enforcing the fixed Companion limits; it MUST have no queue or global default serialization, and a started query retains capacity until it settles after timeout or cancellation.
(Previously: the service exposed only the two Native catalog-context tools.)

#### Scenario: Receipt is rehashed before session creation
- GIVEN an issued Native receipt no longer matches its capability or integrity data
- WHEN Native session creation is attempted
- THEN it fails before a process starts

#### Scenario: Catalog request is bounded
- GIVEN admitted Native catalog work
- WHEN it completes, times out, is cancelled, or errors
- THEN it returns at most 50 candidates or its deterministic outcome

#### Scenario: Exact source selection is acquired and paged
- GIVEN an exact Native selection and valid request
- WHEN source is acquired through fixed operations
- THEN owned cleanup occurs without a generic path

#### Scenario: Cursor or acquisition is invalid
- GIVEN a stale cursor, cancellation, or acquisition failure
- WHEN a Native source page is requested
- THEN no partial page is returned and recovery remains available

#### Scenario: Companion tool surface remains exact
- GIVEN a no-profile Companion invocation starts
- WHEN MCP registration completes
- THEN exactly `session.status` and `sql.query` are exposed
- AND no Native tool is registered

#### Scenario: Provider becomes unavailable
- GIVEN the selected provider later becomes unavailable
- WHEN an operation runs
- THEN it returns that provider's bounded sanitized outcome without switching
