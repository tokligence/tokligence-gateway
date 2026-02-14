// Package postgres_v2_test provides comprehensive tests for the user system v2 PostgreSQL store.
package postgres_v2_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/tokligence/tokligence-gateway/internal/userstore"
	"github.com/tokligence/tokligence-gateway/internal/userstore/postgres_v2"
)

// getTestDSN returns the PostgreSQL DSN for testing.
func getTestDSN() string {
	if dsn := os.Getenv("TOKLIGENCE_DB_DSN"); dsn != "" {
		return dsn
	}
	return "postgres://postgres:postgres@localhost:5432/tokligence_test?sslmode=disable"
}

// setupTestStore creates a new store for testing.
func setupTestStore(t *testing.T) *postgres_v2.Store {
	t.Helper()
	dsn := getTestDSN()
	store, err := postgres_v2.New(dsn, postgres_v2.DefaultConfig())
	if err != nil {
		t.Skipf("Skipping test: cannot connect to database: %v", err)
	}
	return store
}

// ==============================================================================
// User Tests
// ==============================================================================

func TestUserV2_CreateAndGet(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	// Create user
	user, err := store.CreateUserV2(ctx, userstore.CreateUserParams{
		Email:       "test-" + uuid.NewString()[:8] + "@example.com",
		Role:        userstore.UserRoleV2User,
		DisplayName: "Test User",
	})
	if err != nil {
		t.Fatalf("CreateUserV2 failed: %v", err)
	}

	if user.Email == "" {
		t.Error("Email should not be empty")
	}
	if user.Role != userstore.UserRoleV2User {
		t.Errorf("Role mismatch: got %v, want user", user.Role)
	}
	if user.Status != userstore.UserStatusV2Active {
		t.Errorf("Status should be active, got %v", user.Status)
	}

	// Get user
	retrieved, err := store.GetUserV2(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetUserV2 failed: %v", err)
	}
	if retrieved == nil {
		t.Fatal("GetUserV2 returned nil")
	}
	if retrieved.ID != user.ID {
		t.Errorf("ID mismatch: got %v, want %v", retrieved.ID, user.ID)
	}
}

func TestUserV2_GetByEmail(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	email := "test-" + uuid.NewString()[:8] + "@example.com"

	// Create user
	user, _ := store.CreateUserV2(ctx, userstore.CreateUserParams{
		Email:       email,
		Role:        userstore.UserRoleV2User,
		DisplayName: "Test User",
	})

	// Get by email
	found, err := store.GetUserByEmail(ctx, email)
	if err != nil {
		t.Fatalf("GetUserByEmail failed: %v", err)
	}
	if found == nil {
		t.Fatal("GetUserByEmail returned nil")
	}
	if found.ID != user.ID {
		t.Error("User ID should match")
	}
}

func TestUserV2_Update(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	user, _ := store.CreateUserV2(ctx, userstore.CreateUserParams{
		Email:       "test-" + uuid.NewString()[:8] + "@example.com",
		Role:        userstore.UserRoleV2User,
		DisplayName: "Original Name",
	})

	// Update user
	newName := "Updated Name"
	newRole := userstore.UserRoleV2Admin
	updated, err := store.UpdateUserV2(ctx, user.ID, userstore.UserUpdate{
		DisplayName: &newName,
		Role:        &newRole,
	})
	if err != nil {
		t.Fatalf("UpdateUserV2 failed: %v", err)
	}
	if updated.DisplayName != newName {
		t.Errorf("DisplayName not updated: got %v, want %v", updated.DisplayName, newName)
	}
	if updated.Role != newRole {
		t.Errorf("Role not updated: got %v, want %v", updated.Role, newRole)
	}
}

func TestUserV2_EnsureRootAdmin(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	email := "admin-" + uuid.NewString()[:8] + "@example.com"

	// Ensure root admin (creates new)
	admin, err := store.EnsureRootAdminV2(ctx, email)
	if err != nil {
		t.Fatalf("EnsureRootAdminV2 failed: %v", err)
	}
	if admin.Role != userstore.UserRoleV2RootAdmin {
		t.Errorf("Role should be root_admin, got %v", admin.Role)
	}

	// Ensure again (should return same)
	admin2, _ := store.EnsureRootAdminV2(ctx, email)
	if admin2.ID != admin.ID {
		t.Error("Should return existing user")
	}
}

