# AGENTS.md

* Don't be an overachiever. Focus on the task, aiming for perfection in the execution rather than in adding extras.
* Implement the solution using many small, isolated, single-purpose files. Your primary goal is to minimize the Lines of Code (LoC) in each file. Code duplication is explicitly allowed to maintain this structure.
* Follow the Architectural Governance and Project Requirements Change process.
* Use `deepwiki` MCP for knowledge about specific package implementations details when needed.
* Use the file `.deepwiki` as a repository for relevant DeepWiki repositories.
* Continuously keep the `.gitignore` accurate to not bloat the repository.
* `CHANGELOG.md` is managed by release-please and **MUST NOT** be manually updated.

## Project Structure

The project follows the Go standard project layout (see CR-0021):

```
outlook-mcp/
  cmd/
    outlook-local-mcp/
      main.go                  # Entry point: config load, subsystem init, lifecycle
  internal/
    config/                    # Config struct, LoadConfig, ValidateConfig
    auth/                      # Browser/device code auth, token cache, auth record, account registry, account resolver
    logging/                   # InitLogger, SanitizingHandler, PII masking, MultiHandler, file logging
    audit/                     # Audit logging subsystem, AuditWrap middleware
    graph/                     # Graph API utilities: errors, retry, timeout, serialization, enums, recurrence
    validate/                  # Input validation helpers
    observability/             # OpenTelemetry metrics and tracing, WithObservability middleware
    buildinfo/                 # Build identity and host environment snapshot (system.about; CR-0067)
    server/                    # RegisterTools, ReadOnlyGuard, AwaitShutdownSignal
    tools/                     # The 4 aggregate domain tools and their verb registries
  docs/
    ...
```

**Build:** `go build ./cmd/outlook-local-mcp/`
**Install:** `go install github.com/desek/outlook-local-mcp/cmd/outlook-local-mcp@latest`
**New code** must be placed in the appropriate `internal/` package, not the repository root.

## Documentation Standards

All code **MUST** be extensively documented using Go doc comments:

* **Every package**: Include a package-level docstring in a `doc.go` file or the main package file describing the package's purpose and how it fits into the system.
* **Every function/method**: Include a docstring describing:
    * **Purpose**: What the function does and why it exists.
    * **Parameters**: Meaning of each parameter.
    * **Return value**: What is returned.
    * **Side effects**: Any mutations, API calls, or state changes.
    * **Errors**: Conditions under which an error is returned.
* **Every struct/interface**: Include a docstring describing the type's role and intent.
* **Every exported field**: Document the purpose and intent of each exported field.
* **Complex logic**: Add inline comments to explain non-obvious algorithms or business rules. Reference related ADRs where applicable.

### Docstring Style

* Use standard Go doc comment style (start with the name of the symbol).
* Focus on **intent and purpose** — explain *why*, not just *what*.
* Keep comments concise but complete.
* Update docstrings whenever the implementation changes.

## Design Principles

All code **MUST** adhere to the following design principles consistently:

* **SOLID**:
    * **Single Responsibility**: Each package, struct, or function must have one reason to change.
    * **Open/Closed**: Code must be open for extension but closed for modification.
    * **Liskov Substitution**: Interfaces should be satisfied by types without altering correctness.
    * **Interface Segregation**: Prefer small, focused interfaces over large, general-purpose ones.
    * **Dependency Inversion**: Depend on abstractions (interfaces), not concretions.
