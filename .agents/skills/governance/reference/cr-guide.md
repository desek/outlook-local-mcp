---
name: cr-guide
description: Detailed guidance for creating and managing Change Requests (CRs).
metadata:
  copyright: Copyright Daniel Grenemark 2026
  version: "0.0.1"
---

# CR Reference Guide

Detailed guidance for creating and managing Change Requests.

## Table of Contents

- [When to Create a CR](#when-to-create-a-cr)
- [CR Lifecycle](#cr-lifecycle)
- [Requirements](#requirements)
- [Document Numbering](#document-numbering)
- [Source Traceability](#source-traceability)
- [Governance Reference Boundary](#governance-reference-boundary)

## When to Create a CR

Create a CR when:

- **Adding new features**: Functionality not in the original requirements
- **Modifying existing features**: Changes to behavior or user workflows
- **Removing functionality**: Deprecating or removing features
- **Non-functional changes**: Significant changes to performance, security, scalability
- **Scope changes**: Modifying project scope, timeline, or success criteria
- **User feedback response**: Requirement adjustments based on feedback

## CR Lifecycle

| Status | Description |
|--------|-------------|
| **Proposed** | CR is created and awaiting review |
| **Approved** | Stakeholders have approved the change |
| **Implemented** | Change has been developed and merged |
| **Rejected** | Change was declined with reasoning documented |
| **On-Hold** | Change is postponed for future consideration |
| **Cancelled** | Change is no longer needed or relevant |
| **Obsolete** | Change is outdated due to external factors or superseded requirements |

### Status Transitions

```
proposed → approved → implemented
proposed → rejected
proposed → on-hold → approved/cancelled
approved → cancelled
any status → obsolete (when externally invalidated)
```

- A CR starts as `proposed`
- After stakeholder review, it becomes `approved`, `rejected`, or `on-hold`
- Approved CRs move to `implemented` when development is complete
- Any CR may become `obsolete` if external factors invalidate it

## Requirements

### Use RFC 2119 Keywords

Use **MUST**, **SHOULD**, **MAY** keywords only for unambiguous requirements:

- **MUST**: Absolute requirement
- **SHOULD**: Recommended but not mandatory
- **MAY**: Optional

### Write Acceptance Criteria in Gherkin

Use Given-When-Then formula for all acceptance criteria:

```gherkin
Given [precondition]
When [action]
Then [expected result]
  And [additional expectation]
```

### Use Mermaid Diagrams

Include Mermaid diagrams for all visualizations:

- Flowcharts for processes
- Sequence diagrams for interactions
- State diagrams for lifecycle changes

### Ensure Comprehensive Detail

- Minimum 250 lines for complex changes
- Include all affected components
- Document scope boundaries clearly
- List risks and mitigations

### Validate with DeepWiki MCP

When referencing external libraries or frameworks:

- Use DeepWiki MCP to verify implementation details
- Confirm API compatibility
- Check for breaking changes

### Include Test Strategy

For all code changes, document:

- Tests to add
- Tests to modify
- Tests to remove
- Validation methods

## Document Numbering

CRs use sequential four-digit numbering:

- First CR: `CR-0001-{title}.md`
- Second CR: `CR-0002-{title}.md`

**File naming rules:**
- Use lowercase letters, numbers, and hyphens only
- Keep titles short but descriptive
- Example: `CR-0001-add-user-auth.md`

Check existing documents in the project's `docs/cr/` folder to determine the next available number.

## Source Traceability

Every CR **MUST** record the repository state it was based on using two frontmatter fields:

| Field | Purpose | How to populate |
|-------|---------|----------------|
| `source-branch` | Git branch the analysis is based on | `git rev-parse --abbrev-ref HEAD` |
| `source-commit` | Short commit hash at time of creation | `git rev-parse --short HEAD` |

These fields enable:

- **Staleness detection** — Compare `source-commit` against the current HEAD to see what has changed since the CR was written.
- **Conflict identification** — When multiple CRs are in-flight, reviewers can determine if one CR's implementation invalidates another's analysis.
- **Audit trail** — Provides a clear link between the CR and the repository state it was based on.

### When to Rebase a CR

A CR may need to be reviewed and updated ("rebased") when the diff between its `source-commit` and the current HEAD affects:

- The "Current State" section's accuracy
- The implementation plan's feasibility
- The impact assessment's completeness
- The acceptance criteria's validity

## Governance Reference Boundary

Source Traceability records what a CR was written *against*. This section governs the reverse direction: where a governance identifier is allowed to appear once the CR is being implemented. The two read together — a CR points at its source commit, and the implementation points back at the CR through Git metadata, never through the working tree.

### The reference pattern

A **governance reference** is any identifier matching one of the prefixes `CR-`, `ADR-`, `FR-`, `NFR-`, or `AC-` immediately followed by a hyphen already consumed by the prefix and one or more digits. In regular-expression terms it is `(CR|ADR|FR|NFR|AC)-[0-9]+`, covering Change Requests, Architecture Decision Records, Functional Requirements, Non-Functional Requirements, and Acceptance Criteria.

### Permitted territory

A governance reference **MAY** appear only in:

- The governance corpus: any file under `docs/cr/` or `docs/adr/`, including filenames.
- The governance corpus index: `docs/llms.txt`, whose purpose is to enumerate the corpus.
- Git metadata: commit messages, branch names, pull request titles and descriptions, and issue text.
- The governance skill's own definition of the rule and its document-naming conventions, where the pattern appears as a placeholder rather than as a reference to a specific document. These files are exactly `skills/governance/templates/CR.md`, `skills/governance/templates/ADR.md`, `skills/governance/reference/cr-guide.md`, and `skills/governance/reference/adr-guide.md`.
- The boundary test's own machinery: `tests/governance/test_reference_boundary.bats` and `tests/governance/test_helpers/setup.bash`, which must embed the pattern to define and exercise the check.

### Prohibited territory

A governance reference **MUST NOT** appear in:

- Source code of any kind, including code comments.
- Test names, test descriptions, and test assertions.
- User-facing documentation: `README.md`, `AGENTS.md`, `CONTRIBUTING.md`, `WORKFLOW.md`, skill `SKILL.md` files, and any documentation outside the governance corpus that is not on the permitted allowlist above.

### Linking an implementation to its governance document

Git metadata is the permitted mechanism for tying an implementation to the CR or ADR it originated from. **Commit messages, branch names, and pull request descriptions** carry the identifier; a checkpoint commit subject bearing a CR identifier is queryable with `git log --grep` and never appears in the working tree. Embedding the identifier in source code, tests, or user-facing documentation is **prohibited** — describe the behavior on its own terms and let the commit metadata record its provenance.

### Rationale

The distinction is one of audience. The governance corpus is read by people reasoning about decisions, for whom identifiers are how they navigate it. Everything else is read by people using or changing the software, for whom an identifier is a dead end: it is meaningless to a reader without the corpus, it rots silently when a document is renumbered or superseded, and it inverts traceability by forcing a reader of the code to obtain the governance document before understanding what they are reading.
