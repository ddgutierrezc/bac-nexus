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

TLS always uses system CA verification. There is no insecure-TLS option. An endpoint that cannot be verified by the system trust store must fail.

The only supported environment variables are non-secret:

```text
BAC_NEXUS_IBMI_HOST
BAC_NEXUS_IBMI_PORT
BAC_NEXUS_IBMI_USER
BAC_NEXUS_CATALOG_ITEM
BAC_NEXUS_CATALOG_PRODUCTION_LIBRARY
```

## Evidence and limits

Successful execution emits sanitized stage evidence: connection with `tls=system-ca`, `VALUES 1`, the parameterized Catalogados check, pool reuse, concurrent reads, SDK limits, and shutdown. It does not print credentials, endpoint values, usernames, SQL, result rows, or IBM i job names.

The upstream SDK currently has no context-aware connect/query/pool wait, no custom CA or certificate-pinning support, and may lose a pool job on an error path. Live execution remains a controlled manual step; this repository does not provide live IBM i proof.

Rollback is intentionally narrow: remove `cmd/mapepire-spike/` and this guide. No production `nexus` behavior, persisted configuration, or external system state is introduced by this spike.
