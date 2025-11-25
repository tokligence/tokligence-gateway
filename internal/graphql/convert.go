package graphql

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tokligence/tokligence-gateway/internal/userstore"
)

// ==============================================================================
// Enum Conversions (GraphQL uppercase <-> DB lowercase)
// ==============================================================================

// userRoleToStore converts GraphQL UserRole (uppercase) to userstore.UserRoleV2 (lowercase).
func userRoleToStore(role UserRole) userstore.UserRoleV2 {
	return userstore.UserRoleV2(strings.ToLower(string(role)))
}

// userStatusToStore converts GraphQL UserStatus (uppercase) to userstore.UserStatusV2 (lowercase).
func userStatusToStore(status UserStatus) userstore.UserStatusV2 {
	return userstore.UserStatusV2(strings.ToLower(string(status)))
}

// gatewayMemberRoleToStore converts GraphQL GatewayMemberRole (uppercase) to userstore.GatewayMemberRole (lowercase).
func gatewayMemberRoleToStore(role GatewayMemberRole) userstore.GatewayMemberRole {
	return userstore.GatewayMemberRole(strings.ToLower(string(role)))
}

// orgUnitTypeToStore converts GraphQL OrgUnitType (uppercase) to userstore.OrgUnitType (lowercase).
func orgUnitTypeToStore(unitType OrgUnitType) userstore.OrgUnitType {
	return userstore.OrgUnitType(strings.ToLower(string(unitType)))
}

// principalTypeToStore converts GraphQL PrincipalType (uppercase) to userstore.PrincipalType (lowercase).
func principalTypeToStore(ptype PrincipalType) userstore.PrincipalType {
	return userstore.PrincipalType(strings.ToLower(string(ptype)))
}

// orgMemberRoleToStore converts GraphQL OrgMemberRole (uppercase) to userstore.OrgMemberRole (lowercase).
func orgMemberRoleToStore(role OrgMemberRole) userstore.OrgMemberRole {
	return userstore.OrgMemberRole(strings.ToLower(string(role)))
}

// budgetDurationToStore converts GraphQL BudgetDuration (uppercase) to userstore.BudgetDuration (lowercase).
func budgetDurationToStore(dur BudgetDuration) userstore.BudgetDuration {
	return userstore.BudgetDuration(strings.ToLower(string(dur)))
}

// userRoleToGQL converts userstore.UserRoleV2 (lowercase) to GraphQL UserRole (uppercase).
func userRoleToGQL(role userstore.UserRoleV2) UserRole {
	return UserRole(strings.ToUpper(string(role)))
}

// userStatusToGQL converts userstore.UserStatusV2 (lowercase) to GraphQL UserStatus (uppercase).
func userStatusToGQL(status userstore.UserStatusV2) UserStatus {
	return UserStatus(strings.ToUpper(string(status)))
}

// gatewayMemberRoleToGQL converts userstore.GatewayMemberRole (lowercase) to GraphQL GatewayMemberRole (uppercase).
func gatewayMemberRoleToGQL(role userstore.GatewayMemberRole) GatewayMemberRole {
	return GatewayMemberRole(strings.ToUpper(string(role)))
}

// orgUnitTypeToGQL converts userstore.OrgUnitType (lowercase) to GraphQL OrgUnitType (uppercase).
func orgUnitTypeToGQL(unitType userstore.OrgUnitType) OrgUnitType {
	return OrgUnitType(strings.ToUpper(string(unitType)))
}

// principalTypeToGQL converts userstore.PrincipalType (lowercase) to GraphQL PrincipalType (uppercase).
func principalTypeToGQL(ptype userstore.PrincipalType) PrincipalType {
	return PrincipalType(strings.ToUpper(string(ptype)))
}

// orgMemberRoleToGQL converts userstore.OrgMemberRole (lowercase) to GraphQL OrgMemberRole (uppercase).
func orgMemberRoleToGQL(role userstore.OrgMemberRole) OrgMemberRole {
	return OrgMemberRole(strings.ToUpper(string(role)))
}

