# BAC Nexus — Agent Context & Engineering Guide

> **Status:** Discovery / PoC  
> **Product name:** BAC Nexus (working name)  
> **Primary language:** Go  
> **Primary integration protocol:** Model Context Protocol (MCP)  
> **Initial ecosystem:** IBM i  
> **Long-term scope:** BAC technology ecosystem  
> **Repository classification:** Internal / proprietary

---

## 1. Why this file exists

This repository is being developed with AI coding agents. This file provides the persistent context, product intent, engineering constraints, and working rules that every agent must understand before proposing or implementing changes.

Before writing code:

1. Read this file completely.
2. Inspect the current repository state.
3. Read any relevant documents under `docs/`, ADRs, specifications, and skills.
4. Do not assume that ideas described here are already implemented.
5. Distinguish clearly between:
   - current behavior,
   - approved design,
   - proposed design,
   - future vision.
6. For substantial changes, summarize your understanding and proposed approach before implementation.

The purpose of this document is not to freeze the architecture. It is to preserve product intent while allowing the architecture to evolve deliberately.

---

## Project skills

- `.agents/skills/bac-nexus-tui/SKILL.md` — Trigger: BAC Nexus TUI, wizard step/Step 4, Bubble Tea, Bubbles, or Lip Gloss work. This documents project intent; OpenCode auto-discovers project-local `.agents/skills/*/SKILL.md`.

---

# 2. Product vision

## BAC Nexus

BAC Nexus is intended to become a **secure enterprise context layer between AI agents and BAC's technology ecosystem**.

The product should allow AI agents such as GitHub Copilot, OpenCode, and other MCP-compatible agents to discover and retrieve the technical context they need from enterprise systems without requiring developers to manually collect and paste that context into every conversation.

The first major use case is IBM i.

Conceptually:

```text
AI Agents
   │
   │ MCP
   ▼
BAC Nexus
   │
   ├── IBM i Connector
   ├── Git / Repository Connector
   ├── Documentation Connector
   ├── Database Connector
   ├── DevOps Connector
   └── Future Enterprise Connectors
```

IBM i is the **starting point**, not the architectural limit.

---

# 3. Initial problem

Developers working with IBM i often need to manually gather context before an AI assistant can understand an application.

For example, a developer may ask:

> "Explain how program PISA061 works."

The agent may need information such as:

- the compiled IBM i object,
- its object type and attribute,
- its source library,
- source physical file,
- source member,
- referenced programs,
- referenced files,
- service programs,
- bound modules,
- imported/exported procedures,
- related source members,
- library-list-dependent resolution,
- and relevant downstream dependencies.

Today, that context may need to be gathered manually.

BAC Nexus should progressively automate that discovery.

---

# 4. Desired user experience

The eventual experience should be close to:

```text
Developer:
"Explain how PISA061 works."

Agent:
→ asks Nexus to resolve the object
→ asks Nexus for source metadata
→ reads only the relevant source
→ discovers program references
→ inspects bound modules/service programs when necessary
→ follows only the dependencies needed for the question
→ synthesizes the result
```

The developer should not need to manually paste an entire IBM i application into the model context.

The goal is **progressive, agent-driven context retrieval**.

---

# 5. Product principles

## 5.1 Agent-agnostic

BAC Nexus must not be architecturally coupled to GitHub Copilot.

GitHub Copilot is an important initial client, but Nexus should be usable by any compatible agent or client.

Examples may include:

- GitHub Copilot
- OpenCode
- Codex
- other MCP clients
- future BAC internal agents

MCP should serve as a stable integration boundary where appropriate.

---

## 5.2 Ecosystem-agnostic core

IBM i is the first connector.

The core architecture should make it possible to add future connectors without rewriting the product.

Potential future ecosystems include:

- GitHub
- GitLab
- Azure DevOps
- SharePoint
- Confluence
- Jira
- ServiceNow
- SQL Server
- Oracle
- OpenShift
- Kubernetes
- internal APIs
- enterprise documentation systems

Do not introduce these integrations prematurely. They are architectural direction, not v1 scope.

---

## 5.3 Secure by design

BAC Nexus is intended for an enterprise/banking environment.

Security is a product requirement, not an afterthought.

Default principles:

