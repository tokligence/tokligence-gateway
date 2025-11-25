# User System V2 API Reference

This document provides common API usage examples for the Tokligence User System V2.

## Overview

The User System V2 provides:
- **Gateways**: Top-level resource representing a Tokligence instance
- **OrgUnits**: Flexible organizational hierarchy (departments, teams, projects)
- **Principals**: Unified identity for users, services, and environments
- **OrgMemberships**: Many-to-many relationship between principals and org units
- **Budgets**: Spending and rate limit configurations
- **APIKeys**: Scoped API keys with fine-grained permissions

## Base URL

```
http://localhost:8081/api/v2
```

## Authentication

All requests require an `Authorization` header:
```
Authorization: Bearer <api_key>
```

---

## Gateway Operations

### Create Gateway

```bash
curl -X POST http://localhost:8081/api/v2/gateways \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{
    "owner_user_id": "550e8400-e29b-41d4-a716-446655440001",
    "alias": "Acme Corp Gateway"
  }'
```

**Response:**
```json
{
  "ID": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "Alias": "Acme Corp Gateway",
  "OwnerUserID": "550e8400-e29b-41d4-a716-446655440001",
  "ProviderEnabled": false,
  "ConsumerEnabled": true,
  "Metadata": {},
  "CreatedAt": "2024-01-15T10:30:00Z",
  "UpdatedAt": "2024-01-15T10:30:00Z"
}
```

### Get Gateway

```bash
curl http://localhost:8081/api/v2/gateways/7c9e6679-7425-40de-944b-e07fc1f90ae7 \
  -H "Authorization: Bearer $API_KEY"
```

### List User's Gateways

```bash
curl "http://localhost:8081/api/v2/users/550e8400-e29b-41d4-a716-446655440001/gateways" \
  -H "Authorization: Bearer $API_KEY"
```

### Update Gateway

```bash
curl -X PATCH http://localhost:8081/api/v2/gateways/7c9e6679-7425-40de-944b-e07fc1f90ae7 \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{
    "alias": "Acme Corp Production Gateway",
    "provider_enabled": true
  }'
```

---

## Gateway Membership Operations

### Add Gateway Member

```bash
curl -X POST http://localhost:8081/api/v2/gateways/7c9e6679-7425-40de-944b-e07fc1f90ae7/members \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{
    "user_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "role": "admin"
  }'
```

**Roles:** `owner`, `admin`, `member`

### List Gateway Members

```bash
curl http://localhost:8081/api/v2/gateways/7c9e6679-7425-40de-944b-e07fc1f90ae7/members \
  -H "Authorization: Bearer $API_KEY"
```

### Update Member Role

```bash
curl -X PATCH http://localhost:8081/api/v2/gateways/7c9e6679-7425-40de-944b-e07fc1f90ae7/members/b2c3d4e5-f6a7-8901-bcde-f23456789012 \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{
    "role": "member"
  }'
```

---

## OrgUnit Operations

### Create OrgUnit (Root Level)

```bash
curl -X POST http://localhost:8081/api/v2/org-units \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{
    "gateway_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
    "name": "Engineering",
    "slug": "engineering",
    "unit_type": "department"
  }'
```

**Response:**
```json
{
  "ID": "c3d4e5f6-a7b8-9012-cdef-345678901234",
  "GatewayID": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "ParentID": null,
  "Path": "/engineering",
  "Depth": 0,
  "Name": "Engineering",
  "Slug": "engineering",
  "UnitType": "department",
  "BudgetID": null,
  "AllowedModels": [],
  "Metadata": {}
}
```

### Create OrgUnit (Child)

```bash
curl -X POST http://localhost:8081/api/v2/org-units \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{
    "gateway_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
    "parent_id": "c3d4e5f6-a7b8-9012-cdef-345678901234",
    "name": "Backend Team",
    "slug": "backend",
    "unit_type": "team",
    "allowed_models": ["gpt-4", "claude-3-sonnet"]
  }'
```

**UnitTypes:** `department`, `team`, `group`, `project`

### Get OrgUnit Tree

```bash
curl "http://localhost:8081/api/v2/gateways/7c9e6679-7425-40de-944b-e07fc1f90ae7/org-tree" \
  -H "Authorization: Bearer $API_KEY"
```

