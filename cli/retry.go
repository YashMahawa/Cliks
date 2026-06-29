package main

import (
	"math/rand/v2"
	"time"
)

const maxRetryDelay = 30 * time.Second

func randomizedRetryDelay(attempt int) time.Duration {
	return retryDelayWithSample(attempt, rand.Float64())
}

func retryDelayWithSample(attempt int, sample float64) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if sample < 0 {
		sample = 0
	}
	if sample > 1 {
		sample = 1
	}
	base := time.Second << minInt(attempt-1, 5)
	if base > maxRetryDelay {
		base = maxRetryDelay
	}
	delay := time.Duration(float64(base) * (0.9 + sample*0.2))
	if delay > maxRetryDelay {
		return maxRetryDelay
	}
	return delay
}
