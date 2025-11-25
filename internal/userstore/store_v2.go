package userstore

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ==============================================================================
// Store V2 Interface - Extended User System
// ==============================================================================

// StoreV2 extends the base Store interface with v2 user system features.
// It supports flexible organization hierarchy, Principals, and multi-team membership.
type StoreV2 interface {
	Store // Embed base interface for backward compatibility

	// =========================================================================
	// Gateway Management
	// =========================================================================

	// CreateGateway creates a new gateway owned by the given user.
	CreateGateway(ctx context.Context, ownerUserID uuid.UUID, alias string) (*Gateway, error)

	// GetGateway retrieves a gateway by ID.
	GetGateway(ctx context.Context, id uuid.UUID) (*Gateway, error)

	// ListGatewaysForUser returns all gateways the user has access to.
	ListGatewaysForUser(ctx context.Context, userID uuid.UUID) ([]Gateway, error)

	// UpdateGateway updates gateway properties.
	UpdateGateway(ctx context.Context, id uuid.UUID, updates GatewayUpdate) (*Gateway, error)

	// DeleteGateway soft-deletes a gateway.
	DeleteGateway(ctx context.Context, id uuid.UUID) error

	// =========================================================================
	// Gateway Membership
	// =========================================================================

	// AddGatewayMember adds a user to a gateway with the given role.
	AddGatewayMember(ctx context.Context, gatewayID, userID uuid.UUID, role GatewayMemberRole) (*GatewayMembership, error)

	// ListGatewayMembers returns all members of a gateway.
	ListGatewayMembers(ctx context.Context, gatewayID uuid.UUID) ([]GatewayMembershipWithUser, error)

	// UpdateGatewayMember updates a member's role.
	UpdateGatewayMember(ctx context.Context, membershipID uuid.UUID, role GatewayMemberRole) error

	// RemoveGatewayMember removes a user from a gateway.
	RemoveGatewayMember(ctx context.Context, membershipID uuid.UUID) error

	// GetGatewayMembership gets a user's membership in a gateway.
	GetGatewayMembership(ctx context.Context, gatewayID, userID uuid.UUID) (*GatewayMembership, error)

	// =========================================================================
	// OrgUnit Management
	// =========================================================================

	// CreateOrgUnit creates a new organizational unit.
	CreateOrgUnit(ctx context.Context, params CreateOrgUnitParams) (*OrgUnit, error)

	// GetOrgUnit retrieves an org unit by ID.
	GetOrgUnit(ctx context.Context, id uuid.UUID) (*OrgUnit, error)

	// GetOrgUnitTree returns the full org unit tree for a gateway.
	GetOrgUnitTree(ctx context.Context, gatewayID uuid.UUID) ([]OrgUnitWithChildren, error)

	// GetOrgUnitChildren returns direct children of an org unit.
	GetOrgUnitChildren(ctx context.Context, parentID uuid.UUID) ([]OrgUnit, error)

	// GetOrgUnitsByPath returns org units matching a path prefix.
	GetOrgUnitsByPath(ctx context.Context, gatewayID uuid.UUID, pathPrefix string) ([]OrgUnit, error)

	// UpdateOrgUnit updates org unit properties.
	UpdateOrgUnit(ctx context.Context, id uuid.UUID, updates OrgUnitUpdate) (*OrgUnit, error)

	// MoveOrgUnit moves an org unit to a new parent.
	MoveOrgUnit(ctx context.Context, id uuid.UUID, newParentID *uuid.UUID) (*OrgUnit, error)

	// MergeOrgUnits merges source into target (moves all members, deletes source).
	MergeOrgUnits(ctx context.Context, sourceID, targetID uuid.UUID) error

	// DeleteOrgUnit soft-deletes an org unit.
	DeleteOrgUnit(ctx context.Context, id uuid.UUID, force bool) error

	// =========================================================================
	// Principal Management
	// =========================================================================

	// CreatePrincipal creates a new principal.
	CreatePrincipal(ctx context.Context, params CreatePrincipalParams) (*Principal, error)

	// GetPrincipal retrieves a principal by ID.
	GetPrincipal(ctx context.Context, id uuid.UUID) (*Principal, error)

	// GetPrincipalByUserID retrieves a principal by the linked user ID.
	GetPrincipalByUserID(ctx context.Context, gatewayID, userID uuid.UUID) (*Principal, error)

	// ListPrincipals returns principals for a gateway with optional filters.
	ListPrincipals(ctx context.Context, gatewayID uuid.UUID, filter PrincipalFilter) ([]Principal, error)

	// UpdatePrincipal updates principal properties.
	UpdatePrincipal(ctx context.Context, id uuid.UUID, updates PrincipalUpdate) (*Principal, error)

	// DeletePrincipal soft-deletes a principal.
	DeletePrincipal(ctx context.Context, id uuid.UUID) error

	// =========================================================================
	// OrgMembership Management
	// =========================================================================

	// AddOrgMembership adds a principal to an org unit.
	AddOrgMembership(ctx context.Context, params CreateOrgMembershipParams) (*OrgMembership, error)

	// ListOrgMemberships returns all memberships for a principal.
	ListOrgMemberships(ctx context.Context, principalID uuid.UUID) ([]OrgMembershipWithOrgUnit, error)

	// ListOrgUnitMembers returns all members of an org unit.
	ListOrgUnitMembers(ctx context.Context, orgUnitID uuid.UUID) ([]OrgMembershipWithPrincipal, error)

	// UpdateOrgMembership updates membership properties.
	UpdateOrgMembership(ctx context.Context, membershipID uuid.UUID, updates OrgMembershipUpdate) error

	// RemoveOrgMembership removes a principal from an org unit.
	RemoveOrgMembership(ctx context.Context, membershipID uuid.UUID) error

	// SetPrimaryMembership sets a membership as the primary for spend attribution.
	SetPrimaryMembership(ctx context.Context, principalID, membershipID uuid.UUID) error

	// =========================================================================
	// Budget Management
	// =========================================================================

	// CreateBudget creates a new budget configuration.
	CreateBudget(ctx context.Context, gatewayID uuid.UUID, params CreateBudgetParams) (*Budget, error)

	// GetBudget retrieves a budget by ID.
	GetBudget(ctx context.Context, id uuid.UUID) (*Budget, error)

	// ListBudgets returns all budgets for a gateway.
	ListBudgets(ctx context.Context, gatewayID uuid.UUID) ([]Budget, error)

	// UpdateBudget updates budget properties.
	UpdateBudget(ctx context.Context, id uuid.UUID, updates BudgetUpdate) (*Budget, error)

	// DeleteBudget soft-deletes a budget.
	DeleteBudget(ctx context.Context, id uuid.UUID) error

	// ResolveBudget resolves the effective budget for a principal.
	// Follows inheritance: Principal → OrgMembership → OrgUnit → Parent OrgUnit → Gateway
	ResolveBudget(ctx context.Context, principalID uuid.UUID) (*BudgetInheritance, error)

	// =========================================================================
	// API Key v2 Management
	// =========================================================================

	// CreateAPIKeyV2 creates a new API key with v2 features.
	CreateAPIKeyV2(ctx context.Context, params CreateAPIKeyV2Params) (*APIKeyV2, string, error)

	// GetAPIKeyV2 retrieves an API key by ID.
	GetAPIKeyV2(ctx context.Context, id uuid.UUID) (*APIKeyV2, error)

	// ListAPIKeysV2 returns API keys with optional filters.
	ListAPIKeysV2(ctx context.Context, gatewayID uuid.UUID, filter APIKeyFilter) ([]APIKeyV2, error)

	// UpdateAPIKeyV2 updates API key properties.
	UpdateAPIKeyV2(ctx context.Context, id uuid.UUID, updates APIKeyV2Update) (*APIKeyV2, error)

	// RotateAPIKeyV2 generates a new key, optionally keeping the old one valid for a grace period.
	RotateAPIKeyV2(ctx context.Context, id uuid.UUID, gracePeriod time.Duration) (*APIKeyV2, string, error)

	// RevokeAPIKeyV2 soft-deletes an API key.
	RevokeAPIKeyV2(ctx context.Context, id uuid.UUID) error

	// LookupAPIKeyV2 validates a token and returns the key, principal, and gateway.
	LookupAPIKeyV2(ctx context.Context, token string) (*APIKeyV2, *Principal, *Gateway, error)

	// RecordAPIKeyUsage updates the last_used_at and total_spend for a key.
	RecordAPIKeyUsage(ctx context.Context, keyID uuid.UUID, spend float64) error
}

