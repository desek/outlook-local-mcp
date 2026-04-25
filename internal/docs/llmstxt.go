package docs

import (
	"fmt"
	"strings"
)

// githubBase is the absolute GitHub blob URL prefix for the default branch.
// All links in the generated llms.txt must use this prefix so off-repo LLM
// clients can resolve them without access to the local filesystem.
const githubBase = "https://github.com/desek/outlook-local-mcp/blob/main/"

// llmsSections defines the H2 sections in the generated llms.txt.
// Each section has a heading and a list of items in llms.txt format:
// "[Title](url): description".
var llmsSections = []llmsSection{
	{
		heading: "Docs",
		items: []llmsItem{
			{
				title:       "Quick Start Guide",
				path:        "QUICKSTART.md",
				description: "Prerequisites, installation, authentication, and first tool call.",
			},
			{
				title:       "Troubleshooting Guide",
				path:        "docs/troubleshooting.md",
				description: "Auth errors, token refresh, Keychain issues, Graph throttling, mail flags, and account lifecycle.",
			},
			{
				title:       "Reference Specification",
				path:        "docs/reference/outlook-local-mcp-spec.md",
				description: "Full environment variable reference, tool parameter matrix, and configuration schema.",
			},
			{
				title:       "Changelog",
				path:        "CHANGELOG.md",
				description: "Release history and notable changes by version.",
			},
		},
	},
	{
		heading: "Tools",
		items: []llmsItem{
			{
				title:       "Extension Manifest",
				path:        "extension/manifest.json",
				description: "MCP tool manifest listing all four aggregate domain tools (calendar, mail, account, system) with their annotations.",
			},
		},
	},
	{
		heading: "Change Requests",
		items: []llmsItem{
			{
				title:       "Change Request Index",
				path:        "docs/cr/",
				description: "Architecture and feature change requests for outlook-local-mcp.",
			},
		},
	},
	{
		heading: "Optional",
		items: []llmsItem{
			{
				title:       "Research Notes",
				path:        "docs/research/",
				description: "Investigation notes on Graph API quirks, auth flows, and implementation decisions.",
			},
		},
	},
}

// llmsSection groups a set of llmsItem values under a named H2 heading.
type llmsSection struct {
	heading string
	items   []llmsItem
}

// llmsItem represents a single entry in an llms.txt section.
type llmsItem struct {
	title       string
	path        string
	description string
}

// GenerateLLMsTxt returns the full text content of an llms.txt file conforming
// to the AnswerDotAI llms.txt standard (https://llmstxt.org).
//
// The generated content includes:
//   - A single H1 header ("# outlook-local-mcp").
//   - A blockquote one-sentence project summary.
//   - An information paragraph about the in-server documentation surface.
//   - H2 sections (Docs, Tools, Change Requests, Optional) with items as
//     "[Title](absolute-github-url): description" lines.
//
// Every link uses an absolute GitHub blob URL on the main branch so off-repo
// LLM clients can resolve them without local filesystem access.
//
// GenerateLLMsTxt takes no parameters and returns a string; it does not read
// the embedded bundle at runtime (the catalog is used only to verify slugs
// resolve; the llms.txt content is derived from the static llmsSections table).
//
// Side effects: none.
func GenerateLLMsTxt() string {
	var b strings.Builder

	b.WriteString("# outlook-local-mcp\n\n")

	b.WriteString("> A single-binary MCP server that connects Claude Desktop and Claude Code to Microsoft Outlook (Calendar and Mail) via the Microsoft Graph API, with multi-account support and in-server documentation access for LLM self-troubleshooting.\n\n")

	b.WriteString("This server embeds its own documentation and exposes it through three verbs on the `system` aggregate tool: `system.list_docs`, `system.search_docs`, and `system.get_docs`. ")
	b.WriteString("Each embedded document is also available as an MCP resource at `doc://outlook-local-mcp/{slug}`. ")
	b.WriteString("LLM clients that support `resources/list` and `resources/read` can fetch documents natively; ")
	b.WriteString("clients that do not can use the `system.*_docs` verbs instead. ")
	b.WriteString("Call `{tool: \"system\", args: {operation: \"list_docs\"}}` to see the available documents, or `{tool: \"system\", args: {operation: \"status\"}}` for the server entry point including `docs.base_uri`.\n\n")

	for _, section := range llmsSections {
		fmt.Fprintf(&b, "## %s\n\n", section.heading)
		for _, item := range section.items {
			url := githubBase + item.path
			fmt.Fprintf(&b, "- [%s](%s): %s\n", item.title, url, item.description)
		}
		b.WriteString("\n")
	}

	return b.String()
}