* **Composition over Inheritance**: Use embedding and composition to build complex types.
* **DRY (Don't Repeat Yourself)**: Extract shared logic into reusable abstractions. Note: code duplication for file isolation is acceptable in Go if it avoids unnecessary package dependencies.
* **KISS (Keep It Simple, Stupid)**: Choose the simplest solution. Avoid over-engineering and premature abstraction.
* **Law of Demeter**: A unit should only talk to its immediate collaborators.

## MCPB Extension Manifest

All MCP tools **MUST** be registered in `extension/manifest.json` under the `tools` array. When adding or removing a tool in `internal/server/server.go`, the corresponding entry in the manifest **MUST** be updated to match. The manifest is used by Claude Desktop to discover available tools.

## Tool Naming Convention

As of CR-0060 (v0.6.0) the MCP surface is four aggregate domain tools, each dispatched by a required `operation` verb. New work **MUST** add a verb to the appropriate domain registry, not a new top-level MCP tool.

Aggregate tools and their domains:

* `calendar` -- Calendar and event verbs
* `mail` -- Mail message, folder, and draft verbs
* `account` -- Account management verbs
* `system` -- Server-level and diagnostic verbs

The current verb inventory of a domain is the registry's to state, not this
file's: invoke `operation="help"` on the domain.

Verb names **MUST** be self-explanatory English verbs or verb phrases without the domain prefix (for example `create_event`, not `calendar_create_event`). Every domain tool **MUST** expose an `operation="help"` verb documenting its registered verbs. The `{domain}.{operation}` identity is what surfaces in audit logs and OpenTelemetry attributes.

An aggregate tool's registered name **MUST** match the name used in every middleware call for that tool, so the audit and telemetry identity stays consistent.

## MCP Tool Annotations

Rules for authors. The fold semantics and the configuration-dependent published
values are documented once, in [`docs/concepts.md`](docs/concepts.md#tool-annotation-semantics);
do not restate them here.

* Every verb **MUST** declare its own four-hint classification. A verb cannot be registered without one, because the aggregate annotation is computed from the registered verbs rather than hardcoded.
* Annotation values **MUST** be set explicitly, even when they match the MCP spec defaults.
* Per-verb annotation semantics **MUST** be documented in the domain's `operation="help"` output.
* New verbs **MUST** add a value assertion alongside the existing annotation tests in `internal/tools/`.

Specification: CR-0052 (annotation matrix), CR-0060 (conservative aggregation),
CR-0068 (computed from the registry).

## MCP Tool Response Tiering

Rules for authors. What each tier returns, which is the default, and why the model
exists are documented once, in [`docs/concepts.md`](docs/concepts.md#output-tiers);
do not restate them here.

* Read verbs **MUST** implement all three tiers via the `output` parameter. Write verbs **MUST** return a text confirmation unconditionally and **MUST NOT** take an `output` parameter.
* A write confirmation **MUST** name the action, the subject, the resource ID, the key fields that changed, and any contextual consequence the caller cannot otherwise see (for example whether attendees were notified).
* Summary field sets **MUST** be chosen deliberately per verb via a dedicated serialization function (for example `SerializeSummaryEvent`), never derived by filtering empty values out of raw output.
* Text formatters live in `internal/tools/text_format.go`. New formatters follow the established patterns: numbered lists for collections, labeled fields for details, a total count at the end.
* **Body escalation**: a verb that returns a content preview by default (for example `bodyPreview`) **MUST** state in its description that the full content requires `output=raw`, so the LLM can decide from the preview whether the full fetch is warranted.

## MCP Tool Testing Instructions

When a new MCP tool is added or an existing tool's parameters/behavior change, `docs/prompts/mcp-tool-crud-test.md` **MUST** be updated to include testing steps that exercise the new or changed functionality. This keeps the CRUD lifecycle test accurate and ensures all tools are covered by the integration test script.

### Harness Maintenance

The `make crud-test` harness (`scripts/crud-test.sh`) **MUST** be lifecycled alongside MCP tool surface changes. When a verb, domain, or tool is added, renamed, or removed, the following **MUST** be updated in the same change:

* `docs/prompts/mcp-tool-crud-test.md`: the prompt the headless agent executes; new verbs need new test steps, removed verbs need their steps deleted.
* `scripts/crud-test.sh`: a new or removed top-level domain changes the per-domain tool-call accounting. The script's own comments state how; keep it self-consistent.
* `docs/bench/crud-runs.csv`: the header **MUST** match the script's output schema. Reset historical rows when columns change rather than leaving short rows.

The harness's value depends on this coupling: drift means the bench either silently skips new functionality or emits malformed CSV rows.

## Documentation Governance

User-facing documentation has a single source of truth per concern. Future CRs that add or change documentation **MUST** route content according to the rules below.

1. **Per-tool reference** (verb name, parameters, return shape, examples) is owned by the `Verb` registry in `internal/tools/dispatch_registry.go`. Each verb populates `Summary`, `Description`, and where applicable `Examples` and `SeeDocs`. The `system.help` verb renders the registry; do not duplicate this content in markdown.
2. **Narrative concepts** (output tiers, multi-account model, gating modes, authentication flows, OAuth scopes summary, observability overview, well-known client IDs, in-server documentation surface, MCP elicitation) live in `docs/concepts.md`. New concepts are added as new anchored sections; verbs reference them via `SeeDocs`. Detailed contributor-level material (sequence diagrams, token cache schema, middleware chain, OTel attribute lists) does NOT belong here; it lives in `docs/reference/` and is not embedded.
3. **First-run workflow** lives in `docs/quickstart.md`. Configuration steps, integration setup, and end-to-end verification go here.
4. **Failure modes and recovery** live in `docs/troubleshooting.md`. Each entry has a stable anchor for `SeeDocs` references.
5. **Architecture and internals** live in `docs/reference/{architecture,auth-flows,observability,release,site-quality}.md`. These files are not embedded into the binary. The boundary rule: if an LLM helping a user mid-session needs the content to use or troubleshoot the server, it belongs in an embedded file (`concepts.md` or `troubleshooting.md`); if only a contributor modifying the code needs it, it belongs in `docs/reference/`.
6. **Governance** (CRs and ADRs) lives in `docs/cr/` and `docs/adr/`. Not embedded.
7. The repository-root `README.md` is a landing page only. It contains install, the four-domain tool invocation example, a link grid into `docs/`, and the licence. It **MUST NOT** contain per-tool reference, full configuration tables, or narrative concepts.
8. The embedded bundle is exactly four files: `docs/{readme,quickstart,concepts,troubleshooting}.md`. Adding a fifth requires updating `docs/embed.go`, the allowlist test, and this section.
9. **Placement decision tree for new content:**
   - Is it 1:1 with a verb? → registry (`Description`, `Examples`).
   - Does it span verbs and explain a concept? → `docs/concepts.md`.
   - Is it a step-by-step setup workflow? → `docs/quickstart.md`.
   - Is it a failure mode and how to recover? → `docs/troubleshooting.md`.
   - Is it architecture, internals, or a build/release detail? → `docs/reference/`.
   - Is it a decision or scope change? → CR or ADR under `docs/cr/` or `docs/adr/`.

## Measurement and Verification

When a number decides what to do, the instrument producing it is part of the work and gets
the same scepticism as the code. These apply to any measured gate in this repository — the
Lighthouse budget, the visual comparison, benchmarks, coverage, flaky tests — and each of
them, skipped, has already produced a confident wrong answer here.

* **Validate the instrument before trusting a result: run it twice on unchanged input.** A
  measurement that disagrees with itself on a null change cannot detect a real one.
  Disagreement is a defect in the instrument, fixed before any finding is reported.
* **State the noise floor next to the threshold.** A tolerance without a measured floor is
  a guess. Report both, and treat a result inside the noise band as "not measured" rather
  than "unchanged". An intermittently-failing gate is worse than a failing one: it is
  green often enough to be believed.
* **Attribute from the detail, not the headline.** Diagnostic summaries name *correlates*
  prominently and *causes* in the detail. Act on the attribution for the specific
  measurement, not on whatever the tool puts at the top.
* **Falsify by substitution, at an extreme.** To test whether a component is responsible,
  replace it with a null or extreme version and re-measure. One decisive run beats an
  afternoon of reasoning, and reasoning from first principles about performance is usually
  wrong.
* **Derive the model, then check it reproduces every measurement already taken.** A model
  that reproduces them answers further questions without more runs, and converts "this
  seems hard" into a number that can be argued with.
* **A null result falsifies the intervention, not the hypothesis.** "I changed X and the
  metric did not move, so X is not the cause" holds only if the change actually exercised
  X. Verify the intervention did what was intended before concluding anything from its
  lack of effect.
* **Determinism requires controlling every source of it** — time, randomness, scheduling,
  and any clock the platform advances independently. Miss one and the failure is
  intermittent, which is the expensive kind.
* **A failing golden-output diff is a question, not a verdict.** Snapshot and
  reference-output comparisons fail identically for a regression and for a repair. Look at
  what changed before deciding which it was.
* **When a target proves unreachable, publish the ceiling with the measurement chain that
  established it,** and say what would have to change to move it. Amend the governing CR
  rather than quietly relaxing the check. A documented ceiling ends the question; a bare
  assertion invites the same experiments to be re-run.

## Website

The website in `site/` is a separate build with its own quality gate, and it is measured
rather than eyeballed. Before changing it, read the documents rather than re-deriving what
they contain:

* [`docs/reference/site-quality.md`](docs/reference/site-quality.md) — how to run the
  checks, what Lighthouse's simulated throttling does and does not measure, why the
  landing page is held to 96 while the documentation pages are held to 100, the five
  clocks that must be frozen for a reproducible screenshot, and the optimisations that
  look lossless and are not. Read this before attempting any site performance work; most
  of the obvious levers were measured and move nothing.
* [`site/ARCHITECTURE.md`](site/ARCHITECTURE.md) — the design language and per-section
  intent of the page, as authored.
* `.agents/scripts/site.*.mjs` — the measurement harness itself. Each script carries its
  purpose and its caveats in its own docstring.

Three rules for site changes:

* **A visual change is asserted, not asserted-to-be-absent.** Any change claiming to leave
  rendering untouched **MUST** be verified with a screenshot comparison against a build of
  the commit it is compared to, not assumed from the nature of the change. Repairs and
  regressions look identical in a diff until someone opens the tiles.
* **The Lighthouse thresholds in `site/lighthouserc.json` are a measured ceiling, not an
  aspiration.** Raising one requires the measurement that shows it is reachable; lowering
  one requires saying so in the governing CR rather than editing the file quietly.
* **The site and the Go application have separate workflows.** `site.yml` runs on
  `site/**` and `docs/**`; `ci.yml` ignores site-only paths. `docs/**` triggers both,
  because those files are embedded in the binary and also generate the site's
  documentation pages.

## Quality Standards

All code changes **MUST** meet the following quality requirements before committing. Use the `Makefile` targets to run checks:

| Check | Command | Description |
|-------|---------|-------------|
| **Build** | `make build` | Code must compile successfully |
| **Vet** | `make vet` | Static analysis must pass |
| **Format** | `make fmt-check` | All code must be formatted (`make fmt` to auto-fix) |
| **Tidy** | `make tidy` | `go.mod` and `go.sum` must be tidy |
| **Lint** | `make lint` | All `golangci-lint` checks must pass |
| **Test** | `make test` | All tests must pass (includes `-race` and coverage) |
| **SBOM** | `make sbom` | Generate Software Bill of Materials (CycloneDX + SPDX) |
| **Vuln Scan** | `make vuln-scan` | Vulnerability scan must pass with no high-severity findings |
| **License** | `make license-check` | Dependency license compliance check |

* Any accepted linter warnings **MUST** have an explanatory `//nolint` comment.
* Pre-commit hooks must pass (see `.pre-commit-config.yaml`).

Run all quality checks at once before pushing:
```bash
make ci
```

Do not commit code that breaks builds, fails linting, or causes test failures.

## Commit and PR Conventions

Agents **MUST** follow the [Conventional Commits](https://www.conventionalcommits.org/) specification for:

* All commit messages
* GitHub pull request titles

### Conventional Commit Format

```text
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]
```

Common types include:

* `feat`: A new feature
* `fix`: A bug fix
* `docs`: Documentation only changes
* `style`: Changes that do not affect the meaning of the code
* `refactor`: A code change that neither fixes a bug nor adds a feature
* `test`: Adding missing tests or correcting existing tests
* `chore`: Changes to the build process or auxiliary tools

Example: `feat(automation): toggle light when front door is unlocked`

### Branch Protection

Force pushes are **BLOCKED** on protected branches. Always create new commits instead of rewriting history.

Direct commits to `main` (the default branch) are **PROHIBITED**. All changes **MUST** go through a pull request.

### Merge Strategy

Pull requests **MUST** use squash merge only. The PR title will become the commit message, so ensure it follows the Conventional Commits format.

### Linear Commit History

A **linear commit history is required**. Merge commits are not allowed. Use rebase or squash merge strategies to maintain a clean, linear history on the main branch.

