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
