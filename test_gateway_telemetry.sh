#!/bin/bash

set -e

echo "========================================="
echo "Testing Gateway Telemetry Client"
echo "========================================="
echo ""

# Build a small Go test program to use the actual telemetry client
cat > /tmp/test_telemetry.go <<'EOF'
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/tokligence/tokligence-gateway/internal/telemetry"
)

func main() {
	client := telemetry.NewClient("http://localhost:8082", log.Default())

	payload := telemetry.PingPayload{
		InstallID:      "test-gateway-001",
		GatewayVersion: "0.1.0",
		Platform:       "linux/amd64",
		DatabaseType:   "sqlite",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	fmt.Println("Sending ping to marketplace...")
	resp, err := client.SendPing(ctx, payload)
	if err != nil {
		log.Fatalf("Ping failed: %v", err)
	}

	fmt.Printf("\n✓ Ping successful!\n\n")

	if resp.UpdateAvailable {
		if resp.SecurityUpdate {
			fmt.Printf("⚠️  SECURITY UPDATE: %s → %s\n", payload.GatewayVersion, resp.LatestVersion)
		} else {
			fmt.Printf("✨ Update available: %s → %s\n", payload.GatewayVersion, resp.LatestVersion)
		}
		fmt.Printf("   Download: %s\n", resp.UpdateURL)
		fmt.Printf("   Message: %s\n\n", resp.UpdateMessage)
	} else {
		fmt.Println("✓ Gateway is up to date\n")
	}

	if len(resp.Announcements) > 0 {
		fmt.Printf("📢 %d marketplace announcements:\n\n", len(resp.Announcements))
		for _, ann := range resp.Announcements {
			priority := ann.Priority
			if priority == "" {
				priority = "medium"
			}

			emoji := "📢"
			if priority == "critical" || priority == "high" {
				emoji = "⚠️ "
			}

			fmt.Printf("%s [%s] %s\n", emoji, ann.Type, ann.Title)
			fmt.Printf("   %s\n", ann.Message)
			if ann.URL != "" {
				fmt.Printf("   Learn more: %s\n", ann.URL)
			}
			fmt.Println()
		}
	}
}
EOF

echo "Compiling test program..."
cd /tmp && go build -o test_telemetry test_telemetry.go 2>&1 | grep -v "go: finding\|go: downloading" || true

echo "Running telemetry client test..."
echo "--------------------------------------"
/tmp/test_telemetry

echo "========================================="
echo "Gateway telemetry test completed! ✓"
echo "========================================="
