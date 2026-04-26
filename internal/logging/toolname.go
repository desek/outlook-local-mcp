package logging

import (
	"context"
	"log/slog"
)

// toolNameKey is the unexported context key type for the FQN tool name.
// Using a private type avoids collisions with other context values.
type toolNameKey struct{}

// WithToolName returns a copy of ctx with the fully qualified tool name (FQN)
// stored under the package-private toolNameKey. The FQN follows the
// "{domain}.{operation}" convention (e.g. "calendar.create_event").
//
// Parameters:
//   - ctx: the parent context.
//   - fqn: the fully qualified tool name to store.
//
// Returns the derived context containing the FQN.
func WithToolName(ctx context.Context, fqn string) context.Context {
	return context.WithValue(ctx, toolNameKey{}, fqn)
}

// ToolName retrieves the FQN tool name stored by WithToolName. It returns an
// empty string when no tool name has been set in ctx.
//
// Parameters:
//   - ctx: the context from which to read the tool name.
//
// Returns the FQN string, or "" if not set.
func ToolName(ctx context.Context) string {
	v, _ := ctx.Value(toolNameKey{}).(string)
	return v
}

// Logger returns an *slog.Logger derived from slog.Default() with the "tool"
// attribute set to the FQN stored in ctx. When ctx carries no tool name,
// slog.Default() is returned unchanged so that callers never have to guard
// against a nil logger.
//
// Parameters:
//   - ctx: the context from which to read the tool name.
//
// Returns a logger with "tool" set to the FQN, or slog.Default() when no name
// is stored.
func Logger(ctx context.Context) *slog.Logger {
	name := ToolName(ctx)
	if name == "" {
		return slog.Default()
	}
	return slog.Default().With("tool", name)
}
