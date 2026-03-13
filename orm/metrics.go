package orm

import (
	"context"
	"database/sql"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// ObservePool registers OTel metrics for a database connection pool.
func ObservePool(db *sql.DB, meter metric.Meter, driverName string) error {
	if db == nil || meter == nil {
		return nil
	}

	commonAttrs := []attribute.KeyValue{
		attribute.String("db.system", driverName),
	}

	// 1. Max open connections
	_, err := meter.Int64ObservableGauge("db.client.connections.max",
		metric.WithDescription("Maximum number of open connections to the database"),
		metric.WithInt64Callback(func(_ context.Context, obs metric.Int64Observer) error {
			stats := db.Stats()
			obs.Observe(int64(stats.MaxOpenConnections), metric.WithAttributes(commonAttrs...))
			return nil
		}),
	)
	if err != nil {
		return fmt.Errorf("orm/metrics: failed to create max connections gauge: %w", err)
	}

	// 2. Open connections
	_, err = meter.Int64ObservableGauge("db.client.connections.open",
		metric.WithDescription("Number of settled connections, both in-use and idle"),
		metric.WithInt64Callback(func(_ context.Context, obs metric.Int64Observer) error {
			stats := db.Stats()
			obs.Observe(int64(stats.OpenConnections), metric.WithAttributes(commonAttrs...))
			return nil
		}),
	)
	if err != nil {
		return fmt.Errorf("orm/metrics: failed to create open connections gauge: %w", err)
	}

	// 3. In-use connections
	_, err = meter.Int64ObservableGauge("db.client.connections.usage",
		metric.WithDescription("Number of connections currently in-use"),
		metric.WithInt64Callback(func(_ context.Context, obs metric.Int64Observer) error {
			stats := db.Stats()
			obs.Observe(int64(stats.InUse), metric.WithAttributes(commonAttrs...))
			return nil
		}),
	)
	if err != nil {
		return fmt.Errorf("orm/metrics: failed to create usage gauge: %w", err)
	}

	// 4. Idle connections
	_, err = meter.Int64ObservableGauge("db.client.connections.idle",
		metric.WithDescription("Number of idle connections"),
		metric.WithInt64Callback(func(_ context.Context, obs metric.Int64Observer) error {
			stats := db.Stats()
			obs.Observe(int64(stats.Idle), metric.WithAttributes(commonAttrs...))
			return nil
		}),
	)
	if err != nil {
		return fmt.Errorf("orm/metrics: failed to create idle gauge: %w", err)
	}

	// 5. Total wait count
	_, err = meter.Int64ObservableCounter("db.client.connections.wait_count",
		metric.WithDescription("Total number of connections waited for"),
		metric.WithInt64Callback(func(_ context.Context, obs metric.Int64Observer) error {
			stats := db.Stats()
			obs.Observe(stats.WaitCount, metric.WithAttributes(commonAttrs...))
			return nil
		}),
	)
	if err != nil {
		return fmt.Errorf("orm/metrics: failed to create wait count counter: %w", err)
	}

	// 6. Total wait duration
	_, err = meter.Float64ObservableCounter("db.client.connections.wait_duration",
		metric.WithDescription("Total time blocked waiting for a new connection"),
		metric.WithUnit("ms"),
		metric.WithFloat64Callback(func(_ context.Context, obs metric.Float64Observer) error {
			stats := db.Stats()
			obs.Observe(float64(stats.WaitDuration.Milliseconds()), metric.WithAttributes(commonAttrs...))
			return nil
		}),
	)
	if err != nil {
		return fmt.Errorf("orm/metrics: failed to create wait duration counter: %w", err)
	}

	return nil
}