**Response:**
```json
[
  {
    "ID": "c3d4e5f6-a7b8-9012-cdef-345678901234",
    "Name": "Engineering",
    "Slug": "engineering",
    "UnitType": "department",
    "Path": "/engineering",
    "Depth": 0,
    "Children": [
      {
        "ID": "d4e5f6a7-b8c9-0123-def4-567890123456",
        "Name": "Backend Team",
        "Slug": "backend",
        "UnitType": "team",
        "Path": "/engineering/backend",
        "Depth": 1,
        "Children": []
      }
    ]
  }
]
```

### Get OrgUnits by Path

```bash
curl "http://localhost:8081/api/v2/gateways/7c9e6679-7425-40de-944b-e07fc1f90ae7/org-units?path_prefix=/engineering" \
  -H "Authorization: Bearer $API_KEY"
```

### Move OrgUnit

```bash
curl -X POST http://localhost:8081/api/v2/org-units/d4e5f6a7-b8c9-0123-def4-567890123456/move \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{
    "new_parent_id": "e5f6a7b8-c9d0-1234-ef56-789012345678"
  }'
```

### Merge OrgUnits

```bash
curl -X POST http://localhost:8081/api/v2/org-units/source-id/merge \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{
    "target_id": "target-org-unit-id"
  }'
```

---

## Principal Operations

### Create User Principal

```bash
curl -X POST http://localhost:8081/api/v2/principals \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{
    "gateway_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
    "principal_type": "user",
    "user_id": "550e8400-e29b-41d4-a716-446655440001",
    "display_name": "John Doe"
  }'
```

### Create Service Principal

```bash
curl -X POST http://localhost:8081/api/v2/principals \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{
    "gateway_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
    "principal_type": "service",
    "service_name": "payment-service",
    "display_name": "Payment Service"
  }'
```

### Create Environment Principal

```bash
curl -X POST http://localhost:8081/api/v2/principals \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{
    "gateway_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
    "principal_type": "environment",
    "environment_name": "production",
    "display_name": "Production Environment"
  }'
```

**PrincipalTypes:** `user`, `service`, `environment`

### List Principals

```bash
# List all principals
curl "http://localhost:8081/api/v2/gateways/7c9e6679-7425-40de-944b-e07fc1f90ae7/principals" \
  -H "Authorization: Bearer $API_KEY"

# Filter by type
curl "http://localhost:8081/api/v2/gateways/7c9e6679-7425-40de-944b-e07fc1f90ae7/principals?type=service" \
  -H "Authorization: Bearer $API_KEY"

# Filter by org unit
curl "http://localhost:8081/api/v2/gateways/7c9e6679-7425-40de-944b-e07fc1f90ae7/principals?org_unit_id=c3d4e5f6-a7b8-9012-cdef-345678901234" \
  -H "Authorization: Bearer $API_KEY"

# Search by name
curl "http://localhost:8081/api/v2/gateways/7c9e6679-7425-40de-944b-e07fc1f90ae7/principals?search=john" \
  -H "Authorization: Bearer $API_KEY"
```

---

## OrgMembership Operations

### Add Principal to OrgUnit

```bash
curl -X POST http://localhost:8081/api/v2/org-memberships \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{
    "principal_id": "f6a7b8c9-d0e1-2345-f678-901234567890",
    "org_unit_id": "c3d4e5f6-a7b8-9012-cdef-345678901234",
    "role": "member",
    "is_primary": true
  }'
```

**Roles:** `admin`, `member`, `viewer`

### List Principal's Memberships

```bash
curl "http://localhost:8081/api/v2/principals/f6a7b8c9-d0e1-2345-f678-901234567890/memberships" \
  -H "Authorization: Bearer $API_KEY"
```

**Response:**
```json
[
  {
    "ID": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "PrincipalID": "f6a7b8c9-d0e1-2345-f678-901234567890",
    "OrgUnitID": "c3d4e5f6-a7b8-9012-cdef-345678901234",
    "Role": "member",
    "IsPrimary": true,
    "OrgUnit": {
      "ID": "c3d4e5f6-a7b8-9012-cdef-345678901234",
      "Name": "Engineering",
      "Path": "/engineering"
    }
  }
]
```

### List OrgUnit Members

```bash
curl "http://localhost:8081/api/v2/org-units/c3d4e5f6-a7b8-9012-cdef-345678901234/members" \
  -H "Authorization: Bearer $API_KEY"
```

### Set Primary Membership

