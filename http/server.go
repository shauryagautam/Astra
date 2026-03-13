package http

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

// Server wraps the standard http.Server to provide Astra-specific features.
type Server struct {
	*http.Server
}

// NewServer creates a new Astra HTTP server with TLS support.
func NewServer(addr string, handler http.Handler) *Server {
	tlsConfig := LoadTLSConfig()
	tlsCfg, _ := tlsConfig.GetTLSConfig()

	return &Server{
		Server: &http.Server{
			Addr:              addr,
			Handler:           handler,
			ReadHeaderTimeout: 5 * time.Second,
			TLSConfig:         tlsCfg,
		},
	}
}

// Start runs the server in a goroutine and returns nil.
// It complies with the core.Starter interface.
func (s *Server) Start(ctx context.Context) error {
	go func() {
		tlsConfig := LoadTLSConfig()
		var err error
		if tlsConfig.Enabled && tlsConfig.CertFile != "" && tlsConfig.KeyFile != "" {
			err = s.ListenAndServeTLS(tlsConfig.CertFile, tlsConfig.KeyFile)
		} else {
			err = s.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP server error", "error", err)
		}
	}()
	return nil
}
