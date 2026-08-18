# Cataloged Sources live-capable transport spike

This bounded spike can resolve Catalogados metadata over SSH-hosted Mapepire 2.3.5 and retrieve one selected IBM i source member through SFTP. Every operation requires an explicit subcommand. Live access requires the explicit `live` subcommand and was **not executed against BAC or any other live host by the implementing agent**.

## Quick path

### 1. Run the interactive setup wizard

Obtain the expected host-key fingerprint through an approved channel before configuration. Do not derive or accept it during connection.

```text
go run ./cmd/catalogspike setup
```

The wizard asks, in order, for a connection name, host, SSH port (default 22), username, independently verified OpenSSH `SHA256:` host-key fingerprint, optional IBM i Java home, local Mapepire Server 2.3.5 JAR path, IBM i password, vault master passphrase and confirmation, and final approval. Fingerprint discovery and trust are separate from setup: Nexus never discovers, accepts, or auto-trusts a host key. The profile and pinned JAR checksum are validated before anything is persisted. Setup always creates a `vault` profile. Prompts and warnings use stderr; stdout contains exactly one secret-free JSON result. If that final write fails after both files commit, the error explicitly says setup committed, prohibits retrying the mutation, and directs the operator to query current state.

The granular profile command remains available. It stores only non-secret connection metadata, the local JAR path, and an explicit closed credential mode. Choose `vault` to require an encrypted vault or `prompt` to require a terminal-only IBM i password on every live run:

```text
go run ./cmd/catalogspike configure -name <PROFILE> -host <IBM_I_HOST> -port 22 -username <IBM_I_USER> -host-key-sha256 SHA256:<EXPECTED_FINGERPRINT> -mapepire-jar <LOCAL_MAPEPIRE_SERVER_2.3.5_JAR> -credential-mode <vault|prompt>
```

If the approved Java 8 installation differs from the bounded default `/QOpenSys/QIBM/ProdData/JavaVM/jdk80/64bit`, add:

```text
-java-home /QOpenSys/QIBM/ProdData/JavaVM/<APPROVED_JAVA_HOME>
```

Profiles contain no password. They are written to a same-volume temporary file, synchronized, and published through an atomic create-if-absent hard link as one strict JSON file per name under `os.UserConfigDir()/BAC Nexus/profiles` (for example `%AppData%\BAC Nexus\profiles` on Windows). Existing profiles are never overwritten, including concurrent saves; filesystems that cannot provide the no-replace link fail closed. Files and directories use restrictive permissions where the operating system supports POSIX permission bits. Profiles created before `credentialMode` was introduced fail closed and must be recreated explicitly; there is no silent migration to password prompting.

### 2. Manage encrypted credentials

```text
go run ./cmd/catalogspike credentials set -profile <PROFILE>
go run ./cmd/catalogspike credentials status -profile <PROFILE>
go run ./cmd/catalogspike credentials set -profile <PROFILE> -replace
go run ./cmd/catalogspike credentials delete -profile <PROFILE>
```

`set` prompts on a real terminal for the IBM i password and vault master passphrase; neither secret is accepted through arguments, environment variables, pipes, or plaintext files. Creation fails if a vault already exists. Rotation is explicit with `-replace` and uses a same-directory rollback link so an interrupted publication can recover the previous vault. If the new vault is committed but rollback-link cleanup fails, the command first writes one success JSON document with `cleanupPending: true`, then prints a secret-free warning to stderr, and retries cleanup on the next vault operation. A failure delivering either post-commit output returns a committed outcome that explicitly says not to retry the mutation and to query current status; normal command failures still exit with status 2. `status` reveals only whether the named vault exists. `delete` is deliberately idempotent and reports whether a file was removed. Commands never enumerate or decrypt unrelated profiles.

Vaults are stored under `os.UserConfigDir()/BAC Nexus/credentials/<PROFILE>.vault`. The strict version-1 JSON envelope contains only the explicit Argon2id tuple, random salt, random AES-GCM nonce, and ciphertext. Production decoding accepts exactly time=3, memory=65,536 KiB (64 MiB), threads=4, and key length=32 for version 1; weak, modified, or excessive envelope-selected work factors are rejected before Argon2 allocation. AES-256-GCM authentication binds the vault to the application, envelope version, and profile name, so copying or renaming a vault does not authenticate. JSON keys and unpadded base64 are canonical: duplicate, case-variant, unknown, whitespace-bearing, padded, or trailing representations fail closed.

