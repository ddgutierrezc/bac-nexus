# Architecture

## Mapepire boundary

The catalog boundary is deliberately split by ownership:

- `internal/catalog` defines semantic, validated catalog searches and domain candidates. It contains no SQL or transport behavior.
- `internal/connectors/ibmi/catalogados` owns the fixed BAC Catalogados SQL contract, parameter construction, hard row bound, IBM i fixed-width trimming, and row-to-candidate mapping.
- `internal/mapepire` is a standard-library-only, generic `mapepire-server --single` protocol client. It executes bounded parameterized requests and returns generic JSON rows; it has no Catalogados or IBM i launch knowledge.
- `internal/connectors/ibmi/mapepirestdio` owns the current IBM i SSH/stdio launch policy: the pinned 2.3.5 JAR, discovery and verification, artifact activation/rollback, Java safety rules, environment, and `--single` command shape.

Composition selects the current Mapepire session as the Catalogados executor. A future official `mapepire-go` adapter can replace that executor/session at composition without changing `internal/catalog`, `internal/app`, or the MCP contract. `cmd/nexus` intentionally remains unwired for live IBM i access in v1.
