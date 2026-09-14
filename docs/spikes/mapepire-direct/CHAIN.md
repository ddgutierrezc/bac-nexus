# Direct Mapepire Spike Delivery Chain

Issue #221 is delivered as a disposable, non-production experiment. It does not change the `nexus` binary, MCP tools, connector runtime, persistent configuration, or enterprise systems.

This tracker branch is the final integration point for the following review slices:

1. Direct connection and bounded Catalogados validation.
2. SDK-native pool reuse and bounded concurrent-read evidence.
3. Ephemeral command-line operation and the operator guide.

The completed spike is intentionally isolated. Its only purpose is to assess the upstream Mapepire Go SDK in a controlled manual environment; it is not an approved production connectivity design.

Rollback is equally isolated: hold or revert the relevant child slice, or remove the final `cmd/mapepire-spike/`, `internal/spikes/mapepiredirect/`, and spike documentation without affecting production Nexus behavior.
