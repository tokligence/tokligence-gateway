package scheduler

import (
	"time"
)

// PriorityMappingModel represents a database row from api_key_priority_mappings table
type PriorityMappingModel struct {
	// UUID primary key (stored as string in Go)
	ID string `json:"id" db:"id"`

	// Pattern matching
	Pattern  string `json:"pattern" db:"pattern"`
	Priority int    `json:"priority" db:"priority"`
	MatchType string `json:"match_type" db:"match_type"`

	// Multi-tenant metadata
	TenantID   string `json:"tenant_id" db:"tenant_id"`
	TenantName string `json:"tenant_name" db:"tenant_name"`
	TenantType string `json:"tenant_type" db:"tenant_type"`
	Description string `json:"description" db:"description"`

	// Status
	Enabled bool `json:"enabled" db:"enabled"`

	// Standard audit fields
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt time.Time  `json:"updated_at" db:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty" db:"deleted_at"` // Pointer for NULL support
	CreatedBy string     `json:"created_by" db:"created_by"`
	UpdatedBy string     `json:"updated_by" db:"updated_by"`
}

// PriorityMapping represents a compiled mapping rule (in-memory cache)
type PriorityMapping struct {
	ID       string
	Pattern  string
	Priority PriorityTier
	MatchType MatchType

	// Multi-tenant metadata
	TenantID   string
	TenantName string
	TenantType string
	Description string

	Enabled   bool
	matchFunc func(string) bool // Compiled match function (for fast lookup)
}

// MatchType defines the type of pattern matching
type MatchType int

const (
	MatchExact MatchType = iota
	MatchPrefix
	MatchSuffix
	MatchContains
	MatchRegex
)

// String returns the string representation of MatchType
func (mt MatchType) String() string {
	switch mt {
	case MatchExact:
		return "exact"
	case MatchPrefix:
		return "prefix"
	case MatchSuffix:
		return "suffix"
	case MatchContains:
		return "contains"
	case MatchRegex:
		return "regex"
	default:
		return "unknown"
	}
}

// ParseMatchType converts a string to MatchType
func ParseMatchType(s string) MatchType {
	switch s {
	case "exact":
		return MatchExact
	case "prefix":
		return MatchPrefix
	case "suffix":
		return MatchSuffix
	case "contains":
		return MatchContains
	case "regex":
		return MatchRegex
	default:
		return MatchExact
	}
}

// ConfigModel represents a database row from api_key_priority_config table
type ConfigModel struct {
	// UUID primary key
	ID string `json:"id" db:"id"`

	// Config key-value
	Key   string `json:"key" db:"key"`
	Value string `json:"value" db:"value"`
	Description string `json:"description" db:"description"`

	// Standard audit fields
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt time.Time  `json:"updated_at" db:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`
	CreatedBy string     `json:"created_by" db:"created_by"`
	UpdatedBy string     `json:"updated_by" db:"updated_by"`
}