The vault protects the IBM i password at rest, but it does not protect against malware or another process that can capture the master passphrase or process memory while unlocked. The master passphrase is required for every live run and is not stored. Therefore unattended MCP execution is not supported yet. Back up the profile and vault together; securely retain the master passphrase separately. Rotate with `credentials set -replace`; remove access with `credentials delete`. A lost master passphrase cannot be recovered.

### 3. Run the safe offline diagnostic

Offline diagnostics require the explicit `offline` subcommand. Root flags such as `catalogspike -item PISA061` are rejected with usage status 2:

```text
go run ./cmd/catalogspike offline -item <ITEM> -production-library <PRODUCTION_LIBRARY>
```

To verify a caller-supplied JAR without any network operation:

```text
go run ./cmd/catalogspike offline -item <ITEM> -mapepire-jar <LOCAL_MAPEPIRE_SERVER_2.3.5_JAR>
```

### 4. Run live only after approval

```text
go run ./cmd/catalogspike live -profile <PROFILE> -item <ITEM> -production-library <PRODUCTION_LIBRARY>
```

For a `vault` profile, `live` requires the matching vault, prompts without echo for its master passphrase, and decrypts the saved IBM i password. A missing vault fails closed and never downgrades to an IBM i password prompt. Only a profile explicitly created with `credential-mode prompt` uses the terminal-only IBM i password prompt. `-mapepire-jar` may explicitly override the non-secret configured JAR path. Secret byte slices are zeroed best-effort after use; the SSH library's required immutable password string is the unavoidable boundary. Secrets are never accepted from arguments, environment variables, configuration, pipes, source files, fixtures, or logs.

On an approved work computer, run `setup`, inspect the secret-free summary, run `offline` to confirm redacted diagnostics, then run `live` only after all target, identity, fingerprint, JAR-license, and data-exposure approvals are in place. Copying profile and vault files from another computer is supported only when organizational policy permits it; preserve both files and enter the same master passphrase on the destination.

## Ambiguous results and source selection

Zero candidates returns `not-found`. One candidate is selected automatically only when its item equals the requested item after case normalization; a sole substring-only match returns `not-exact` metadata and stops. Two through 50 candidates return `ambiguous` metadata and stop before source retrieval unless all exact coordinates are supplied:

```text
go run ./cmd/catalogspike live -profile <PROFILE> -item <ITEM> -mapepire-jar <LOCAL_JAR> -selector-library <SOURCE_LIBRARY> -selector-file-base <SOURCE_FILE_BASE> -selector-object-type <OBJECT_TYPE> -selector-source-type <SOURCE_TYPE>
```

More than 50 candidates is a typed limit error. A selector must exactly match a candidate returned by the same query; Nexus never guesses.

## Sensitive output policy

Default JSON reports classification, candidate coordinates, counts, byte/line limits, truncation, and remote cleanup. Offline query diagnostics expose only the stable `catalogados.search.v1` identifier, parameter count, row cap, and protocol message IDs/types; executable SQL and parameter values remain internal to live execution. Default output does **not** print source, credentials, raw SQL, parameter values, or remote stderr.

Source output requires an explicit warning-bearing opt-in and remains bounded:

```text
go run ./cmd/catalogspike live <REQUIRED_FLAGS> -show-source -max-bytes 1048576 -max-lines 10000
```

Treat this output as sensitive. Redirecting it creates a new copy outside Nexus control. Hard ceilings are 4 MiB and 50,000 lines.

## Fixed live behavior