// ==============================================================================
// Gateway Tests
// ==============================================================================

func TestGateway_CreateAndGet(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	// First create a user (gateway owner)
	owner, _ := store.CreateUserV2(ctx, userstore.CreateUserParams{
		Email:       "owner-" + uuid.NewString()[:8] + "@example.com",
		Role:        userstore.UserRoleV2User,
		DisplayName: "Gateway Owner",
	})

	// Create gateway
	gw, err := store.CreateGateway(ctx, owner.ID, "Test Gateway")
	if err != nil {
		t.Fatalf("CreateGateway failed: %v", err)
	}

	if gw.OwnerUserID != owner.ID {
		t.Errorf("OwnerUserID mismatch: got %v, want %v", gw.OwnerUserID, owner.ID)
	}
	if gw.Alias != "Test Gateway" {
		t.Errorf("Alias mismatch: got %v, want Test Gateway", gw.Alias)
	}
	if !gw.ConsumerEnabled {
		t.Error("ConsumerEnabled should default to true")
	}

	// Get gateway
	retrieved, err := store.GetGateway(ctx, gw.ID)
	if err != nil {
		t.Fatalf("GetGateway failed: %v", err)
	}
	if retrieved == nil {
		t.Fatal("GetGateway returned nil")
	}
}

func TestGateway_Update(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	owner, _ := store.CreateUserV2(ctx, userstore.CreateUserParams{
		Email:       "owner-" + uuid.NewString()[:8] + "@example.com",
		Role:        userstore.UserRoleV2User,
		DisplayName: "Gateway Owner",
	})
	gw, _ := store.CreateGateway(ctx, owner.ID, "Original Alias")

	// Update
	newAlias := "Updated Alias"
	providerEnabled := true
	updated, err := store.UpdateGateway(ctx, gw.ID, userstore.GatewayUpdate{
		Alias:           &newAlias,
		ProviderEnabled: &providerEnabled,
	})
	if err != nil {
		t.Fatalf("UpdateGateway failed: %v", err)
	}
	if updated.Alias != newAlias {
		t.Error("Alias not updated")
	}
	if !updated.ProviderEnabled {
		t.Error("ProviderEnabled should be true")
	}
}

// ==============================================================================
// OrgUnit Tests
// ==============================================================================

func TestOrgUnit_CreateTree(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	owner, _ := store.CreateUserV2(ctx, userstore.CreateUserParams{
		Email:       "owner-" + uuid.NewString()[:8] + "@example.com",
		Role:        userstore.UserRoleV2User,
		DisplayName: "Owner",
	})
	gw, _ := store.CreateGateway(ctx, owner.ID, "Org Gateway")

	// Create root org unit
	engineering, err := store.CreateOrgUnit(ctx, userstore.CreateOrgUnitParams{
		GatewayID: gw.ID,
		Name:      "Engineering",
		Slug:      "engineering",
		UnitType:  userstore.OrgUnitTypeDepartment,
	})
	if err != nil {
		t.Fatalf("CreateOrgUnit (root) failed: %v", err)
	}

	if engineering.Depth != 0 {
		t.Errorf("Root depth should be 0, got %d", engineering.Depth)
	}
	if engineering.Path != "/engineering" {
		t.Errorf("Root path should be /engineering, got %s", engineering.Path)
	}

	// Create child org unit
	backend, err := store.CreateOrgUnit(ctx, userstore.CreateOrgUnitParams{
		GatewayID: gw.ID,
		ParentID:  &engineering.ID,
		Name:      "Backend Team",
		Slug:      "backend",
		UnitType:  userstore.OrgUnitTypeTeam,
	})
	if err != nil {
		t.Fatalf("CreateOrgUnit (child) failed: %v", err)
	}

	if backend.Depth != 1 {
		t.Errorf("Child depth should be 1, got %d", backend.Depth)
	}
	if backend.Path != "/engineering/backend" {
		t.Errorf("Child path should be /engineering/backend, got %s", backend.Path)
	}
}

