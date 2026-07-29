// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the about handler for the system.about verb (CR-0067),
// a read-only, no-Graph verb that returns a snapshot of build identity and host
// environment metadata in three output tiers.
package tools

import (
	"context"
	"encoding/json"

	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/desek/outlook-local-mcp/internal/buildinfo"
	"github.com/desek/outlook-local-mcp/internal/logging"
	"github.com/mark3labs/mcp-go/mcp"
)

// HandleAbout returns a handler for the system.about verb. The handler is
// read-only, makes no Microsoft Graph call, and is safe to invoke without
// authentication.
//
// Parameters:
//   - version: the binary version string injected at build time.
//   - commit: the short Git commit SHA injected at build time.
//   - buildDate: the RFC3339 UTC build timestamp injected at build time.
//
// The auth backend is read at request time from auth.ActiveBackend() so it
// reflects the resolved backend even when called early in startup.
//
// Returns a handler compatible with the MCP server dispatch layer.
func HandleAbout(version, commit, buildDate string) func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := logging.Logger(ctx)
		logger.Debug("tool called")

		outputMode, err := ValidateOutputMode(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		info := buildinfo.Snapshot(version, commit, buildDate, auth.ActiveBackend())

		switch outputMode {
		case "summary":
			summary := map[string]string{
				"version":      info.Version,
				"commit":       info.Commit,
				"buildDate":    info.BuildDate,
				"goVersion":    info.GoVersion,
				"os":           info.OS,
				"arch":         info.Arch,
				"runtime":      info.Runtime,
				"distribution": info.Distribution,
				"authBackend":  info.AuthBackend,
				"homepage":     info.Homepage,
				"issueTracker": info.IssueTracker,
				"docsBase":     info.DocsBase,
			}
			data, err := json.Marshal(summary)
			if err != nil {
				return mcp.NewToolResultError("failed to serialize about summary"), nil
			}
			logger.Info("tool completed", "output", "summary")
			return mcp.NewToolResultText(string(data)), nil

		case "raw":
			data, err := json.Marshal(info)
			if err != nil {
				return mcp.NewToolResultError("failed to serialize about raw"), nil
			}
			logger.Info("tool completed", "output", "raw")
			return mcp.NewToolResultText(string(data)), nil

		default:
			logger.Info("tool completed", "output", "text")
			return mcp.NewToolResultText(FormatAboutText(info)), nil
		}
	}
}
