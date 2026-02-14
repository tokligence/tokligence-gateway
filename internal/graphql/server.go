package graphql

import (
	"net/http"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/tokligence/tokligence-gateway/internal/userstore"
)

// NewHandler creates a new GraphQL HTTP handler with the given store.
func NewHandler(store userstore.StoreV2) http.Handler {
	resolver := NewResolver(store)
	srv := handler.NewDefaultServer(NewExecutableSchema(Config{Resolvers: resolver}))
	return srv
}

// NewPlaygroundHandler creates a GraphQL playground handler.
func NewPlaygroundHandler(endpoint string) http.Handler {
	return playground.Handler("GraphQL Playground", endpoint)
}
