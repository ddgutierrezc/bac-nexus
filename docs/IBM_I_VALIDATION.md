# IBM i Field Validation Runbook

## Status and scope

Release status: `ready_for_controlled_ibmi_validation`.

Validation status: `not_validated_on_ibmi`.

This is a manual, read-only rollout gate. GitHub Actions, fakes, loopback tests,
package checks, and local execution do not prove IBM i behavior and must not be
reported as live validation.

The only executable gate is `go test -tags=ibmi_integration ./integration/ibmi -run '^TestControlledNexusServe$' -count=1`, after the exact dedicated opt-in is set by the approved operator. Normal tests and CI exclude this tag.

## Prerequisites

The authorized operator must confirm, before transfer:

- approved operator identity and local OS principal;
- reachable IBM i at a supported version and approved read-only identity;
- approved libraries, source/data classification, policies, endpoint controls,
  validation window, MCP client, and local database directory;
- exact approved Nexus binary and matching `nexus.manifest.json`;
- exact profile, target, validation-window, `verified-readonly` policy, Mapepire 2.3.6 artifact, and approved configuration, audit, and ownership roots;
- no control bypass, credential sharing, or unapproved logging is permitted.

The gate validates every prerequisite before it starts the child process or permits remote work. Credentials remain exclusively in the native keyring; they are never supplied by argv, files, logs, or evidence.

## Controlled validation approval

The operator provisions one owner-only `0600` JSON record at exactly
`<UserConfigDir>/BAC Nexus/controlled-validation-approvals/<profile>.json`.
It is separate from serve eligibility and binds the current eligibility digest,
policy and target digests, UTC window, exact item/library, issued time, expiry,
and schema. Runtime environment values only request target/item/library; they
never approve them. The record is accepted only while all bindings and times
match. Revocation is fail-closed removal of that exact record; do not issue or
modify records through Nexus in this release. Roll back by stopping the gate,
removing the record, and retaining `not_validated_on_ibmi` until separately
approved field evidence exists.

## Binary and manifest verification

1. Transfer the complete offline ZIP through the approved channel.
2. Confirm the selected platform sidecar contains only schema version, release version, VCS
   revision, target, byte length, binary SHA-256, and the two status fields.
3. Extract the complete offline ZIP and run the local platform binary in place.
   Copying only `nexus` breaks its required shared Mapepire bundle topology.
4. Recompute SHA-256 and byte length locally; require exact equality with the
   sidecar and with the approved release record.
5. Run `nexus version` and require exact release-version and VCS-revision
   equality. A missing field, mismatch, dirty/unapproved revision, or path that
   is not the versioned target path aborts handoff.

Do not record command output, source paths, coordinates, host/user IDs, or
sensitive hashes in external evidence. The binary checksum is the sole allowed
hash in the handoff identity.

## Manual validation checklist

- [ ] Prerequisites and approval window confirmed.
- [ ] Binary, version, target, and checksum match.
- [ ] Start an approved read-only traversal at line 1.
- [ ] Continue pages through EOF; record only requested/returned counts and
      outcome classification, never source text.
- [ ] Classify empty, LF-terminated, and final-unterminated newline behavior.
- [ ] Complete one traversal and confirm cleanup.
- [ ] Cancel one traversal and confirm cancellation plus cleanup.
- [ ] Stop Nexus and invalidate leases after the checks.
- [ ] Confirm eligibility is revoked on abort and cleanup is limited to exact ledger-owned paths.
- [ ] Confirm no source appears in logs, evidence, attachments, or the release
      bundle; retain no source, paths, coordinates, credentials, raw errors,
      commands, SQL, cursors, or remote-cleanup details.

## Sanitized external evidence template

Store completed evidence only in the approved external system. Do not commit
completed evidence or attachments.

```text
status: ready_for_controlled_ibmi_validation
ibmi_status: not_validated_on_ibmi
checklist_complete: <yes|no>
binary_version: <approved version>
binary_sha256: <approved binary checksum only>
requested_count: <bounded integer>
returned_count: <bounded integer>
eof_reached: <yes|no|not-run>
newline_classification: <empty|lf-terminated|unterminated-final|not-run>
successful_cleanup: <confirmed|not-confirmed|not-run>
cancellation: <confirmed|not-confirmed|not-run>
cancellation_cleanup: <confirmed|not-confirmed|not-run>
retained_source: <none-confirmed|not-confirmed|not-run>
window: <approved-window classification only>
outcome: <pass|abort>
```

The template must never be expanded with source, paths, coordinates, host/user
IDs, credentials, raw errors, commands, SQL, cursors, sensitive hashes, or
remote cleanup details.

## Abort and rollback

Abort immediately on a failed prerequisite, identity mismatch, unexpected
result, unconfirmed cleanup, cancellation failure, or prohibited-data
exposure. Stop Nexus, invalidate leases, restore the approved binary and
configuration, revoke affected credentials, and request approved cleanup of
only exact recorded owned paths. Never broaden cleanup, delete ledger rows for
capacity, or retain cleanup details. Revoke serving eligibility before any
retry; release status remains `not_validated_on_ibmi` until separately approved
field evidence records success. Escalate through the approved incident process.
