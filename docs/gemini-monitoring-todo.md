# Gemini Integration - Monitoring TODO

**Created:** 2025-11-19
**Status:** Pending Implementation

---

## Overview

Add basic Prometheus-style monitoring for Gemini endpoints to make the integration production-ready.

## Current State

- ✅ Metrics package exists (`internal/metrics/metrics.go`, `internal/metrics/prometheus.go`)
- ✅ Metrics collector implementation available
- ❌ Metrics collector NOT integrated into Server struct
- ❌ NO metrics recording in any HTTP handlers (including Gemini)
- ❌ NO middleware for automatic metrics collection

## Required Changes

### 1. Add Metrics to Server Struct

**File:** `internal/httpserver/server.go`

```go
type Server struct {
    // ... existing fields ...

    metrics *metrics.Collector  // Add this field
}
```

**Initialization in constructor:**
```go
func NewServer(gateway GatewayFacade, ...) *Server {
    return &Server{
        // ... existing fields ...
        metrics: metrics.NewCollector(),
    }
}
```

---

### 2. Add Basic Metrics to Gemini Handler

**File:** `internal/httpserver/gemini_handler.go`

Add metrics recording at key points:

#### 2.1 Request Start/End Tracking

```go
func (s *Server) HandleGeminiProxy(w http.ResponseWriter, r *http.Request) {
    start := time.Now()
    endpoint := "gemini:" + r.URL.Path

    // Record request start
    if s.metrics != nil {
        s.metrics.RecordRequestStart(endpoint)
        defer s.metrics.RecordRequestEnd(endpoint)
    }

    // ... existing handler logic ...

    // Record request completion
    if s.metrics != nil {
        duration := time.Since(start)
        s.metrics.RecordRequest(endpoint, duration)
    }
}
```

#### 2.2 Error Tracking

```go
// When errors occur:
if err != nil {
    if s.metrics != nil {
        s.metrics.RecordError(endpoint)
    }
    http.Error(w, ...)
    return
}
```

#### 2.3 Adapter Metrics

```go
// Before calling Gemini adapter:
adapterStart := time.Now()

// After adapter call:
if s.metrics != nil {
    s.metrics.RecordAdapterRequest("gemini", time.Since(adapterStart), err)
}
```

---

### 3. Token Usage Tracking (Optional)

If Gemini responses include token usage:

```go
// Parse response and extract token counts
if s.metrics != nil && hasUsageMetadata {
    s.metrics.RecordTokenUsage(
        model,
        userID,
        promptTokens,
        completionTokens,
    )
}
```

---

## Metrics to Track

### Basic Metrics (Priority: High)

1. **Request Count**
   - Metric: `gemini_requests_total{endpoint, method}`
   - Labels: endpoint path, HTTP method
   - Type: Counter

2. **Request Duration**
   - Metric: `gemini_request_duration_ms{endpoint}`
   - Labels: endpoint path
   - Type: Histogram/Summary

3. **Error Rate**
   - Metric: `gemini_errors_total{endpoint}`
   - Labels: endpoint path
   - Type: Counter

4. **In-Flight Requests**
   - Metric: `gemini_requests_in_progress{endpoint}`
   - Labels: endpoint path
   - Type: Gauge

5. **Adapter Performance**
   - Metric: `gemini_adapter_latency_ms`
   - Labels: operation (generateContent, embedContent, etc.)
   - Type: Histogram

### Advanced Metrics (Priority: Medium)

6. **Token Usage**
   - Metric: `gemini_tokens_total{model, type}`
   - Labels: model name, type (prompt/completion)
   - Type: Counter

7. **Model Usage**
   - Metric: `gemini_model_requests_total{model}`
   - Labels: model name
   - Type: Counter

8. **Endpoint Usage**
   - Metric: `gemini_endpoint_requests{operation}`
   - Labels: operation (generate, embed, list, etc.)
   - Type: Counter

---

## Implementation Plan

### Phase 1: Basic Integration (1-2 hours)

1. Add `metrics` field to Server struct
2. Initialize metrics collector in constructor
3. Add request timing in `HandleGeminiProxy`
4. Add error counting
5. Test basic metrics collection

### Phase 2: Detailed Metrics (2-3 hours)

1. Add adapter-level metrics
2. Add per-operation metrics
3. Add model-specific tracking
4. Add token usage tracking (if available)

### Phase 3: Metrics Endpoint (1 hour)

Add Prometheus endpoint to expose metrics:

```go
// In server.go
func (s *Server) HandleMetrics(w http.ResponseWriter, r *http.Request) {
    snapshot := s.metrics.GetSnapshot()

    w.Header().Set("Content-Type", "text/plain; version=0.0.4")

    // Write metrics in Prometheus format
    // Example:
    // tokligence_gemini_requests_total{endpoint="/v1beta/models"} 123
    // tokligence_gemini_request_duration_ms{endpoint="/v1beta/models"} 45.2
}
```

---

## Testing

### Manual Testing

1. Start gateway with Gemini enabled
2. Make several requests to different endpoints
3. Check `/metrics` endpoint
4. Verify counters increment
5. Verify latency is reasonable

### Example Metrics Output

```
# HELP tokligence_gemini_requests_total Total Gemini API requests
# TYPE tokligence_gemini_requests_total counter
tokligence_gemini_requests_total{endpoint="/v1beta/models/gemini-pro:generateContent"} 42
tokligence_gemini_requests_total{endpoint="/v1beta/models"} 5

# HELP tokligence_gemini_request_duration_ms Request duration in milliseconds
# TYPE tokligence_gemini_request_duration_ms histogram
tokligence_gemini_request_duration_ms_sum{endpoint="/v1beta/models/gemini-pro:generateContent"} 12450
tokligence_gemini_request_duration_ms_count{endpoint="/v1beta/models/gemini-pro:generateContent"} 42

# HELP tokligence_gemini_errors_total Total Gemini API errors
# TYPE tokligence_gemini_errors_total counter
tokligence_gemini_errors_total{endpoint="/v1beta/models/gemini-pro:generateContent"} 2

# HELP tokligence_gemini_adapter_requests Total adapter requests
# TYPE tokligence_gemini_adapter_requests counter
tokligence_gemini_adapter_requests{adapter="gemini"} 47
```

---

## Notes

### Keep It Simple

- Don't add too many metrics initially
- Focus on request/error/latency basics
- Add more detailed metrics only if needed

### Consistency

- Use same pattern as other providers (OpenAI, Anthropic) when they add metrics
- Use existing metrics.Collector methods
- Follow Prometheus naming conventions

### Performance

- Metrics collection should add <1ms overhead
- Use atomic operations for counters
- Don't block request handling

---

## Related Files

- `internal/metrics/metrics.go` - Metrics collector implementation
- `internal/metrics/prometheus.go` - Prometheus format export
- `internal/httpserver/server.go` - Server struct definition
- `internal/httpserver/gemini_handler.go` - Gemini request handler

---

## Reference

- [Prometheus Naming Best Practices](https://prometheus.io/docs/practices/naming/)
- [Go Metrics Libraries](https://github.com/prometheus/client_golang)
- Existing metrics collector in `internal/metrics/`

---

**Estimated Effort:** 3-5 hours for full implementation
**Priority:** Medium (makes it production-ready, but not blocking)
**Complexity:** Low (infrastructure exists, just needs integration)
