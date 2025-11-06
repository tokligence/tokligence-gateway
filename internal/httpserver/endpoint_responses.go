package httpserver

import (
	"net/http"

	"github.com/tokligence/tokligence-gateway/internal/httpserver/protocol"
)

type responsesEndpoint struct {
	server *Server
}

func newResponsesEndpoint(server *Server) protocol.Endpoint {
	return &responsesEndpoint{server: server}
}

func (e *responsesEndpoint) Name() string {
	return "openai_responses"
}

func (e *responsesEndpoint) Routes() []protocol.EndpointRoute {
	return []protocol.EndpointRoute{
		{
			Method:  http.MethodPost,
			Path:    "/v1/responses",
			Handler: http.HandlerFunc(e.server.HandleResponses),
		},
	}
}
