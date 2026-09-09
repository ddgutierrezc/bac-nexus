# Changelog

## 0.2.6 - Pre-release

- Add authenticated fixed Catalogados resolution with bounded metadata candidates through the zero-touch active Code for IBM i session. Live IBM i validation remains separate.

## 0.2.5 - Pre-release

- Deliver zero-touch local Companion authentication with automatic 256-bit token rotation/private state, mandatory pre-dispatch checks, and immediate bounded 401 responses for incomplete unauthorized bodies. Requires the Nexus token-aware release; IBM i live validation and catalog/source tools are not included.
- Verify the packaged VSIX transitive local runtime-import closure before release use.

## 0.2.4 - Pre-release

- Preserve request correlation for program inspection responses. (#187)

## 0.2.3 - Pre-release

- Bind one Code for IBM i connection snapshot for each program-inspection attempt and report bounded preflight diagnostics. (#185)

## 0.2.2 - Pre-release

- Add privacy-safe SQL capability and program-resolution failure-stage diagnostics. (#183)

## 0.2.1 - Pre-release

- Include the complete emitted runtime import closure in packaged VSIX files.

## 0.2.0 - Pre-release

- Add `resolve_program` and metadata-only `find_program_source` for bounded IBM i program inspection.
- Report configured Code for IBM i library provenance and require an explicit choice for ambiguous program matches.
- Bind resolved programs to opaque, bounded selections; source-content reading remains unavailable.

## 0.1.1 - Pre-release

- Add a status bar diagnostic snapshot for Code for IBM i session availability.

## 0.1.0 - Pre-release

- Initial unofficial personal preview of the bounded local Code for IBM i Companion bridge.