func TestOrgUnit_GetTree(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	owner, _ := store.CreateUserV2(ctx, userstore.CreateUserParams{
		Email:       "owner-" + uuid.NewString()[:8] + "@example.com",
		Role:        userstore.UserRoleV2User,
		DisplayName: "Owner",
	})
	gw, _ := store.CreateGateway(ctx, owner.ID, "Tree Gateway")

	// Create tree structure
	eng, _ := store.CreateOrgUnit(ctx, userstore.CreateOrgUnitParams{
		GatewayID: gw.ID, Name: "Engineering", Slug: "eng", UnitType: userstore.OrgUnitTypeDepartment,
	})
	_, _ = store.CreateOrgUnit(ctx, userstore.CreateOrgUnitParams{
		GatewayID: gw.ID, ParentID: &eng.ID, Name: "Backend", Slug: "backend", UnitType: userstore.OrgUnitTypeTeam,
	})
	_, _ = store.CreateOrgUnit(ctx, userstore.CreateOrgUnitParams{
		GatewayID: gw.ID, ParentID: &eng.ID, Name: "Frontend", Slug: "frontend", UnitType: userstore.OrgUnitTypeTeam,
	})
	_, _ = store.CreateOrgUnit(ctx, userstore.CreateOrgUnitParams{
		GatewayID: gw.ID, Name: "Sales", Slug: "sales", UnitType: userstore.OrgUnitTypeDepartment,
	})

	// Get tree
	tree, err := store.GetOrgUnitTree(ctx, gw.ID)
	if err != nil {
		t.Fatalf("GetOrgUnitTree failed: %v", err)
	}

	if len(tree) != 2 { // Engineering and Sales at root
		t.Errorf("Expected 2 root units, got %d", len(tree))
	}
}

// ==============================================================================
// Principal Tests
// ==============================================================================

func TestPrincipal_CreateUser(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	owner, _ := store.CreateUserV2(ctx, userstore.CreateUserParams{
		Email:       "owner-" + uuid.NewString()[:8] + "@example.com",
		Role:        userstore.UserRoleV2User,
		DisplayName: "Owner",
	})
	gw, _ := store.CreateGateway(ctx, owner.ID, "Principal Gateway")

	// Create a real user for the principal to reference
	principalUser, err := store.CreateUserV2(ctx, userstore.CreateUserParams{
		Email:       "principal-user-" + uuid.NewString()[:8] + "@example.com",
		Role:        userstore.UserRoleV2User,
		DisplayName: "John Doe",
	})
	if err != nil {
		t.Fatalf("CreateUserV2 for principal failed: %v", err)
	}

	principal, err := store.CreatePrincipal(ctx, userstore.CreatePrincipalParams{
		GatewayID:     gw.ID,
		PrincipalType: userstore.PrincipalTypeUser,
		UserID:        &principalUser.ID,
		DisplayName:   "John Doe",
	})
	if err != nil {
		t.Fatalf("CreatePrincipal (user) failed: %v", err)
	}

	if principal.PrincipalType != userstore.PrincipalTypeUser {
		t.Errorf("PrincipalType mismatch: got %v, want user", principal.PrincipalType)
	}
}

func TestPrincipal_CreateService(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	owner, _ := store.CreateUserV2(ctx, userstore.CreateUserParams{
		Email:       "owner-" + uuid.NewString()[:8] + "@example.com",
		Role:        userstore.UserRoleV2User,
		DisplayName: "Owner",
	})
	gw, _ := store.CreateGateway(ctx, owner.ID, "Service Gateway")

	serviceName := "payment-service"
	principal, err := store.CreatePrincipal(ctx, userstore.CreatePrincipalParams{
		GatewayID:     gw.ID,
		PrincipalType: userstore.PrincipalTypeService,
		ServiceName:   &serviceName,
		DisplayName:   "Payment Service",
	})
	if err != nil {
		t.Fatalf("CreatePrincipal (service) failed: %v", err)
	}

	if principal.PrincipalType != userstore.PrincipalTypeService {
		t.Errorf("PrincipalType mismatch: got %v, want service", principal.PrincipalType)
	}
}

// ==============================================================================
// OrgMembership Tests
// ==============================================================================

