package ratelimiter_test

import (
	"context"
	"testing"
	"time"

	ratelimiter "github.com/rohmanhakim/rate-limiter"
)

// Black-box tests for backoff behavior through the public API
func TestBackoffExponentialGrowth(t *testing.T) {
	limiter := ratelimiter.NewConcurrentRateLimiter(
		ratelimiter.WithInitialDuration(1*time.Second),
		ratelimiter.WithMultiplier(2.0),
		ratelimiter.WithMaxDuration(1*time.Minute),
	)

	resource := "api.example.com"

	// First backoff
	limiter.Backoff(context.Background(), resource)
	timings := limiter.ResourceTimings()
	firstDelay := timings[resource].BackoffDelay()

	// Second backoff
	limiter.Backoff(context.Background(), resource)
	timings = limiter.ResourceTimings()
	secondDelay := timings[resource].BackoffDelay()

	// Second delay should be roughly double the first (allowing for some variance)
	if secondDelay < firstDelay*15/10 { // at least 1.5x growth
		t.Errorf("expected exponential growth: first=%v, second=%v", firstDelay, secondDelay)
	}
}

func TestBackoffMaxCap(t *testing.T) {
	maxDuration := 10 * time.Second
	limiter := ratelimiter.NewConcurrentRateLimiter(
		ratelimiter.WithInitialDuration(1*time.Second),
		ratelimiter.WithMultiplier(10.0), // Large multiplier
		ratelimiter.WithMaxDuration(maxDuration),
	)

	resource := "api.example.com"

	// Trigger multiple backoffs to exceed max
	for i := 0; i < 10; i++ {
		limiter.Backoff(context.Background(), resource)
	}

	timings := limiter.ResourceTimings()
	delay := timings[resource].BackoffDelay()

	if delay > maxDuration {
		t.Errorf("expected delay to be capped at %v, got %v", maxDuration, delay)
	}
}

func TestBackoffReset(t *testing.T) {
	limiter := ratelimiter.NewConcurrentRateLimiter(
		ratelimiter.WithInitialDuration(1 * time.Second),
	)

	resource := "api.example.com"

	// Trigger backoff
	limiter.Backoff(context.Background(), resource)
	timings := limiter.ResourceTimings()
	if timings[resource].BackoffCount() != 1 {
		t.Error("expected backoff count 1")
	}

	// Reset
	limiter.ResetBackoff(resource)
	timings = limiter.ResourceTimings()
	if timings[resource].BackoffCount() != 0 {
		t.Error("expected backoff count 0 after reset")
	}

	// Next backoff should start fresh
	limiter.Backoff(context.Background(), resource)
	timings = limiter.ResourceTimings()
	if timings[resource].BackoffCount() != 1 {
		t.Error("expected backoff count 1 after fresh start")
	}
}
