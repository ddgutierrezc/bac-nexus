# Cataloged Sources live-capable transport spike

This bounded spike can resolve Catalogados metadata over SSH-hosted Mapepire 2.3.5 and retrieve one selected IBM i source member through SFTP. Every operation requires an explicit subcommand. Live access requires the explicit `live` subcommand and was **not executed against BAC or any other live host by the implementing agent**.

## Quick path

### 1. Run the interactive setup wizard

For production, obtain the expected host-key fingerprint through an approved independent channel before configuration. For this spike only, setup can inspect the key presented by the current SSH connection and enroll it explicitly as trust on first use (TOFU).

```text
go run ./cmd/catalogspike setup
```

After validating the host and port, the wizard recommends `manual` host-key enrollment from an independently verified fingerprint. `inspect` remains a spike-only TOFU fallback before the wizard asks for the IBM i password or vault master passphrase. It makes one no-auth SSH handshake with a harmless probe username, captures the key during key exchange, prints its algorithm and OpenSSH `SHA256:` fingerprint to stderr, and aborts before authentication. It clearly identifies the result as current-connection discovery rather than independent verification. Enrollment then requires the entered line content to be byte-for-byte exact lowercase `yes`; leading or trailing spaces or tabs, case variants, Enter, `no`, probe failure, or any other input aborts before credentials and leaves no profile or vault. Accepted inspection records `hostKeyTrust: "tofu"`. `manual` requires a canonical independently verified fingerprint and records `hostKeyTrust: "verified"`.

The wizard then asks for username and optional IBM i Java home. Before any secret prompt, it checks only the current user's standard VS Code `.vscode/extensions` directory and only the exact, case-sensitive Code for IBM i **3.0.12** directory basename `halcyontechltd.code-for-ibmi-3.0.12` with the exact relative file `dist/mapepire-server-2.3.5.jar`. Before traversal, every existing path component through the extensions root and matched extension directory must pass `os.Lstat` without `ModeSymlink`; links are rejected rather than resolved and accepted. It automatically selects the JAR only when the canonical candidate passes the existing regular non-link, 64 MiB, stable-file, and pinned SHA-256 verifier. No candidate, a rejected exact-location candidate, or an inaccessible or linked inspection path produces a sanitized explanation and the existing `Local Mapepire Server 2.3.5 JAR path:` prompt. Target-platform suffix directories, VS Code Insiders, and custom extension locations deliberately use this manual fallback. The manual path must be absolute and is fully verified before the IBM i password or vault master passphrase is requested. No home-directory recursion, repository search, download, extraction, embedding, or administrator access is involved.

Ordinary non-secret fields continue to trim surrounding whitespace. The profile and pinned JAR checksum are validated before anything is persisted. Setup always creates a `vault` profile. It creates the encrypted vault first and publishes the profile last as the commit marker, so an existing setup profile always has its vault. If profile publication fails, setup deletes the new vault; if deletion also fails, a typed secret-free error identifies the orphan by profile name for recovery through `credentials status` and `credentials delete`, while no profile is published. Prompts and warnings use stderr; stdout contains exactly one secret-free JSON result. If that final write fails after both files commit, the error explicitly says setup committed, prohibits retrying the mutation, and directs the operator to query current state. The `live` command never inspects or fetches a key: it always verifies the exact pinned fingerprint, regardless of trust provenance.

The granular profile command remains available. It stores only non-secret connection metadata, the local JAR path, and an explicit closed credential mode. Choose `vault` to require an encrypted vault or `prompt` to require a terminal-only IBM i password on every live run:

```text
go run ./cmd/catalogspike configure -name <PROFILE> -host <IBM_I_HOST> -port 22 -username <IBM_I_USER> -host-key-sha256 SHA256:<EXPECTED_FINGERPRINT> -host-key-trust <tofu|verified> -mapepire-jar <LOCAL_MAPEPIRE_SERVER_2.3.5_JAR> -credential-mode <vault|prompt>
```

If the approved Java 8 installation differs from the bounded default `/QOpenSys/QIBM/ProdData/JavaVM/jdk80/64bit`, add:

```text
-java-home /QOpenSys/QIBM/ProdData/JavaVM/<APPROVED_JAVA_HOME>
```

Profiles contain no password. They are written to a same-volume temporary file, synchronized, and published through an atomic create-if-absent hard link as one strict JSON file per name under `os.UserConfigDir()/BAC Nexus/profiles` (for example `%AppData%\BAC Nexus\profiles` on Windows). Existing profiles are never overwritten, including concurrent saves; filesystems that cannot provide the no-replace link fail closed. Files and directories use restrictive permissions where the operating system supports POSIX permission bits. Profiles created before `credentialMode` or `hostKeyTrust` was introduced fail closed and must be recreated explicitly; there is no silent migration to password prompting or an assumed trust level. Unknown and case-variant trust values are rejected.

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

On an approved work computer, use this exact sequence:

1. Run `go run ./cmd/catalogspike setup`.
2. Prefer `manual` and enter a fingerprint obtained through an approved independent channel. For spike-only first contact, choose `inspect`, compare the displayed algorithm and fingerprint through an independent channel when available, understand that the current connection could be intercepted, and type exact lowercase `yes` only if accepting that TOFU risk.
3. Inspect the secret-free setup summary and stored profile trust provenance.
4. Run `go run ./cmd/catalogspike offline -item <ITEM> -mapepire-jar <STORED_OR_MANUAL_LOCAL_JAR>` to verify redacted diagnostics and the pinned JAR without network access.
5. Run `live` only after all target, identity, fingerprint, JAR-license, and data-exposure approvals are in place. Production use requires recreating enrollment from an independently verified fingerprint; TOFU discovery alone is insufficient.

