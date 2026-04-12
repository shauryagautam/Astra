package http

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// PrometheusHandler returns an Astra HandlerFunc that serves Prometheus metrics.
//
// Usage:
//
//	router.Handle(http.MethodGet, "/metrics", http.PrometheusHandler())
func PrometheusHandler() http.Handler {
	return promhttp.Handler()
}
