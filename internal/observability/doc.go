// Package observability provides OpenTelemetry initialization and tool handler
// instrumentation for the Outlook Calendar MCP Server. When OTEL is disabled
// (the default), noop providers are installed with zero runtime overhead. When
// enabled, OTLP gRPC exporters push metrics and traces to an external
// collector. The WithObservability middleware wraps tool handlers with automatic
// metrics recording and distributed tracing.
package observability