| Area | Enforced behavior |
|---|---|
| SSH | `net.Dialer.DialContext` plus `ssh.NewClientConn`; 60-second operation deadline; connection/channel closure on cancellation |
| Host key | Exact configured OpenSSH `SHA256:` fingerprint; unknown or mismatch fails closed; no TOFU and no `InsecureIgnoreHostKey` |
| JAR | Caller supplies Mapepire Server 2.3.5; one opened bounded regular non-link descriptor is identity-checked, hashed, and uploaded with SHA-256 `41b1cfa67778ac204426f1dda0b51bd3f45fe3b89c91121d968660140acc0876` |
| Remote component | SFTP only, under `<authenticated-home>/.bac-nexus/components/mapepire/2.3.5/`; cryptographically random temporary paths are created exclusively, bytes are re-hashed, and a verified rollback copy preserves the prior artifact through activation and final verification |
| Code for IBM i | `.vscode` artifacts are never inspected, reused, copied, or deleted |
| Java | Fixed Code for IBM i 3.0.12-compatible environment and `-Dos400.stdio.convert=N -jar <NEXUS_JAR> --single`; no command fragments |
| SQL | Serialized `connect`, one fixed prepared Catalogados query, `sqlclose`, and `exit`; 51-row sentinel and 1 MiB frame cap |
| Source | Validated QSYS.LIB coordinates, one fixed `CPYTOSTMF` UTF-8 template, cryptographically random Nexus-owned `/tmp` path, bounded direct SFTP read, UTF-8 validation with rune-safe truncation, deferred deletion with joined primary/cleanup failures |

Mapepire errors expose typed SQLSTATE and SQL code when supplied, not raw server error text. The protocol mapping follows Mapepire Server 2.3.5 and `@ibm/mapepire-js` 0.6.1 response fields used by Code for IBM i 3.0.12.

## Prerequisites

- BAC approval for the target IBM i, identity, libraries, source exposure, and manual test window.
- Approval to upload and execute the GPL-3.0 Mapepire Server 2.3.5 JAR and use its local JTOpen/JDBC connection.
- An independently verified SSH host-key fingerprint.
- A caller-supplied JAR with the pinned checksum; Nexus does not download or bundle it.
- SSH/SFTP and the bounded Java 8 path available to the authenticated user.
- A read-oriented identity authorized for the Catalogados tables and selected source members.

## Manual live checklist

- [ ] Confirm the target, account, libraries, and data handling are approved.
- [ ] Confirm the host-key fingerprint through an independent approved channel.
- [ ] Confirm the local JAR version, checksum, and GPL handling approval.
- [ ] Run offline JAR verification first.
- [ ] Run setup or configure a new non-secret profile and encrypted vault; inspect that neither JSON file contains plaintext secrets.
- [ ] Run live without `-show-source`; verify typed candidate metadata only.
- [ ] If ambiguous, copy all selector coordinates from one returned candidate exactly.
- [ ] Confirm the remote JAR is only under `.bac-nexus/components/mapepire/2.3.5` with restrictive mode.
- [ ] Confirm the spike-owned `/tmp/bac-nexus-catalog-*.utf8` file was removed.
- [ ] Enable source output only when the destination and model exposure are approved.

## Acceptance and disqualifiers

Accept the transport only if strict fingerprint verification works on the approved IBM i, secrets remain terminal-only and encrypted at rest, the pinned JAR launches with the fixed environment, actual responses match the typed parser, ambiguity stops reads, source bounds hold, and every temporary source path is removed.

Disqualify it if the required IBM i algorithms are unavailable without enabling insecure SSH algorithms, any approval is denied, host keys cannot be verified independently, the JAR/hash differs, arbitrary SSH/SQL/CL input becomes reachable, source or stderr leaks by default, deadlines cannot stop the remote job, or cleanup cannot be proven.

## Verification scope

Normal tests use consumer-owned fakes and temporary directories. They perform no network operation. There is no automatic live integration test and no credentials are stored. Manual execution is the only live harness for this spike and must follow the checklist above.

The implementing agent executed only offline tests and temporary-profile CLI checks. **No live BAC test was executed.**

## Dependencies and licenses

| Module | Version | License |
|---|---:|---|
| `golang.org/x/crypto` | `v0.41.0` | BSD-3-Clause |
| `golang.org/x/term` | `v0.34.0` | BSD-3-Clause |
| `github.com/pkg/sftp` | `v1.13.9` | BSD-2-Clause |
| `github.com/kr/fs` (indirect) | `v0.1.0` | BSD-3-Clause |

The selected tags are compatible with the repository's Go 1.23 module baseline. Mapepire Server 2.3.5 is a separately supplied GPL-3.0 artifact and is not a Go dependency, downloaded asset, or repository file.