```bash
curl -X POST http://localhost:8081/api/v2/principals/f6a7b8c9-d0e1-2345-f678-901234567890/primary-membership \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{
    "membership_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
  }'
```

---

## Budget Operations

### Create Budget

```bash
curl -X POST http://localhost:8081/api/v2/budgets \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{
    "gateway_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
    "name": "Team Monthly Budget",
    "max_budget": 1000.00,
    "budget_duration": "monthly",
    "tpm_limit": 100000,
    "rpm_limit": 1000,
    "soft_limit": 80
  }'
```

**BudgetDurations:** `daily`, `weekly`, `monthly`, `total`

**Response:**
```json
{
  "ID": "b2c3d4e5-f6a7-8901-bcde-f23456789012",
  "GatewayID": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "Name": "Team Monthly Budget",
  "MaxBudget": "1000.00",
  "BudgetDuration": "monthly",
  "TPMLimit": 100000,
  "RPMLimit": 1000,
  "SoftLimit": "80.00"
}
```

### Resolve Effective Budget

Get the effective budget for a principal following inheritance chain:

```bash
curl "http://localhost:8081/api/v2/principals/f6a7b8c9-d0e1-2345-f678-901234567890/effective-budget" \
  -H "Authorization: Bearer $API_KEY"
```

**Response:**
```json
{
  "EffectiveBudget": {
    "ID": "b2c3d4e5-f6a7-8901-bcde-f23456789012",
    "Name": "Team Monthly Budget",
    "MaxBudget": "1000.00"
  },
  "Source": "orgunit:Engineering",
  "Chain": [
    {"Type": "principal", "ID": "...", "Name": "John Doe", "Budget": null},
    {"Type": "membership", "ID": "...", "Name": "Membership: Engineering", "Budget": null},
    {"Type": "orgunit", "ID": "...", "Name": "Engineering", "Budget": {...}}
  ]
}
```

---

## API Key Operations

### Create API Key

```bash
curl -X POST http://localhost:8081/api/v2/api-keys \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{
    "gateway_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
    "principal_id": "f6a7b8c9-d0e1-2345-f678-901234567890",
    "org_unit_id": "c3d4e5f6-a7b8-9012-cdef-345678901234",
    "key_name": "Development Key",
    "allowed_models": ["gpt-4", "gpt-3.5-turbo"],
    "allowed_ips": ["192.168.1.0/24"],
    "scopes": ["read", "write"],
    "expires_at": "2025-12-31T23:59:59Z"
  }'
```

**Response:**
```json
{
  "key": {
    "ID": "e5f6a7b8-c9d0-1234-ef56-789012345678",
    "GatewayID": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
    "PrincipalID": "f6a7b8c9-d0e1-2345-f678-901234567890",
    "KeyPrefix": "tok_abc12345",
    "KeyName": "Development Key",
    "AllowedModels": ["gpt-4", "gpt-3.5-turbo"],
    "AllowedIPs": ["192.168.1.0/24"],
    "Scopes": ["read", "write"],
    "ExpiresAt": "2025-12-31T23:59:59Z",
    "Blocked": false
  },
  "raw_key": "tok_abc12345def67890..."
}
```

**Important:** The `raw_key` is only returned once at creation. Store it securely!

### Lookup API Key (Validate)

```bash
curl -X POST http://localhost:8081/api/v2/api-keys/lookup \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{
    "token": "tok_abc12345def67890..."
  }'
```

**Response:**
```json
{
  "key": {...},
  "principal": {...},
  "gateway": {...}
}
```

### List API Keys

```bash
# List all keys for gateway
curl "http://localhost:8081/api/v2/gateways/7c9e6679-7425-40de-944b-e07fc1f90ae7/api-keys" \
  -H "Authorization: Bearer $API_KEY"

# Filter by principal
curl "http://localhost:8081/api/v2/gateways/7c9e6679-7425-40de-944b-e07fc1f90ae7/api-keys?principal_id=f6a7b8c9-d0e1-2345-f678-901234567890" \
  -H "Authorization: Bearer $API_KEY"

# Filter by blocked status
curl "http://localhost:8081/api/v2/gateways/7c9e6679-7425-40de-944b-e07fc1f90ae7/api-keys?blocked=false" \
  -H "Authorization: Bearer $API_KEY"
```

### Rotate API Key

