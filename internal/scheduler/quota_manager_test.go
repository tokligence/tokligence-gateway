package scheduler

import (
	"testing"
	"time"
)

func TestQuotaOverridesEnforceLimits(t *testing.T) {
	maxConcurrent := 1
	maxRPS := 2
	maxTokens := int64(100)

	qm := &QuotaManager{
		enabled:       true,
		quotas:        make(map[string]*AccountQuota),
		usageCache:    make(map[string]int64),
		adjustments:   []QuotaAdjustment{{AccountPattern: "acct-*", MaxConcurrent: &maxConcurrent, MaxRPS: &maxRPS, MaxTokensPerSec: &maxTokens}},
		overrideState: make(map[string]*accountOverrideState),
	}

	req := QuotaCheckRequest{AccountID: "acct-123", EstimatedTokens: 50}

	// First request should pass
	res, err := qm.CheckAndReserve(nil, req)
	if err != nil || !res.Allowed {
		t.Fatalf("expected first request to be allowed, got err=%v res=%+v", err, res)
	}

	// Second concurrent request should be rejected due to concurrency limit
	res, err = qm.CheckAndReserve(nil, req)
	if err != nil {
		t.Fatalf("expected graceful rejection, got err=%v", err)
	}
	if res.Allowed {
		t.Fatalf("expected concurrency limit rejection")
	}

	// Release and try again
	qm.ReleaseOverride(req.AccountID)
	res, err = qm.CheckAndReserve(nil, req)
	if err != nil || !res.Allowed {
		t.Fatalf("expected request to be allowed after release, got err=%v res=%+v", err, res)
	}

	// RPS limit (max 2 per window) should reject the next immediate request
	res, err = qm.CheckAndReserve(nil, req)
	if err != nil {
		t.Fatalf("expected graceful rejection for RPS, got err=%v", err)
	}
	if res.Allowed {
		t.Fatalf("expected RPS limit rejection")
	}

	// Advance window to reset per-second counters
	state := qm.overrideState[req.AccountID]
	state.windowStart = time.Now().Add(-2 * time.Second)
	res, err = qm.CheckAndReserve(nil, req)
	if err != nil || !res.Allowed {
		t.Fatalf("expected request to be allowed after window reset, got err=%v res=%+v", err, res)
	}

	// Token estimate exceeding limit should be rejected
	reqLarge := QuotaCheckRequest{AccountID: "acct-123", EstimatedTokens: 200}
	res, err = qm.CheckAndReserve(nil, reqLarge)
	if err != nil {
		t.Fatalf("expected graceful rejection for tokens, got err=%v", err)
	}
	if res.Allowed {
		t.Fatalf("expected token limit rejection")
	}
}
