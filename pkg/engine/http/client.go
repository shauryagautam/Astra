// Package http provides HTTP transport utilities for the Astra framework.
//
// In addition to the inbound-traffic helpers (router, middleware, context),
// this file adds a resilient outbound client factory that mirrors the same
// levels of observability and fault-tolerance applied to incoming requests:
//
//   - Distributed trace headers (W3C TraceContext + Baggage) are injected on
//     every outbound call so that downstream services appear in the same trace.
//   - A per-client circuit breaker (backed by fault_tolerance.CircuitBreaker)
//     trips automatically when a downstream dependency is unhealthy, preventing
//     cascading failures inside the Astra application.
package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/shauryagautam/Astra/pkg/observability/fault_tolerance"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

// ─── Client Options ──────────────────────────────────────────────────────────

// ClientOption is a functional configuration option for NewResilientClient.
type ClientOption func(*clientConfig)

type clientConfig struct {
	// name is used to namespace the circuit breaker and OTel span names.
	name string

	// timeout is the per-request timeout applied by the http.Client.
	timeout time.Duration

	// maxFailures is forwarded to the CircuitBreaker constructor.
	maxFailures int

	// resetTimeout is forwarded to the CircuitBreaker constructor.
	resetTimeout time.Duration

	// circuitBreaker allows callers to supply a pre-configured CircuitBreaker
	// (e.g. one backed by a Redis state store for cluster-wide coordination).
	// When nil, an in-memory breaker is created automatically.
	circuitBreaker *fault_tolerance.CircuitBreaker

	// transport is the base transport to wrap. Defaults to http.DefaultTransport.
	transport http.RoundTripper

	// tracer is the OTel tracer used for outbound spans.
	// Defaults to otel.Tracer("astra/http-client").
	tracer trace.Tracer
}

func defaultClientConfig(name string) *clientConfig {
	return &clientConfig{
		name:         name,
		timeout:      30 * time.Second,
		maxFailures:  5,
		resetTimeout: 30 * time.Second,
		transport:    http.DefaultTransport,
		tracer:       otel.Tracer("astra/http-client"),
	}
}

// WithName sets the logical name used for circuit-breaker keying and OTel span naming.
func WithName(name string) ClientOption {
	return func(c *clientConfig) { c.name = name }
}

// WithTimeout sets the per-request deadline applied by the returned *http.Client.
func WithTimeout(d time.Duration) ClientOption {
	return func(c *clientConfig) { c.timeout = d }
}

// WithMaxFailures configures the failure threshold that trips the circuit breaker.
func WithMaxFailures(n int) ClientOption {
	return func(c *clientConfig) { c.maxFailures = n }
}

// WithResetTimeout configures how long the circuit breaker stays open before
// transitioning to the half-open probe state.
func WithResetTimeout(d time.Duration) ClientOption {
	return func(c *clientConfig) { c.resetTimeout = d }
}

// WithCircuitBreaker injects a pre-built CircuitBreaker, for example one that
// uses a Redis StateStore for distributed cluster-wide state sharing.
func WithCircuitBreaker(cb *fault_tolerance.CircuitBreaker) ClientOption {
	return func(c *clientConfig) { c.circuitBreaker = cb }
}

// WithBaseTransport replaces the underlying http.RoundTripper (useful in tests).
func WithBaseTransport(t http.RoundTripper) ClientOption {
	return func(c *clientConfig) { c.transport = t }
}

// WithTracer replaces the OTel tracer used for outbound spans.
func WithTracer(t trace.Tracer) ClientOption {
	return func(c *clientConfig) { c.tracer = t }
}

// ─── Factory ─────────────────────────────────────────────────────────────────