// ==============================================================================
// Parameter Types for Create/Update Operations
// ==============================================================================

// GatewayUpdate contains fields that can be updated on a Gateway.
type GatewayUpdate struct {
	Alias           *string
	ProviderEnabled *bool
	ConsumerEnabled *bool
	Metadata        *JSONMap
}

// CreateOrgUnitParams contains parameters for creating an OrgUnit.
type CreateOrgUnitParams struct {
	GatewayID     uuid.UUID
	ParentID      *uuid.UUID
	Name          string
	Slug          string
	UnitType      OrgUnitType
	BudgetID      *uuid.UUID
	AllowedModels []string
	Metadata      JSONMap
}

// OrgUnitUpdate contains fields that can be updated on an OrgUnit.
type OrgUnitUpdate struct {
	Name          *string
	Slug          *string
	UnitType      *OrgUnitType
	BudgetID      *uuid.UUID  // Set to zero UUID to remove budget
	AllowedModels *[]string
	Metadata      *JSONMap
}

// CreatePrincipalParams contains parameters for creating a Principal.
type CreatePrincipalParams struct {
	GatewayID       uuid.UUID
	PrincipalType   PrincipalType
	UserID          *uuid.UUID // Required when type=user
	ServiceName     *string    // Required when type=service
	EnvironmentName *string    // Required when type=environment
	DisplayName     string
	BudgetID        *uuid.UUID
	AllowedModels   []string
	Metadata        JSONMap
}

