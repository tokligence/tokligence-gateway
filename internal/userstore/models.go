package userstore

import (
	"time"

	"github.com/google/uuid"
)

// ==============================================================================
// Core Domain Models for User System v2
// ==============================================================================

// Gateway represents a Tokligence gateway instance that can consume and/or provide
// AI tokens. Each Gateway has its own organizational hierarchy and user management.
type Gateway struct {
	ID              uuid.UUID
	Alias           string     // Human-friendly name (e.g., "Acme Corp Gateway")
	OwnerUserID     uuid.UUID  // The user who owns this gateway
	ProviderEnabled bool       // Whether this gateway can sell tokens
	ConsumerEnabled bool       // Whether this gateway can buy tokens
	Metadata        JSONMap    // Flexible metadata storage
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DeletedAt       *time.Time
}

// GatewayMemberRole represents the role of a user within a gateway.
type GatewayMemberRole string

const (
	GatewayRoleOwner  GatewayMemberRole = "owner"
	GatewayRoleAdmin  GatewayMemberRole = "admin"
	GatewayRoleMember GatewayMemberRole = "member"
)

// GatewayMembership links a User to a Gateway with a specific role.
type GatewayMembership struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	GatewayID uuid.UUID
	Role      GatewayMemberRole
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

// ==============================================================================
// Organization Unit (Flexible Hierarchy)
// ==============================================================================

// OrgUnitType categorizes organizational units.
type OrgUnitType string

const (
	OrgUnitTypeDepartment OrgUnitType = "department"
	OrgUnitTypeTeam       OrgUnitType = "team"
	OrgUnitTypeGroup      OrgUnitType = "group"
	OrgUnitTypeProject    OrgUnitType = "project"
)

// OrgUnit represents a node in the organizational hierarchy.
// Uses Materialized Path pattern for efficient tree queries.
type OrgUnit struct {
	ID            uuid.UUID
	GatewayID     uuid.UUID
	ParentID      *uuid.UUID  // nil = root level
	Path          string      // Materialized path, e.g., "/engineering/backend"
	Depth         int         // 0 = root level
	Name          string      // Display name
	Slug          string      // URL-safe identifier
	UnitType      OrgUnitType // department, team, group, project
	BudgetID      *uuid.UUID  // Optional budget constraint
	AllowedModels []string    // Model restrictions (empty = inherit from parent)
	Metadata      JSONMap
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DeletedAt     *time.Time
}

// OrgUnitWithChildren is OrgUnit plus its children for tree responses.
type OrgUnitWithChildren struct {
	OrgUnit
	Children []OrgUnitWithChildren `json:"children,omitempty"`
}

// ==============================================================================
// Principal (Unified Consumer Identity)
// ==============================================================================

// PrincipalType distinguishes different kinds of API consumers.
type PrincipalType string

const (
	PrincipalTypeUser        PrincipalType = "user"        // Human user
	PrincipalTypeService     PrincipalType = "service"     // Service account
	PrincipalTypeEnvironment PrincipalType = "environment" // Environment (dev/staging/prod)
)

// Principal is a unified entity representing something that can consume tokens.
// A Principal can be a user, a service account, or an environment.
type Principal struct {
	ID              uuid.UUID
	GatewayID       uuid.UUID
	PrincipalType   PrincipalType
	UserID          *uuid.UUID // Non-nil when type=user
	ServiceName     *string    // Non-nil when type=service
	EnvironmentName *string    // Non-nil when type=environment
	DisplayName     string     // Human-friendly name
	BudgetID        *uuid.UUID // Optional personal budget
	AllowedModels   []string   // Personal model restrictions (empty = inherit)
	Metadata        JSONMap
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DeletedAt       *time.Time
}

// ==============================================================================
// Organization Membership (Many-to-Many)
// ==============================================================================

// OrgMemberRole represents a principal's role within an org unit.
type OrgMemberRole string

const (
	OrgMemberRoleAdmin  OrgMemberRole = "admin"
	OrgMemberRoleMember OrgMemberRole = "member"
	OrgMemberRoleViewer OrgMemberRole = "viewer"
)

// OrgMembership links a Principal to an OrgUnit.
// A Principal can belong to multiple OrgUnits.
type OrgMembership struct {
	ID          uuid.UUID
	PrincipalID uuid.UUID
	OrgUnitID   uuid.UUID
	Role        OrgMemberRole
	BudgetID    *uuid.UUID // Optional budget override for this membership
	IsPrimary   bool       // Primary membership for default spend attribution
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

// ==============================================================================
// Budget Configuration
// ==============================================================================

// BudgetDuration specifies the budget reset period.
type BudgetDuration string

const (
	BudgetDurationDaily   BudgetDuration = "daily"
	BudgetDurationWeekly  BudgetDuration = "weekly"
	BudgetDurationMonthly BudgetDuration = "monthly"
	BudgetDurationTotal   BudgetDuration = "total" // No reset
)

// Budget defines spending/rate limits for a Principal, OrgUnit, or APIKey.
type Budget struct {
	ID             uuid.UUID
	GatewayID      uuid.UUID
	Name           string         // Optional name for this budget config
	MaxBudget      *float64       // Max spend in currency (nil = unlimited)
	BudgetDuration BudgetDuration // Reset period
	TPMLimit       *int64         // Tokens per minute limit (nil = unlimited)
	RPMLimit       *int64         // Requests per minute limit (nil = unlimited)
	SoftLimit      *float64       // Warning threshold (percentage of max)
	Metadata       JSONMap
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time
}

// ==============================================================================
// API Key v2
// ==============================================================================

// APIKeyV2 represents an API key with enhanced v2 features.
type APIKeyV2 struct {
	ID            uuid.UUID
	GatewayID     uuid.UUID
	PrincipalID   uuid.UUID   // Which Principal this key belongs to
	OrgUnitID     *uuid.UUID  // Optional: spend attribution to a specific OrgUnit
	KeyHash       string      // SHA256 hash of the key (never stored raw)
	KeyPrefix     string      // First 8 chars for identification (e.g., "tok_abc12345")
	KeyName       string      // User-provided name
	BudgetID      *uuid.UUID  // Optional budget constraint on this key
	AllowedModels []string    // Model restrictions (empty = inherit from Principal)
	AllowedIPs    []string    // IP whitelist (empty = no restriction)
	Scopes        []string    // Permission scopes
	ExpiresAt     *time.Time  // Expiration time (nil = never expires)
	LastUsedAt    *time.Time  // Last usage timestamp
	TotalSpend    float64     // Accumulated spend on this key
	Blocked       bool        // Manually blocked
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DeletedAt     *time.Time
}

// ==============================================================================
// Helper Types
// ==============================================================================

// JSONMap is a type alias for flexible JSON metadata.
type JSONMap map[string]interface{}

// BudgetInheritance represents the resolved budget for a Principal.
type BudgetInheritance struct {
	EffectiveBudget *Budget        // The budget that applies
	Source          string         // Where the budget came from
	Chain           []BudgetSource // The inheritance chain
}

// BudgetSource identifies where a budget constraint originated.
type BudgetSource struct {
	Type   string    // "principal", "membership", "orgunit", "gateway"
	ID     uuid.UUID // ID of the entity
	Name   string    // Name for display
	Budget *Budget   // The budget at this level (nil = no budget set)
}
