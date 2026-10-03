// Package server — this file builds the contacts domain verb slice for the
// aggregate "contacts" MCP tool.
//
// It lives in the server package rather than tools to avoid the import cycle
// that would arise from tools importing tools/help (which itself imports tools).
//
// The whole domain is gated: the caller builds and registers it only when
// cfg.ContactsEnabled is true, so a default server registers no contacts tool
// and requests no contacts scope. Every verb reads, so the file declares no
// write chain and no ReadOnlyGuard layer: there is nothing for the guard to
// block.
//
// @agents-index: builds the five read verbs of the opt-in contacts domain tool.
package server

import (
	"time"

	"github.com/desek/outlook-local-mcp/internal/audit"
	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/desek/outlook-local-mcp/internal/observability"
	"github.com/desek/outlook-local-mcp/internal/tools"
	"github.com/desek/outlook-local-mcp/internal/tools/help"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"go.opentelemetry.io/otel/trace"
)

// contactsVerbsConfig holds the dependencies required to build the contacts
// domain verb slice. All fields are captured at server start.
//
// It carries no readOnly field, unlike the mail and calendar configurations,
// because the domain registers no write verb for ReadOnlyGuard to block.
type contactsVerbsConfig struct {
	// retryCfg is the Graph API retry configuration applied to all handlers.
	retryCfg graph.RetryConfig

	// timeout is the maximum duration for a single Graph API call.
	timeout time.Duration

	// m is the ToolMetrics instance for observability instrumentation.
	m *observability.ToolMetrics

	// tracer is the OTEL tracer for span creation.
	tracer trace.Tracer

	// authMW is the authentication middleware factory applied to every verb.
	authMW func(mcpserver.ToolHandlerFunc) mcpserver.ToolHandlerFunc

	// accountResolverMW is the account-resolver middleware applied to every
	// verb: contacts verbs are Graph reads that need a resolved account.
	accountResolverMW func(mcpserver.ToolHandlerFunc) mcpserver.ToolHandlerFunc
}

// contactsOutputDescription is the shared description of the output parameter,
// identical across the four read verbs.
const contactsOutputDescription = "Output mode: 'text' (default), 'summary', or 'raw'."

// contactsAccountDescription is the shared description of the account
// parameter, identical across the four read verbs.
const contactsAccountDescription = "Account label or UPN to use. Omit to auto-select the default account."

// buildContactsVerbs constructs the ordered []tools.Verb slice for the contacts
// domain aggregate tool and returns a pointer to an initially empty
// VerbRegistry.
//
// The slice is exactly five verbs — help, search, get_contact, list_people,
// get_person — and every one of them is a read. Each verb's Handler is
// pre-wrapped with authMW, accountResolverMW, observability, and audit
// middleware under the fully-qualified identity "contacts.<verb>" with the
// audit operation "read", so the audit record and the OpenTelemetry attributes
// carry the same {domain}.{operation} identity every other verb carries.
//
// The returned registry pointer is empty at the time of return. The caller
// MUST call RegisterDomainTool with the returned verbs, then assign the
// returned VerbRegistry back through the pointer so the help verb can
// introspect all registered verbs at call time.
//
// Parameters:
//   - c: contactsVerbsConfig with all required dependencies.
//
// Returns:
//   - verbs: ordered Verb slice for use with RegisterDomainTool.
//   - registryPtr: pointer whose value is assigned after registration.
func buildContactsVerbs(c contactsVerbsConfig) ([]tools.Verb, *tools.VerbRegistry) {
	empty := make(tools.VerbRegistry)
	registryPtr := &empty

	// wrap builds the read-verb chain:
	// authMW -> accountResolverMW -> WithObservability -> AuditWrap -> Handler.
	wrap := func(name, auditOp string, h mcpserver.ToolHandlerFunc) tools.Handler {
		return tools.Handler(c.authMW(c.accountResolverMW(observability.WithObservability(name, c.m, c.tracer, audit.AuditWrap(name, auditOp, h)))))
	}

	rc := c.retryCfg

	verbs := []tools.Verb{
		help.NewHelpVerb(registryPtr),
		buildContactsSearchVerb(c, rc, wrap),
		buildGetContactVerb(c, rc, wrap),
		buildListPeopleVerb(c, rc, wrap),
		buildGetPersonVerb(c, rc, wrap),
	}

	return verbs, registryPtr
}

// buildContactsSearchVerb constructs the search Verb, the domain's entry point:
// one free-text query answered from both of Graph's answer sets.
func buildContactsSearchVerb(c contactsVerbsConfig, rc graph.RetryConfig, wrap func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:        "search",
		Summary:     "free-text search over saved contacts and relevance-ranked people",
		Description: "Resolves a name to an email address. Searches personal contacts and relevance-ranked people in one call, and labels every match with the source it came from, so a saved contact is distinguishable from an inferred correspondent. Requires query; optional account, limit, and output ('text' by default, 'summary', or 'raw'). Each collection is read as one page of at most limit matches (default 25). The verb does not follow further pages. When Graph reports more matches, a second content block says \"more results available\". Read-only, non-destructive, idempotent, and open-world: it calls Microsoft Graph and writes nothing.",
		Examples: []tools.Example{
			{Args: map[string]any{"query": "Alex"}, Comment: "resolve a first name to an address"},
			{Args: map[string]any{"query": "Smith", "output": "summary"}, Comment: "return display names and primary addresses only"},
		},
		SeeDocs: []string{"concepts#output-tiers", "concepts#contacts-gating", "troubleshooting#contacts-disabled", "troubleshooting#contacts-consent"},
		Handler: wrap("contacts.search", "read", tools.NewHandleContactsSearch(rc, c.timeout)),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(true),
		},
		Schema: []mcp.ToolOption{
			mcp.WithString("query",
				mcp.Required(),
				mcp.Description("Free-text search value, typically a name or part of an email address. An empty or whitespace-only value is rejected before any request is issued."),
			),
			mcp.WithNumber("limit", mcp.Min(1), mcp.Max(100), mcp.DefaultNumber(25), mcp.Description("Maximum matches read from each collection (saved contacts and ranked people), sent as $top. Default 25, maximum 100; values above 100 are clamped.")),
			mcp.WithString("account", mcp.Description(contactsAccountDescription)),
			mcp.WithString("output",
				mcp.Description(contactsOutputDescription),
				mcp.Enum("text", "summary", "raw"),
			),
		},
	}
}

