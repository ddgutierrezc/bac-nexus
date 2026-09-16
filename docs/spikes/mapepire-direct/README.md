# Direct Mapepire Spike Operator Guide

`mapepire-spike` is a disposable, non-production experiment for validating a direct Mapepire Go connection. It is not part of the `nexus` binary or production runtime wiring.

## Run immediately

Both commands collect an ephemeral configuration, validate it, and execute the spike immediately. They do not write a profile, `.env` file, keyring entry, cache, history, or any other configuration file.

```text
mapepire-spike configure
mapepire-spike run -host HOST -port 8076 -user USER -item ITEM -production-library LIBRARY
```

`configure` guides collection of the host, port, username, Catalogados item, and optional production library. The port defaults to `8076`; the production library defaults to empty.

`run` accepts only non-secret flags. A provided flag takes precedence over its approved environment variable. Missing values are requested interactively, using the same defaults.

## Credentials and TLS

The password is always requested through a hidden terminal prompt. It is never accepted from a flag, argument, or environment variable, and no password is persisted. Interactive terminal input is required; piped input or an unavailable terminal fails safely rather than exposing a visible password prompt.

**WARNING: TLS certificate verification is disabled unconditionally for this disposable spike. This creates a MITM risk.** This temporary, insecure behavior accepts the current untrusted or self-signed Mapepire Server certificate to test workplace viability. The CLI emits a prominent spike-only warning before it attempts a connection. There is no flag because this behavior is explicitly fixed for the temporary experiment.

This exception applies only to `cmd/mapepire-spike` and `internal/spikes/mapepiredirect`. It must not be copied into the production `nexus` transport or any other Nexus connector.

The only supported environment variables are non-secret:

```text
BAC_NEXUS_IBMI_HOST
BAC_NEXUS_IBMI_PORT
BAC_NEXUS_IBMI_USER
BAC_NEXUS_CATALOG_ITEM
BAC_NEXUS_CATALOG_PRODUCTION_LIBRARY
```

## Evidence and limits

RC3 first performs a bounded official-style control handshake over `wss://host:port/db/` with HTTP Basic authentication, `type=connect`, `technique=tcp`, and a non-sensitive application identity. It then runs the existing unofficial SDK proof only after that handshake succeeds.

The output is sanitized. `control_handshake: success cleanup=success` followed by `sdk: failure classification=connect` and `conclusion=unofficial_sdk_path_failed_after_control_success` is the decisive RC3 result: the daemon accepted the official-style control path, while the unofficial SDK connection failed. A control failure is classified as `tls_dial`, `http_upgrade`, `server_connect_rejection`, `malformed_protocol_response`, `timeout`, or `cleanup`; raw server errors are never printed. Successful execution continues with SDK, `VALUES 1`, parameterized Catalogados, pool reuse, concurrent reads, SDK limits, and shutdown. It never prints credentials, endpoint values, usernames, SQL, result rows, or IBM i job names.

The upstream SDK currently has no context-aware connect/query/pool wait, no custom CA or certificate-pinning support, and may lose a pool job on an error path. This spike intentionally bypasses its normal certificate verification through `IgnoreUnauthorized`; it is vulnerable to man-in-the-middle attacks and must remain disposable. Live execution remains a controlled manual step; this repository does not provide live IBM i proof.

Rollback is intentionally narrow: remove `cmd/mapepire-spike/`, `internal/spikes/mapepiredirect/`, and this guide, then run `go mod tidy` to remove dependencies no longer required after deleting the spike. No production `nexus` behavior, persisted configuration, or external system state is introduced by this spike.