- least privilege,
- explicit allowlists,
- narrow MCP tools,
- no arbitrary remote shell exposed to agents,
- no unrestricted command execution,
- no credentials in source code,
- no secrets in logs,
- auditable access,
- bounded results,
- predictable errors,
- fail closed when authorization or resolution is ambiguous.

For IBM i specifically, the preferred model is:

```text
Agent
   ↓
Nexus MCP Tool
   ↓
Nexus authorization / validation
   ↓
IBM i connector
   ↓
controlled SQL / APIs / read operations
   ↓
IBM i
```

Avoid a design equivalent to:

```text
LLM → arbitrary ssh_exec(command) → IBM i
```

even if the backing identity is expected to be read-only.

---

## 5.4 Read-oriented first

The initial PoC should focus on **context discovery and inspection**, not modification of enterprise systems.

Examples of appropriate early capabilities:

- resolve object,
- inspect object metadata,
- locate source member,
- read bounded source ranges,
- search source,
- inspect program references,
- inspect bound modules,
- inspect service programs,
- inspect imports/exports,
- build a bounded dependency view.

Mutation capabilities are out of scope unless explicitly designed and approved later.

---

## 5.5 On-demand context, not context dumping

Do not retrieve an entire application when a small subset is sufficient.

Prefer:

```text
question
  ↓
resolve root object
  ↓
retrieve immediate context
  ↓
agent decides what is relevant
  ↓
retrieve additional context only when needed
```

This reduces:

- model context usage,
- latency,
- noise,
- accidental exposure,
- and unnecessary load on enterprise systems.

---

## 5.6 Trustworthy uncertainty

Nexus must not pretend that static analysis can always determine runtime behavior.

Examples of uncertainty include:

- IBM i `*LIBL` resolution,
- dynamically constructed program names,
- dynamic SQL,
- dynamically generated commands,
- runtime overrides,
- configuration-driven dispatch.

When Nexus cannot resolve something reliably, return the ambiguity explicitly.

Do not guess.

---

# 6. IBM i initial connector

The IBM i connector is the first concrete implementation target.

Potential sources of information include IBM i SQL services, system APIs, source members, and controlled commands where necessary.

Likely capabilities to investigate include:

- object resolution,
- object/source metadata,
- program references,
- bound modules,
- bound service programs,
- program import/export information,
- source member access,
- source search,
- library list awareness.

The implementation must be validated against the IBM i versions supported by BAC.

Do not assume that a service available in one IBM i release exists identically in another.

---

# 7. MCP tool philosophy

MCP tools should represent **business/technical capabilities**, not raw infrastructure primitives.

Prefer tools such as:

```text
resolve_object
read_source_member
search_source
get_program_references
get_bound_modules
get_bound_service_programs
get_program_imports_exports
get_dependency_tree
```

Avoid tools such as:

```text
ssh_exec
run_any_command
execute_sql
run_cl
shell
```

A narrow tool is easier to:

- secure,
- authorize,
- audit,
- test,
- document,
- reason about,
- and expose safely to an autonomous agent.

Tool input and output contracts should be deterministic and typed.

Errors should be useful to an agent and should distinguish cases such as:

- not found,
- ambiguous,
- unauthorized,
- unsupported,
- unavailable,
- malformed request,
- connector failure.

---

# 8. Language and implementation direction

BAC Nexus should be implemented primarily in **Go**.

Reasons include:

- simple deployment,
- strong concurrency primitives,
- good support for CLI/server software,
- cross-platform builds,
- static binaries,
- strong testing support,
- alignment with the architecture patterns being studied from Engram and Gentle AI.

Use the **official Model Context Protocol Go SDK** when practical.

Do not introduce an alternative MCP framework without a documented reason.

---

# 9. Reference projects

BAC Nexus should study and borrow good engineering patterns from:

## Engram — Gentleman Programming

Useful patterns to study:

- single Go binary where appropriate,
- explicit package ownership,
- `cmd/<binary>` as composition/entrypoint,
- `internal/` packages for implementation boundaries,
- isolated MCP layer,
- CLI / MCP / TUI as separate interfaces over shared core behavior,
- deterministic MCP contracts,
- architecture documentation,
- repository maps,
- agent skills,
- tests located near behavior,
- issue-first development,
- conventional commits,
- documentation updated with behavior.

Do **not** blindly clone Engram's architecture.

BAC Nexus is a different product with different security, integration, and enterprise requirements.

Borrow principles, not accidental implementation details.

---

