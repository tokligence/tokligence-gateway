package metrics

import (
	"fmt"
	"sort"
	"strings"
)

// FormatPrometheus formats metrics in Prometheus text format.
// See: https://prometheus.io/docs/instrumenting/exposition_formats/
func FormatPrometheus(snap Snapshot) string {
	var sb strings.Builder

	// Process uptime
	sb.WriteString("# HELP gateway_uptime_seconds Time since gateway started\n")
	sb.WriteString("# TYPE gateway_uptime_seconds gauge\n")
	sb.WriteString(fmt.Sprintf("gateway_uptime_seconds %d\n", snap.Uptime))
	sb.WriteString("\n")

	// Total requests by endpoint
	sb.WriteString("# HELP gateway_requests_total Total number of requests by endpoint\n")
	sb.WriteString("# TYPE gateway_requests_total counter\n")
	for _, endpoint := range sortedKeys(snap.TotalRequests) {
		count := snap.TotalRequests[endpoint]
		sb.WriteString(fmt.Sprintf("gateway_requests_total{endpoint=\"%s\"} %d\n", endpoint, count))
	}
	sb.WriteString("\n")

	// Request errors by endpoint
	sb.WriteString("# HELP gateway_request_errors_total Total number of request errors by endpoint\n")
	sb.WriteString("# TYPE gateway_request_errors_total counter\n")
	for _, endpoint := range sortedKeys(snap.RequestErrors) {
		count := snap.RequestErrors[endpoint]
		sb.WriteString(fmt.Sprintf("gateway_request_errors_total{endpoint=\"%s\"} %d\n", endpoint, count))
	}
	sb.WriteString("\n")

	// Requests in progress
	sb.WriteString("# HELP gateway_requests_in_progress Current number of requests being processed\n")
	sb.WriteString("# TYPE gateway_requests_in_progress gauge\n")
	for _, endpoint := range sortedKeys(snap.RequestsInProgress) {
		count := snap.RequestsInProgress[endpoint]
		if count > 0 { // Only show active endpoints
			sb.WriteString(fmt.Sprintf("gateway_requests_in_progress{endpoint=\"%s\"} %d\n", endpoint, count))
		}
	}
	sb.WriteString("\n")

	// Request duration (average)
	sb.WriteString("# HELP gateway_request_duration_ms_total Total request duration in milliseconds\n")
	sb.WriteString("# TYPE gateway_request_duration_ms_total counter\n")
	for _, endpoint := range sortedKeys(snap.TotalRequestsDur) {
		duration := snap.TotalRequestsDur[endpoint]
		sb.WriteString(fmt.Sprintf("gateway_request_duration_ms_total{endpoint=\"%s\"} %d\n", endpoint, duration))
	}
	sb.WriteString("\n")

	// Rate limit hits
	sb.WriteString("# HELP gateway_rate_limit_hits_total Total number of rate limit rejections\n")
	sb.WriteString("# TYPE gateway_rate_limit_hits_total counter\n")
	sb.WriteString(fmt.Sprintf("gateway_rate_limit_hits_total %d\n", snap.RateLimitHits))
	sb.WriteString("\n")

	// Rate limits by key
	sb.WriteString("# HELP gateway_rate_limit_by_key_total Rate limit hits by user/apikey\n")
	sb.WriteString("# TYPE gateway_rate_limit_by_key_total counter\n")
	for _, key := range sortedKeys(snap.RateLimitByKey) {
		count := snap.RateLimitByKey[key]
		sb.WriteString(fmt.Sprintf("gateway_rate_limit_by_key_total{key=\"%s\"} %d\n", key, count))
	}
	sb.WriteString("\n")

	// Token usage
	sb.WriteString("# HELP gateway_prompt_tokens_total Total prompt tokens processed\n")
	sb.WriteString("# TYPE gateway_prompt_tokens_total counter\n")
	sb.WriteString(fmt.Sprintf("gateway_prompt_tokens_total %d\n", snap.TotalPromptTokens))
	sb.WriteString("\n")

	sb.WriteString("# HELP gateway_completion_tokens_total Total completion tokens generated\n")
	sb.WriteString("# TYPE gateway_completion_tokens_total counter\n")
	sb.WriteString(fmt.Sprintf("gateway_completion_tokens_total %d\n", snap.TotalCompletionTokens))
	sb.WriteString("\n")

	// Tokens by model
	sb.WriteString("# HELP gateway_tokens_by_model_total Total tokens by model\n")
	sb.WriteString("# TYPE gateway_tokens_by_model_total counter\n")
	for _, model := range sortedKeys(snap.TokensByModel) {
		count := snap.TokensByModel[model]
		sb.WriteString(fmt.Sprintf("gateway_tokens_by_model_total{model=\"%s\"} %d\n", model, count))
	}
	sb.WriteString("\n")

	// Tokens by user
	sb.WriteString("# HELP gateway_tokens_by_user_total Total tokens by user\n")
	sb.WriteString("# TYPE gateway_tokens_by_user_total counter\n")
	for _, user := range sortedKeys(snap.TokensByUser) {
		count := snap.TokensByUser[user]
		// Mask user IDs for privacy
		maskedUser := maskUserID(user)
		sb.WriteString(fmt.Sprintf("gateway_tokens_by_user_total{user=\"%s\"} %d\n", maskedUser, count))
	}
	sb.WriteString("\n")

	// Adapter requests
	sb.WriteString("# HELP gateway_adapter_requests_total Total requests to adapters\n")
	sb.WriteString("# TYPE gateway_adapter_requests_total counter\n")
	for _, adapter := range sortedKeys(snap.AdapterRequests) {
		count := snap.AdapterRequests[adapter]
		sb.WriteString(fmt.Sprintf("gateway_adapter_requests_total{adapter=\"%s\"} %d\n", adapter, count))
	}
	sb.WriteString("\n")

	// Adapter errors
	sb.WriteString("# HELP gateway_adapter_errors_total Total adapter errors\n")
	sb.WriteString("# TYPE gateway_adapter_errors_total counter\n")
	for _, adapter := range sortedKeys(snap.AdapterErrors) {
		count := snap.AdapterErrors[adapter]
		sb.WriteString(fmt.Sprintf("gateway_adapter_errors_total{adapter=\"%s\"} %d\n", adapter, count))
	}
	sb.WriteString("\n")

	// Adapter latency
	sb.WriteString("# HELP gateway_adapter_latency_ms_total Total adapter latency in milliseconds\n")
	sb.WriteString("# TYPE gateway_adapter_latency_ms_total counter\n")
	for _, adapter := range sortedKeys(snap.AdapterLatency) {
		latency := snap.AdapterLatency[adapter]
		sb.WriteString(fmt.Sprintf("gateway_adapter_latency_ms_total{adapter=\"%s\"} %d\n", adapter, latency))
	}
	sb.WriteString("\n")

	return sb.String()
}

func sortedKeys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func maskUserID(userID string) string {
	if len(userID) <= 4 {
		return "user_***"
	}
	// Show last 4 characters only
	return "user_***" + userID[len(userID)-4:]
}
