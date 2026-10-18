package scheduler

import (
	"testing"
	"time"
)

func TestQuotaOverridesEnforceLimits(t *testing.T) {
	maxConcurrent := 2      // Allow 2 concurrent requests
	maxRPS := 3             // Allow 3 requests per second
	maxTokens := int64(300) // Allow 300 tokens per second (enough for 6 requests @ 50 each)

	qm := &QuotaManager{
		enabled:       true,
		quotas:        make(map[string]*AccountQuota),
		usageCache:    make(map[string]int64),
		adjustments:   []QuotaAdjustment{{AccountPattern: "acct-*", MaxConcurrent: &maxConcurrent, MaxRPS: &maxRPS, MaxTokensPerSec: &maxTokens}},
		overrideState: make(map[string]*accountOverrideState),
	}

	req := QuotaCheckRequest{AccountID: "acct-123", EstimatedTokens: 50}

	// First request should pass (concurrent=1, rps=1)
	res, err := qm.CheckAndReserve(nil, req)
	if err != nil || !res.Allowed {
		t.Fatalf("expected first request to be allowed, got err=%v res=%+v", err, res)
	}

	// Second request should pass (concurrent=2, rps=2)
	res, err = qm.CheckAndReserve(nil, req)
	if err != nil || !res.Allowed {
		t.Fatalf("expected second request to be allowed, got err=%v res=%+v", err, res)
	}

	// Third request should be rejected due to concurrency limit (maxConcurrent=2)
	res, err = qm.CheckAndReserve(nil, req)
	if err != nil {
		t.Fatalf("expected graceful rejection, got err=%v", err)
	}
	if res.Allowed {
		t.Fatalf("expected concurrency limit rejection")
	}

	// Release one and try again - should pass (concurrent=2, rps=3)
	qm.ReleaseOverride(req.AccountID)
	res, err = qm.CheckAndReserve(nil, req)
	if err != nil || !res.Allowed {
		t.Fatalf("expected request to be allowed after release, got err=%v res=%+v", err, res)
	}

	// RPS limit (max 3) should reject the next immediate request
	res, err = qm.CheckAndReserve(nil, req)
	if err != nil {
		t.Fatalf("expected graceful rejection for RPS, got err=%v", err)
	}
	if res.Allowed {
		t.Fatalf("expected RPS limit rejection")
	}

	// Release all concurrent requests before testing window reset
	qm.ReleaseOverride(req.AccountID)
	qm.ReleaseOverride(req.AccountID)

	// Advance window to reset per-second counters (RPS)
	qm.overrideMu.Lock()
	state := qm.overrideState[req.AccountID]
	state.windowStart = time.Now().Add(-2 * time.Second)
	qm.overrideMu.Unlock()

	// After window reset, RPS is cleared but concurrent should be 0 (we released all)
	res, err = qm.CheckAndReserve(nil, req)
	if err != nil || !res.Allowed {
		t.Fatalf("expected request to be allowed after window reset, got err=%v res=%+v", err, res)
	}

	// Token estimate exceeding limit should be rejected (300 tokens limit, request 400)
	reqLarge := QuotaCheckRequest{AccountID: "acct-123", EstimatedTokens: 400}
	res, err = qm.CheckAndReserve(nil, reqLarge)
	if err != nil {
		t.Fatalf("expected graceful rejection for tokens, got err=%v", err)
	}
	if res.Allowed {
		t.Fatalf("expected token limit rejection")
	}
}

// TestConcurrentCountNotResetOnWindowExpiry verifies that currentConcurrent is NOT
// reset when the rate-limit window expires. In-flight requests should still be tracked.
func TestConcurrentCountNotResetOnWindowExpiry(t *testing.T) {
	maxConcurrent := 2
	maxRPS := 100 // high enough to not interfere
	maxTokens := int64(10000)

	qm := &QuotaManager{
		enabled:       true,
		quotas:        make(map[string]*AccountQuota),
		usageCache:    make(map[string]int64),
		adjustments:   []QuotaAdjustment{{AccountPattern: "acct-*", MaxConcurrent: &maxConcurrent, MaxRPS: &maxRPS, MaxTokensPerSec: &maxTokens}},
		overrideState: make(map[string]*accountOverrideState),
	}

	req := QuotaCheckRequest{AccountID: "acct-test", EstimatedTokens: 10}

	// Reserve two concurrent slots
	res1, err := qm.CheckAndReserve(nil, req)
	if err != nil || !res1.Allowed {
		t.Fatalf("first request should be allowed: err=%v res=%+v", err, res1)
	}

	res2, err := qm.CheckAndReserve(nil, req)
	if err != nil || !res2.Allowed {
		t.Fatalf("second request should be allowed: err=%v res=%+v", err, res2)
	}

	// Third request should be rejected (maxConcurrent=2)
	res3, err := qm.CheckAndReserve(nil, req)
	if err != nil {
		t.Fatalf("expected graceful rejection: err=%v", err)
	}
	if res3.Allowed {
		t.Fatalf("third request should be rejected due to concurrent limit")
	}

	// Simulate window expiry by backdating windowStart
	qm.overrideMu.Lock()
	state := qm.overrideState[req.AccountID]
	state.windowStart = time.Now().Add(-2 * time.Second) // force window reset on next call
	qm.overrideMu.Unlock()

	// After window expires, concurrent count should NOT be reset
	// The 2 in-flight requests are still running, so this should still be rejected
	res4, err := qm.CheckAndReserve(nil, req)
	if err != nil {
		t.Fatalf("expected graceful rejection after window: err=%v", err)
	}
	if res4.Allowed {
		t.Fatalf("BUG: request allowed after window expiry but 2 requests still in-flight! concurrent limit bypassed")
	}

	// Release one, now we should be able to submit
	qm.ReleaseOverride(req.AccountID)
	res5, err := qm.CheckAndReserve(nil, req)
	if err != nil || !res5.Allowed {
		t.Fatalf("request should be allowed after release: err=%v res=%+v", err, res5)
	}
}

// TestConcurrentReleaseDoesNotGoNegative ensures releasing more than reserved doesn't cause issues
func TestConcurrentReleaseDoesNotGoNegative(t *testing.T) {
	maxConcurrent := 5

	qm := &QuotaManager{
		enabled:       true,
		quotas:        make(map[string]*AccountQuota),
		usageCache:    make(map[string]int64),
		adjustments:   []QuotaAdjustment{{AccountPattern: "*", MaxConcurrent: &maxConcurrent}},
		overrideState: make(map[string]*accountOverrideState),
	}

	req := QuotaCheckRequest{AccountID: "acct-x", EstimatedTokens: 1}

	// Reserve one
	res, _ := qm.CheckAndReserve(nil, req)
	if !res.Allowed {
		t.Fatal("should be allowed")
	}

	// Release multiple times (should not panic or go negative)
	qm.ReleaseOverride(req.AccountID)
	qm.ReleaseOverride(req.AccountID)
	qm.ReleaseOverride(req.AccountID)

	qm.overrideMu.Lock()
	state := qm.overrideState[req.AccountID]
	if state.currentConcurrent < 0 {
		t.Fatalf("currentConcurrent went negative: %d", state.currentConcurrent)
	}
	qm.overrideMu.Unlock()
}
