# BAC Nexus Security Boundary

BAC Nexus v1 is a read-oriented, local-first foundation for bounded IBM i source context. It is not a generic remote administration interface. When a security property cannot be established, the operation fails closed.

## Quick operator path

1. Run Nexus only with an approved local OS identity, approved profile, native credential access, and a pinned IBM i host key.
2. Treat any recovery, ownership, integrity, contention, or platform-policy failure as a stop condition; do not bypass it by deleting files or database rows broadly.
3. Before rollout, obtain the unresolved IBM i, data-classification, source/audit-policy, and corporate endpoint approvals.

## MCP server surface (v1)

The `nexus serve` subcommand runs the typed MCP stdio server built on the official `github.com/modelcontextprotocol/go-sdk`. The server exposes exactly two tools: `resolve_catalog_candidates` (bounded candidate coordinates) and `read_selected_source` (one page at a time). Wire schemas forbid temporary, listing, or delete fields; the cursor is the opaque server binding and is never echoed. Start-up runs the same pre-acquire recovery gate that every acquisition uses, and a failed recovery, missing credential, denied selector, stale coordinate, or cancelled context all fail closed before any remote work begins.

## Trust and threat model

| Boundary | Decision and residual risk |
|---|---|
| Local caller | The current local OS principal is the v1 trust boundary. Profile and client selectors are advisory capability selectors, not product authentication. A malicious same-principal process remains a residual risk. |
| MCP wire surface | The two allowed tools are the only path between an MCP client and the Nexus service. The typed input schemas forbid temporary, listing, or delete fields; audit and retained artifacts never include source, cursor, raw error, path, host, user, command, SQL, credential, or model content. The requested source page is the sole bounded MCP source result. |
| IBM i target | A fresh profile, credential, canonical target binding, and pinned host key are checked before recovery opens a cleanup connection. A privileged IBM i operator can still race or replace remote files; Nexus does not claim absolute protection from a privileged account. |
| Local ownership data | SQLite records only ownership metadata: version, 16-byte token, exact private path, profile selector, target digest, and canonical creation time. It never stores secrets, source, commands, cursors, or model content. |
| Uncertainty | Missing, malformed, conflicting, unavailable, corrupt, or inconclusive evidence fails closed. |

## Remote temporary ownership and recovery

Nexus can create only an owned temporary path below the validated authenticated home:

```text
<validated-home>/.bac-nexus/tmp/<32-hex-token>.utf8
```

The directory is private (`0700`), files are exclusively reserved (`0600`), tokens are 128 random bits, and the durable ownership ledger is limited to 64 records. Recovery reads at most 65 rows (`LIMIT 65`) so a 65th row is an overflow failure rather than an unbounded cleanup attempt.

Each recovery record is revalidated against a fresh profile, credential, binding, and pin before the exact recorded path is used. Cleanup is deliberately narrow:

```text
exact Remove → exact Stat reports not found → exact ownership Delete
```

No path is discovered, listed, globbed, or prefix-deleted. Historical shared `/tmp/bac-nexus-catalog-*` paths are outside automatic recovery.

Before every new `source.Acquirer.Acquire`, the pre-acquire gate runs recovery. A missing recovery dependency, cancelled context, recovery error, or retained unsafe row stops acquisition before a new acquisition remote connection, private-directory operation, ownership admission, token generation, copy, or other new remote operation. Real Nexus process start-up also runs this gate through the MCP server's composition root; the gate is invoked during `nexus serve` before the server is exposed to clients.

## Failure, crash, and contention behavior

- Crash before copy: recovery removes the empty reservation or accepts an exact not-found result.
- Crash during copy or download: recovery removes the exact partial or complete temporary file.
- Crash after remote removal: exact not-found permits record deletion; a retry is idempotent because tokens and paths are never reused.
- Corruption, overflow, lock contention, ambiguous cleanup, or failed deletion retains the ownership row and blocks recovery and new acquisition.
- Operators must not force-delete rows to restore capacity. Investigate the exact profile, target binding, pin, and path under approved IBM i controls first.

## Least privilege, credentials, and logs

Use a read-oriented IBM i identity limited to approved libraries and source access. Credentials are retrieved through the approved native-store boundary and must never appear in source, SQLite, logs, audit records, argv, environment, fixtures, errors, or MCP results. Source content, tokens, target digests, and cursors are also excluded from logs and audit output. Redact before export and retain only necessary outcome metadata.

Nexus exposes no generic SSH, SQL, shell, or MCP recovery operation. The MCP server's typed input schemas forbid path, listing, or delete fields; recovery is an internal, exact-record lifecycle control, not an operator command surface.

## Direct IBM i onboarding

`nexus configure` persists only host and IBM i username in its Bubble Tea model.
The password is captured after the operator selects Connect and Save through a
fixed in-process terminal command. It is zeroed after handoff and never appears
in model state, messages, views, logs, audit, profile metadata, or files.

Automatic first contact accepts one unverified observed host key only under the
documented TOFU policy. The durable audit records `identity_bootstrap_allowed`
before authenticated proof and `identity_pin_committed` after persistence.
Missing or failed audit is fail-closed; a failed committed audit compensates the
new profile and native keyring credential. A changed or ambiguous existing pin is
rejected.

Native keyring support stores credentials only in the operating-system keyring.
Unsupported or unavailable capability selects prompt-on-use mode; a supported
keyring failure is not downgraded. WSS is preferred. Only an eligible
non-security WSS failure may receive an internal policy-bound fallback grant.
The grant creates bound `SSHConsent` and a single-use ticket consumed
immediately. Identity, trust, protocol, malformed-response, credential, and
other security failures never downgrade. This is not an operator choice and
exposes no generic SSH capability.

## Evidence and rollout limits

| Evidence | What it proves | What it does not prove |
|---|---|---|
| Go unit tests with fakes and temporary SQLite | Ordering, fail-closed contracts, exact ownership behavior, and bounded ledger handling | Live IBM i behavior or durability across power loss |
| Opt-in local SSH transport harness | Loopback-only pre-auth host-key observation and authenticated SSH/SFTP transport prerequisites | IBM i, Step 4 HTTPS `/version`, Mapepire `--single`, SQL, or complete Step 8 fallback |
| GitHub Actions | Available runner/package evidence, including available platform and cross-process evidence where recorded | Windows/macOS/Linux runtime behavior on runners that were not available |
| WDAC-constrained developer environment | A local runtime restriction that is not bypassed | A substitute runtime harness |

Available evidence uses CI, fakes, and temporary SQLite only. The release status is `ready_for_controlled_ibmi_validation` and `not_validated_on_ibmi`. There is no live IBM i validation, and automated evidence is not equivalent to field evidence. A live MCP client and a real IBM i remain an external rollout gate.

Rollout remains blocked on approved IBM i access, source/data classification, audit policy, corporate dependency and endpoint policy, signed/approved Nexus distribution, and an approved local database directory.

## Incident and rollback guidance

1. Stop the `nexus` process before changing recovery state.
2. Preserve the SQLite ownership record and collect redacted diagnostics; never copy source or credentials into an incident ticket.
3. Verify the exact recorded path, approved profile, target binding, and host-key pin using approved IBM i procedures.
4. If rollback is required, revert the MCP server slice (`cmd/nexus` and `internal/mcp`) and this document before reverting earlier recovery slices. Retain unresolved ownership rows; do not replace recovery with broad remote deletion.
