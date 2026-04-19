// Package tools_test validates CR-0056 contractual language in tool
// descriptions. Descriptions are part of the MCP contract shipped to the
// LLM; regressing them silently would re-introduce the "default account"
// assumption this CR exists to eliminate.
package tools_test

import (
	"strings"
	"testing"

	"github.com/desek/outlook-local-mcp/internal/tools"
)

// TestAccountParamDescription_ForbidsDefaultAssumption locks in FR-49/FR-50
// and AC-18: the shared `account` parameter description must explicitly
// forbid assuming a default account and must direct the LLM to consider
// disconnected accounts before acting.
func TestAccountParamDescription_ForbidsDefaultAssumption(t *testing.T) {
	desc := tools.AccountParamDescription

	// Must explicitly forbid silent default-account assumption.
	required := []string{
		"Never assume a default account",
		"disconnected",
	}
	for _, phrase := range required {
		if !strings.Contains(desc, phrase) {
			t.Errorf("AccountParamDescription missing required phrase %q\n  got: %s", phrase, desc)
		}
	}
}

// TestAccountLifecycleTools_DescribeProactiveSuggestion locks in FR-53: the
// descriptions of account_login, account_logout, account_refresh, and
// account_remove must direct the LLM to proactively suggest the tool to the
// user when the situation warrants it, rather than waiting to be asked.
func TestAccountLifecycleTools_DescribeProactiveSuggestion(t *testing.T) {
	cases := []struct {
		name        string
		description string
	}{
		{"account_login", tools.NewLoginAccountTool().Description},
		{"account_logout", tools.NewLogoutAccountTool().Description},
		{"account_refresh", tools.NewRefreshAccountTool().Description},
		{"account_remove", tools.NewRemoveAccountTool().Description},
	}
	for _, tc := range cases {
		if !strings.Contains(strings.ToLower(tc.description), "proactively suggest") {
			t.Errorf("%s description missing 'Proactively suggest' guidance\n  got: %s", tc.name, tc.description)
		}
	}
}