func TestOrgMembership_AddAndList(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	owner, _ := store.CreateUserV2(ctx, userstore.CreateUserParams{
		Email:       "owner-" + uuid.NewString()[:8] + "@example.com",
		Role:        userstore.UserRoleV2User,
		DisplayName: "Owner",
	})
	gw, _ := store.CreateGateway(ctx, owner.ID, "Membership Gateway")

	// Create org unit
	eng, _ := store.CreateOrgUnit(ctx, userstore.CreateOrgUnitParams{
		GatewayID: gw.ID, Name: "Engineering", Slug: "eng", UnitType: userstore.OrgUnitTypeDepartment,
	})

	// Create a real user for the principal to reference
	principalUser, err := store.CreateUserV2(ctx, userstore.CreateUserParams{
		Email:       "membership-user-" + uuid.NewString()[:8] + "@example.com",
		Role:        userstore.UserRoleV2User,
		DisplayName: "John",
	})
	if err != nil {
		t.Fatalf("CreateUserV2 for principal failed: %v", err)
	}

	// Create principal
	principal, _ := store.CreatePrincipal(ctx, userstore.CreatePrincipalParams{
		GatewayID: gw.ID, PrincipalType: userstore.PrincipalTypeUser, UserID: &principalUser.ID, DisplayName: "John",
	})

	// Add membership
	membership, err := store.AddOrgMembership(ctx, userstore.CreateOrgMembershipParams{
		PrincipalID: principal.ID,
		OrgUnitID:   eng.ID,
		Role:        userstore.OrgMemberRoleMember,
		IsPrimary:   true,
	})
	if err != nil {
		t.Fatalf("AddOrgMembership failed: %v", err)
	}

	if !membership.IsPrimary {
		t.Error("IsPrimary should be true")
	}

	// List memberships for principal
	memberships, err := store.ListOrgMemberships(ctx, principal.ID)
	if err != nil {
		t.Fatalf("ListOrgMemberships failed: %v", err)
	}

	if len(memberships) != 1 {
		t.Errorf("Expected 1 membership, got %d", len(memberships))
	}
}

// ==============================================================================
// Budget Tests
// ==============================================================================

func TestBudget_CreateAndGet(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	owner, _ := store.CreateUserV2(ctx, userstore.CreateUserParams{
		Email:       "owner-" + uuid.NewString()[:8] + "@example.com",
		Role:        userstore.UserRoleV2User,
		DisplayName: "Owner",
	})
	gw, _ := store.CreateGateway(ctx, owner.ID, "Budget Gateway")

	maxBudget := 1000.0
	tpmLimit := int64(10000)
	rpmLimit := int64(100)

	budget, err := store.CreateBudget(ctx, gw.ID, userstore.CreateBudgetParams{
		Name:           "Team Budget",
		MaxBudget:      &maxBudget,
		BudgetDuration: userstore.BudgetDurationMonthly,
		TPMLimit:       &tpmLimit,
		RPMLimit:       &rpmLimit,
	})
	if err != nil {
		t.Fatalf("CreateBudget failed: %v", err)
	}

	if budget.Name != "Team Budget" {
		t.Errorf("Name mismatch: got %v, want Team Budget", budget.Name)
	}
	if budget.MaxBudget == nil || *budget.MaxBudget != maxBudget {
		t.Error("MaxBudget should be 1000")
	}

	// Get budget
	retrieved, err := store.GetBudget(ctx, budget.ID)
	if err != nil {
		t.Fatalf("GetBudget failed: %v", err)
	}
	if retrieved == nil {
		t.Fatal("GetBudget returned nil")
	}
}

// ==============================================================================
// API Key Tests
// ==============================================================================

