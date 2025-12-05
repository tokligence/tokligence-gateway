package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/tokligence/tokligence-gateway/internal/userstore"
	"github.com/tokligence/tokligence-gateway/internal/userstore/sqlite"
)

func main() {
	if len(os.Args) < 3 {
		log.Fatalf("Usage: %s <db_path> <email> [role]", os.Args[0])
	}

	dbPath := os.Args[1]
	email := os.Args[2]
	role := "consumer"
	if len(os.Args) > 3 {
		role = os.Args[3]
	}

	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Fatalf("Failed to create directory %s: %v", dir, err)
	}

	// Create identity store (don't remove DB - it may already have admin user)
	store, err := sqlite.New(dbPath, 1, 1, 0, 0)
	if err != nil {
		log.Fatalf("Failed to create sqlite store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Ensure admin exists if this is an admin user
	var userID int64
	if role == "admin" {
		admin, err := store.EnsureRootAdmin(ctx, email)
		if err != nil {
			log.Fatalf("Failed to ensure root admin: %v", err)
		}
		userID = admin.ID
	} else {
		// For non-admin users, first ensure root admin exists (required for DB schema)
		_, err := store.EnsureRootAdmin(ctx, "admin@system.local")
		if err != nil {
			log.Fatalf("Failed to ensure system admin: %v", err)
		}

		// Then create regular user
		user, err := store.CreateUser(ctx, email, userstore.RoleGatewayUser, "Test User")
		if err != nil {
			log.Fatalf("Failed to create user: %v", err)
		}
		userID = user.ID
	}

	// Create API key
	roles := []string{role}
	if role == "admin" {
		roles = []string{"root_admin"}
	}
	key, token, err := store.CreateAPIKey(ctx, userID, roles, nil)
	if err != nil {
		log.Fatalf("Failed to create API key: %v", err)
	}

	// Print the token to stdout so the shell script can capture it
	fmt.Print(token)
	log.Printf("Created API key %s for %s user %d", key.Prefix, role, userID)
}
