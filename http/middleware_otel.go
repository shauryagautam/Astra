package http

import (
	"github.com/astraframework/astra/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// OpenTelemetry returns a middleware that injects OTEL tracing into the request.
func OpenTelemetry() MiddlewareFunc {
	return func(next HandlerFunc) HandlerFunc {
		return func(c *Context) error {
			ctx := otel.GetTextMapPropagator().Extract(c.Request.Context(), propagation.HeaderCarrier(c.Request.Header))

			tracer := telemetry.GetTracer()
			ctx, span := tracer.Start(ctx, c.Request.Method+" "+c.Request.URL.Path,
				trace.WithSpanKind(trace.SpanKindServer),
			)
			defer span.End()

			c.Request = c.Request.WithContext(ctx)
			return next(c)
		}
	}
}
