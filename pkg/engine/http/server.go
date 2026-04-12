package http

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/soheilhy/cmux"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
)


// Server wraps the standard http.Server to provide Astra-specific features.
type Server struct {
	*http.Server
	grpcServer *grpc.Server
}

// NewServer creates a new Astra HTTP server with TLS support.
func NewServer(addr string, handler http.Handler) *Server {
	tlsConfig := LoadTLSConfig()
	tlsCfg, _ := tlsConfig.GetTLSConfig()

	return &Server{
		Server: &http.Server{
			Addr:              addr,
			Handler:           h2c.NewHandler(handler, &http2.Server{}),
			ReadHeaderTimeout: 5 * time.Second,
			TLSConfig:         tlsCfg,
		},
	}
}


// ServeGRPC registers a gRPC server to be multiplexed on the same port as the
// HTTP server. When a gRPC server is registered, both Astra REST handlers and
// gRPC endpoints share a single TCP listener via cmux content-based routing.
//
// If grpcSrv is nil, this is a no-op and the server behaves as a pure HTTP server.
//
//	srv := http.NewServer(":8080", router)
//	srv.ServeGRPC(grpc.NewServer())
//	srv.Start(ctx) // now serves HTTP/1.1 + gRPC on :8080
func (s *Server) ServeGRPC(grpcSrv *grpc.Server) *Server {
	s.grpcServer = grpcSrv
	return s
}

// Start runs the server in a goroutine and returns nil.
// It complies with the framework.Starter interface.
//
// When a gRPC server has been registered via ServeGRPC, cmux is used to route
// HTTP/1.1 traffic to the Astra router and HTTP/2 gRPC traffic to the gRPC server
// on the same port, with zero configuration overhead for the application.
func (s *Server) Start(ctx context.Context) error {
	if s.grpcServer != nil {
		return s.startMuxed(ctx)
	}
	return s.startHTTPOnly(ctx)
}

// startHTTPOnly is the classic single-protocol startup path.
func (s *Server) startHTTPOnly(_ context.Context) error {

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

// startMuxed binds a single TCP listener and routes gRPC vs HTTP using cmux.
// gRPC traffic is detected by its Content-Type: application/grpc header.
// All other traffic (HTTP/1.1 and h2c) is routed to the HTTP handler.
func (s *Server) startMuxed(_ context.Context) error {

	ln, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return fmt.Errorf("astra: failed to listen on %s: %w", s.Addr, err)
	}

	m := cmux.New(ln)

	// gRPC connections are identified by "application/grpc" in their header.
	grpcL := m.MatchWithWriters(
		cmux.HTTP2MatchHeaderFieldSendSettings("content-type", "application/grpc"),
	)

	// All other traffic goes to the HTTP server.
	httpL := m.Match(cmux.Any())

	// Serve gRPC
	go func() {
		if err := s.grpcServer.Serve(grpcL); err != nil {
			slog.Error("Astra gRPC server error", "error", err)
		}
	}()

	// Serve HTTP
	go func() {
		httpSrv := &http.Server{
			Handler:           h2c.NewHandler(s.Handler, &http2.Server{}),
			ReadHeaderTimeout: s.ReadHeaderTimeout,
			TLSConfig:         s.TLSConfig,
		}

		if err := httpSrv.Serve(httpL); err != nil && err != http.ErrServerClosed {
			slog.Error("Astra HTTP server error (muxed)", "error", err)
		}
	}()

	// Start the mux router
	go func() {
		if err := m.Serve(); err != nil {
			slog.Error("Astra cmux error", "error", err)
		}
	}()

	slog.Info("Astra server started (HTTP + gRPC multiplexed)", "addr", s.Addr)
	return nil
}
