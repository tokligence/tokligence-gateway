package scheduler

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
)

// ConfigFromGatewayConfig converts gateway config settings to scheduler.Config
func ConfigFromGatewayConfig(
	enabled bool,
	priorityLevels int,
	defaultPriority int,
	maxQueueDepth int,
	queueTimeoutSec int,
	weightsStr string,
) (*Config, error) {
	if !enabled {
		log.Printf("[INFO] Scheduler: Disabled (scheduler_enabled=false)")
		return nil, nil
	}

	log.Printf("[INFO] Scheduler: Building config from gateway settings (priority_levels=%d, default_priority=%d, policy=configurable)",
		priorityLevels, defaultPriority)

	config := &Config{
		NumPriorityLevels: priorityLevels,
		DefaultPriority:   PriorityTier(defaultPriority),
		MaxQueueDepth:     maxQueueDepth,
		QueueTimeout:      time.Duration(queueTimeoutSec) * time.Second,
	}

	// Parse weights if provided
	if weightsStr != "" {
		weights, err := parseWeights(weightsStr, priorityLevels)
		if err != nil {
			log.Printf("[WARN] Scheduler: Failed to parse weights %q: %v, using defaults", weightsStr, err)
			config.Weights = generateDefaultWeights(priorityLevels)
		} else {
			config.Weights = weights
			log.Printf("[INFO] Scheduler: Using custom weights from config: %v", weights)
		}
	} else {
		config.Weights = generateDefaultWeights(priorityLevels)
		log.Printf("[INFO] Scheduler: Using default exponential weights for %d levels", priorityLevels)
	}

	return config, nil
}

// parseWeights parses comma-separated weight string
func parseWeights(weightsStr string, expectedCount int) ([]float64, error) {
	parts := strings.Split(weightsStr, ",")
	if len(parts) != expectedCount {
		return nil, fmt.Errorf("weights count mismatch: got %d, expected %d", len(parts), expectedCount)
	}

	weights := make([]float64, expectedCount)
	for i, part := range parts {
		weight, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
		if err != nil {
			return nil, fmt.Errorf("invalid weight at index %d: %q: %w", i, part, err)
		}
		weights[i] = weight
	}

	return weights, nil
}

// generateDefaultWeights generates exponential weights for the given number of levels
// P0 = 2^(n-1), P1 = 2^(n-2), ..., P(n-1) = 2^0 = 1
func generateDefaultWeights(numLevels int) []float64 {
	weights := make([]float64, numLevels)
	for i := 0; i < numLevels; i++ {
		shift := uint(numLevels - i - 1)
		weights[i] = float64(int(1) << shift)
	}
	return weights
}

// CapacityFromGatewayConfig creates Capacity from gateway config
func CapacityFromGatewayConfig(
	maxTokensPerSec int,
	maxRPS int,
	maxConcurrent int,
	maxContextLength int,
) *Capacity {
	log.Printf("[INFO] Scheduler: Building capacity config (tokens/sec=%d, rps=%d, concurrent=%d, context=%d)",
		maxTokensPerSec, maxRPS, maxConcurrent, maxContextLength)

	return NewCapacity(maxTokensPerSec, maxRPS, maxConcurrent, maxContextLength)
}

// PolicyFromString converts policy string to SchedulingPolicy
func PolicyFromString(policyStr string) SchedulingPolicy {
	switch strings.ToLower(strings.TrimSpace(policyStr)) {
	case "strict":
		return PolicyStrictPriority
	case "wfq":
		return PolicyWFQ
	case "hybrid":
		return PolicyHybrid
	default:
		log.Printf("[WARN] Scheduler: Unknown policy %q, defaulting to hybrid", policyStr)
		return PolicyHybrid
	}
}
