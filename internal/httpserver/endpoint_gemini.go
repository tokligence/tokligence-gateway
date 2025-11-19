package httpserver

import (
	"net/http"

	"github.com/tokligence/tokligence-gateway/internal/httpserver/protocol"
)

type geminiEndpoint struct {
	server *Server
}

func newGeminiEndpoint(server *Server) protocol.Endpoint {
	return &geminiEndpoint{server: server}
}

func (e *geminiEndpoint) Name() string { return "gemini_native" }

func (e *geminiEndpoint) Routes() []protocol.EndpointRoute {
	handler := http.HandlerFunc(e.server.HandleGeminiProxy)
	return []protocol.EndpointRoute{
		// Catch-all handler for all Gemini requests using native Google Gemini API paths
		{Method: http.MethodGet, Path: "/v1beta/*", Handler: handler},
		{Method: http.MethodPost, Path: "/v1beta/*", Handler: handler},
	}
}
