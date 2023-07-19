package internal

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"
)

// Server wrapper http.Server and net.Listener to make access to
// certain internal fields more easily accessible.
type Server struct {
	srv      *http.Server
	listener net.Listener
}

// NewServer creates an http server with a reverse proxy handler.
// We split the live server and proxy handler for testability.
func NewServer(target *url.URL) *Server {
	proxy := NewProxy(target)

	srv := &http.Server{
		Handler:           proxy,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
	}

	return &Server{
		srv: srv,
	}
}

// Listen creates a listener on the given address.
// It stores the listener for later calls to Serve,
// and to allow programmatic retrieval of the listening address
// for cases where it is randomized (e.g. ':0').
func (s *Server) Listen(address string) error {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to create listener: %s", err)
	}
	s.listener = listener
	return nil
}

// Serve starts the http server with the existing listener.
func (s *Server) Serve() error {
	if s.listener == nil {
		return fmt.Errorf("must call Listen() before Serve()")
	}
	return s.srv.Serve(s.listener)
}

// ListenAndServe is a convenience method for Listen() and Serve().
// Listen and Serve primarily make sense to call separately in testing,
// so the listener can allocate a port before we spin of Serve() in a
// goroutine. In production, you'd likely only need listen and server,
// unless you want programmatic access to the listener address (e.g. multiple servers).
func (s *Server) ListenAndServe(address string) error {
	if err := s.Listen(address); err != nil {
		return err
	}
	return s.srv.Serve(s.listener)
}

// Shutdown cleanly shuts down the server. It's primarily used for testing.
func (s *Server) Shutdown(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return s.srv.Shutdown(ctx)
}

// URL returns the server listening URL when a random port is used.
// This allows programmatic randomization of ports during testing.
func (s *Server) URL() string {
	return fmt.Sprintf("http://%s", s.listener.Addr().String())
}
