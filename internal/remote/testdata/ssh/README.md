# Local SSH Transport Harness

This opt-in Docker Compose harness verifies two `internal/remote` production boundaries: Step 3's pre-auth SSH host-key observation and the authenticated SSH/SFTP transport prerequisites used before Step 8.

It does **not** simulate IBM i, Step 4 HTTPS `/version`, Mapepire `--single`, SQL, or the complete Step 8 fallback. A successful harness run is not live IBM i evidence.

## Security model

The service binds only to loopback. It uses a generic test account (`transporttest`), accepts password authentication, disables sudo, and writes logs to stdout. The password is supplied as a Docker Compose secret from a file outside this repository; it is not present in source, Compose environment values, command arguments, or test output. The container has no persistent volume. Its generated host keys are ephemeral and rotate whenever the container is recreated, so rerun Step 3 observation before each authenticated test run.

The pinned image includes `nc`; the healthcheck uses `nc -z 127.0.0.1 2222` inside the container to prove SSH port readiness.

## Run

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
$env:BAC_NEXUS_SSH_INTEGRATION = "1"
```

### Portable shell

```sh
umask 077
secret_directory=$(mktemp -d)
secret_file="$secret_directory/password"
openssl rand -base64 32 > "$secret_file"
export BAC_NEXUS_SSH_TEST_PASSWORD_FILE="$secret_file"
export BAC_NEXUS_SSH_INTEGRATION=1
```

From the repository root, validate configuration, start the service, and run the opt-in test:

```sh
docker compose --project-name bac-nexus-ssh-harness -f internal/remote/testdata/ssh/compose.yaml config --quiet
docker compose --project-name bac-nexus-ssh-harness -f internal/remote/testdata/ssh/compose.yaml up -d --wait
go test -count=1 ./internal/remote -run TestSSHTransportHarnessObservesIdentityAndAuthenticatesSFTP
```

Always tear down the service and delete the external secret when finished:

```sh
docker compose --project-name bac-nexus-ssh-harness -f internal/remote/testdata/ssh/compose.yaml down --volumes --remove-orphans
rm -f "$BAC_NEXUS_SSH_TEST_PASSWORD_FILE"
rmdir "$(dirname "$BAC_NEXUS_SSH_TEST_PASSWORD_FILE")" 2>/dev/null || true
```

In PowerShell, replace the last two cleanup commands with:

```powershell
Remove-Item -LiteralPath $env:BAC_NEXUS_SSH_TEST_PASSWORD_FILE -Force
Remove-Item -LiteralPath (Split-Path -Parent $env:BAC_NEXUS_SSH_TEST_PASSWORD_FILE) -Force
```
