package observability

import (
	"context"
	"strconv"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// ToolMetrics holds references to all five OpenTelemetry metric instruments
// used by the observability middleware and recording helpers.
type ToolMetrics struct {
	// toolCallsTotal counts the total number of tool invocations, partitioned
	// by tool_name and status (success/error).
	toolCallsTotal metric.Int64Counter

	// toolCallDuration records the wall-clock duration of each tool invocation
	// in seconds, partitioned by tool_name.
	toolCallDuration metric.Float64Histogram

	// graphAPICallsTotal counts the total number of Microsoft Graph API HTTP
	// calls, partitioned by method and status_code.
	graphAPICallsTotal metric.Int64Counter

	// graphAPIRetryTotal counts the total number of Graph API retry attempts,
	// partitioned by tool_name and attempt number.
	graphAPIRetryTotal metric.Int64Counter

	// activeRequests tracks the number of tool invocations currently in
	// progress. Value increases on entry, decreases on exit.
	activeRequests metric.Int64UpDownCounter
}

// InitMetrics creates all five metric instruments from the given Meter and
// returns a populated ToolMetrics struct.
//
// Parameters:
//   - meter: the OTEL Meter used to create metric instruments.
//
// Returns a ToolMetrics pointer with all instruments initialized, or an error
// if any instrument creation fails.
//
// Side effects: registers metric instruments with the provided Meter.
func InitMetrics(meter metric.Meter) (*ToolMetrics, error) {
	callsTotal, err := meter.Int64Counter("tool_calls_total",
		metric.WithDescription("Total number of tool invocations"),
	)
	if err != nil {
		return nil, err
	}

	callDuration, err := meter.Float64Histogram("tool_call_duration_seconds",
		metric.WithDescription("Wall-clock duration of tool invocations in seconds"),
	)
	if err != nil {
		return nil, err
	}

	apiCallsTotal, err := meter.Int64Counter("graph_api_calls_total",
		metric.WithDescription("Total number of Graph API HTTP calls"),
	)
	if err != nil {
		return nil, err
	}

	apiRetryTotal, err := meter.Int64Counter("graph_api_retry_total",
		metric.WithDescription("Total number of Graph API retry attempts"),
	)
	if err != nil {
		return nil, err
	}

	active, err := meter.Int64UpDownCounter("active_requests",
		metric.WithDescription("Number of tool invocations currently in progress"),
	)
	if err != nil {
		return nil, err
	}

	return &ToolMetrics{
		toolCallsTotal:     callsTotal,
		toolCallDuration:   callDuration,
		graphAPICallsTotal: apiCallsTotal,
		graphAPIRetryTotal: apiRetryTotal,
		activeRequests:     active,
	}, nil
}

// RecordGraphAPICall records a single Graph API HTTP call by incrementing
// the graph_api_calls_total counter with the given HTTP method and status code.
//
// Parameters:
//   - ctx: the context for metric recording.
//   - m: the ToolMetrics instance holding the counter.
//   - method: the HTTP method used (e.g., "GET", "POST", "DELETE").
//   - statusCode: the HTTP response status code (e.g., 200, 429).
//
// Side effects: increments the graph_api_calls_total counter.
func RecordGraphAPICall(ctx context.Context, m *ToolMetrics, method string, statusCode int) {
	m.graphAPICallsTotal.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("method", method),
			attribute.String("status_code", strconv.Itoa(statusCode)),
		),
	)
}

// RecordGraphAPIRetry records a Graph API retry attempt by incrementing the
// graph_api_retry_total counter with the given tool name and attempt number.
//
// Parameters:
//   - ctx: the context for metric recording.
//   - m: the ToolMetrics instance holding the counter.
//   - toolName: the name of the tool that triggered the retry.
//   - attempt: the retry attempt number (1-based).
//
// Side effects: increments the graph_api_retry_total counter.
func RecordGraphAPIRetry(ctx context.Context, m *ToolMetrics, toolName string, attempt int) {
	m.graphAPIRetryTotal.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("tool_name", toolName),
			attribute.String("attempt", strconv.Itoa(attempt)),
		),
	)
}
