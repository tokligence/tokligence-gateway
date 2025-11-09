package httpserver

import (
	"net/http"

	"github.com/tokligence/tokligence-gateway/internal/httpserver/protocol"
)

type openaiEndpoint struct {
	server *Server
}

func newOpenAIEndpoint(server *Server) protocol.Endpoint {
	return &openaiEndpoint{server: server}
}

func (e *openaiEndpoint) Name() string { return "openai_chat_embeddings" }

func (e *openaiEndpoint) Routes() []protocol.EndpointRoute {
	return []protocol.EndpointRoute{
		{Method: http.MethodPost, Path: "/v1/chat/completions", Handler: http.HandlerFunc(e.server.HandleChatCompletions)},
		{Method: http.MethodPost, Path: "/v1/embeddings", Handler: http.HandlerFunc(e.server.HandleEmbeddings)},
		{Method: http.MethodGet, Path: "/v1/models", Handler: http.HandlerFunc(e.server.HandleModels)},
	}
}