```bash
curl -X POST http://localhost:8081/api/v2/api-keys/e5f6a7b8-c9d0-1234-ef56-789012345678/rotate \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{
    "grace_period_minutes": 60
  }'
```

**Response:**
```json
{
  "key": {...},
  "raw_key": "tok_new123456789..."
}
```

### Revoke API Key

```bash
curl -X DELETE http://localhost:8081/api/v2/api-keys/e5f6a7b8-c9d0-1234-ef56-789012345678 \
  -H "Authorization: Bearer $API_KEY"
```

### Block/Unblock API Key

```bash
curl -X PATCH http://localhost:8081/api/v2/api-keys/e5f6a7b8-c9d0-1234-ef56-789012345678 \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{
    "blocked": true
  }'
```

---

## Go SDK Usage Examples

### Initialize Store

```go
package main

import (
    "context"
    "log"

    "github.com/google/uuid"
    "github.com/tokligence/tokligence-gateway/internal/userstore"
    "github.com/tokligence/tokligence-gateway/internal/userstore/postgres_v2"
)

func main() {
    // Initialize store
    dsn := "postgres://user:pass@localhost:5432/tokligence?sslmode=disable"
    store, err := postgres_v2.New(dsn, postgres_v2.DefaultConfig())
    if err != nil {
        log.Fatal(err)
    }
    defer store.Close()

    ctx := context.Background()
    ownerUserID := uuid.New()

    // Create gateway
    gw, err := store.CreateGateway(ctx, ownerUserID, "My Gateway")
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("Created gateway: %s", gw.ID)
}
```

### Build Organization Tree

```go
// Create departments
engineering, _ := store.CreateOrgUnit(ctx, userstore.CreateOrgUnitParams{
    GatewayID: gw.ID,
    Name:      "Engineering",
    Slug:      "engineering",
    UnitType:  userstore.OrgUnitTypeDepartment,
})

// Create teams under engineering
backend, _ := store.CreateOrgUnit(ctx, userstore.CreateOrgUnitParams{
    GatewayID: gw.ID,
    ParentID:  &engineering.ID,
    Name:      "Backend",
    Slug:      "backend",
    UnitType:  userstore.OrgUnitTypeTeam,
    AllowedModels: []string{"gpt-4", "claude-3-sonnet"},
})

frontend, _ := store.CreateOrgUnit(ctx, userstore.CreateOrgUnitParams{
    GatewayID: gw.ID,
    ParentID:  &engineering.ID,
    Name:      "Frontend",
    Slug:      "frontend",
    UnitType:  userstore.OrgUnitTypeTeam,
})

// Get full tree
tree, _ := store.GetOrgUnitTree(ctx, gw.ID)
for _, root := range tree {
    printTree(root, 0)
}

func printTree(node userstore.OrgUnitWithChildren, depth int) {
    indent := strings.Repeat("  ", depth)
    log.Printf("%s%s (%s)", indent, node.Name, node.UnitType)
    for _, child := range node.Children {
        printTree(child, depth+1)
    }
}
```

### Create Principals and Memberships

```go
// Create user principal
userID := uuid.New()
userPrincipal, _ := store.CreatePrincipal(ctx, userstore.CreatePrincipalParams{
    GatewayID:     gw.ID,
    PrincipalType: userstore.PrincipalTypeUser,
    UserID:        &userID,
    DisplayName:   "John Doe",
})

// Create service principal
serviceName := "payment-service"
servicePrincipal, _ := store.CreatePrincipal(ctx, userstore.CreatePrincipalParams{
    GatewayID:     gw.ID,
    PrincipalType: userstore.PrincipalTypeService,
    ServiceName:   &serviceName,
    DisplayName:   "Payment Service",
})

// Add user to multiple teams (multi-team membership)
_, _ = store.AddOrgMembership(ctx, userstore.CreateOrgMembershipParams{
    PrincipalID: userPrincipal.ID,
    OrgUnitID:   backend.ID,
    Role:        userstore.OrgMemberRoleMember,
    IsPrimary:   true, // Primary team for spend attribution
})

_, _ = store.AddOrgMembership(ctx, userstore.CreateOrgMembershipParams{
    PrincipalID: userPrincipal.ID,
    OrgUnitID:   frontend.ID,
    Role:        userstore.OrgMemberRoleMember,
    IsPrimary:   false,
})

// List user's memberships
memberships, _ := store.ListOrgMemberships(ctx, userPrincipal.ID)
for _, m := range memberships {
    log.Printf("Member of: %s (primary: %v)", m.OrgUnit.Name, m.IsPrimary)
}
```

