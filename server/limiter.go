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
	for storedKey, value := range r.hits {
		if value.ResetAt.Before(now) {
			delete(r.hits, storedKey)
		}
	}
	current := r.hits[key]
	if current.ResetAt.Before(now) || current.Count == 0 {
		r.hits[key] = rateLimitHit{Count: 1, ResetAt: now.Add(r.window)}
		return true
	}
	current.Count++
	r.hits[key] = current
	return current.Count <= r.maxRequests
}
