# Nexus Code for IBM i Companion (Preview)

Nexus Code for IBM i Companion is an unofficial personal preview that provides a bounded local bridge to an active Code for IBM i session. It is not an organizational product or endorsement.

## Requirements

- Visual Studio Code 1.74 or later.
- Code for IBM i installed and connected.
- A local Nexus process.

## Install and use

1. Install this preview extension and Code for IBM i.
2. Connect Code for IBM i to the intended IBM i environment.
3. Run `nexus serve` without `-profile` to select the Companion.

The extension starts automatically after VS Code finishes starting.

Passing `-profile` selects Native instead. There is no fallback between the two modes.

## Security boundary

The bridge listens only on `127.0.0.1:64139` without authentication. Any process on the same machine can call its narrow local interface. Browser-origin requests are rejected before their bodies are parsed. Do not expose or forward this port.

## Capabilities and limits

The bridge supports only:

- `session_status` to report whether Code for IBM i is available and connected.
- One canonical proof query with a single bounded result.

It does not provide arbitrary SQL, commands, source access, credential access, remote listening, or fallback transport.

## Proof status

This preview was verified with offline checks. Live Extension Development Host validation requires Code for IBM i 3.0.12 and an active session, and has not been performed in this package environment.

## Troubleshooting

- Select the Nexus Companion status bar item or run `Nexus Companion: Show Diagnostics` to open a sanitized diagnostic snapshot.
- Confirm Code for IBM i is installed, enabled, and connected.
- Confirm no other process is using `127.0.0.1:64139`.
- Restart VS Code after installing or updating either extension.
- Run `nexus serve` without `-profile`; an explicit `-profile` selects Native and does not fall back to the Companion.
- If Nexus reports the Companion unavailable, verify that this extension activated successfully.

## Removal

Uninstall or disable this extension from VS Code. Restart VS Code to stop the local bridge.