## Gentle AI — Gentleman Programming

Useful patterns to study:

- Go CLI/TUI implementation,
- clear architecture documentation,
- extensive package-level tests,
- E2E coverage where useful,
- golden fixtures for deterministic generated output,
- explicit contribution workflow,
- Conventional Commits,
- structured large-feature workflow,
- reviewable change sizes,
- verification before merge.

Again, use these as engineering references rather than requirements to reproduce the repository exactly.

---

# 10. Proposed repository architecture

This structure is a starting hypothesis and may evolve through ADRs.

```text
bac-nexus/
├── cmd/
│   └── nexus/
│       └── main.go
│
├── internal/
│   ├── app/                 # runtime composition / orchestration
│   ├── mcp/                 # MCP server and tool adapters
│   ├── core/                # product-level interfaces and domain behavior
│   ├── connectors/
│   │   ├── ibmi/            # IBM i implementation
│   │   └── ...
│   ├── security/            # policy, authorization, allowlists
│   ├── audit/               # audit events and sinks
│   ├── config/              # configuration loading/validation
│   └── tui/                 # optional future TUI
│
├── docs/
│   ├── PRODUCT.md
│   ├── ARCHITECTURE.md
│   ├── SECURITY.md
│   └── adr/
│
├── skills/                  # optional agent skills as the repo grows
├── testdata/
├── .github/
│   ├── workflows/
│   └── instructions/
│
├── AGENTS.md
├── CONTRIBUTING.md
├── README.md
├── go.mod
└── go.sum
```

Do not create all directories merely because they appear above.

Create structure only when the behavior requiring it exists.

---

# 11. Package ownership principles

## `cmd/nexus`

Responsibilities:

- CLI parsing,
- application startup,
- dependency wiring,
- process lifecycle.

Should not contain:

- IBM i query logic,
- MCP business rules,
- authorization rules,
- dependency discovery algorithms.

---

## `internal/core`

Responsibilities:

- stable product abstractions,
- connector-neutral domain concepts,
- orchestration rules that belong to Nexus rather than a specific transport.

Should not depend directly on:

- GitHub Copilot,
- OpenCode,
- terminal UI concerns,
- IBM i implementation details.

---

## `internal/mcp`

Responsibilities:

- expose Nexus capabilities as MCP tools,
- validate MCP-facing request/response contracts,
- translate MCP calls into core operations,
- map domain errors into deterministic tool results.

Should not:

- implement IBM i transport directly,
- duplicate connector logic,
- contain arbitrary shell execution.

---

## `internal/connectors/ibmi`

Responsibilities:

- IBM i connectivity,
- IBM i-specific query/API behavior,
- IBM i object semantics,
- IBM i result normalization.

The rest of Nexus should depend on interfaces rather than IBM i implementation details whenever practical.

---

## `internal/security`

Responsibilities may eventually include:

- allowlists,
- authorization decisions,
- policy evaluation,
- access boundaries,
- connector/tool restrictions.

Security rules must be enforced in code, not only documented in prompts.

---

## `internal/audit`

Responsibilities may eventually include structured records such as:

- tool invoked,
- connector,
- target system,
- target object/resource,
- timestamp,
- result classification,
- duration.

Do not log secrets or unnecessary source content.

---

# 12. Engineering rules

## Keep interfaces narrow

Prefer small interfaces owned by the consumer.

Avoid creating large generic abstractions prematurely.

---

## Separate transport from domain behavior

MCP is an interface to Nexus, not Nexus itself.

SSH is a possible transport to an IBM i environment, not a product capability.

IBM i SQL is an implementation mechanism, not an MCP API design.

---

## Prefer typed contracts

Use explicit Go structs for tool input/output.

Avoid loosely typed `map[string]any` when a stable schema is known.

---

## Errors are part of the API

Errors should be deterministic enough that an agent can decide what to do next.

Prefer structured error categories over parsing human prose.

---

## Context cancellation

Any network, SSH, SQL, or potentially long-running operation should respect `context.Context`.

---

## Timeouts and bounds

Remote calls must have sensible:

- timeouts,
- maximum result sizes,
- maximum traversal depth,
- pagination/limits where appropriate.

An agent should never be able to accidentally request an unbounded traversal of an enterprise environment.

---

## Dependency traversal

Dependency discovery should be bounded.

Example:

