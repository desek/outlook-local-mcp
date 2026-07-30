---
name: cr-implementation-workflow
description: Sequential agent-team workflow for implementing an existing Change Request (CR) through review, phased implementation, finalization, validation, gap-fixing, and documentation.
metadata:
  copyright: Copyright Daniel Grenemark 2026
  version: "0.0.1"
---

# CR Implementation Workflow

A language- and stack-agnostic agent-team workflow that takes an **existing, authored CR**
and carries it through to a validated, documented implementation.

Authoring is out of scope. The CR must already exist under `docs/cr/` before this workflow
starts. See [cr-guide.md](cr-guide.md) for authoring guidance.

## Table of Contents

- [Project Conventions](#project-conventions)
- [Prompt Template](#prompt-template)

## Project Conventions

Every agent in this workflow **MUST** follow the host project's own conventions rather than
any language-specific commands assumed here. Before acting, each agent discovers:

1. **The convention source.** Read the project's agent instructions (`AGENTS.md`,
   `CLAUDE.md`, or equivalent), then `CONTRIBUTING.md` and the project README.
2. **The quality gate.** Identify the project's build, lint, and test commands from its task
   runner or manifest (for example `Makefile`, `mise.toml`, `justfile`, `package.json`
   scripts, or the language's native toolchain). Prefer a single aggregate target when the
   project defines one (such as `make ci`) over invoking each step separately.
3. **The source boundary.** Identify which file types constitute source code in this project,
   so documentation-only agents know what they must not modify.

Where this document says **run the project's quality checks**, it means: run the discovered
build, lint, and test commands, in that order, and treat a non-zero exit from any of them as
a failure that must be fixed before the agent completes.

## Prompt Template

Copy the template below, replace `<cr_reference/>` with the path to the CR being implemented,
and run it as a single orchestration turn.

```xml
<objective>
  Implement the Change Request at <cr_reference/> by orchestrating a sequential agent team
  that reviews the CR for contradictions and ambiguity, implements it phase-by-phase with
  checkpoint governance, finalizes the CR, validates the implementation, fixes any gaps,
  and updates project documentation.
</objective>

<cr_reference>
  docs/cr/CR-NNNN-{short-title}.md
</cr_reference>

<preconditions>
  The CR at <cr_reference/> already exists and is authored. Do NOT author a new CR as part
  of this workflow. If <cr_reference/> does not resolve to an existing file, stop and report
  which path is missing before running any agent.
</preconditions>

<project_conventions>
  This workflow is project-agnostic. Every agent MUST follow the conventions of the project
  it runs in, discovered as follows:

  1. Read the project's agent instructions (AGENTS.md, CLAUDE.md, or equivalent), then
     CONTRIBUTING.md and the README, before making any change.
  2. Determine the project's build, lint, and test commands from its task runner or manifest
     (Makefile, mise.toml, justfile, package.json scripts, or the native toolchain). Prefer a
     single aggregate quality target when the project defines one.
  3. Determine which file extensions constitute source code, so documentation-only agents
     know what they must not modify.

  "Run the project's quality checks" below means: run the discovered build, lint, and test
  commands in that order, and fix any failure before completing. Do NOT assume any specific
  language, toolchain, or command.
</project_conventions>

<execution_model>
  Run a team of agents sequentially. Each agent completes fully before the next starts.
  Agents use /checkpoint-read and /checkpoint-commit for continuity across contexts.

  1. Agent 1 (CR Reviewer) -- reviews the CR for internal contradictions, ambiguous
     language, requirement/AC consistency, and overall quality. Produces a review with
     actionable findings and applies fixes directly to the CR.
  2. Agents 2..N (Phase Implementors) -- one agent per phase from the CR's implementation
     plan. Each does /checkpoint-read at start, implements only its phase, runs the
     project's quality checks, and /checkpoint-commit at end.
  3. Agent N+1 (CR Finalizer) -- updates CR status to completed with branch/commit metadata.
  4. Agent N+2 (Validation Agent) -- documentation-only audit tracing every requirement,
     acceptance criterion, and test spec to the implementation. Produces a structured
     validation report.
  5. Agent N+3 (Gap Fixer) -- conditional: only runs if the validation report has FAIL,
     PARTIAL, or GAP items. Fixes each gap and updates the report.
  6. Agent N+4 (Documentation Updater) -- updates existing project documentation and
     creates any missing documentation to reflect the implemented feature.
</execution_model>

<agents>

  <agent id="1" name="CR Reviewer">
    <instructions>
      Review-and-fix only. Do not implement feature code.
      1. /checkpoint-read
      2. Read the CR at <cr_reference/> in full.
      3. Check for internal contradictions: verify every Acceptance Criterion is
         logically consistent with the Functional Requirements and the Implementation
         Approach. Flag any AC whose expected behavior conflicts with a requirement
         or the specified component interactions.
      4. Check for ambiguity: identify requirements or ACs that use vague language
         (e.g., "should", "may", "appropriate") instead of precise, testable
         statements. Each requirement MUST use "MUST", "MUST NOT", or "SHALL".
      5. Check requirement-AC coverage: verify every Functional Requirement has at
         least one AC that exercises it. Flag requirements with no corresponding AC.
      6. Check AC-test coverage: verify every AC has at least one entry in the Test
         Strategy table. Flag ACs with no corresponding test.
      7. Check scope consistency: verify the Affected Components list matches the
         files referenced in the Implementation Approach phases.
      8. Check diagram accuracy: verify Mermaid diagrams match the described
         component interactions and data flow.
      9. For each finding: apply the fix directly to the CR markdown file.
         Contradictions should be resolved in favor of the Implementation Approach
         (the authoritative source for how the system is built). Ambiguous language
         should be replaced with precise, testable wording.
      10. Write a brief review summary as a comment block at the bottom of the CR
          listing: findings count, fixes applied, and any unresolvable items that
          require human decision. Surface unresolvable items to the user before the
          next agent starts -- they require a human decision.
      11. /checkpoint-commit {CR_ID} CR reviewed: contradictions resolved, ambiguity fixed
    </instructions>
  </agent>

  <agent id="2..N" name="Phase Implementor">
    <instructions>
      Spawn one agent per phase in the CR's Implementation Approach. Each agent:
      1. /checkpoint-read
      2. Read the CR in full. Implement ONLY your assigned phase.
      3. Follow the project's conventions per <project_conventions/>.
      4. Run the project's quality checks.
      5. Fix any failures. If the phase cannot be completed, stop and report the blocker
         rather than proceeding to later phases.
      6. /checkpoint-commit {CR_ID} {phase summary}
    </instructions>
  </agent>

  <agent id="N+1" name="CR Finalizer">
    <instructions>
      1. /checkpoint-read
      2. Run the project's quality checks. Refuse to finalize if any of them fail.
      3. Verify the branch-level diff conforms to the CR's declared scope and the
         project's conventions.
      4. Update CR frontmatter: status=completed, completed-date=today,
         source-branch=current branch, source-commit=HEAD short hash.
      5. Check all applicable Quality Standards Compliance boxes.
      6. /checkpoint-commit {CR_ID} CR finalized
    </instructions>
  </agent>

  <agent id="N+2" name="Validation Agent">
    <instructions>
      Documentation-only. Do not modify source code.
      1. /checkpoint-read
      2. Read the CR and every file in its Affected Components.
      3. Trace each Functional Requirement, Acceptance Criterion, and Test Strategy
         entry to the implementation. Record PASS/FAIL with file:line evidence.
      4. Run the project's quality checks.
      5. Write docs/cr/{CR_ID}-validation-report.md:

         ## Summary
         Requirements: X/Y | Acceptance Criteria: X/Y | Tests: X/Y | Gaps: X

         ## Requirement Verification
         | Req # | Description | Status | Evidence |

         ## Acceptance Criteria Verification
         | AC # | Description | Status | Evidence |

         ## Test Strategy Verification
         | Test File | Test Name | Specified | Exists | Matches Spec |

         ## Gaps
         List gaps or "None".

      6. /checkpoint-commit {CR_ID} validation report completed
    </instructions>
  </agent>

  <agent id="N+3" name="Gap Fixer" conditional="true">
    <condition>Validation report contains FAIL, PARTIAL, or GAP items.</condition>
    <instructions>
      1. /checkpoint-read
      2. Read the validation report. For each FAIL, PARTIAL, or GAP: read the CR
         requirement, read affected source, implement the minimal fix.
      3. Run the project's quality checks.
      4. Update the validation report: FAIL -> FIXED with evidence.
      5. Repeat until zero FAIL and zero GAP items remain.
      6. /checkpoint-commit {CR_ID} gaps fixed per validation report
    </instructions>
  </agent>

  <agent id="N+4" name="Documentation Updater">
    <instructions>
      Documentation-only. Do not modify source code, as defined by
      <project_conventions/>.
      1. /checkpoint-read
      2. Read the CR, the validation report, and all source files in Affected Components
         to understand what was implemented.
      3. Inventory existing documentation: scan the project's documentation directory,
         README, and any in-source documentation entry points for content that references
         the areas changed by this CR (e.g., configuration, environment variables,
         architecture, public interfaces).
      4. Update existing documentation:
         - Add any new configuration options, environment variables, or behavioral
           changes to their existing documentation locations.
         - Update any architecture or component diagrams affected by the change.
      5. Create missing documentation:
         - If the feature introduces user-facing behavior not covered by existing docs,
           create a concise section or file describing: purpose, how to enable/configure,
           affected components, error behavior, and any observable side effects (e.g., log
           fields, metrics).
      6. Verify all Mermaid diagrams in updated docs render correctly (valid syntax).
      7. Do NOT duplicate content already in the CR. Describe the implemented behavior on
         its own terms -- what it does and how to use it -- and do NOT name the governance
         document it originated from anywhere in the documentation. Provenance belongs in
         commit metadata, never in the working tree.
      8. /checkpoint-commit {CR_ID} documentation updated for implemented feature
    </instructions>
  </agent>

</agents>
```
