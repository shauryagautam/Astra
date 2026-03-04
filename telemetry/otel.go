package telemetry

import (
	"context"
	"net/http"

	"github.com/astraframework/astra/config"
	"github.com/astraframework/astra/core"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
)

// OTel manages OpenTelemetry configuration.
type OTel struct {
	tp  *tracesdk.TracerProvider
	cfg config.TelemetryConfig
}

// Name returns the service name.
func (o *OTel) Name() string {
	return "otel"
}

// Start initializes OpenTelemetry.
func (o *OTel) Start(ctx context.Context) error {
	tp, err := InitTracer(o.cfg)
	if err != nil {
		return err
	}
	o.tp = tp
	return nil
}

// Stop shuts down OpenTelemetry.
func (o *OTel) Stop(ctx context.Context) error {
	if o.tp != nil {
		return o.tp.Shutdown(ctx)
	}
	return nil
}

// Register registers the OTel service.
func Register(app *core.App) {
	app.Register("otel", &OTel{
		cfg: app.Config.Telemetry,
	})
}

// InitTracer initializes a functional but minimal OpenTelemetry setup.
func InitTracer(cfg config.TelemetryConfig) (*tracesdk.TracerProvider, error) {
	var exporter tracesdk.SpanExporter
	var err error

	if cfg.Endpoint == "" {
		// If cfg.Endpoint == "" → use stdout exporter (dev mode)
		exporter, err = stdouttrace.New(stdouttrace.WithPrettyPrint())
	} else {
		// If cfg.Endpoint != "" → use OTLP HTTP exporter
		exporter, err = otlptracehttp.New(context.Background(),
			otlptracehttp.WithEndpoint(cfg.Endpoint),
			otlptracehttp.WithInsecure(), // Common for OTLP development/internal endpoints
		)
	}
	if err != nil {
		return nil, err
	}

	tp := tracesdk.NewTracerProvider(
		tracesdk.WithBatcher(exporter),
		tracesdk.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(cfg.ServiceName),
		)),
	)

	// Register as global tracer provider
	otel.SetTracerProvider(tp)
	return tp, nil
}

// TraceMiddleware provides middleware for automatic HTTP span creation.
func TraceMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tracer := otel.Tracer("astra")
		ctx, span := tracer.Start(r.Context(), r.Method+" "+r.URL.Path,
			trace.WithAttributes(
				attribute.String("http.method", r.Method),
				attribute.String("http.url", r.URL.String()),
			),
		)
		defer span.End()

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
