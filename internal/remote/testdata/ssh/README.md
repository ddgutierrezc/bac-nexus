# Local SSH Transport Harness

This opt-in Docker Compose harness provides two truthful `internal/remote` test modes.

- **Identity mode** is Step 3 pre-auth SSH host-key observation only. It needs no credential.
- **Authenticated override** proves the SSH/SFTP prerequisite before Step 8 with an external password secret.

Neither mode simulates Step 4 HTTPS `/version`, IBM i, Mapepire `--single`, SQL, or the full fallback. A successful harness run is not live IBM i evidence.

## Security model

The service binds only to loopback, disables password and sudo access in its base mode, and writes logs to stdout. It has no persistent volume, fixed container name, or restart policy. Its generated host keys are ephemeral and rotate whenever the container is recreated.

The authenticated override enables password access for the generic test account (`transporttest`). Its password is supplied as a Docker Compose secret from a file outside this repository; it is not present in source, Compose environment values, command arguments, or test output.

The pinned image includes `nc`; the healthcheck uses `nc -z 127.0.0.1 2222` inside the container to prove SSH port readiness.

## Identity mode (Step 3 pre-auth only)

From this directory, start the credential-free service:

```sh
docker compose up -d
```

Wait for or check the bounded healthcheck:

```sh
docker compose ps
docker compose up -d --wait
```

Run the credential-free identity integration test from the repository root:

```sh
BAC_NEXUS_SSH_IDENTITY_INTEGRATION=1 go test -count=1 ./internal/remote -run TestSSHTransportHarnessObservesIdentity
```

PowerShell equivalent:

```powershell
$env:BAC_NEXUS_SSH_IDENTITY_INTEGRATION = "1"
go test -count=1 ./internal/remote -run TestSSHTransportHarnessObservesIdentity
Remove-Item Env:BAC_NEXUS_SSH_IDENTITY_INTEGRATION
```

Tear down containers, networks, volumes, and orphans:

```sh
docker compose down --volumes --remove-orphans
```

## Authenticated override (SSH/SFTP prerequisite before Step 8)

Generate a random password file outside the repository. Do not print or paste its value.

### PowerShell

```powershell
$secretDirectory = Join-Path $env:TEMP "bac-nexus-ssh-harness"
New-Item -ItemType Directory -Force -Path $secretDirectory | Out-Null
$secretFile = Join-Path $secretDirectory "password"
$bytes = New-Object byte[] 32
[System.Security.Cryptography.RandomNumberGenerator]::Fill($bytes)
[System.IO.File]::WriteAllText($secretFile, [Convert]::ToBase64String($bytes), (New-Object System.Text.UTF8Encoding($false)))
$env:BAC_NEXUS_SSH_TEST_PASSWORD_FILE = $secretFile
```

### Portable shell

```sh
umask 077
secret_directory=$(mktemp -d)
secret_file="$secret_directory/password"
secret=$(openssl rand -base64 32)
printf %s "$secret" > "$secret_file"
export BAC_NEXUS_SSH_TEST_PASSWORD_FILE="$secret_file"
```

From this directory, validate the layered configuration and start the service:

```sh
docker compose -f compose.yaml -f compose.auth.yaml config --quiet
docker compose -f compose.yaml -f compose.auth.yaml up -d --wait
```

From the repository root, run the authenticated integration test:

```sh
BAC_NEXUS_SSH_INTEGRATION=1 go test -count=1 ./internal/remote -run TestSSHTransportHarnessAuthenticatesSFTP
```

PowerShell equivalent:

```powershell
$env:BAC_NEXUS_SSH_INTEGRATION = "1"
go test -count=1 ./internal/remote -run TestSSHTransportHarnessAuthenticatesSFTP
Remove-Item Env:BAC_NEXUS_SSH_INTEGRATION
```

Always tear down containers, networks, volumes, and orphans, then delete the external secret:

```sh
docker compose -f compose.yaml -f compose.auth.yaml down --volumes --remove-orphans
rm -f "$BAC_NEXUS_SSH_TEST_PASSWORD_FILE"
rmdir "$(dirname "$BAC_NEXUS_SSH_TEST_PASSWORD_FILE")" 2>/dev/null || true
```

In PowerShell, replace the last two cleanup commands with:

```powershell
Remove-Item -LiteralPath $env:BAC_NEXUS_SSH_TEST_PASSWORD_FILE -Force
Remove-Item -LiteralPath (Split-Path -Parent $env:BAC_NEXUS_SSH_TEST_PASSWORD_FILE) -Force
```
