package middleware

import (
	"net/http"

	"github.com/astraframework/astra/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// OpenTelemetry returns a middleware that injects OTEL tracing into the request.
func OpenTelemetry() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))

			tracer := telemetry.GetTracer()
			ctx, span := tracer.Start(ctx, r.Method+" "+r.URL.Path,
				trace.WithSpanKind(trace.SpanKindServer),
			)
			defer span.End()

			// Inject tracing context back into request
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
