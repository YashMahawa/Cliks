package main

import (
	"sync"
	"time"
)

type rateLimiter struct {
	mu          sync.Mutex
	window      time.Duration
	maxRequests int
	hits        map[string]rateLimitHit
	nextCleanup time.Time
}

type rateLimitHit struct {
	Count   int
	ResetAt time.Time
}

func newRateLimiter(window time.Duration, maxRequests int) *rateLimiter {
	return &rateLimiter{
		window:      window,
		maxRequests: maxRequests,
		hits:        map[string]rateLimitHit{},
	}
}

func (r *rateLimiter) Allow(key string) bool {
	now := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.prepareLocked(now)
	current := r.hits[key]
	if current.ResetAt.Before(now) || current.Count == 0 {
		r.hits[key] = rateLimitHit{Count: 1, ResetAt: now.Add(r.window)}
		return true
	}
	current.Count++
	r.hits[key] = current
	return current.Count <= r.maxRequests
}

func (r *rateLimiter) Blocked(key string) bool {
	now := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.prepareLocked(now)
	current := r.hits[key]
	return current.ResetAt.After(now) && current.Count >= r.maxRequests
}

func (r *rateLimiter) prepareLocked(now time.Time) {
	if r.nextCleanup.IsZero() || !now.Before(r.nextCleanup) {
		r.pruneExpiredLocked(now)
		cleanupEvery := r.window / 4
		if cleanupEvery < 30*time.Second {
			cleanupEvery = 30 * time.Second
		}
		r.nextCleanup = now.Add(cleanupEvery)
	}
}

func (r *rateLimiter) Reset(key string) {
	r.mu.Lock()
	delete(r.hits, key)
	r.mu.Unlock()
}

func (r *rateLimiter) pruneExpiredLocked(now time.Time) {
	for key, value := range r.hits {
		if !value.ResetAt.After(now) {
			delete(r.hits, key)
		}
	}
}