// PrincipalFilter specifies filters for listing principals.
type PrincipalFilter struct {
	Type      *PrincipalType
	OrgUnitID *uuid.UUID
	Search    *string // Search by display name
}

// PrincipalUpdate contains fields that can be updated on a Principal.
type PrincipalUpdate struct {
	DisplayName   *string
	BudgetID      *uuid.UUID // Set to zero UUID to remove budget
	AllowedModels *[]string
	Metadata      *JSONMap
}

// CreateOrgMembershipParams contains parameters for creating an OrgMembership.
type CreateOrgMembershipParams struct {
	PrincipalID uuid.UUID
	OrgUnitID   uuid.UUID
	Role        OrgMemberRole
	BudgetID    *uuid.UUID
	IsPrimary   bool
}

// OrgMembershipUpdate contains fields that can be updated on an OrgMembership.
type OrgMembershipUpdate struct {
	Role      *OrgMemberRole
	BudgetID  *uuid.UUID // Set to zero UUID to remove budget
	IsPrimary *bool
}

// CreateBudgetParams contains parameters for creating a Budget.
type CreateBudgetParams struct {
	Name           string
	MaxBudget      *float64
	BudgetDuration BudgetDuration
	TPMLimit       *int64
	RPMLimit       *int64
	SoftLimit      *float64 // Percentage (0-100)
	Metadata       JSONMap
}

// BudgetUpdate contains fields that can be updated on a Budget.
type BudgetUpdate struct {
	Name           *string
	MaxBudget      *float64
	BudgetDuration *BudgetDuration
	TPMLimit       *int64
	RPMLimit       *int64
	SoftLimit      *float64
	Metadata       *JSONMap
}

// CreateAPIKeyV2Params contains parameters for creating an API key.
type CreateAPIKeyV2Params struct {
	GatewayID     uuid.UUID
	PrincipalID   uuid.UUID
	OrgUnitID     *uuid.UUID
	KeyName       string
	BudgetID      *uuid.UUID
	AllowedModels []string
	AllowedIPs    []string
	Scopes        []string
	ExpiresAt     *time.Time
}

// APIKeyFilter specifies filters for listing API keys.
type APIKeyFilter struct {
	PrincipalID *uuid.UUID
	OrgUnitID   *uuid.UUID
	Blocked     *bool
}

// APIKeyV2Update contains fields that can be updated on an API key.
type APIKeyV2Update struct {
	KeyName       *string
	BudgetID      *uuid.UUID
	AllowedModels *[]string
	AllowedIPs    *[]string
	ExpiresAt     *time.Time
	Blocked       *bool
}

// ==============================================================================
// Join Types for Queries
// ==============================================================================

// GatewayMembershipWithUser includes user details with membership.
type GatewayMembershipWithUser struct {
	GatewayMembership
	User User
}

// OrgMembershipWithOrgUnit includes org unit details with membership.
type OrgMembershipWithOrgUnit struct {
	OrgMembership
	OrgUnit OrgUnit
}

// OrgMembershipWithPrincipal includes principal details with membership.
type OrgMembershipWithPrincipal struct {
	OrgMembership
	Principal Principal
}