// NewResilientClient returns a *http.Client pre-configured with a two-layer
// RoundTripper middleware chain:
//
//  1. otelRoundTripper — extracts the caller's active span from the request
//     context, starts a child CLIENT span, and injects W3C TraceContext /
//     Baggage propagation headers so that downstream services appear in the
//     same distributed trace.
//
//  2. circuitBreakerRoundTripper — wraps the actual network call with a
//     fault_tolerance.CircuitBreaker. When the breaker is open, the request
//     is rejected immediately with ErrCircuitOpen rather than blocking the
//     caller until a connection timeout fires.
//
// The returned client is safe to share across goroutines.
//
// Example (standalone, no *App dependency required):
//
//	client := http.NewResilientClient(
//	    http.WithName("payment-service"),
//	    http.WithTimeout(10*time.Second),
//	    http.WithMaxFailures(3),
//	)
//	resp, err := client.Get("https://payments.internal/charge")
func NewResilientClient(opts ...ClientOption) *http.Client {
	cfg := defaultClientConfig("astra-outbound")
	for _, o := range opts {
		o(cfg)
	}

	// Build or adopt the circuit breaker.
	cb := cfg.circuitBreaker
	if cb == nil {
		cb = fault_tolerance.NewCircuitBreaker(cfg.name).
			WithMaxFailures(cfg.maxFailures).
			WithResetTimeout(cfg.resetTimeout)
	}

	// Stack the transport middleware (innermost first):
	//   base transport → circuitBreaker → otel
	transport := &circuitBreakerRoundTripper{
		name:    cfg.name,
		cb:      cb,
		wrapped: cfg.transport,
	}
	traced := &otelRoundTripper{
		name:    cfg.name,
		tracer:  cfg.tracer,
		wrapped: transport,
	}

	return &http.Client{
		Timeout:   cfg.timeout,
		Transport: traced,
	}
}

// ─── OTel RoundTripper ────────────────────────────────────────────────────────

// otelRoundTripper propagates the caller's distributed trace onto the outbound
// request and records a CLIENT span for each call.
type otelRoundTripper struct {
	name    string
	tracer  trace.Tracer
	wrapped http.RoundTripper
}

func (t *otelRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	spanName := fmt.Sprintf("%s %s %s", t.name, req.Method, req.URL.Host)

	ctx, span := t.tracer.Start(req.Context(), spanName,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.HTTPRequestMethodKey.String(req.Method),
			semconv.ServerAddress(req.URL.Host),
			semconv.URLFull(req.URL.String()),
		),
	)
	defer span.End()

	// Inject W3C TraceContext + Baggage propagation headers.
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	resp, err := t.wrapped.RoundTrip(req.WithContext(ctx))
	if err != nil {
		// Distinguish circuit-open errors from genuine network errors in traces.
		if errors.Is(err, fault_tolerance.ErrCircuitOpen) {
			span.SetStatus(codes.Error, "circuit breaker open")
			span.SetAttributes(attribute.Bool("circuit_breaker.open", true))
		} else {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		return nil, err
	}

	span.SetAttributes(semconv.HTTPResponseStatusCode(resp.StatusCode))
	if resp.StatusCode >= http.StatusInternalServerError {
		span.SetStatus(codes.Error, http.StatusText(resp.StatusCode))
	} else {
		span.SetStatus(codes.Ok, "")
	}
	return resp, nil
}

// ─── Circuit Breaker RoundTripper ─────────────────────────────────────────────

// circuitBreakerRoundTripper wraps each outbound request with circuit-breaker
// semantics. It propagates a request-scoped context into the breaker's Execute
// method so that per-request deadlines are respected while tracking state.
type circuitBreakerRoundTripper struct {
	name    string
	cb      *fault_tolerance.CircuitBreaker
	wrapped http.RoundTripper
}

func (t *circuitBreakerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	var resp *http.Response

	err := t.cb.Execute(req.Context(), func() error {
		var innerErr error
		resp, innerErr = t.wrapped.RoundTrip(req)
		if innerErr != nil {
			return innerErr
		}
		// Treat 5xx responses as failures so the breaker counts them.
		if resp.StatusCode >= http.StatusInternalServerError {
			return fmt.Errorf("upstream %s %s returned HTTP %d",
				req.Method, req.URL.Host, resp.StatusCode)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// ─── Context-aware helper ─────────────────────────────────────────────────────

// DoWithContext is a convenience wrapper that sets a per-call context deadline
// and issues the request through the supplied client. It respects any timeout
// already encoded in ctx (e.g. the Astra shutdown context) and falls back to
// the client's own Timeout field.
func DoWithContext(ctx context.Context, client *http.Client, req *http.Request) (*http.Response, error) {
	return client.Do(req.WithContext(ctx))
}
