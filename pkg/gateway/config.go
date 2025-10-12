package gateway

import (
	internalcfg "github.com/tokligence/tokligence-gateway/internal/config"
)

// Config re-exports the open gateway's configuration structure so downstream
// integrations can reuse the same parsed values without importing internal
// packages.
type Config = internalcfg.GatewayConfig

// LoadConfig delegates to the internal loader while keeping the consumer API
// inside the public pkg/gateway namespace.
func LoadConfig(root string) (Config, error) {
	return internalcfg.LoadGatewayConfig(root)
}
