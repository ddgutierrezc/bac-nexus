# Nexus Code for IBM i Companion (Preview)

Nexus Code for IBM i Companion is an unofficial personal preview that gives a local Nexus process bounded, read-only access to the active Code for IBM i session. It is not an organizational product or endorsement.

## Install and use

1. Install this preview extension and Code for IBM i in Visual Studio Code 1.74 or later.
2. Connect Code for IBM i to the intended IBM i environment.
3. Run `nexus serve` without `-profile` to select the Companion automatically.

The extension starts after VS Code finishes starting. There is no token, SQL, endpoint, path, or credential setup: Code for IBM i retains IBM i credentials. Passing `-profile` selects Native instead; Companion and Native do not fall back to one another.

## Local security boundary

The Companion listens only on `127.0.0.1:64139` and requires its private, automatically rotated 256-bit loopback token before request dispatch. Nexus reads that private state locally; the token is not an MCP input or output. Browser-origin requests are rejected, and the port must not be exposed or forwarded.

The Companion exposes no generic SQL, CL, QSH, shell, path, credential, mutation, remote-listening, endpoint-discovery, or forwarding capability. `sql_query` remains only the fixed bounded proof query, not a general SQL interface.

## Canonical Companion tools

| Tool | Bounded behavior |
|---|---|
| `session_status` | Reports Companion and Code for IBM i availability. |
| `sql_query` | Runs the one fixed, bounded proof query only. |
| `resolve_program` | Resolves supported `*PGM` metadata against configured Code for IBM i context, not runtime `*LIBL`. |
| `find_program_source` | Returns metadata-only source evidence for an opaque resolved-program selection. |
| `resolve_catalog_candidates` | Returns up to 50 ordered Catalogados candidates for a bounded query. |
| `read_selected_source` | Reads one bounded source page for an exact Catalogados selection. |

`read_selected_source` first requires the complete candidate selected from `resolve_catalog_candidates`; Nexus never chooses an ambiguous candidate. Later page requests accept only the opaque cursor. Cursors are capability values bound to the issuing Nexus process, exact selection, and client policy, rather than paths or reusable source coordinates.

## Source paging and lifecycle

Each source page requests one-based lines and is limited to 200 complete lines. The complete serialized page response, including protocol framing, is limited to 128 KiB; it never returns a partial line. Cursors are omitted at EOF, when Nexus disposes the source artifact.

The Companion invalidates and cleans up active artifacts when the Code for IBM i session changes or the extension deactivates. It also discards artifacts after source failures or an oversized response. Nexus cursor leases expire after inactivity and are unavailable outside their issuing process/session. Invalid, expired, ambiguous, unavailable, malformed, oversized, and cleanup-failure outcomes are deterministic and do not include source content.

## Diagnostics and validation status

Select the Nexus Companion status bar item or run `Nexus Companion: Show Diagnostics` for a sanitized diagnostic snapshot. It reports listener state, Code for IBM i availability and version, connection state, SQL capability, and the last operation failure stage without exposing credentials or source content.

This preview has offline protocol, bounds, package, and fake/loopback verification. It has not received live workplace IBM i or Extension Development Host validation; that requires separate authorization and an active approved environment.

## Removal

Uninstall or disable this extension from VS Code, then restart VS Code to stop the local bridge.