// buildGetContactVerb constructs the get_contact Verb.
func buildGetContactVerb(c contactsVerbsConfig, rc graph.RetryConfig, wrap func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:        "get_contact",
		Summary:     "get one saved personal contact by ID with all of its email addresses",
		Description: "Fetches a single personal contact by its identifier and returns its display name and every entry of its emailAddresses collection. Use search or list_people to obtain an identifier. Requires contact_id; optional account and output ('text' by default, 'summary', or 'raw'). Read-only, non-destructive, idempotent, and open-world: it calls Microsoft Graph and writes nothing.",
		Examples: []tools.Example{
			{Args: map[string]any{"contact_id": "AAMkAGI2..."}, Comment: "fetch a contact returned by search"},
		},
		SeeDocs: []string{"concepts#output-tiers", "concepts#contacts-gating", "troubleshooting#contacts-disabled"},
		Handler: wrap("contacts.get_contact", "read", tools.NewHandleGetContact(rc, c.timeout)),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(true),
		},
		Schema: []mcp.ToolOption{
			mcp.WithString("contact_id",
				mcp.Required(),
				mcp.Description("The unique identifier of a saved personal contact, as returned by search."),
			),
			mcp.WithString("account", mcp.Description(contactsAccountDescription)),
			mcp.WithString("output",
				mcp.Description(contactsOutputDescription),
				mcp.Enum("text", "summary", "raw"),
			),
		},
	}
}

// buildListPeopleVerb constructs the list_people Verb.
func buildListPeopleVerb(c contactsVerbsConfig, rc graph.RetryConfig, wrap func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:        "list_people",
		Summary:     "list relevance-ranked people, most relevant correspondent first",
		Description: "Lists the people Microsoft Graph ranks as most relevant to the signed-in user, in the order Graph returns them; the order is the answer and is never re-sorted. These are inferred correspondents rather than saved contacts, so a person here may have no contact record. No required parameters; optional account, limit, skip, and output ('text' by default, 'summary', or 'raw'). Returns at most limit people (default 10, maximum 100), starting after skip. When Graph reports a further page, a second content block says \"more results available\" and gives the skip value for the next page. Read-only, non-destructive, idempotent, and open-world: it calls Microsoft Graph and writes nothing.",
		Examples: []tools.Example{
			{Args: map[string]any{}, Comment: "list the most relevant people"},
			{Args: map[string]any{"output": "summary"}, Comment: "return display names and primary addresses only"},
		},
		SeeDocs: []string{"concepts#output-tiers", "concepts#contacts-gating", "troubleshooting#contacts-disabled"},
		Handler: wrap("contacts.list_people", "read", tools.NewHandleListPeople(rc, c.timeout)),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(true),
		},
		Schema: []mcp.ToolOption{
			mcp.WithNumber("limit", mcp.Min(1), mcp.Max(100), mcp.DefaultNumber(10), mcp.Description("Maximum people to return, sent as $top. Default 10, maximum 100; values above 100 are clamped.")),
			mcp.WithNumber("skip", mcp.Min(0), mcp.DefaultNumber(0), mcp.Description("Number of ranked people to skip before the returned page, sent as $skip. Use the previous skip plus limit to read the next page.")),
			mcp.WithString("account", mcp.Description(contactsAccountDescription)),
			mcp.WithString("output",
				mcp.Description(contactsOutputDescription),
				mcp.Enum("text", "summary", "raw"),
			),
		},
	}
}

// buildGetPersonVerb constructs the get_person Verb.
func buildGetPersonVerb(c contactsVerbsConfig, rc graph.RetryConfig, wrap func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:        "get_person",
		Summary:     "get one relevance-ranked person by ID with all of its scored addresses",
		Description: "Fetches a single relevance-ranked person by its identifier and returns its display name and every entry of its scoredEmailAddresses collection. A scored address carries no name of its own, so each address is labelled with the person's display name. Use search or list_people to obtain an identifier. Requires person_id; optional account and output ('text' by default, 'summary', or 'raw'). Read-only, non-destructive, idempotent, and open-world: it calls Microsoft Graph and writes nothing.",
		Examples: []tools.Example{
			{Args: map[string]any{"person_id": "d4b8b3a0-..."}, Comment: "fetch a person returned by list_people"},
		},
		SeeDocs: []string{"concepts#output-tiers", "concepts#contacts-gating", "troubleshooting#contacts-disabled"},
		Handler: wrap("contacts.get_person", "read", tools.NewHandleGetPerson(rc, c.timeout)),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(true),
		},
		Schema: []mcp.ToolOption{
			mcp.WithString("person_id",
				mcp.Required(),
				mcp.Description("The unique identifier of a relevance-ranked person, as returned by list_people or search."),
			),
			mcp.WithString("account", mcp.Description(contactsAccountDescription)),
			mcp.WithString("output",
				mcp.Description(contactsOutputDescription),
				mcp.Enum("text", "summary", "raw"),
			),
		},
	}
}
