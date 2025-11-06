package httpserver

import (
	"net/http"

	"github.com/tokligence/tokligence-gateway/internal/httpserver/protocol"
)

type adminEndpoint struct {
	server *Server
}

func newAdminEndpoint(server *Server) protocol.Endpoint {
	return &adminEndpoint{server: server}
}

func (e *adminEndpoint) Name() string { return "admin" }

func (e *adminEndpoint) Routes() []protocol.EndpointRoute {
	wrap := e.server.wrapAdminHandler
	return []protocol.EndpointRoute{
		{Method: http.MethodGet, Path: "/api/v1/admin/users", Handler: wrap(e.server.HandleAdminListUsers)},
		{Method: http.MethodPost, Path: "/api/v1/admin/users", Handler: wrap(e.server.HandleAdminCreateUser)},
		{Method: http.MethodPost, Path: "/api/v1/admin/users/import", Handler: wrap(e.server.HandleAdminImportUsers)},
		{Method: http.MethodPatch, Path: "/api/v1/admin/users/{id}", Handler: wrap(e.server.HandleAdminUpdateUser)},
		{Method: http.MethodDelete, Path: "/api/v1/admin/users/{id}", Handler: wrap(e.server.HandleAdminDeleteUser)},
		{Method: http.MethodGet, Path: "/api/v1/admin/users/{id}/api-keys", Handler: wrap(e.server.HandleAdminListAPIKeys)},
		{Method: http.MethodPost, Path: "/api/v1/admin/users/{id}/api-keys", Handler: wrap(e.server.HandleAdminCreateAPIKey)},
		{Method: http.MethodDelete, Path: "/api/v1/admin/api-keys/{id}", Handler: wrap(e.server.HandleAdminDeleteAPIKey)},
	}
}