// budgetDurationToGQL converts userstore.BudgetDuration (lowercase) to GraphQL BudgetDuration (uppercase).
func budgetDurationToGQL(dur userstore.BudgetDuration) BudgetDuration {
	return BudgetDuration(strings.ToUpper(string(dur)))
}

// ==============================================================================
// UUID Helpers
// ==============================================================================

// parseUUID converts a string to uuid.UUID.
func parseUUID(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}

// ptrString returns a pointer to the string if non-empty, nil otherwise.
func ptrString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// ptrUUID returns a pointer to a UUID parsed from string, nil if empty or invalid.
func ptrUUID(s *string) *uuid.UUID {
	if s == nil || *s == "" {
		return nil
	}
	id, err := uuid.Parse(*s)
	if err != nil {
		return nil
	}
	return &id
}

// ==============================================================================
// User Conversions
// ==============================================================================

// userToGQL converts a userstore.UserV2 to GraphQL User.
func userToGQL(u *userstore.UserV2) *User {
	if u == nil {
		return nil
	}
	return &User{
		ID:           u.ID.String(),
		Email:        u.Email,
		Role:         userRoleToGQL(u.Role),
		DisplayName:  u.DisplayName,
		AvatarURL:    u.AvatarURL,
		Status:       userStatusToGQL(u.Status),
		AuthProvider: u.AuthProvider,
		ExternalID:   u.ExternalID,
		LastLoginAt:  u.LastLoginAt,
		Metadata:     u.Metadata,
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
	}
}

// usersToGQL converts a slice of userstore.UserV2 to GraphQL Users.
func usersToGQL(users []userstore.UserV2) []*User {
	result := make([]*User, len(users))
	for i := range users {
		result[i] = userToGQL(&users[i])
	}
	return result
}

// createUserInputToParams converts GraphQL CreateUserInput to userstore.CreateUserParams.
func createUserInputToParams(input CreateUserInput) userstore.CreateUserParams {
	authProvider := "local"
	if input.AuthProvider != nil {
		authProvider = *input.AuthProvider
	}
	return userstore.CreateUserParams{
		Email:        input.Email,
		Role:         userRoleToStore(input.Role),
		DisplayName:  input.DisplayName,
		AvatarURL:    input.AvatarURL,
		AuthProvider: authProvider,
		ExternalID:   input.ExternalID,
		Metadata:     input.Metadata,
	}
}

// updateUserInputToUpdate converts GraphQL UpdateUserInput to userstore.UserUpdate.
func updateUserInputToUpdate(input UpdateUserInput) userstore.UserUpdate {
	var update userstore.UserUpdate
	if input.DisplayName != nil {
		update.DisplayName = input.DisplayName
	}
	if input.AvatarURL != nil {
		update.AvatarURL = input.AvatarURL
	}
	if input.Role != nil {
		role := userRoleToStore(*input.Role)
		update.Role = &role
	}
	if input.Status != nil {
		status := userStatusToStore(*input.Status)
		update.Status = &status
	}
	if input.Metadata != nil {
		meta := userstore.JSONMap(input.Metadata)
		update.Metadata = &meta
	}
	return update
}

// userFilterToStore converts GraphQL UserFilter to userstore.UserFilter.
func userFilterToStore(filter *UserFilter) userstore.UserFilter {
	if filter == nil {
		return userstore.UserFilter{}
	}
	var f userstore.UserFilter
	if filter.Role != nil {
		role := userRoleToStore(*filter.Role)
		f.Role = &role
	}
	if filter.Status != nil {
		status := userStatusToStore(*filter.Status)
		f.Status = &status
	}
	f.Search = filter.Search
	return f
}

// ==============================================================================
// Gateway Conversions
// ==============================================================================

// gatewayToGQL converts a userstore.Gateway to GraphQL Gateway.
func gatewayToGQL(g *userstore.Gateway) *Gateway {
	if g == nil {
		return nil
	}
	return &Gateway{
		ID:              g.ID.String(),
		Alias:           g.Alias,
		OwnerUserID:     g.OwnerUserID.String(),
		ProviderEnabled: g.ProviderEnabled,
		ConsumerEnabled: g.ConsumerEnabled,
		Metadata:        g.Metadata,
		CreatedAt:       g.CreatedAt,
		UpdatedAt:       g.UpdatedAt,
	}
}

