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

// NewServer creates a new Astra HTTP server with sensible production timeouts and
// optional TLS support.
//
// maxBodyBytes limits the size of request bodies at the transport level before
// any handler is invoked. Pass 0 to disable (not recommended in production).
// The recommended value for most APIs is 10 MB (10 * 1024 * 1024).
func NewServer(addr string, handler http.Handler, maxBodyBytes int64) *Server {
	tlsConfig := LoadTLSConfig()
	tlsCfg, _ := tlsConfig.GetTLSConfig()

	// Auto-wrap the handler with MaxBodySize so every endpoint is protected
	// against request body flooding regardless of whether middleware is registered.
	// This is applied at the transport level, before routing.
	if maxBodyBytes > 0 {
		original := handler
		handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
			}
			original.ServeHTTP(w, r)
		})
	}

	return &Server{
		Server: &http.Server{
			Addr:    addr,
			Handler: h2c.NewHandler(handler, &http2.Server{}),

			// ReadHeaderTimeout guards against slowloris attacks where an
			// attacker slowly sends headers to hold the connection open.
			ReadHeaderTimeout: 5 * time.Second,

			// ReadTimeout is the maximum time to read the entire request
			// including the body. Set conservatively for upload workloads;
			// individual handlers can override via context deadline.
			ReadTimeout: 30 * time.Second,

			// WriteTimeout is the maximum duration before timing out writes of
			// the response. Includes the time to read the request body.
			WriteTimeout: 60 * time.Second,

			// IdleTimeout is the maximum amount of time to wait for the next
			// request on a keep-alive connection.
			IdleTimeout: 120 * time.Second,

			TLSConfig: tlsCfg,
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
// It respects the provided context: when ctx is cancelled the server is gracefully
// shut down using a 15-second budget, ensuring in-flight requests can complete.
func (s *Server) startHTTPOnly(ctx context.Context) error {
	tlsConfig := LoadTLSConfig()
	ln, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return fmt.Errorf("astra: failed to listen on %s: %w", s.Addr, err)
	}

	// Serve in background goroutine.
	go func() {
		var serveErr error
		if tlsConfig.Enabled && tlsConfig.CertFile != "" && tlsConfig.KeyFile != "" {
			serveErr = s.ServeTLS(ln, tlsConfig.CertFile, tlsConfig.KeyFile)
		} else {
			serveErr = s.Serve(ln)
		}
		if serveErr != nil && serveErr != http.ErrServerClosed {
			slog.Error("HTTP server error", "error", serveErr)
		}
	}()

	// Watch the context: when it is cancelled (e.g. SIGTERM / App.Shutdown())
	// initiate a graceful shutdown so in-flight requests are given time to finish.
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := s.Server.Shutdown(shutdownCtx); err != nil {
			slog.Error("astra: HTTP server shutdown error", "error", err)
		}
	}()

	return nil
}

// startMuxed binds a single TCP listener and routes gRPC vs HTTP using cmux.
// gRPC traffic is detected by its Content-Type: application/grpc header.
// All other traffic (HTTP/1.1 and h2c) is routed to the HTTP handler.
func (s *Server) startMuxed(ctx context.Context) error {
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
			ReadTimeout:       s.ReadTimeout,
			WriteTimeout:      s.WriteTimeout,
			IdleTimeout:       s.IdleTimeout,
			TLSConfig:         s.TLSConfig,
		}

		if err := httpSrv.Serve(httpL); err != nil && err != http.ErrServerClosed {
			slog.Error("Astra HTTP server error (muxed)", "error", err)
		}
	}()

	// Start the mux router
	go func() {
		if err := m.Serve(); err != nil {
			// cmux returns an error when the underlying listener is closed,
			// which is expected during graceful shutdown — log only unexpected errors.
			if ctx.Err() == nil {
				slog.Error("Astra cmux error", "error", err)
			}
		}
	}()

	// Graceful shutdown: close the cmux listener when the context is cancelled.
	go func() {
		<-ctx.Done()
		ln.Close()
		if s.grpcServer != nil {
			s.grpcServer.GracefulStop()
		}
	}()

	slog.Info("Astra server started (HTTP + gRPC multiplexed)", "addr", s.Addr)
	return nil
}

// Shutdown gracefully shuts down both the HTTP server and the gRPC server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}
	if s.Server != nil {
		return s.Server.Shutdown(ctx)
	}
	return nil
}
