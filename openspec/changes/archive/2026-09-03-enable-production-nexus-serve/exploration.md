## Exploration: Enable Production Nexus Serve

### Current State
`nexus serve -profile <name>` builds the typed MCP server but cannot pass `Service.Startup`: `defaultDeps` supplies only native credentials, policy, in-memory audit, server factory, and clock. Resolver, acquirer, recovery, and leases remain nil. The requested profile is not loaded or validated. The two MCP tools and their service contracts are already implemented, bounded, and fail closed; README's quick path is therefore inaccurate.

Catalogados has a concrete fixed-query resolver over a Mapepire executor. Source acquisition is inherently SSH/SFTP: it gets the authenticated home, creates a private owned file, runs the fixed `CPYTOSTMF`, downloads it, and confirms exact deletion. Current WSS supports proof, not a catalog-executor adapter, so it cannot make both tools functional by itself. The SQLite ownership ledger and in-memory lease store are concrete implementations; the ledger uses the local config root and process leases use fresh random epoch/capabilities.

### Affected Areas
- `cmd/nexus/main.go` — production composition, profile admission, resource ownership, and stderr-only diagnostics.
- `internal/connectors/ibmi/catalogados/` and `internal/mapepire/sshstdio/` — adapt the fixed catalog query to an approved SSH Mapepire session.
- `internal/remote/ssh.go` and `internal/source/` — request-scoped SSH/SFTP acquisition and startup recovery adapters.
- `internal/ownership/sqlite/ledger.go` — approved local-state root and durable ownership lifecycle.
- `internal/profile/` and `internal/credential/` — profile validation and non-interactive keyring-only credential admission.
- `internal/audit/` — current recorder is in-memory only; controlled-validation retention policy needs an explicit decision.
- `README.md`, `docs/ARCHITECTURE.md`, `docs/SECURITY.md`, `docs/IBM_I_VALIDATION.md`, `.github/workflows/go-verification.yml` — correct serve readiness, release identity/runbook, and validation evidence claims.

### Approaches
1. **SSH-only controlled validation slice** — compose catalog resolution and source acquisition through one approved SSH/SFTP transport policy, with request-scoped clients and a keyring-backed V3 profile.
   - Pros: Makes both registered tools functional using existing concrete primitives; source acquisition already requires SSH/SFTP; keeps one trust/credential path and no generic surface.
   - Cons: Requires an explicit approved non-interactive Mapepire launch/artifact policy and cannot use WSS preference yet.
   - Effort: High.

2. **WSS catalog plus SSH source** — add a WSS catalog executor and use SSH only when source is requested.
   - Pros: Aligns with the existing preferred-WSS direction.
   - Cons: No current WSS executor adapter exists; two transport/trust lifecycles increase validation and failure-state complexity without removing SSH for source.
   - Effort: High.

3. **WSS-only serve** — restrict v1 to catalog resolution.
   - Pros: Avoids SSH/SFTP acquisition.
   - Cons: Cannot make `read_selected_source` functional, so it fails the required vertical slice.
   - Effort: Medium.

### Recommendation
Adopt approach 1 for one process and one validated V3 `keyring` profile. Reject `prompt` (and legacy vault) profiles before server startup because stdio is owned by MCP and cannot safely prompt. Load the profile from `profile.DefaultRoot()` (`<UserConfigDir>/BAC Nexus/profiles`); use a sibling approved local-only state root `<UserConfigDir>/BAC Nexus/ownership` for `ownership.db`, subject to deployment approval. Derive the target digest with the existing recovery binding (host, port, username, SSH pin, trust) and use crypto-random lease entropy.

Startup order should be: validate profile and keyring availability; open/verify ledger; construct recovery; run recovery; construct the service, leases, and server; run stdio. Runtime clients must be opened and closed per operation; `LeaseReader.Close` remains request-scoped, while the SQLite ledger closes only after `Server.Run` returns. All lifecycle and sanitized operational diagnostics go to stderr; stdout remains exclusively MCP JSON-RPC.

The proposal should require contract tests with profile/keyring, catalog, SSH/SFTP, Mapepire, ledger, and MCP fakes/loopback fixtures; no normal test contacts IBM i. It should add bounded work-machine validation only after approved target, libraries, identity, local-state directory, artifact/JAR policy, and audit-retention decision. Current `audit.Recorder` is in-memory; it is sufficient for deterministic tests but not evidence retention. Controlled validation must either receive an approved redacting persistent sink/retention policy or fail closed before the field gate.

### Risks
- The fixed SSH Mapepire launcher currently requires explicit consent and a verified local JAR; unattended serve needs an approved persisted eligibility model, not an implicit bypass.
- No WSS catalog executor exists, and source retrieval cannot avoid SSH/SFTP under the current implementation.
- Existing README and architecture/security wording claim serve composition that does not exist; documentation must be corrected only with the implementation slice.
- Audit persistence, approved ownership DB directory, IBM i permissions, source/data classification, and work-machine endpoint policy remain deployment decisions.
- Do not touch unrelated `.atl` or UI-recovery/OpenSpec archive changes.

### Ready for Proposal
Yes — propose the SSH-only, keyring-only vertical slice, but mark these as explicit approval gates: approved fixed Mapepire SSH launch/artifact policy; `<UserConfigDir>/BAC Nexus/ownership` local-state convention; redacting audit retention for controlled validation; and one-process/one-profile lifecycle. Do not claim live IBM i validation.
