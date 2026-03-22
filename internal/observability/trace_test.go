package observability

import (
	"context"
	"errors"
	"testing"

	"github.com/desek/outlook-local-mcp/internal/config"
	"github.com/mark3labs/mcp-go/mcp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric/noop"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

// TestInitOTEL_Disabled validates that when OTELEnabled is false, InitOTEL
// returns a noop shutdown function and nil error without creating exporters.
func TestInitOTEL_Disabled(t *testing.T) {
	cfg := config.Config{OTELEnabled: false}

	shutdown, err := InitOTEL(cfg)
	if err != nil {
		t.Fatalf("InitOTEL() error: %v", err)
	}
	if shutdown == nil {
		t.Fatal("expected non-nil shutdown function")
	}

	// The noop shutdown should succeed.
	if err := shutdown(context.Background()); err != nil {
		t.Errorf("shutdown() error: %v", err)
	}
}

// TestInitOTEL_Enabled validates that when OTELEnabled is true, InitOTEL
// returns a non-nil shutdown function and installs SDK providers.
func TestInitOTEL_Enabled(t *testing.T) {
	cfg := config.Config{
		OTELEnabled:     true,
		OTELEndpoint:    "localhost:4317",
		OTELServiceName: "test-service",
	}

	shutdown, err := InitOTEL(cfg)
	if err != nil {
		t.Fatalf("InitOTEL() error: %v", err)
	}
	if shutdown == nil {
		t.Fatal("expected non-nil shutdown function")
	}

	// Verify SDK providers are installed (not noop).
	tp := otel.GetTracerProvider()
	if _, ok := tp.(*sdktrace.TracerProvider); !ok {
		t.Errorf("expected *sdktrace.TracerProvider, got %T", tp)
	}

	// Shutdown to clean up global state. Flush errors are expected when no
	// OTLP collector is reachable in the test environment.
	_ = shutdown(context.Background())
}

// TestInitOTEL_DefaultEndpoint validates that an empty OTELEndpoint resolves to
// the default localhost:4317 without error.
func TestInitOTEL_DefaultEndpoint(t *testing.T) {
	cfg := config.Config{
		OTELEnabled:     true,
		OTELEndpoint:    "",
		OTELServiceName: "test-service",
	}

	shutdown, err := InitOTEL(cfg)
	if err != nil {
		t.Fatalf("InitOTEL() error: %v", err)
	}
	defer func() { _ = shutdown(context.Background()) }()
}

// TestWithObservability_Success validates that the middleware records success
// status when the wrapped handler returns a successful result.
func TestWithObservability_Success(t *testing.T) {
	meter := noop.NewMeterProvider().Meter("test")
	m, err := InitMetrics(meter)
	if err != nil {
		t.Fatalf("InitMetrics() error: %v", err)
	}

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	tracer := tp.Tracer("test")

	handler := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{}, nil
	}

	wrapped := WithObservability("test_tool", m, tracer, handler)
	result, err := wrapped(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("wrapped handler error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Flush spans.
	_ = tp.ForceFlush(context.Background())

	spans := exporter.GetSpans()
	if len(spans) == 0 {
		t.Fatal("expected at least one span")
	}

	span := spans[0]
	if span.Name != "test_tool" {
		t.Errorf("span name = %q, want %q", span.Name, "test_tool")
	}

	// Check span attributes.
	foundName := false
	foundStatus := false
	for _, attr := range span.Attributes {
		if string(attr.Key) == "tool.name" && attr.Value.AsString() == "test_tool" {
			foundName = true
		}
		if string(attr.Key) == "tool.status" && attr.Value.AsString() == "success" {
			foundStatus = true
		}
	}
	if !foundName {
		t.Error("expected tool.name attribute on span")
	}
	if !foundStatus {
		t.Error("expected tool.status=success attribute on span")
	}
}

// TestWithObservability_Error validates that the middleware records error status
// when the wrapped handler returns an error.
func TestWithObservability_Error(t *testing.T) {
	meter := noop.NewMeterProvider().Meter("test")
	m, err := InitMetrics(meter)
	if err != nil {
		t.Fatalf("InitMetrics() error: %v", err)
	}

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	tracer := tp.Tracer("test")

	handler := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return nil, errors.New("graph error")
	}

	wrapped := WithObservability("test_tool", m, tracer, handler)
	_, err = wrapped(context.Background(), mcp.CallToolRequest{})
	if err == nil {
		t.Fatal("expected error from wrapped handler")
	}

	_ = tp.ForceFlush(context.Background())

	spans := exporter.GetSpans()
	if len(spans) == 0 {
		t.Fatal("expected at least one span")
	}

	foundStatus := false
	for _, attr := range spans[0].Attributes {
		if string(attr.Key) == "tool.status" && attr.Value.AsString() == "error" {
			foundStatus = true
		}
	}
	if !foundStatus {
		t.Error("expected tool.status=error attribute on span")
	}
}

// TestWithObservability_ToolResultError validates that the middleware detects
// IsError on CallToolResult and records error status.
func TestWithObservability_ToolResultError(t *testing.T) {
	meter := noop.NewMeterProvider().Meter("test")
	m, err := InitMetrics(meter)
	if err != nil {
		t.Fatalf("InitMetrics() error: %v", err)
	}

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	tracer := tp.Tracer("test")

	handler := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultError("something failed"), nil
	}

	wrapped := WithObservability("test_tool", m, tracer, handler)
	result, err := wrapped(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("wrapped handler error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError to be true")
	}

	_ = tp.ForceFlush(context.Background())

	spans := exporter.GetSpans()
	if len(spans) == 0 {
		t.Fatal("expected at least one span")
	}

	foundStatus := false
	for _, attr := range spans[0].Attributes {
		if string(attr.Key) == "tool.status" && attr.Value.AsString() == "error" {
			foundStatus = true
		}
	}
	if !foundStatus {
		t.Error("expected tool.status=error attribute on span")
	}
}

// TestWithObservability_NoopTracer validates that the middleware works correctly
// with noop providers (zero overhead path).
func TestWithObservability_NoopTracer(t *testing.T) {
	meter := noop.NewMeterProvider().Meter("test")
	m, err := InitMetrics(meter)
	if err != nil {
		t.Fatalf("InitMetrics() error: %v", err)
	}
	tracer := tracenoop.NewTracerProvider().Tracer("test")

	called := false
	handler := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		called = true
		return &mcp.CallToolResult{}, nil
	}

	wrapped := WithObservability("test_tool", m, tracer, handler)
	_, err = wrapped(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("wrapped handler error: %v", err)
	}
	if !called {
		t.Error("expected handler to be called")
	}
}