```text
get_dependency_tree(object, depth=2, max_nodes=100)
```

Do not assume a graph database is required for the PoC.

A graph is a useful conceptual model even if dependencies are resolved on demand.

Persistent indexing/graph storage should only be introduced when measurements justify it.

---

# 13. Testing philosophy

Testing is required for behavior, not optional cleanup.

Follow the spirit of Engram and Gentle AI: behavior changes should ship with verification.

## Unit tests

Every package with meaningful behavior should have package-level tests.

Use table-driven tests where they improve clarity.

Run:

```bash
go test ./...
```

---

## Race detection

For concurrent behavior, run:

```bash
go test -race ./...
```

where supported by the development/CI environment.

---

## Static validation

At minimum:

```bash
gofmt
go vet ./...
```

Consider additional approved tooling only when it provides clear value.

---

## MCP contract tests

Every MCP tool should have tests for:

- valid request,
- malformed request,
- not found,
- ambiguous result,
- unauthorized result,
- connector error,
- deterministic response shape.

MCP tests should not require a real IBM i unless explicitly marked as integration tests.

---

## Connector tests

IBM i behavior should be tested behind interfaces.

Prefer:

- fixtures,
- fakes,
- deterministic recorded data where policy allows,
- package-level integration seams.

Do not make the normal unit test suite depend on corporate network connectivity.

---

## Real IBM i integration tests

Tests against a real IBM i should be:

- explicitly enabled,
- non-destructive,
- read-oriented,
- bounded to approved environments,
- skipped by default when configuration is absent.

Never place real credentials in the repository.

---

## Regression tests

A bug fix should include a regression test whenever the defect can reasonably be reproduced in automation.

---

# 14. Development workflow

Use a disciplined workflow inspired by Engram and Gentle AI.

For non-trivial work:

```text
Issue / problem
    ↓
Explore
    ↓
Proposal
    ↓
Design / ADR when needed
    ↓
Implementation
    ↓
Tests
    ↓
Documentation
    ↓
Verification
```

Large features should follow a Spec-Driven Development style:

```text
explore → propose → spec → design → implement → verify
```

Small and obvious changes should not be burdened with unnecessary ceremony.

---

# 15. Agent behavior rules

When acting as a coding agent in this repository:

## Before implementation

- inspect existing code first,
- locate relevant tests,
- identify affected architecture boundaries,
- state important assumptions,
- identify unresolved product decisions,
- do not invent infrastructure that does not exist.

## During implementation

- make the smallest coherent change,
- keep security boundaries explicit,
- keep IBM i logic inside the connector,
- keep MCP logic transport-facing,
- avoid speculative abstractions,
- add/update tests with behavior,
- update relevant docs when visible behavior changes.

## After implementation

Report:

1. what changed,
2. why,
3. files/packages affected,
4. tests executed,
5. actual test results,
6. unresolved risks or follow-ups.

Never claim a command passed unless it was actually executed.

---

# 16. Documentation rules

As the project matures, separate concerns:

## `README.md`

For developers/users discovering Nexus.

Should explain:

- what Nexus is,
- quick start,
- supported clients/connectors,
- basic commands.

---

## `docs/PRODUCT.md`

Canonical product vision and scope.

---

## `docs/ARCHITECTURE.md`

Canonical technical architecture and runtime model.

---

## `docs/SECURITY.md`

Threat model, access model, secrets, auditing, connector security, and enterprise constraints.

---

## `docs/adr/`

Use Architecture Decision Records for consequential decisions.

Examples:

```text
0001-use-go.md
0002-use-official-mcp-go-sdk.md
0003-connector-boundary.md
0004-no-arbitrary-remote-shell-tools.md
```

---

# 17. Initial PoC scope

The first PoC should prove the smallest valuable vertical slice.

Target experience:

```text
Agent
  ↓ MCP
Nexus
  ↓
IBM i
```

A good first vertical slice may be:

1. start Nexus as an MCP server,
2. connect GitHub Copilot or another MCP client,
3. expose one safe IBM i inspection tool,
4. connect using an approved read-oriented identity,
5. resolve a known IBM i object,
6. return typed metadata,
7. prove tests around the tool and connector boundary,
8. add audit output without leaking sensitive content.

Only after that is reliable should the PoC progressively add:

- source resolution,
- bounded source reading,
- program references,
- module/service-program relationships,
- dependency traversal.

---

