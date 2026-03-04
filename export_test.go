package ratelimiter

import (
	"time"

	exponentialbackoff "github.com/rohmanhakim/exponential-backoff"
)

// Export internal functions for testing purposes only.
// This file is only compiled during "go test" and does not affect the public API.

var ComputeJitter = exponentialbackoff.ComputeJitter
var DetermineRateLimitReason = determineRateLimitReason

// ExponentialBackoffDelay is a test helper that wraps the external library's CalculateDelay.
func ExponentialBackoffDelay(backoffCount int, jitter time.Duration, config exponentialbackoff.Config, serverDelay time.Duration) time.Duration {
	opts := []exponentialbackoff.DelayOption{}
	if serverDelay > 0 {
		opts = append(opts, exponentialbackoff.WithServerDelay(serverDelay))
	}
	return exponentialbackoff.CalculateDelay(backoffCount, jitter, config, opts...)
}

// MarkLastConsumedAsNow exports the private markLastConsumedAsNow method for testing.
func (r *ConcurrentRateLimiter) MarkLastConsumedAsNow(host string) {
	r.markLastConsumedAsNow(host)
}

// Export internal types for testing via type aliases.
type ResourceTiming = resourceTiming
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

// NewBackoffConfig creates an exponentialbackoff.Config instance for testing.
func NewBackoffConfig(initialDuration time.Duration, multiplier float64, maxDuration time.Duration) exponentialbackoff.Config {
	return exponentialbackoff.MustConfig(initialDuration, maxDuration, multiplier)
}
