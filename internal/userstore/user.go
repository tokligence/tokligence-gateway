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

// Store persists gateway users across SQLite/Postgres backends.
type Store interface {
	EnsureRootAdmin(ctx context.Context, email string) (*User, error)
	FindByEmail(ctx context.Context, email string) (*User, error)
	Close() error
}