# 18. Non-goals for the first PoC

Unless explicitly approved, do not build:

- a generic SSH administration tool,
- a generic SQL execution tool,
- mutation of IBM i objects,
- source editing on IBM i,
- deployment/promotion functionality,
- a graph database,
- enterprise-wide indexing,
- every future connector,
- a complex TUI,
- cloud architecture,
- multi-tenant infrastructure.

Prove the context retrieval model first.

---

# 19. Relationship to other BAC tooling

BAC Nexus should complement existing BAC developer/platform tooling rather than duplicate unrelated responsibilities.

Its core responsibility is:

> securely expose useful enterprise technical context to agents.

Repository management, promotion pipelines, deployment orchestration, or unrelated DevOps workflows should remain separate unless a future approved integration requires them.

---

# 20. Licensing and confidentiality

This project is intended to be internal and proprietary.

Agents must not:

- add an MIT, Apache, GPL, BSD, or other open-source project license without explicit approval,
- assume the exact legal copyright entity,
- publish internal code or architecture externally,
- copy substantial source code from reference repositories merely because they are open source,
- commit credentials, tokens, keys, certificates, or internal secrets.

Open-source dependencies may be used only in accordance with BAC's approved dependency/security process.

Reference repositories are architectural inspiration, not source templates to copy indiscriminately.

---

# 21. Current architectural hypotheses

These are intentionally labeled hypotheses.

They may change after discovery.

### H1 — Go

The primary implementation language will be Go.

### H2 — MCP-first external agent boundary

MCP will be the first agent-facing integration protocol.

### H3 — Connector architecture

Enterprise systems will be isolated behind connector boundaries.

### H4 — IBM i first

The IBM i connector will be the first production-quality connector.

### H5 — Read-oriented PoC

The PoC will expose discovery/inspection operations before mutation capabilities.

### H6 — Agent-agnostic product

Nexus will not depend conceptually on a single AI coding agent.

### H7 — No persistent graph initially

Dependency relationships will initially be discovered on demand unless measurements justify persistent indexing.

Any of these can be superseded by an ADR.

---

# 22. Questions still to resolve

Agents should not silently invent answers to these questions.

Examples:

- Which IBM i versions must be supported?
- Which BAC environments may the PoC access?
- What connectivity mechanism is officially approved?
- Which IBM i identity and authorities will be used?
- Which libraries are allowed?
- Is SSH permitted from the developer environment?
- Is direct Db2 for i connectivity permitted?
- What audit requirements apply?
- What data/source content is permitted to reach the configured AI model?
- Will the MCP server run locally, centrally, or both?
- Which GitHub Copilot enterprise policies apply?
- What is the final internal product name?
- What is the exact legal entity that owns the repository?
- Which open-source dependencies are approved?

When one of these becomes relevant to implementation, surface it explicitly.

---

# 23. First tasks for a new agent

If this repository is still at bootstrap stage, do **not** immediately generate a large implementation.

Start with:

1. inspect repository contents,
2. report what already exists,
3. compare the current structure against this document,
4. identify the smallest useful PoC,
5. research the current official MCP Go SDK API before coding,
6. inspect current Engram and Gentle AI patterns relevant to the specific task,
7. propose the initial package structure,
8. propose the first MCP tool contract,
9. propose the testing strategy,
10. ask for approval on consequential architecture decisions.

The first implementation should be intentionally small and testable.

---

# 24. Definition of good progress

A change is good progress when it makes BAC Nexus:

- safer,
- easier to reason about,
- easier to test,
- more deterministic for agents,
- less coupled to a specific client,
- clearer in architecture,
- and incrementally closer to proving useful enterprise context retrieval.

Avoid measuring progress by number of files, abstractions, tools, or connectors.

---

# 25. Short mental model

When uncertain, return to this:

```text
                AI AGENTS
                    │
                    │ MCP
                    ▼
            ┌────────────────┐
            │   BAC NEXUS    │
            │ Context Layer  │
            └───────┬────────┘
                    │
       ┌────────────┼────────────┐
       │            │            │
       ▼            ▼            ▼
     IBM i         Git        Knowledge
   Connector    Connector      Connector
       │
       ▼
 Enterprise systems
```

**Nexus connects agents to enterprise context.  
Connectors understand their ecosystems.  
Policies control what can be accessed.  
Agents retrieve only what they need.**
