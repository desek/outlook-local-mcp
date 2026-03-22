// Package config provides configuration loading and validation for the
// Outlook Calendar MCP Server. All configuration values are read from
// environment variables with sensible defaults. The Config struct is
// loaded once at startup via LoadConfig and validated via ValidateConfig
// before being passed to subsystem initializers.
package config
