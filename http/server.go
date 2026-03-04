package http

import (
	"net/http"
)

// ListenAndServe is a helper to start the net/http server.
func ListenAndServe(addr string, handler http.Handler) error {
	return http.ListenAndServe(addr, handler)
}
