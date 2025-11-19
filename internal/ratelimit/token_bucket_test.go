package ratelimit

import (
	"testing"
	"time"
)

func TestTokenBucket_Allow(t *testing.T) {
	tb := NewTokenBucket(10, 5) // 10 burst, 5/sec sustained

	// Should allow first 10 requests immediately
	for i := 0; i < 10; i++ {
		if !tb.Allow() {
			t.Errorf("request %d should be allowed (burst)", i)
		}
	}

	// 11th request should be denied
	if tb.Allow() {
		t.Error("11th request should be denied (bucket empty)")
	}

	// Wait for 1 second, should refill 5 tokens
	time.Sleep(1 * time.Second)

	// Should allow 5 more requests
	for i := 0; i < 5; i++ {
		if !tb.Allow() {
			t.Errorf("request after refill %d should be allowed", i)
		}
	}

	// 6th request should be denied
	if tb.Allow() {
		t.Error("request after 5 refills should be denied")
	}
}

func TestTokenBucket_AllowN(t *testing.T) {
	tb := NewTokenBucket(100, 10)

	// Consume 50 tokens
	if !tb.AllowN(50) {
		t.Error("should allow 50 tokens")
	}

	// Should have ~50 remaining (allow for float precision)
	remaining := tb.Remaining()
	if remaining < 49 || remaining > 51 {
		t.Errorf("expected ~50 remaining, got %f", remaining)
	}

	// Should deny 60 tokens (only 50 available)
	if tb.AllowN(60) {
		t.Error("should deny 60 tokens when only 50 available")
	}
}

func TestTokenBucket_Remaining(t *testing.T) {
	tb := NewTokenBucket(100, 20)

	// Initial should be full
	if remaining := tb.Remaining(); remaining != 100 {
		t.Errorf("expected 100 remaining, got %f", remaining)
	}

	// After consuming 30
	tb.AllowN(30)
	remaining := tb.Remaining()
	if remaining < 69.9 || remaining > 70.1 {
		t.Errorf("expected ~70 remaining, got %f", remaining)
	}
}

func TestTokenBucket_Reset(t *testing.T) {
	tb := NewTokenBucket(100, 10)

	// Consume all tokens
	tb.AllowN(100)

	// Reset should restore capacity
	tb.Reset()

	if remaining := tb.Remaining(); remaining != 100 {
		t.Errorf("expected 100 after reset, got %f", remaining)
	}
}

func TestTokenBucket_WaitTime(t *testing.T) {
	tb := NewTokenBucket(10, 10) // 10 tokens/sec

	// With tokens available, wait time should be 0
	if wait := tb.WaitTime(); wait != 0 {
		t.Errorf("expected 0 wait time with tokens, got %v", wait)
	}

	// Consume all tokens
	tb.AllowN(10)

	// Wait time should be ~100ms (1 token / 10 tokens/sec)
	wait := tb.WaitTime()
	if wait < 90*time.Millisecond || wait > 110*time.Millisecond {
		t.Errorf("expected ~100ms wait time, got %v", wait)
	}
}

func TestTokenBucket_Refill(t *testing.T) {
	tb := NewTokenBucket(100, 50) // 50 tokens/sec

	// Consume all tokens
	tb.AllowN(100)

	// Wait 500ms, should refill 25 tokens
	time.Sleep(500 * time.Millisecond)

	remaining := tb.Remaining()
	if remaining < 23 || remaining > 27 {
		t.Errorf("expected ~25 tokens after 500ms, got %f", remaining)
	}
}

func TestTokenBucket_Concurrent(t *testing.T) {
	tb := NewTokenBucket(1000, 100)

	// Run 100 goroutines concurrently
	done := make(chan bool)
	for i := 0; i < 100; i++ {
		go func() {
			for j := 0; j < 10; j++ {
				tb.Allow()
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 100; i++ {
		<-done
	}

	// Should have consumed 1000 tokens
	remaining := tb.Remaining()
	if remaining > 1 {
		t.Errorf("expected ~0 remaining after concurrent access, got %f", remaining)
	}
}
