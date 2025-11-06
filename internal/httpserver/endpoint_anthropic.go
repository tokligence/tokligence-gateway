package httpserver

import (
	"net/http"

	"github.com/tokligence/tokligence-gateway/internal/httpserver/protocol"
)

type anthropicEndpoint struct {
	server *Server
}

func newAnthropicEndpoint(server *Server) protocol.Endpoint {
	return &anthropicEndpoint{server: server}
}

func (e *anthropicEndpoint) Name() string {
	return "anthropic_messages"
}

func (e *anthropicEndpoint) Routes() []protocol.EndpointRoute {
	return []protocol.EndpointRoute{
		{
			Method:  http.MethodPost,
			Path:    "/anthropic/v1/messages",
			Handler: http.HandlerFunc(e.server.HandleAnthropicMessages),
		},
		{
			Method:  http.MethodPost,
			Path:    "/v1/messages",
			Handler: http.HandlerFunc(e.server.HandleAnthropicMessages),
		},
		{
			Method:  http.MethodPost,
			Path:    "/anthropic/v1/messages/count_tokens",
			Handler: http.HandlerFunc(e.server.HandleAnthropicCountTokens),
		},
		{
			Method:  http.MethodPost,
			Path:    "/v1/messages/count_tokens",
			Handler: http.HandlerFunc(e.server.HandleAnthropicCountTokens),
		},
	}
}
