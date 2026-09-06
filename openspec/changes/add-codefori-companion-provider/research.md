---
schema: gentle-ai.sdd-research/v1
revision: 1
change: add-codefori-companion-provider
outcome: blocked
artifact_store: openspec
skill_resolution: none
---

# Research: add-codefori-companion-provider

## Research request

### Questions

1. What exact public Code for IBM i 3.0.12 APIs support extension discovery, activation, instance access, connection status, event subscriptions, and SQL execution?
2. What stability and type-version compatibility warnings apply to that public API?
3. Which VS Code extension host must run the Companion, including under WSL, SSH, and development containers?
4. What restrictions are necessary before forwarding an MVP SQL request to `runSQL`, including handling of functions, CTEs, multiple statements, and CL input?
5. What does the available evidence establish—and not establish—about concurrent `runSQL` safety and ten-way request handling?
6. Which local transport, authentication, and discovery choices can provide bounded, fail-closed host-local communication?
7. What is the current trusted-publishing workflow for `@vscode/vsce`, including GitHub Actions permissions and Marketplace policy?

### Requested source classes

- Official documentation
- Open-web release, tagged-source, package-registry, and publisher documentation

### Requested references retained without validation

- https://codefori.github.io/docs/dev/api/
- https://codefori.github.io/docs/dev/examples/
- https://github.com/codefori/vscode-ibmi/releases/tag/3.0.12
- Code for IBM i tag `3.0.12` files requested by the orchestrator: `src/Instance.ts`, `src/api/IBMi.ts`, `src/extension.ts`, `src/typings.ts`, and `package.json`
- https://registry.npmjs.org/@halcyontech/vscode-ibmi-types/latest
- https://code.visualstudio.com/api/advanced-topics/remote-extensions
- https://code.visualstudio.com/api/advanced-topics/extension-host
- https://raw.githubusercontent.com/microsoft/vscode-vsce/main/README.md

These are retained request inputs only. They were not admitted as evidence and support no claim in this artifact.

## Admission

```yaml
admitted: false
reason: >-
  This executor has no evidence grants. Supplied URLs and asserted facts cannot be
  validated or admitted as evidence in this runtime.
observed_exact_grants:
  documentation: []
  open-web: []
```

## Sources

```yaml
sources: []
```

No source was accessed or validated for evidence.

## Validated claims

```yaml
claims: []
```

No evidence claim is emitted. In particular, the supplied API signatures, release/version details, extension-host behavior, SQL behavior, concurrency observations, and publishing instructions remain unvalidated in this research phase.

## Outcome

Research is blocked by admission denial. The exploration at `openspec/changes/add-codefori-companion-provider/exploration.md` was read as required, but its external-evidence statements were not promoted into research claims. Product choices—including protocol, bearer capability, descriptor permissions, SQL grammar, concurrency serialization, and publishing configuration—remain non-authoritative and pending. Proposal readiness is false.
