package config

import (
	"log/slog"
	"strings"
)

// WellKnownClientIDs maps friendly names to Microsoft 365 application client
// IDs. These allow users to configure OUTLOOK_MCP_CLIENT_ID by name instead
// of raw UUID.
var WellKnownClientIDs = map[string]string{
	"outlook-local-mcp": "dd5fc5c5-eb9a-4f6f-97bd-1a9fecb277d3",
	"teams-desktop":     "1fec8e78-bce4-4aaf-ab1b-5451cc387264",
	"teams-web":         "5e3ce6c0-2b1f-4285-8d4b-75ee78787346",
	"m365-web":          "4765445b-32c6-49b0-83e6-1d93765276ca",
	"m365-desktop":      "0ec893e0-5785-4de6-99da-4ed124e5296c",
	"m365-mobile":       "d3590ed6-52b3-4102-aeff-aad2292ab01c",
	"outlook-desktop":   "d3590ed6-52b3-4102-aeff-aad2292ab01c",
	"outlook-web":       "bc59ab01-8403-45c6-8796-ac3ef710b3e3",
	"outlook-mobile":    "27922004-5251-4030-b22d-91ecd9a37ea4",
}

// ResolveClientID resolves a client ID value to a UUID. If the value matches
// a well-known friendly name (case-insensitive), the corresponding UUID is
// returned. If the value contains a hyphen (looks like a UUID), it is returned
// as-is. Otherwise, the value is returned as-is with a warning logged listing
// valid well-known names.
//
// Parameters:
//   - value: the client ID string from configuration, either a friendly name or UUID.
//
// Returns the resolved UUID string, or the original value if not a well-known name.
//
// Side effects: logs a warning via slog.Warn when the value is an unrecognized
// non-UUID string.
func ResolveClientID(value string) string {
	lower := strings.ToLower(value)
	if uuid, ok := WellKnownClientIDs[lower]; ok {
		return uuid
	}

	if strings.Contains(value, "-") {
		return value
	}

	names := make([]string, 0, len(WellKnownClientIDs))
	for name := range WellKnownClientIDs {
		names = append(names, name)
	}
	slog.Warn("unrecognized client ID, not a well-known name or UUID",
		"value", value,
		"valid_names", names,
	)

	return value
}
