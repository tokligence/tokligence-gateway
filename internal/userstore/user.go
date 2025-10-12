package userstore

import (
	"context"
	"time"
)

// Role represents a high level capability within the gateway.
type Role string

const (
	RoleRootAdmin    Role = "root_admin"
	RoleGatewayAdmin Role = "gateway_admin"
	RoleGatewayUser  Role = "gateway_user"
)

// Status captures whether a user is active or suspended.
type Status string

const (
	StatusActive   Status = "active"
	StatusInactive Status = "inactive"
)

// User represents an identity managed by the gateway.
type User struct {
	ID          int64
	Email       string
	Role        Role
	DisplayName string
	Status      Status
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// APIKey represents an issued access token tied to a user.
type APIKey struct {
	ID        int64
	UserID    int64
	Prefix    string
	Scopes    []string
	ExpiresAt *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Store persists gateway users across SQLite/Postgres backends.
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
