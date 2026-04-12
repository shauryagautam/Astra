package trace

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

var (
	globalTracer trace.Tracer
)

// Init initializes the global OpenTelemetry tracer.
// It returns a shutdown function that should be called when the application exits.
func Init(ctx context.Context, endpoint, serviceName string) (func(context.Context) error, error) {
	if endpoint == "" {
		// No tracing configured, use no-op tracer
		globalTracer = otel.Tracer("astra-noop")
		return func(ctx context.Context) error { return nil }, nil
	}

	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(), // Adjust if TLS is needed
	)
	if err != nil {
		return nil, fmt.Errorf("trace: failed to create OTLP exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("trace: failed to create resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	globalTracer = tp.Tracer("astra")

	return tp.Shutdown, nil
}

// GetTracer returns the global tracer instance.
func GetTracer() trace.Tracer {
	if globalTracer == nil {
		return otel.Tracer("astra-uninitialized")
	}
	return globalTracer
}

// StartSpan starts a new span with the given name.
func StartSpan(ctx context.Context, name string) (context.Context, trace.Span) {
	return GetTracer().Start(ctx, name)
}
