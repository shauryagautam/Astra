package metrics

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

var (
	globalMeter metric.Meter
)

// Init initializes the global OpenTelemetry meter.
// It returns a shutdown function that should be called when the application exits.
func Init(ctx context.Context, endpoint, serviceName string) (func(context.Context) error, error) {
	if endpoint == "" {
		// No metrics configured, use no-op meter
		globalMeter = otel.Meter("astra-noop")
		return func(ctx context.Context) error { return nil }, nil
	}

	exporter, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithEndpoint(endpoint),
		otlpmetricgrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("metrics: failed to create OTLP metric exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("metrics: failed to create resource: %w", err)
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter)),
		sdkmetric.WithResource(res),
	)

	otel.SetMeterProvider(mp)
	globalMeter = mp.Meter("astra")

	return mp.Shutdown, nil
}

// GetMeter returns the global meter instance.
func GetMeter() metric.Meter {
	if globalMeter == nil {
		return otel.Meter("astra-uninitialized")
	}
	return globalMeter
}
