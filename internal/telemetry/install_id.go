package telemetry

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// GetOrCreateInstallID returns a persistent installation UUID.
// It is stored in ~/.tokligence/install_id and persists across restarts.
func GetOrCreateInstallID(basePath string) (string, error) {
	if basePath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		basePath = filepath.Join(home, ".tokligence")
	}

	// Ensure directory exists
	if err := os.MkdirAll(basePath, 0700); err != nil {
		return "", fmt.Errorf("failed to create .tokligence directory: %w", err)
	}

	installIDPath := filepath.Join(basePath, "install_id")

	// Try to read existing install_id
	data, err := os.ReadFile(installIDPath)
	if err == nil {
		installID := strings.TrimSpace(string(data))
		// Validate it's a valid UUID
		if _, err := uuid.Parse(installID); err == nil {
			return installID, nil
		}
		// If invalid, generate a new one
	}

	// Generate new UUID v4
	installID := uuid.New().String()

	// Write to file
	if err := os.WriteFile(installIDPath, []byte(installID+"\n"), 0600); err != nil {
		return "", fmt.Errorf("failed to write install_id: %w", err)
	}

	return installID, nil
}