// gatewaysToGQL converts a slice of userstore.Gateway to GraphQL Gateways.
func gatewaysToGQL(gateways []userstore.Gateway) []*Gateway {
	result := make([]*Gateway, len(gateways))
	for i := range gateways {
		result[i] = gatewayToGQL(&gateways[i])
	}
	return result
}

// ==============================================================================
// GatewayMembership Conversions
// ==============================================================================

// gatewayMembershipToGQL converts a userstore.GatewayMembership to GraphQL GatewayMembership.
func gatewayMembershipToGQL(m *userstore.GatewayMembership) *GatewayMembership {
	if m == nil {
		return nil
	}
	return &GatewayMembership{
		ID:        m.ID.String(),
		UserID:    m.UserID.String(),
		GatewayID: m.GatewayID.String(),
		Role:      gatewayMemberRoleToGQL(m.Role),
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}

// ==============================================================================
// OrgUnit Conversions
// ==============================================================================

// orgUnitToGQL converts a userstore.OrgUnit to GraphQL OrgUnit.
func orgUnitToGQL(o *userstore.OrgUnit) *OrgUnit {
	if o == nil {
		return nil
	}
	var parentID *string
	if o.ParentID != nil {
		s := o.ParentID.String()
		parentID = &s
	}
	var budgetID *string
	if o.BudgetID != nil {
		s := o.BudgetID.String()
		budgetID = &s
	}
	return &OrgUnit{
		ID:            o.ID.String(),
		GatewayID:     o.GatewayID.String(),
		ParentID:      parentID,
		Path:          o.Path,
		Depth:         o.Depth,
		Name:          o.Name,
		Slug:          o.Slug,
		UnitType:      orgUnitTypeToGQL(o.UnitType),
		BudgetID:      budgetID,
		AllowedModels: o.AllowedModels,
		Metadata:      o.Metadata,
		CreatedAt:     o.CreatedAt,
		UpdatedAt:     o.UpdatedAt,
	}
}

// orgUnitsToGQL converts a slice of userstore.OrgUnit to GraphQL OrgUnits.
func orgUnitsToGQL(units []userstore.OrgUnit) []*OrgUnit {
	result := make([]*OrgUnit, len(units))
	for i := range units {
		result[i] = orgUnitToGQL(&units[i])
	}
	return result
}

// ==============================================================================
// Principal Conversions
// ==============================================================================

// principalToGQL converts a userstore.Principal to GraphQL Principal.
func principalToGQL(p *userstore.Principal) *Principal {
	if p == nil {
		return nil
	}
	var userID, budgetID *string
	if p.UserID != nil {
		s := p.UserID.String()
		userID = &s
	}
	if p.BudgetID != nil {
		s := p.BudgetID.String()
		budgetID = &s
	}
	return &Principal{
		ID:              p.ID.String(),
		GatewayID:       p.GatewayID.String(),
		PrincipalType:   principalTypeToGQL(p.PrincipalType),
		UserID:          userID,
		ServiceName:     p.ServiceName,
		EnvironmentName: p.EnvironmentName,
		DisplayName:     p.DisplayName,
		BudgetID:        budgetID,
		AllowedModels:   p.AllowedModels,
		Metadata:        p.Metadata,
		CreatedAt:       p.CreatedAt,
		UpdatedAt:       p.UpdatedAt,
	}
}

// principalsToGQL converts a slice of userstore.Principal to GraphQL Principals.
func principalsToGQL(principals []userstore.Principal) []*Principal {
	result := make([]*Principal, len(principals))
	for i := range principals {
		result[i] = principalToGQL(&principals[i])
	}
	return result
}

// ==============================================================================
// OrgMembership Conversions
// ==============================================================================

// orgMembershipToGQL converts a userstore.OrgMembership to GraphQL OrgMembership.
func orgMembershipToGQL(m *userstore.OrgMembership) *OrgMembership {
	if m == nil {
		return nil
	}
	var budgetID *string
	if m.BudgetID != nil {
		s := m.BudgetID.String()
		budgetID = &s
	}
	return &OrgMembership{
		ID:          m.ID.String(),
		PrincipalID: m.PrincipalID.String(),
		OrgUnitID:   m.OrgUnitID.String(),
		Role:        orgMemberRoleToGQL(m.Role),
		BudgetID:    budgetID,
		IsPrimary:   m.IsPrimary,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}

// ==============================================================================
// Budget Conversions
// ==============================================================================

// budgetToGQL converts a userstore.Budget to GraphQL Budget.
func budgetToGQL(b *userstore.Budget) *Budget {
	if b == nil {
		return nil
	}
	var tpmLimit, rpmLimit *int
	if b.TPMLimit != nil {
		v := int(*b.TPMLimit)
		tpmLimit = &v
	}
	if b.RPMLimit != nil {
		v := int(*b.RPMLimit)
		rpmLimit = &v
	}
	return &Budget{
		ID:             b.ID.String(),
		GatewayID:      b.GatewayID.String(),
		Name:           b.Name,
		MaxBudget:      b.MaxBudget,
		BudgetDuration: budgetDurationToGQL(b.BudgetDuration),
		TpmLimit:       tpmLimit,
		RpmLimit:       rpmLimit,
		SoftLimit:      b.SoftLimit,
		Metadata:       b.Metadata,
		CreatedAt:      b.CreatedAt,
		UpdatedAt:      b.UpdatedAt,
	}
}

// budgetsToGQL converts a slice of userstore.Budget to GraphQL Budgets.
func budgetsToGQL(budgets []userstore.Budget) []*Budget {
	result := make([]*Budget, len(budgets))
	for i := range budgets {
		result[i] = budgetToGQL(&budgets[i])
	}
	return result
}

// ==============================================================================
// APIKey Conversions
// ==============================================================================

// apiKeyToGQL converts a userstore.APIKeyV2 to GraphQL APIKey.
func apiKeyToGQL(k *userstore.APIKeyV2) *APIKey {
	if k == nil {
		return nil
	}
	var orgUnitID, budgetID *string
	if k.OrgUnitID != nil {
		s := k.OrgUnitID.String()
		orgUnitID = &s
	}
	if k.BudgetID != nil {
		s := k.BudgetID.String()
		budgetID = &s
	}
	return &APIKey{
		ID:            k.ID.String(),
		GatewayID:     k.GatewayID.String(),
		PrincipalID:   k.PrincipalID.String(),
		OrgUnitID:     orgUnitID,
		KeyPrefix:     k.KeyPrefix,
		KeyName:       k.KeyName,
		BudgetID:      budgetID,
		AllowedModels: k.AllowedModels,
		AllowedIps:    k.AllowedIPs,
		Scopes:        k.Scopes,
		ExpiresAt:     k.ExpiresAt,
		LastUsedAt:    k.LastUsedAt,
		TotalSpend:    k.TotalSpend,
		Blocked:       k.Blocked,
		CreatedAt:     k.CreatedAt,
		UpdatedAt:     k.UpdatedAt,
	}
}

// apiKeysToGQL converts a slice of userstore.APIKeyV2 to GraphQL APIKeys.
func apiKeysToGQL(keys []userstore.APIKeyV2) []*APIKey {
	result := make([]*APIKey, len(keys))
	for i := range keys {
		result[i] = apiKeyToGQL(&keys[i])
	}
	return result
}

// ==============================================================================
// Input Conversions
// ==============================================================================

// createOrgUnitInputToParams converts GraphQL input to userstore params.
func createOrgUnitInputToParams(input CreateOrgUnitInput) userstore.CreateOrgUnitParams {
	gatewayID, _ := parseUUID(input.GatewayID)
	return userstore.CreateOrgUnitParams{
		GatewayID:     gatewayID,
		ParentID:      ptrUUID(input.ParentID),
		Name:          input.Name,
		Slug:          input.Slug,
		UnitType:      orgUnitTypeToStore(input.UnitType),
		BudgetID:      ptrUUID(input.BudgetID),
		AllowedModels: input.AllowedModels,
		Metadata:      input.Metadata,
	}
}

// createPrincipalInputToParams converts GraphQL input to userstore params.
func createPrincipalInputToParams(input CreatePrincipalInput) userstore.CreatePrincipalParams {
	gatewayID, _ := parseUUID(input.GatewayID)
	return userstore.CreatePrincipalParams{
		GatewayID:       gatewayID,
		PrincipalType:   principalTypeToStore(input.PrincipalType),
		UserID:          ptrUUID(input.UserID),
		ServiceName:     input.ServiceName,
		EnvironmentName: input.EnvironmentName,
		DisplayName:     input.DisplayName,
		BudgetID:        ptrUUID(input.BudgetID),
		AllowedModels:   input.AllowedModels,
		Metadata:        input.Metadata,
	}
}

// createOrgMembershipInputToParams converts GraphQL input to userstore params.
func createOrgMembershipInputToParams(input CreateOrgMembershipInput) userstore.CreateOrgMembershipParams {
	principalID, _ := parseUUID(input.PrincipalID)
	orgUnitID, _ := parseUUID(input.OrgUnitID)
	isPrimary := false
	if input.IsPrimary != nil {
		isPrimary = *input.IsPrimary
	}
	return userstore.CreateOrgMembershipParams{
		PrincipalID: principalID,
		OrgUnitID:   orgUnitID,
		Role:        orgMemberRoleToStore(input.Role),
		BudgetID:    ptrUUID(input.BudgetID),
		IsPrimary:   isPrimary,
	}
}

// createBudgetInputToParams converts GraphQL input to userstore params.
func createBudgetInputToParams(input CreateBudgetInput) userstore.CreateBudgetParams {
	var tpmLimit, rpmLimit *int64
	if input.TpmLimit != nil {
		v := int64(*input.TpmLimit)
		tpmLimit = &v
	}
	if input.RpmLimit != nil {
		v := int64(*input.RpmLimit)
		rpmLimit = &v
	}
	return userstore.CreateBudgetParams{
		Name:           input.Name,
		MaxBudget:      input.MaxBudget,
		BudgetDuration: budgetDurationToStore(input.BudgetDuration),
		TPMLimit:       tpmLimit,
		RPMLimit:       rpmLimit,
		SoftLimit:      input.SoftLimit,
		Metadata:       input.Metadata,
	}
}

// createAPIKeyInputToParams converts GraphQL input to userstore params.
func createAPIKeyInputToParams(input CreateAPIKeyInput) userstore.CreateAPIKeyV2Params {
	gatewayID, _ := parseUUID(input.GatewayID)
	principalID, _ := parseUUID(input.PrincipalID)
	var expiresAt *time.Time
	if input.ExpiresAt != nil {
		expiresAt = input.ExpiresAt
	}
	return userstore.CreateAPIKeyV2Params{
		GatewayID:     gatewayID,
		PrincipalID:   principalID,
		OrgUnitID:     ptrUUID(input.OrgUnitID),
		KeyName:       input.KeyName,
		BudgetID:      ptrUUID(input.BudgetID),
		AllowedModels: input.AllowedModels,
		AllowedIPs:    input.AllowedIps,
		Scopes:        input.Scopes,
		ExpiresAt:     expiresAt,
	}
}

// principalFilterToStore converts GraphQL filter to userstore filter.
func principalFilterToStore(filter *PrincipalFilter) userstore.PrincipalFilter {
	if filter == nil {
		return userstore.PrincipalFilter{}
	}
	var f userstore.PrincipalFilter
	if filter.Type != nil {
		ptype := principalTypeToStore(*filter.Type)
		f.Type = &ptype
	}
	f.OrgUnitID = ptrUUID(filter.OrgUnitID)
	f.Search = filter.Search
	return f
}

// apiKeyFilterToStore converts GraphQL filter to userstore filter.
func apiKeyFilterToStore(filter *APIKeyFilter) userstore.APIKeyFilter {
	if filter == nil {
		return userstore.APIKeyFilter{}
	}
	return userstore.APIKeyFilter{
		PrincipalID: ptrUUID(filter.PrincipalID),
		OrgUnitID:   ptrUUID(filter.OrgUnitID),
		Blocked:     filter.Blocked,
	}
}
