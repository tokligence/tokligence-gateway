package userstore

import (
	"context"
	"time"
)

// ==============================================================================
// Legacy V1 Types (for backward compatibility with sqlite/postgres stores)
// ==============================================================================

// Role represents a high level capability within the gateway (legacy v1).
// Deprecated: Use UserRoleV2 instead.
type Role string

const (
	RoleRootAdmin    Role = "root_admin"
	RoleGatewayAdmin Role = "gateway_admin"
	RoleGatewayUser  Role = "gateway_user"
)

// Status captures whether a user is active or suspended (legacy v1).
// Deprecated: Use UserStatusV2 instead.
type Status string

const (
	StatusActive   Status = "active"
	StatusInactive Status = "inactive"
)

// User represents an identity managed by the gateway (legacy v1).
// Deprecated: Use UserV2 instead.
type User struct {
	ID          int64
	UUID        string
	Email       string
	Role        Role
	DisplayName string
	Status      Status
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

// APIKey represents an issued access token tied to a user (legacy v1).
// Deprecated: Use APIKeyV2 instead.
type APIKey struct {
	ID        int64
	UUID      string
	UserID    int64
	Prefix    string
	Scopes    []string
	ExpiresAt *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

// Store persists gateway users across SQLite/Postgres backends (legacy v1).
// Deprecated: Use StoreV2 interface instead.
type Store interface {
	EnsureRootAdmin(ctx context.Context, email string) (*User, error)
	FindByEmail(ctx context.Context, email string) (*User, error)
	GetUser(ctx context.Context, id int64) (*User, error)
	ListUsers(ctx context.Context) ([]User, error)
	CreateUser(ctx context.Context, email string, role Role, displayName string) (*User, error)
	UpdateUser(ctx context.Context, id int64, displayName string, role Role) (*User, error)
	SetUserStatus(ctx context.Context, id int64, status Status) error
	DeleteUser(ctx context.Context, id int64) error
	CreateAPIKey(ctx context.Context, userID int64, scopes []string, expiresAt *time.Time) (*APIKey, string, error)
	ListAPIKeys(ctx context.Context, userID int64) ([]APIKey, error)
	DeleteAPIKey(ctx context.Context, id int64) error
	LookupAPIKey(ctx context.Context, token string) (*APIKey, *User, error)
	Close() error
}
