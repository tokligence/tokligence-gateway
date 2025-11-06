package testutil

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"
)

type IPv4Server struct {
	URL       string
	listener  net.Listener
	server    *http.Server
	transport *http.Transport
	client    *http.Client
}

// NewIPv4Server starts an HTTP server bound to the IPv4 loopback interface.
func NewIPv4Server(t *testing.T, handler http.Handler) *IPv4Server {
	t.Helper()
	if handler == nil {
		handler = http.NewServeMux()
	}
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("skipping test: tcp4 loopback unavailable (%v)", err)
	}
	transport := &http.Transport{}
	s := &IPv4Server{
		URL:       "http://" + l.Addr().String(),
		listener:  l,
		server:    &http.Server{Handler: handler},
		transport: transport,
		client:    &http.Client{Transport: transport},
	}
	go func() {
		if err := s.server.Serve(l); err != nil && !errors.Is(err, http.ErrServerClosed) {
			// Log via testing.T to aid debugging without failing the test.
			t.Logf("IPv4Server serve error: %v", err)
		}
	}()
	return s
}

// Client returns an HTTP client configured for the server.
func (s *IPv4Server) Client() *http.Client {
	return s.client
}

// Close shuts down the underlying server and frees resources.
func (s *IPv4Server) Close() {
	_ = s.server.Shutdown(context.Background())
	s.transport.CloseIdleConnections()
}