func TestAPIKeyV2_CreateAndLookup(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	owner, _ := store.CreateUserV2(ctx, userstore.CreateUserParams{
		Email:       "owner-" + uuid.NewString()[:8] + "@example.com",
		Role:        userstore.UserRoleV2User,
		DisplayName: "Owner",
	})
	gw, _ := store.CreateGateway(ctx, owner.ID, "API Key Gateway")

	// Create a real user for the principal to reference
	principalUser, err := store.CreateUserV2(ctx, userstore.CreateUserParams{
		Email:       "apikey-user-" + uuid.NewString()[:8] + "@example.com",
		Role:        userstore.UserRoleV2User,
		DisplayName: "Key Owner",
	})
	if err != nil {
		t.Fatalf("CreateUserV2 for principal failed: %v", err)
	}

	// Create principal
	principal, _ := store.CreatePrincipal(ctx, userstore.CreatePrincipalParams{
		GatewayID: gw.ID, PrincipalType: userstore.PrincipalTypeUser, UserID: &principalUser.ID, DisplayName: "Key Owner",
	})

	// Create API key
	key, rawKey, err := store.CreateAPIKeyV2(ctx, userstore.CreateAPIKeyV2Params{
		GatewayID:   gw.ID,
		PrincipalID: principal.ID,
		KeyName:     "Development Key",
	})
	if err != nil {
		t.Fatalf("CreateAPIKeyV2 failed: %v", err)
	}

	if !hasPrefix(rawKey, "tok_") {
		t.Errorf("Raw key should have prefix 'tok_', got %s", rawKey[:8])
	}
	if key.KeyName != "Development Key" {
		t.Errorf("KeyName mismatch: got %v", key.KeyName)
	}

	// Validate key using LookupAPIKeyV2
	validated, foundPrincipal, gateway, err := store.LookupAPIKeyV2(ctx, rawKey)
	if err != nil {
		t.Fatalf("LookupAPIKeyV2 failed: %v", err)
	}
	if validated == nil {
		t.Fatal("LookupAPIKeyV2 returned nil key")
	}
	if validated.ID != key.ID {
		t.Error("Validated key ID should match created key")
	}
	if foundPrincipal == nil {
		t.Error("LookupAPIKeyV2 should return principal")
	}
	if gateway == nil {
		t.Error("LookupAPIKeyV2 should return gateway")
	}
}

func TestAPIKeyV2_Rotate(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()
	ctx := context.Background()

	owner, _ := store.CreateUserV2(ctx, userstore.CreateUserParams{
		Email:       "owner-" + uuid.NewString()[:8] + "@example.com",
		Role:        userstore.UserRoleV2User,
		DisplayName: "Owner",
	})
	gw, _ := store.CreateGateway(ctx, owner.ID, "Rotate Gateway")

	// Create a real user for the principal to reference
	principalUser, err := store.CreateUserV2(ctx, userstore.CreateUserParams{
		Email:       "rotate-user-" + uuid.NewString()[:8] + "@example.com",
		Role:        userstore.UserRoleV2User,
		DisplayName: "Key Owner",
	})
	if err != nil {
		t.Fatalf("CreateUserV2 for principal failed: %v", err)
	}

	principal, _ := store.CreatePrincipal(ctx, userstore.CreatePrincipalParams{
		GatewayID: gw.ID, PrincipalType: userstore.PrincipalTypeUser, UserID: &principalUser.ID, DisplayName: "Key Owner",
	})

	// Create initial key
	key, oldRawKey, _ := store.CreateAPIKeyV2(ctx, userstore.CreateAPIKeyV2Params{
		GatewayID: gw.ID, PrincipalID: principal.ID, KeyName: "Rotating Key",
	})

	// Rotate key (no grace period, immediate revocation)
	newKey, newRawKey, err := store.RotateAPIKeyV2(ctx, key.ID, 0)
	if err != nil {
		t.Fatalf("RotateAPIKeyV2 failed: %v", err)
	}

	if newRawKey == oldRawKey {
		t.Error("New key should be different from old key")
	}
	if newKey == nil {
		t.Fatal("RotateAPIKeyV2 should return new key")
	}

	// Old key should not work (revoked)
	oldValidated, _, _, _ := store.LookupAPIKeyV2(ctx, oldRawKey)
	if oldValidated != nil {
		t.Error("Old key should not be valid after rotation")
	}

	// New key should work
	validated, _, _, err := store.LookupAPIKeyV2(ctx, newRawKey)
	if err != nil {
		t.Fatalf("LookupAPIKeyV2 (new key) failed: %v", err)
	}
	if validated == nil {
		t.Error("New key should be valid")
	}
}

// ==============================================================================
// Helper Functions
// ==============================================================================

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
