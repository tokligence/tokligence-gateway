package httpserver

import (
	"net/http"

	"github.com/tokligence/tokligence-gateway/internal/httpserver/protocol"
)

type healthEndpoint struct {
	server *Server
}

func newHealthEndpoint(server *Server) protocol.Endpoint {
	return &healthEndpoint{server: server}
}

func (e *healthEndpoint) Name() string { return "health" }

func (e *healthEndpoint) Routes() []protocol.EndpointRoute {
	return []protocol.EndpointRoute{
		{Method: http.MethodGet, Path: "/health", Handler: http.HandlerFunc(e.server.HandleHealth)},
	}
}
