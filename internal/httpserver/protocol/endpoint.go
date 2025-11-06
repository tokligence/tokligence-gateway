package protocol

import "net/http"

type EndpointRoute struct {
	Method  string
	Path    string
	Handler http.Handler
}

type Endpoint interface {
	Name() string
	Routes() []EndpointRoute
}
