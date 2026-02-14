package graphql

import (
	"github.com/tokligence/tokligence-gateway/internal/userstore"
)

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

// Resolver holds the dependencies for all GraphQL resolvers.
type Resolver struct {
	Store userstore.StoreV2
}

// NewResolver creates a new Resolver with the given store.
func NewResolver(store userstore.StoreV2) *Resolver {
	return &Resolver{
		Store: store,
	}
}
