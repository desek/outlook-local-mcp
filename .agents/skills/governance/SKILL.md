---
name: governance
description: Creates Architecture Decision Records (ADRs) and Change Requests (CRs) for project governance. Activates on keywords like "ADR", "architecture decision", "CR", "change request", "governance", "technical decision", or "requirement change". Use for documenting technology choices, architectural patterns, or scope modifications.
license: Apache-2.0
metadata:
  copyright: Copyright Daniel Grenemark 2026
  author: desek
  version: "1.1"
---

# Governance Documentation

Creates and manages governance documents: ADRs for architectural decisions, CRs for requirement changes.

## Document Selection

| Need | Document | Guide |
|------|----------|-------|
| Technical/architectural decision | ADR | [reference/adr-guide.md](reference/adr-guide.md) |
| Requirement or scope change | CR | [reference/cr-guide.md](reference/cr-guide.md) |

## Governance Reference Boundary

Governance identifiers (the `CR-`, `ADR-`, `FR-`, `NFR-`, and `AC-` prefixes followed by digits) belong in the governance corpus under `docs/` and in Git metadata — commit messages, branch names, and pull request descriptions — and **MUST NOT** be written into source code, test names, or user-facing documentation. To link an implementation back to its governance document, put the identifier in the commit message and describe the behavior itself in the code; see [reference/cr-guide.md#governance-reference-boundary](reference/cr-guide.md#governance-reference-boundary) for the full pattern definition, the permitted and prohibited territories, and the rationale.

## ADR Workflow

> **Documentation-only task.** Creating an ADR is a documentation-only task. No code compilation, test execution, or linting is required.

Use this checklist when creating an Architecture Decision Record:

```
- [ ] Read the template: templates/ADR.md
- [ ] Check docs/adr/ for the next available number
- [ ] Capture current branch (`git rev-parse --abbrev-ref HEAD`) and commit (`git rev-parse --short HEAD`)
- [ ] Create file: docs/adr/ADR-NNNN-{short-title}.md
- [ ] Fill in all required sections (including source-branch and source-commit in frontmatter)
- [ ] Set status to "proposed"
```

**Template frontmatter:** A created ADR carries its own `name` and `description` reflecting the specific decision, and no template metadata. The template's `name` and `description` describe the template itself, so replace them; the template carries no other metadata to copy.

**Strict requirements:**
- File naming: `ADR-NNNN-{title}.md` (four-digit number, lowercase, hyphens)
- Initial status: `proposed`
- Location: project's `docs/adr/` folder

**Flexible (adapt to context):**
- Level of detail in alternatives section
- Number of consequences listed
- Diagram inclusion (recommended for complex decisions)

## CR Workflow

> **Documentation-only task.** Creating a CR is a documentation-only task. No code compilation, test execution, or linting is required.

Use this checklist when creating a Change Request:

```
- [ ] Read the template: templates/CR.md
- [ ] Check docs/cr/ for the next available number
- [ ] Capture current branch (`git rev-parse --abbrev-ref HEAD`) and commit (`git rev-parse --short HEAD`)
- [ ] Create file: docs/cr/CR-NNNN-{short-title}.md
- [ ] Fill in all required sections (including source-branch and source-commit in frontmatter)
- [ ] Write acceptance criteria in Gherkin format
- [ ] Set status to "proposed"
```

**Template frontmatter:** A created CR carries its own `name` and `description` reflecting the specific change request, and no template metadata. The template's `name` and `description` describe the template itself, so replace them; the template carries no other metadata to copy.

**Strict requirements:**
- File naming: `CR-NNNN-{title}.md` (four-digit number, lowercase, hyphens)
- Initial status: `proposed`
- Location: project's `docs/cr/` folder
- Acceptance criteria: Gherkin format (Given-When-Then)
- Requirements keywords: RFC 2119 (MUST, SHOULD, MAY)

**Flexible (adapt to context):**
- Document length (minimum 250 lines for complex changes)
- Number of diagrams
- Depth of impact assessment

### Implementing an Authored CR

Once a CR exists, [reference/cr-implementation-workflow.md](reference/cr-implementation-workflow.md) provides a project-agnostic agent-team workflow that carries it from review through phased implementation, finalization, validation, gap-fixing, and documentation. It follows the host project's own build, lint, and test conventions.

## Templates

- **ADR**: [templates/ADR.md](templates/ADR.md)
- **CR**: [templates/CR.md](templates/CR.md)

## Reference Guides

For detailed lifecycle information, best practices, and examples:

- **ADR Guide**: [reference/adr-guide.md](reference/adr-guide.md)
- **CR Guide**: [reference/cr-guide.md](reference/cr-guide.md)
- **CR Implementation Workflow**: [reference/cr-implementation-workflow.md](reference/cr-implementation-workflow.md) — agent-team workflow for implementing an existing CR

## Commit Message Format

Committing governance documents is the human's responsibility. When ready to commit, use these formats:

- **ADR**: `docs(adr): add ADR-NNNN {title}`
- **CR**: `docs(cr): add CR-NNNN {title}`
