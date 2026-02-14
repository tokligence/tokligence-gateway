package graphql_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/tokligence/tokligence-gateway/internal/graphql"
	"github.com/tokligence/tokligence-gateway/internal/userstore/postgres_v2"
)

// uniqueEmail generates a unique email for testing.
func uniqueEmail(prefix string) string {
	return fmt.Sprintf("%s-%d@example.com", prefix, time.Now().UnixNano())
}

// getTestDSN returns the PostgreSQL DSN for testing.
func getTestDSN() string {
	if dsn := os.Getenv("TOKLIGENCE_DB_DSN"); dsn != "" {
		return dsn
	}
	return "postgres://postgres:postgres@localhost:5432/tokligence_test?sslmode=disable"
}

// setupTestServer creates a new GraphQL test server.
func setupTestServer(t *testing.T) (*httptest.Server, *postgres_v2.Store) {
	t.Helper()
	dsn := getTestDSN()
	store, err := postgres_v2.New(dsn, postgres_v2.DefaultConfig())
	if err != nil {
		t.Skipf("Skipping test: cannot connect to database: %v", err)
	}

	handler := graphql.NewHandler(store)
	server := httptest.NewServer(handler)
	return server, store
}

// graphQLRequest makes a GraphQL request and returns the response.
func graphQLRequest(t *testing.T, server *httptest.Server, query string, variables map[string]interface{}) map[string]interface{} {
	t.Helper()

	body := map[string]interface{}{
		"query":     query,
		"variables": variables,
	}
	bodyBytes, _ := json.Marshal(body)

	resp, err := http.Post(server.URL, "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	return result
}

// ==============================================================================
// Tests
// ==============================================================================

func TestPing(t *testing.T) {
	server, store := setupTestServer(t)
	defer server.Close()
	defer store.Close()

	result := graphQLRequest(t, server, `query { ping }`, nil)

	if errors, ok := result["errors"]; ok {
		t.Fatalf("Unexpected errors: %v", errors)
	}

	data := result["data"].(map[string]interface{})
	if data["ping"] != "pong" {
		t.Errorf("Expected 'pong', got %v", data["ping"])
	}
}

func TestCreateAndGetUser(t *testing.T) {
	server, store := setupTestServer(t)
	defer server.Close()
	defer store.Close()

	// Create user
	createQuery := `
		mutation CreateUser($input: CreateUserInput!) {
			createUser(input: $input) {
				id
				email
				role
				displayName
				status
			}
		}
	`
	email := uniqueEmail("test-graphql")
	createVars := map[string]interface{}{
		"input": map[string]interface{}{
			"email":       email,
			"role":        "USER",
			"displayName": "Test GraphQL User",
		},
	}

	result := graphQLRequest(t, server, createQuery, createVars)
	if errors, ok := result["errors"]; ok {
		t.Fatalf("Create user failed: %v", errors)
	}

	data := result["data"].(map[string]interface{})
	createUser := data["createUser"].(map[string]interface{})

	userID := createUser["id"].(string)
	if userID == "" {
		t.Error("User ID should not be empty")
	}
	if createUser["email"] != email {
		t.Errorf("Email mismatch: got %v", createUser["email"])
	}
	if createUser["role"] != "USER" {
		t.Errorf("Role mismatch: got %v", createUser["role"])
	}

	// Get user by ID
	getQuery := `
		query GetUser($id: ID!) {
			user(id: $id) {
				id
				email
				displayName
			}
		}
	`
	getVars := map[string]interface{}{
		"id": userID,
	}

	result = graphQLRequest(t, server, getQuery, getVars)
	if errors, ok := result["errors"]; ok {
		t.Fatalf("Get user failed: %v", errors)
	}

	data = result["data"].(map[string]interface{})
	getUser := data["user"].(map[string]interface{})
	if getUser["id"] != userID {
		t.Errorf("ID mismatch: got %v, want %v", getUser["id"], userID)
	}
}

func TestListUsers(t *testing.T) {
	server, store := setupTestServer(t)
	defer server.Close()
	defer store.Close()

	// List users
	query := `
		query ListUsers {
			users {
				id
				email
				role
			}
		}
	`

	result := graphQLRequest(t, server, query, nil)
	if errors, ok := result["errors"]; ok {
		t.Fatalf("List users failed: %v", errors)
	}

	data := result["data"].(map[string]interface{})
	users := data["users"].([]interface{})
	// Should have at least one user (from previous test or existing data)
	t.Logf("Found %d users", len(users))
}

func TestUpdateUser(t *testing.T) {
	server, store := setupTestServer(t)
	defer server.Close()
	defer store.Close()

	// Create user first
	createQuery := `
		mutation CreateUser($input: CreateUserInput!) {
			createUser(input: $input) {
				id
			}
		}
	`
	createVars := map[string]interface{}{
		"input": map[string]interface{}{
			"email":       uniqueEmail("update-test"),
			"role":        "USER",
			"displayName": "Original Name",
		},
	}

	result := graphQLRequest(t, server, createQuery, createVars)
	if errors, ok := result["errors"]; ok {
		t.Fatalf("Create user failed: %v", errors)
	}

	data := result["data"].(map[string]interface{})
	userID := data["createUser"].(map[string]interface{})["id"].(string)

	// Update user
	updateQuery := `
		mutation UpdateUser($id: ID!, $input: UpdateUserInput!) {
			updateUser(id: $id, input: $input) {
				id
				displayName
			}
		}
	`
	updateVars := map[string]interface{}{
		"id": userID,
		"input": map[string]interface{}{
			"displayName": "Updated Name",
		},
	}

	result = graphQLRequest(t, server, updateQuery, updateVars)
	if errors, ok := result["errors"]; ok {
		t.Fatalf("Update user failed: %v", errors)
	}

	data = result["data"].(map[string]interface{})
	updateUser := data["updateUser"].(map[string]interface{})
	if updateUser["displayName"] != "Updated Name" {
		t.Errorf("DisplayName not updated: got %v", updateUser["displayName"])
	}
}

func TestUserByEmail(t *testing.T) {
	server, store := setupTestServer(t)
	defer server.Close()
	defer store.Close()

	// Create user
	createQuery := `
		mutation CreateUser($input: CreateUserInput!) {
			createUser(input: $input) {
				id
				email
			}
		}
	`
	email := uniqueEmail("email-lookup")
	createVars := map[string]interface{}{
		"input": map[string]interface{}{
			"email":       email,
			"role":        "USER",
			"displayName": "Email Lookup Test",
		},
	}

	result := graphQLRequest(t, server, createQuery, createVars)
	if errors, ok := result["errors"]; ok {
		t.Fatalf("Create user failed: %v", errors)
	}

	// Get by email
	getQuery := `
		query GetUserByEmail($email: String!) {
			userByEmail(email: $email) {
				id
				email
			}
		}
	`
	getVars := map[string]interface{}{
		"email": email,
	}

	result = graphQLRequest(t, server, getQuery, getVars)
	if errors, ok := result["errors"]; ok {
		t.Fatalf("Get user by email failed: %v", errors)
	}

	data := result["data"].(map[string]interface{})
	user := data["userByEmail"].(map[string]interface{})
	if user["email"] != email {
		t.Errorf("Email mismatch: got %v, want %v", user["email"], email)
	}
}

func TestDeleteUser(t *testing.T) {
	server, store := setupTestServer(t)
	defer server.Close()
	defer store.Close()

	// Create user
	createQuery := `
		mutation CreateUser($input: CreateUserInput!) {
			createUser(input: $input) {
				id
			}
		}
	`
	createVars := map[string]interface{}{
		"input": map[string]interface{}{
			"email":       uniqueEmail("delete-test"),
			"role":        "USER",
			"displayName": "Delete Test",
		},
	}

	result := graphQLRequest(t, server, createQuery, createVars)
	if errors, ok := result["errors"]; ok {
		t.Fatalf("Create user failed: %v", errors)
	}

	data := result["data"].(map[string]interface{})
	userID := data["createUser"].(map[string]interface{})["id"].(string)

	// Delete user
	deleteQuery := `
		mutation DeleteUser($id: ID!) {
			deleteUser(id: $id)
		}
	`
	deleteVars := map[string]interface{}{
		"id": userID,
	}

	result = graphQLRequest(t, server, deleteQuery, deleteVars)
	if errors, ok := result["errors"]; ok {
		t.Fatalf("Delete user failed: %v", errors)
	}

	data = result["data"].(map[string]interface{})
	if data["deleteUser"] != true {
		t.Error("Delete should return true")
	}

	// Verify user is deleted (should return null)
	getQuery := `
		query GetUser($id: ID!) {
			user(id: $id) {
				id
			}
		}
	`
	getVars := map[string]interface{}{
		"id": userID,
	}

	result = graphQLRequest(t, server, getQuery, getVars)
	data = result["data"].(map[string]interface{})
	if data["user"] != nil {
		t.Error("Deleted user should not be found")
	}
}
