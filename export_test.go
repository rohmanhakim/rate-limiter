package ratelimiter

import "time"

// Export internal functions for testing purposes only.
// This file is only compiled during "go test" and does not affect the public API.

var ComputeJitter = computeJitter
var ExponentialBackoffDelay = exponentialBackoffDelay
var DetermineRateLimitReason = determineRateLimitReason

// MarkLastConsumedAsNow exports the private markLastConsumedAsNow method for testing.
func (r *ConcurrentRateLimiter) MarkLastConsumedAsNow(host string) {
	r.markLastConsumedAsNow(host)
}

// Export internal types for testing via type aliases.
type ResourceTiming = resourceTiming
type BackoffConfig = backoffConfig
type RateLimiterConfig = rateLimiterConfig

// NewResourceTiming creates a resourceTiming instance for testing.
func NewResourceTiming(lastConsumedAt time.Time, backoffDelay, delay time.Duration, backoffCount int) resourceTiming {
	return resourceTiming{
		lastConsumedAt: lastConsumedAt,
		backoffDelay:   backoffDelay,
		delay:          delay,
		backoffCount:   backoffCount,
	}
}

// NewBackoffConfig creates a backoffConfig instance for testing.
func NewBackoffConfig(initialDuration time.Duration, multiplier float64, maxDuration time.Duration) backoffConfig {
	return backoffConfig{
		initialDuration: initialDuration,
		multiplier:      multiplier,
		maxDuration:     maxDuration,
	}
}
