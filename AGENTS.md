# AGENTS.md

* Don't be an overachiever. Focus on the task, aiming for perfection in the execution rather than in adding extras.
* Implement the solution using many small, isolated, single-purpose files. Your primary goal is to minimize the Lines of Code (LoC) in each file. Code duplication is explicitly allowed to maintain this structure.
* Follow the Architectural Governance and Project Requirements Change process.
* Use `deepwiki` MCP for knowledge about specific package implementations details when needed.
* Use the file `.deepwiki` as a repository for relevant DeepWiki repositories.
* Continuously keep the `.gitignore` accurate to not bloat the repository.

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
    server/                    # RegisterTools, ReadOnlyGuard, AwaitShutdownSignal
    tools/                     # 4 aggregate domain tools dispatching verb sets: calendar (15 verbs), mail (5-13 verbs gated by MailEnabled/MailManageEnabled), account (7 verbs), system (2-3 verbs gated by auth_code); see CR-0056, CR-0058, CR-0060
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

All MCP tools **MUST** follow the domain-prefixed naming pattern:

```
{domain}_{operation}[_{resource}]
```

Recognized domain prefixes:

* `calendar_` -- Calendar and event operations
* `mail_` -- Mail message and folder operations
* `account_` -- Account management operations

System tools (`status`, `complete_auth`) are exempt from domain prefixing.

New tools **MUST** follow this convention. The tool name in `mcp.NewTool()` must match the name used in all middleware calls (`wrap`/`wrapWrite`, `WithObservability`, `AuditWrap`) in `server.go`.

## MCP Tool Annotations

All MCP tools **MUST** include the full set of five MCP annotations for Anthropic Software Directory compliance (see CR-0052):

* `mcp.WithTitleAnnotation(string)` -- human-readable display name for UI tool pickers.
* `mcp.WithReadOnlyHintAnnotation(bool)` -- `true` for read tools, `false` for write/delete tools.
* `mcp.WithDestructiveHintAnnotation(bool)` -- `true` only for tools that irreversibly delete data or send cancellation notices (`calendar_delete_event`, `calendar_cancel_meeting`, `account_remove`).
* `mcp.WithIdempotentHintAnnotation(bool)` -- `true` when calling with the same arguments produces the same result (e.g., GET, PATCH, DELETE); `false` for tools that create new resources each call.
* `mcp.WithOpenWorldHintAnnotation(bool)` -- `true` for tools that call external APIs (Microsoft Graph); `false` for local-only tools (`account_list`, `account_remove`, `status`).

Annotation values **MUST** be explicitly set even when they match MCP spec defaults. The complete annotation matrix is defined in CR-0052. New tools **MUST** add a corresponding annotation test in `internal/tools/tool_annotations_test.go`.

## MCP Tool Response Tiering

All MCP tool responses **MUST** follow a three-tier output model to minimize token consumption for LLM consumers while preserving data access for programmatic use cases:

| Tier | Mode | Default | Format | Use Case |
|------|------|---------|--------|----------|
| 1 | `text` | **Yes** | CLI-like plain text with numbered lists and labeled fields | General LLM consumption, human-readable display |
| 2 | `summary` | No | Compact JSON with an intentionally curated field set per tool | Programmatic LLM reasoning requiring structured data |
| 3 | `raw` | No | Full, unmodified JSON matching Graph API response shape | Debugging, full-field inspection |

### Rules

* **Read tools** accept an `output` parameter with values `text` (default), `summary`, and `raw`. All three tiers **MUST** be implemented.
* **Write tools** return text confirmations unconditionally (no `output` parameter). Confirmations include the action, subject, resource ID, key fields, and contextual message strings (e.g., attendee notification status).
* **`text` is the default** — tool handlers **MUST** return plain text when `output` is empty or omitted.
* **`raw` must be explicitly requested** — it is never the default for any tool. Raw output is the complete, unmodified Graph API serialization including empty values.
* **`summary` field sets are intentional** — each tool's summary mode uses a deliberately chosen field set via a dedicated serialization function (e.g., `SerializeSummaryEvent`). Summary fields are **not** derived by filtering empty values from raw output. New tools **MUST** define their summary field set explicitly.
* **Text formatters** live in `internal/tools/text_format.go`. New formatters follow the established patterns: numbered lists for collections, labeled fields for details, total counts at the end.
* **Body escalation pattern**: Tools that return content previews by default (e.g., `bodyPreview`) **MUST** document in their tool description that the full content (e.g., HTML body) requires `output=raw`. This guides the LLM to use the preview to decide whether fetching the full content is necessary, avoiding unnecessary token consumption.

### Why This Matters

Every token in an MCP tool response competes with user instructions, conversation history, and LLM reasoning space. A 10-event JSON list consumes ~2,500 tokens; the same data as text consumes ~800 tokens — a 68% reduction. Over a session with 20-30 tool calls, this saves thousands of tokens and materially improves LLM performance.

## MCP Tool Testing Instructions

When a new MCP tool is added or an existing tool's parameters/behavior change, `docs/prompts/mcp-tool-crud-test.md` **MUST** be updated to include testing steps that exercise the new or changed functionality. This keeps the CRUD lifecycle test accurate and ensures all tools are covered by the integration test script.

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

