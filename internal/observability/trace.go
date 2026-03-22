package observability

import (
	"context"
	"log/slog"
	"time"

	"github.com/desek/outlook-local-mcp/internal/config"
	"github.com/mark3labs/mcp-go/mcp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/credentials/insecure"
)

// otelDefaultEndpoint is the default OTLP gRPC collector endpoint used when
// cfg.OTELEndpoint is empty and OTEL is enabled.
const otelDefaultEndpoint = "localhost:4317"

// InitOTEL initializes OpenTelemetry meter and tracer providers based on the
// server configuration. When cfg.OTELEnabled is false, noop global providers
// are set (the SDK default) and a noop shutdown function is returned.
//
// When cfg.OTELEnabled is true, OTLP gRPC exporters are created targeting
// cfg.OTELEndpoint (or localhost:4317 if empty), insecure transport is used,
// and SDK providers are installed as the global meter and tracer providers.
//
// Parameters:
//   - cfg: the server configuration containing OTEL settings.
//
// Returns a shutdown function that flushes pending telemetry and an error if
// initialization fails. The caller must defer the shutdown function.
//
// Side effects: sets global OTEL meter and tracer providers, creates gRPC
// connections to the OTLP endpoint when enabled.
func InitOTEL(cfg config.Config) (func(context.Context) error, error) {
	if !cfg.OTELEnabled {
		slog.Info("otel disabled, using noop providers")
		return func(context.Context) error { return nil }, nil
	}

	endpoint := cfg.OTELEndpoint
	if endpoint == "" {
		endpoint = otelDefaultEndpoint
	}

	ctx := context.Background()

	res, err := resource.New(ctx,
		resource.WithAttributes(semconv.ServiceName(cfg.OTELServiceName)),
	)
	if err != nil {
		return nil, err
	}

	// Create OTLP trace exporter.
	traceExporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithTLSCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}

	// Create OTLP metric exporter.
	metricExporter, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithEndpoint(endpoint),
		otlpmetricgrpc.WithTLSCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(mp)

	slog.Info("otel initialized", "endpoint", endpoint, "service_name", cfg.OTELServiceName)

	shutdown := func(ctx context.Context) error {
		var firstErr error
		if err := tp.Shutdown(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
		if err := mp.Shutdown(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
		return firstErr
	}

	return shutdown, nil
}

// WithObservability wraps a tool handler with automatic metrics recording and
// distributed tracing. For each invocation it creates a span, tracks active
// requests, records call duration, and increments the call counter with the
// appropriate success/error status.
//
// Parameters:
//   - name: the tool name used as the span name and metric label.
//   - metrics: the ToolMetrics instance holding all metric instruments.
//   - tracer: the OTEL tracer used to create spans.
//   - handler: the original tool handler function to wrap.
//
// Returns a new handler function with the same signature that transparently
// instruments the wrapped handler. When noop providers are active, the
// overhead is negligible.
//
// Side effects: creates spans, records metrics, increments/decrements the
// active requests counter.
func WithObservability(
	name string,
	metrics *ToolMetrics,
	tracer trace.Tracer,
	handler func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error),
) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		ctx, span := tracer.Start(ctx, name)
		defer span.End()

		span.SetAttributes(attribute.String("tool.name", name))
		metrics.activeRequests.Add(ctx, 1)
		defer metrics.activeRequests.Add(ctx, -1)

		start := time.Now()
		result, err := handler(ctx, request)
		duration := time.Since(start).Seconds()

		status := "success"
		if err != nil || (result != nil && result.IsError) {
			status = "error"
			span.SetStatus(codes.Error, status)
		} else {
			span.SetStatus(codes.Ok, "")
		}
		span.SetAttributes(attribute.String("tool.status", status))

		metrics.toolCallsTotal.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("tool_name", name),
				attribute.String("status", status),
			),
		)
		metrics.toolCallDuration.Record(ctx, duration,
			metric.WithAttributes(attribute.String("tool_name", name)),
		)

		return result, err
	}
}
