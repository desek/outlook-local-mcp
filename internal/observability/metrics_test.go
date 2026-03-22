package observability

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/metric/noop"
)

// TestInitMetrics_AllInstruments validates that InitMetrics creates all five
// metric instruments without error when given a noop meter.
func TestInitMetrics_AllInstruments(t *testing.T) {
	meter := noop.NewMeterProvider().Meter("test")

	m, err := InitMetrics(meter)
	if err != nil {
		t.Fatalf("InitMetrics() error: %v", err)
	}
	if m == nil {
		t.Fatal("expected non-nil *ToolMetrics")
	}
	if m.toolCallsTotal == nil {
		t.Error("toolCallsTotal is nil")
	}
	if m.toolCallDuration == nil {
		t.Error("toolCallDuration is nil")
	}
	if m.graphAPICallsTotal == nil {
		t.Error("graphAPICallsTotal is nil")
	}
	if m.graphAPIRetryTotal == nil {
		t.Error("graphAPIRetryTotal is nil")
	}
	if m.activeRequests == nil {
		t.Error("activeRequests is nil")
	}
}

// TestRecordGraphAPICall validates that RecordGraphAPICall does not panic
// when recording a Graph API call with method and status code labels.
func TestRecordGraphAPICall(t *testing.T) {
	meter := noop.NewMeterProvider().Meter("test")
	m, err := InitMetrics(meter)
	if err != nil {
		t.Fatalf("InitMetrics() error: %v", err)
	}

	// Must not panic.
	RecordGraphAPICall(context.Background(), m, "GET", 200)
	RecordGraphAPICall(context.Background(), m, "POST", 429)
	RecordGraphAPICall(context.Background(), m, "DELETE", 404)
}

// TestRecordGraphAPIRetry validates that RecordGraphAPIRetry does not panic
// when recording a retry attempt with tool name and attempt number labels.
func TestRecordGraphAPIRetry(t *testing.T) {
	meter := noop.NewMeterProvider().Meter("test")
	m, err := InitMetrics(meter)
	if err != nil {
		t.Fatalf("InitMetrics() error: %v", err)
	}

	// Must not panic.
	RecordGraphAPIRetry(context.Background(), m, "list_events", 1)
	RecordGraphAPIRetry(context.Background(), m, "list_events", 2)
	RecordGraphAPIRetry(context.Background(), m, "get_event", 3)
}
