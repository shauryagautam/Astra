package http

import (
	"net/http"

	"github.com/shauryagautam/Astra/pkg/observability/trace"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	stdtrace "go.opentelemetry.io/otel/trace"
)

// OpenTelemetry returns a middleware that injects OTEL tracing into the request.
func OpenTelemetry() MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))

			tracer := trace.GetTracer()
			ctx, span := tracer.Start(ctx, r.Method+" "+r.URL.Path,
				stdtrace.WithSpanKind(stdtrace.SpanKindServer),
			)
			defer span.End()

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