### Manage Budgets

```go
// Create budget configuration
maxBudget := 1000.0
tpmLimit := int64(100000)

budget, _ := store.CreateBudget(ctx, gw.ID, userstore.CreateBudgetParams{
    Name:           "Team Monthly Budget",
    MaxBudget:      &maxBudget,
    BudgetDuration: userstore.BudgetDurationMonthly,
    TPMLimit:       &tpmLimit,
})

// Assign budget to org unit
store.UpdateOrgUnit(ctx, backend.ID, userstore.OrgUnitUpdate{
    BudgetID: &budget.ID,
})

// Resolve effective budget for a principal
inheritance, _ := store.ResolveBudget(ctx, userPrincipal.ID)
if inheritance.EffectiveBudget != nil {
    log.Printf("Effective budget: $%.2f (%s)", *inheritance.EffectiveBudget.MaxBudget, inheritance.Source)
} else {
    log.Println("No budget limit (unlimited)")
}

// Show inheritance chain
for _, source := range inheritance.Chain {
    budgetStr := "no budget"
    if source.Budget != nil {
        budgetStr = source.Budget.Name
    }
    log.Printf("  %s: %s -> %s", source.Type, source.Name, budgetStr)
}
```

### API Key Management

```go
// Create API key
key, rawKey, _ := store.CreateAPIKeyV2(ctx, userstore.CreateAPIKeyV2Params{
    GatewayID:     gw.ID,
    PrincipalID:   userPrincipal.ID,
    OrgUnitID:     &backend.ID, // Spend attributed to this org unit
    KeyName:       "Development Key",
    AllowedModels: []string{"gpt-4"},
    AllowedIPs:    []string{"192.168.1.0/24"},
})
log.Printf("Created key: %s (prefix: %s)", rawKey, key.KeyPrefix)

// Validate key on API request
validatedKey, principal, gateway, err := store.LookupAPIKeyV2(ctx, rawKey)
if err != nil || validatedKey == nil {
    log.Fatal("Invalid API key")
}
log.Printf("Authenticated: %s (gateway: %s)", principal.DisplayName, gateway.Alias)

// Record usage
err = store.RecordAPIKeyUsage(ctx, key.ID, 0.05) // $0.05 spent
if err != nil {
    log.Printf("Failed to record usage: %v", err)
}

// Rotate key with grace period
newKey, newRawKey, _ := store.RotateAPIKeyV2(ctx, key.ID, time.Hour)
log.Printf("New key: %s (old key valid for 1 hour)", newRawKey)

// Revoke key
store.RevokeAPIKeyV2(ctx, key.ID)
```

---

## Error Handling

All endpoints return standard HTTP status codes:

| Status | Meaning |
|--------|---------|
| 200 | Success |
| 201 | Created |
| 400 | Bad Request (invalid input) |
| 401 | Unauthorized (invalid API key) |
| 403 | Forbidden (insufficient permissions) |
| 404 | Not Found |
| 409 | Conflict (e.g., duplicate membership) |
| 500 | Internal Server Error |

Error response format:
```json
{
  "error": {
    "code": "INVALID_PRINCIPAL_TYPE",
    "message": "Principal type must be one of: user, service, environment"
  }
}
```

---

## Budget Inheritance Chain

When resolving the effective budget for a principal, the system follows this inheritance chain:

1. **Principal's own budget** - If set, this takes priority
2. **OrgMembership budget** - Budget on the primary membership
3. **OrgUnit budget** - Budget on the primary org unit
4. **Parent OrgUnit budget** - Walk up the tree recursively
5. **Gateway default** - If no budget found, unlimited access

This allows flexible budget configuration at any level of the hierarchy while ensuring predictable behavior.

---

## GraphQL API

The User System V2 also provides a GraphQL API for more flexible queries and mutations.

### GraphQL Endpoint

```
POST /graphql
```

### GraphQL Playground

For interactive development:
```
GET /graphql/playground
```

### GraphQL Schema Overview

The GraphQL API supports the following types:

**Scalars:**
- `Time` - ISO 8601 timestamp
- `Map` - JSON object (key-value pairs)

