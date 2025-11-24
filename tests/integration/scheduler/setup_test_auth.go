package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/tokligence/tokligence-gateway/internal/userstore/sqlite"
)

func main() {
	dbPath := os.Getenv("TOKLIGENCE_IDENTITY_PATH")
	if dbPath == "" {
		dbPath = "/tmp/tokligence_identity.db"
	}

	// Remove existing DB to ensure a clean state
	_ = os.Remove(dbPath)

	store, err := sqlite.New(dbPath, 1, 1, 0, 0)
	if err != nil {
		log.Fatalf("Failed to create sqlite store: %v", err)
	}
	defer store.Close()

	admin, err := store.EnsureRootAdmin(context.Background(), "admin@test.dev")
	if err != nil {
		log.Fatalf("Failed to create root admin: %v", err)
	}

	key, token, err := store.CreateAPIKey(context.Background(), admin.ID, []string{"admin"}, nil)
	if err != nil {
		log.Fatalf("Failed to create API key: %v", err)
	}

	// Print the token to stdout so the shell script can capture it
	fmt.Print(token)
	log.Printf("Created API key %s for admin user %d", key.Prefix, admin.ID)
}
