package ratelimiter

import (
	"testing"
	"time"
)

func TestComputeJitter(t *testing.T) {
	t.Run("returns zero when max is zero", func(t *testing.T) {
		result := ComputeJitter(0)
		if result != 0 {
			t.Errorf("expected 0, got %v", result)
		}
	})

	t.Run("returns zero when max is negative", func(t *testing.T) {
		result := ComputeJitter(-1 * time.Second)
		if result != 0 {
			t.Errorf("expected 0 for negative input, got %v", result)
		}
	})

	t.Run("returns value within bounds", func(t *testing.T) {
		max := 100 * time.Millisecond
		for i := 0; i < 100; i++ {
			result := ComputeJitter(max)
			if result < 0 {
				t.Errorf("jitter should never be negative, got %v", result)
			}
			if result > max {
				t.Errorf("jitter should not exceed max, got %v (max: %v)", result, max)
			}
		}
	})

	t.Run("produces varied results", func(t *testing.T) {
		max := 1 * time.Second
		results := make(map[time.Duration]bool)
		for i := 0; i < 100; i++ {
			results[ComputeJitter(max)] = true
		}
		// With randomness, we should get at least 10 different values in 100 tries
		if len(results) < 10 {
			t.Errorf("jitter should produce varied results, only got %d unique values", len(results))
		}
	})
}

func TestExponentialBackoffDelay(t *testing.T) {
	config := NewBackoffConfig(1*time.Second, 2.0, 1*time.Minute)

	t.Run("first backoff uses initial duration", func(t *testing.T) {
		delay := ExponentialBackoffDelay(1, 0, config, 0)
		if delay != 1*time.Second {
			t.Errorf("expected 1s for first backoff, got %v", delay)
		}
	})

	t.Run("second backoff doubles the delay", func(t *testing.T) {
		delay := ExponentialBackoffDelay(2, 0, config, 0)
		if delay != 2*time.Second {
			t.Errorf("expected 2s for second backoff, got %v", delay)
		}
	})

	t.Run("third backoff continues exponential growth", func(t *testing.T) {
		delay := ExponentialBackoffDelay(3, 0, config, 0)
		if delay != 4*time.Second {
			t.Errorf("expected 4s for third backoff, got %v", delay)
		}
	})

	t.Run("caps at max duration", func(t *testing.T) {
		// With count=10, exponential would be 512s, but max is 60s
		delay := ExponentialBackoffDelay(10, 0, config, 0)
		if delay > 1*time.Minute {
			t.Errorf("expected delay to be capped at 1 minute, got %v", delay)
		}
	})

	t.Run("uses server delay when greater than initial", func(t *testing.T) {
		serverDelay := 5 * time.Second
		delay := ExponentialBackoffDelay(1, 0, config, serverDelay)
		if delay < serverDelay {
			t.Errorf("expected delay >= server delay (%v), got %v", serverDelay, delay)
		}
	})

	t.Run("ignores server delay when less than initial", func(t *testing.T) {
		serverDelay := 500 * time.Millisecond // less than 1s initial
		delay := ExponentialBackoffDelay(1, 0, config, serverDelay)
		if delay != 1*time.Second {
			t.Errorf("expected initial duration (1s), got %v", delay)
		}
	})

	t.Run("adds jitter when provided", func(t *testing.T) {
		jitter := 100 * time.Millisecond
		// Run multiple times to ensure jitter is being applied
		results := make(map[time.Duration]bool)
		for i := 0; i < 50; i++ {
			delay := ExponentialBackoffDelay(1, jitter, config, 0)
			results[delay] = true
		}
		// With jitter, we should see varied results
		if len(results) < 5 {
			t.Errorf("expected varied results with jitter, got %d unique values", len(results))
		}
	})

	t.Run("respects max duration even with large multiplier", func(t *testing.T) {
		largeMultiplierConfig := NewBackoffConfig(1*time.Second, 10.0, 30*time.Second)
		delay := ExponentialBackoffDelay(10, 0, largeMultiplierConfig, 0)
		if delay > 30*time.Second {
			t.Errorf("expected delay capped at 30s, got %v", delay)
		}
	})
}
