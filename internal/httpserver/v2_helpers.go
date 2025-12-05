package httpserver

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/tokligence/tokligence-gateway/internal/userstore"
)

// identityV2Store holds the optional V2 store reference.
// This is set via SetIdentityV2 when the application uses the v2 user system.
var (
	identityV2Store   userstore.StoreV2
	identityV2StoreMu sync.RWMutex
)

// SetIdentityV2 sets the v2 identity store for enhanced user management.
func SetIdentityV2(store userstore.StoreV2) {
	identityV2StoreMu.Lock()
	defer identityV2StoreMu.Unlock()
	identityV2Store = store
}

// GetIdentityV2 returns the v2 identity store.
func GetIdentityV2() userstore.StoreV2 {
	identityV2StoreMu.RLock()
	defer identityV2StoreMu.RUnlock()
	return identityV2Store
}

// Server extension: identityV2 field
// This is accessed via getStoreV2() method defined in gateway_handlers.go
// The Server struct in server.go should be extended to include:
//   identityV2 userstore.StoreV2
// For now, we use the package-level variable as a workaround.

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeJSONError writes a JSON error response.
func writeJSONError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]interface{}{
		"error": map[string]interface{}{
			"code":    http.StatusText(status),
			"message": message,
		},
	})
}

// authenticatedUser retrieves the authenticated user from the request context.
func (s *Server) authenticatedUser(r *http.Request) (*userstore.User, bool) {
	// Try to get from context (set by authentication middleware)
	if u, ok := r.Context().Value(userContextKey{}).(*userstore.User); ok {
		return u, true
	}

	// Fallback to root admin for backward compatibility
	if s.rootAdmin != nil {
		return s.rootAdmin, true
	}

	return nil, false
}

// userContextKey is the context key for storing the authenticated user.
type userContextKey struct{}