On the current Windows work computer, `ssh-keyscan` can reach an OpenSSH 9.6 server yet fail locally because that OpenSSH build does not support the server's preferred `sntrup761x25519-sha512@openssh.com` post-quantum key exchange. That failure is not evidence that the server requires legacy SHA-1. Go `x/crypto/ssh` and Code for IBM i use different SSH stacks and can negotiate another mutually supported secure key exchange, such as Curve25519, when offered. The probe uses only `ssh.SupportedAlgorithms()` from the pinned Go dependency, excludes every `ssh.InsecureAlgorithms()` value, and never retries with weak algorithms.

Copying profile and vault files from another computer is supported only when organizational policy permits it; preserve both files and enter the same master passphrase on the destination.

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
| Host key | Setup-only no-auth inspection can explicitly enroll TOFU; manual enrollment records independent verification; live always requires the exact pinned OpenSSH `SHA256:` fingerprint; unknown or mismatch fails closed; no `InsecureIgnoreHostKey` |
| JAR | Setup may discover the separately installed Code for IBM i 3.0.12 file only below the exact standard extension basename at `dist/mapepire-server-2.3.5.jar`; otherwise the caller supplies an absolute path. Discovery rejects path components exposed by portable `os.Lstat` as `ModeSymlink` instead of resolving them. One opened bounded regular non-link descriptor is identity-checked, hashed, and uploaded with SHA-256 `41b1cfa67778ac204426f1dda0b51bd3f45fe3b89c91121d968660140acc0876` |
| Remote component | SFTP only, under `<authenticated-home>/.bac-nexus/components/mapepire/2.3.5/`; cryptographically random temporary paths are created exclusively, bytes are re-hashed, and a verified rollback copy preserves the prior artifact through activation and final verification |
| Code for IBM i | Setup performs one bounded, read-only inspection of the current user's standard `.vscode/extensions` root and exact extension/JAR locations; target-platform suffixes, Insiders, and custom locations require a manual absolute path; the JAR is never copied into, embedded in, downloaded by, or deleted by the repository |
| Java | Fixed Code for IBM i 3.0.12-compatible environment and `-Dos400.stdio.convert=N -jar <NEXUS_JAR> --single`; no command fragments |
| SQL | Serialized `connect`, one fixed prepared Catalogados query, `sqlclose`, and `exit`; 51-row sentinel and 1 MiB frame cap |
| Source | Validated QSYS.LIB coordinates, one fixed `CPYTOSTMF` UTF-8 template, cryptographically random Nexus-owned `/tmp` path, bounded direct SFTP read, UTF-8 validation with rune-safe truncation, deferred deletion with joined primary/cleanup failures |

Mapepire errors expose typed SQLSTATE and SQL code when supplied, not raw server error text. The protocol mapping follows Mapepire Server 2.3.5 and `@ibm/mapepire-js` 0.6.1 response fields used by Code for IBM i 3.0.12.

## Prerequisites

- BAC approval for the target IBM i, identity, libraries, source exposure, and manual test window.
- Approval to upload and execute the GPL-3.0 Mapepire Server 2.3.5 JAR and use its local JTOpen/JDBC connection.
- An independently verified SSH host-key fingerprint for production. Spike-only TOFU enrollment is explicitly labeled and is not independent verification.
- A separately installed or caller-supplied JAR with the pinned checksum; Nexus does not download, bundle, embed, decompress, or license it.
- SSH/SFTP and the bounded Java 8 path available to the authenticated user.
- A read-oriented identity authorized for the Catalogados tables and selected source members.

## Manual live checklist

- [ ] Confirm the target, account, libraries, and data handling are approved.
- [ ] Confirm the host-key fingerprint through an independent approved channel; do not treat TOFU provenance as production verification.
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

Normal tests use consumer-owned fakes, temporary directories, and an in-process loopback SSH server that proves key capture happens during key exchange before authentication. They perform no external network operation. There is no automatic live integration test and no credentials are stored. Manual execution is the only live-host harness for this spike and must follow the checklist above.

The implementing agent executed only offline tests and temporary-profile CLI checks. **No live BAC test was executed.**

If setup cannot auto-discover the verified file, confirm Code for IBM i is exactly version 3.0.12 and inspect only `<HOME>/.vscode/extensions/halcyontechltd.code-for-ibmi-3.0.12/dist/mapepire-server-2.3.5.jar`. A checksum/type/read/link rejection is intentionally sanitized; independently calculate SHA-256 and require `41b1cfa67778ac204426f1dda0b51bd3f45fe3b89c91121d968660140acc0876`, then provide the absolute path. Target-platform suffixes, Insiders, and custom extension roots are unsupported by automatic discovery and require that manual path. Portable link rejection is limited to reparse points that the Go runtime exposes through `os.Lstat` as `ModeSymlink`; exotic Windows reparse tags not surfaced that way remain an operating-system/runtime limitation. The candidate JAR leaf is still canonicalized and checked by the descriptor-based verifier.

## Dependencies and licenses

| Module | Version | License |
|---|---:|---|
| `golang.org/x/crypto` | `v0.41.0` | BSD-3-Clause |
| `golang.org/x/term` | `v0.34.0` | BSD-3-Clause |
| `github.com/pkg/sftp` | `v1.13.9` | BSD-2-Clause |
| `github.com/kr/fs` (indirect) | `v0.1.0` | BSD-3-Clause |

The selected tags are compatible with the repository's Go 1.23 module baseline. Mapepire Server 2.3.5 is a separately supplied GPL-3.0 artifact and is not a Go dependency, downloaded asset, or repository file.
