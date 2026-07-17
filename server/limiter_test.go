package main

import (
	"testing"
	"time"
)

func TestRateLimiterResetAllowsLegitimateJoin(t *testing.T) {
	limiter := newRateLimiter(time.Minute, 2)
	if !limiter.Allow("client") || !limiter.Allow("client") {
		t.Fatal("initial attempts should be allowed")
	}
	if limiter.Allow("client") {
		t.Fatal("attempt above the limit should be rejected")
	}

	limiter.Reset("client")
	if !limiter.Allow("client") {
		t.Fatal("reset should allow a legitimate join immediately")
	}
}

func TestRateLimiterReportsBlockedWithoutConsumingValidAttempts(t *testing.T) {
	limiter := newRateLimiter(time.Minute, 2)
	if limiter.Blocked("client") {
		t.Fatal("new client should not be blocked")
	}
	if !limiter.Allow("client") || limiter.Blocked("client") {
		t.Fatal("client should remain below the limit after one failure")
	}
	if !limiter.Allow("client") || !limiter.Blocked("client") {
		t.Fatal("client should be blocked after two failures")
	}
}

func TestRateLimiterExpiresEntries(t *testing.T) {
	limiter := newRateLimiter(time.Millisecond, 1)
	if !limiter.Allow("client") || limiter.Allow("client") {
		t.Fatal("limit should apply inside the active window")
	}
	time.Sleep(2 * time.Millisecond)
	if !limiter.Allow("client") {
		t.Fatal("expired limit should allow a new attempt")
	}
}

func TestRateLimiterBackgroundPruneRemovesIdleKeys(t *testing.T) {
	limiter := newRateLimiter(time.Minute, 1)
	limiter.Allow("old-client")
	limiter.PruneExpired(time.Now().Add(2 * time.Minute))
	if len(limiter.hits) != 0 {
		t.Fatalf("expired entries retained after background prune: %d", len(limiter.hits))
	}
}