**Enums:**
- `UserRole`: `ROOT_ADMIN`, `ADMIN`, `USER`
- `UserStatus`: `ACTIVE`, `INACTIVE`, `SUSPENDED`
- `GatewayMemberRole`: `OWNER`, `ADMIN`, `MEMBER`
- `OrgUnitType`: `DEPARTMENT`, `TEAM`, `GROUP`, `PROJECT`
- `PrincipalType`: `USER`, `SERVICE`, `ENVIRONMENT`
- `OrgMemberRole`: `ADMIN`, `MEMBER`, `VIEWER`
- `BudgetDuration`: `DAILY`, `WEEKLY`, `MONTHLY`, `TOTAL`

### Example Queries

#### Ping (Health Check)

```graphql
query {
  ping
}
```

Response:
```json
{"data": {"ping": "pong"}}
```

#### Get User by ID

```graphql
query GetUser($id: ID!) {
  user(id: $id) {
    id
    email
    displayName
    role
    status
    createdAt
  }
}
```

Variables:
```json
{"id": "550e8400-e29b-41d4-a716-446655440001"}
```

#### Get User by Email

```graphql
query GetUserByEmail($email: String!) {
  userByEmail(email: $email) {
    id
    email
    displayName
  }
}
```

#### List All Users

```graphql
query ListUsers {
  users {
    id
    email
    role
    status
  }
}
```

#### List Users with Filters

```graphql
query ListUsersFiltered($filter: UserFilter) {
  users(filter: $filter) {
    id
    email
    role
  }
}
```

Variables:
```json
{
  "filter": {
    "role": "USER",
    "status": "ACTIVE",
    "search": "john"
  }
}
```

### Example Mutations

#### Create User

```graphql
mutation CreateUser($input: CreateUserInput!) {
  createUser(input: $input) {
    id
    email
    displayName
    role
    status
  }
}
```

Variables:
```json
{
  "input": {
    "email": "john@example.com",
    "role": "USER",
    "displayName": "John Doe"
  }
}
```

#### Update User

```graphql
mutation UpdateUser($id: ID!, $input: UpdateUserInput!) {
  updateUser(id: $id, input: $input) {
    id
    displayName
    status
  }
}
```

Variables:
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440001",
  "input": {
    "displayName": "John D.",
    "status": "INACTIVE"
  }
}
```

#### Delete User

```graphql
mutation DeleteUser($id: ID!) {
  deleteUser(id: $id)
}
```

### Testing GraphQL API

#### Using curl

```bash
# Ping query
curl -X POST http://localhost:8081/graphql \
  -H "Content-Type: application/json" \
  -d '{"query": "query { ping }"}'

# Create user
curl -X POST http://localhost:8081/graphql \
  -H "Content-Type: application/json" \
  -d '{
    "query": "mutation CreateUser($input: CreateUserInput!) { createUser(input: $input) { id email } }",
    "variables": {
      "input": {
        "email": "test@example.com",
        "role": "USER",
        "displayName": "Test User"
      }
    }
  }'
```

#### Using Test Scripts

```bash
# Run Go unit tests
./scripts/test_graphql_unit.sh -v

# Run integration tests against running server
./scripts/graphql_test.sh http://localhost:8081/graphql
```

### Go SDK - Using GraphQL Handler

```go
package main

import (
    "net/http"

    "github.com/tokligence/tokligence-gateway/internal/graphql"
    "github.com/tokligence/tokligence-gateway/internal/userstore/postgres_v2"
)

func main() {
    // Initialize store
    store, _ := postgres_v2.New(dsn, postgres_v2.DefaultConfig())
    defer store.Close()

    // Create GraphQL handler
    gqlHandler := graphql.NewHandler(store)

    // Create playground handler (optional, for development)
    playgroundHandler := graphql.NewPlaygroundHandler("/graphql")

    // Register handlers
    http.Handle("/graphql", gqlHandler)
    http.Handle("/graphql/playground", playgroundHandler)

    http.ListenAndServe(":8081", nil)
}
```

### Error Handling in GraphQL

GraphQL errors are returned in the `errors` array:

```json
{
  "data": null,
  "errors": [
    {
      "message": "user not found",
      "path": ["user"]
    }
  ]
}
```

Common error messages:
- `"user not found"` - User with given ID doesn't exist
- `"invalid UUID format"` - Invalid ID format
- `"email already exists"` - Duplicate email on user creation
- `"create user: pq: ..."` - Database constraint violation
