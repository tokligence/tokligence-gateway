package hooks

import (
	"fmt"
	"time"
)

// Config captures hook-related settings exposed via files/env/CLI.
type Config struct {
	Enabled    bool              `mapstructure:"enabled" json:"enabled"`
	ScriptPath string            `mapstructure:"script_path" json:"script_path"`
	ScriptArgs []string          `mapstructure:"script_args" json:"script_args"`
	Env        map[string]string `mapstructure:"env" json:"env"`
	Timeout    time.Duration     `mapstructure:"timeout" json:"timeout"`
}

// Validate ensures the configuration is coherent before we wire handlers.
func (c Config) Validate() error {
	if !c.Enabled {
		return nil
	}
	if c.ScriptPath == "" {
		return fmt.Errorf("hooks: script_path required when enabled")
	}
	return nil
}

// BuildScriptHandler constructs the handler declared in Config.
func (c Config) BuildScriptHandler() Handler {
	if !c.Enabled {
		return nil
	}
	cfg := ScriptConfig{
		Command: c.ScriptPath,
		Args:    c.ScriptArgs,
		Env:     c.Env,
		Timeout: c.Timeout,
	}
	return NewScriptHandler(cfg)
}
