package ratelimiter

import (
	"context"
	"testing"
	"time"
)

func TestDetermineRateLimitReason(t *testing.T) {
	tests := []struct {
		name           string
		baseDelay      time.Duration
		consumeDelay   time.Duration
		backoffDelay   time.Duration
		expectedReason RateLimitReason
	}{
		{
			name:           "backoff is dominant",
			baseDelay:      1 * time.Second,
			consumeDelay:   2 * time.Second,
			backoffDelay:   5 * time.Second,
			expectedReason: RateLimitReasonBackoff,
		},
		{
			name:           "consume delay is dominant",
			baseDelay:      1 * time.Second,
			consumeDelay:   5 * time.Second,
			backoffDelay:   2 * time.Second,
			expectedReason: RateLimitReasonConsumeDelay,
		},
		{
			name:           "base delay is dominant",
			baseDelay:      5 * time.Second,
			consumeDelay:   2 * time.Second,
			backoffDelay:   3 * time.Second,
			expectedReason: RateLimitReasonBaseDelay,
		},
		{
			name:           "all zeros returns backoff (first condition wins)",
			baseDelay:      0,
			consumeDelay:   0,
			backoffDelay:   0,
			expectedReason: RateLimitReasonBackoff,
		},
		{
			name:           "backoff ties with consume delay, backoff wins",
			baseDelay:      1 * time.Second,
			consumeDelay:   5 * time.Second,
			backoffDelay:   5 * time.Second,
			expectedReason: RateLimitReasonBackoff,
		},
		{
			name:           "consume delay ties with base, consume wins",
			baseDelay:      5 * time.Second,
			consumeDelay:   5 * time.Second,
			backoffDelay:   2 * time.Second,
			expectedReason: RateLimitReasonConsumeDelay,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reason := DetermineRateLimitReason(tt.baseDelay, tt.consumeDelay, tt.backoffDelay)
			if reason != tt.expectedReason {
				t.Errorf("expected %v, got %v", tt.expectedReason, reason)
			}
		})
	}
}

func TestResourceTiming(t *testing.T) {
	t.Run("creates resource timing with all fields", func(t *testing.T) {
		now := time.Now()
		rt := NewResourceTiming(now, 5*time.Second, 2*time.Second, 3)

		if rt.LastConsumedAt() != now {
			t.Errorf("expected LastConsumedAt %v, got %v", now, rt.LastConsumedAt())
		}
		if rt.BackoffDelay() != 5*time.Second {
			t.Errorf("expected BackoffDelay 5s, got %v", rt.BackoffDelay())
		}
		if rt.Delay() != 2*time.Second {
			t.Errorf("expected Delay 2s, got %v", rt.Delay())
		}
		if rt.BackoffCount() != 3 {
			t.Errorf("expected BackoffCount 3, got %d", rt.BackoffCount())
		}
	})
}

func TestNewBackoffConfig(t *testing.T) {
	config := NewBackoffConfig(2*time.Second, 3.0, 30*time.Second)

	if config.InitialDuration() != 2*time.Second {
		t.Errorf("expected InitialDuration 2s, got %v", config.InitialDuration())
	}
	if config.Multiplier() != 3.0 {
		t.Errorf("expected Multiplier 3.0, got %v", config.Multiplier())
	}
	if config.MaxDuration() != 30*time.Second {
		t.Errorf("expected MaxDuration 30s, got %v", config.MaxDuration())
	}
}

func TestWithLogAttrs(t *testing.T) {
	t.Run("sets attrs in config", func(t *testing.T) {
		attrs := []any{"key1", "value1", "key2", "value2"}
		limiter := NewConcurrentRateLimiter(WithLogAttrs(attrs...))

		// Verify attrs are set in the config
		config := limiter.config
		if len(config.attrs) != 4 {
			t.Errorf("expected 4 attrs, got %d", len(config.attrs))
		}
		if config.attrs[0] != "key1" || config.attrs[1] != "value1" {
			t.Errorf("expected attrs[0:2] to be key1/value1, got %v/%v", config.attrs[0], config.attrs[1])
		}
		if config.attrs[2] != "key2" || config.attrs[3] != "value2" {
			t.Errorf("expected attrs[2:4] to be key2/value2, got %v/%v", config.attrs[2], config.attrs[3])
		}
	})

	t.Run("works with empty attrs", func(t *testing.T) {
		limiter := NewConcurrentRateLimiter(WithLogAttrs())

		config := limiter.config
		if len(config.attrs) != 0 {
			t.Errorf("expected 0 attrs, got %d", len(config.attrs))
		}
	})
}

func TestSetDebugLogger(t *testing.T) {
	t.Run("updates debug logger", func(t *testing.T) {
		// Create limiter with default NoOpLogger
		limiter := NewConcurrentRateLimiter()

		// Verify default logger is NoOpLogger (Enabled returns false)
		if limiter.debugLogger.Enabled() {
			t.Error("expected default logger to be disabled")
		}

		// Set a new MockLogger that is enabled
		mockLogger := NewMockLogger(true)
		limiter.SetDebugLogger(mockLogger)

		// Verify logger is updated
		if !limiter.debugLogger.Enabled() {
			t.Error("expected new logger to be enabled")
		}
	})

	t.Run("can replace existing logger", func(t *testing.T) {
		initialLogger := NewMockLogger(true)
		limiter := NewConcurrentRateLimiter(WithDebugLogger(initialLogger))

		if !limiter.debugLogger.Enabled() {
			t.Error("expected initial logger to be enabled")
		}

		// Replace with disabled logger
		newLogger := NewMockLogger(false)
		limiter.SetDebugLogger(newLogger)

		if limiter.debugLogger.Enabled() {
			t.Error("expected new logger to be disabled")
		}
	})
}

func TestSetResourceDelay(t *testing.T) {
	t.Run("creates new resource timing when not exists", func(t *testing.T) {
		limiter := NewConcurrentRateLimiter()
		limiter.SetResourceDelay("api.example.com", 2*time.Second)

		timings := limiter.ResourceTimings()
		if _, exists := timings["api.example.com"]; !exists {
			t.Fatal("expected resource timing to be created")
		}
		if timings["api.example.com"].Delay() != 2*time.Second {
			t.Errorf("expected delay 2s, got %v", timings["api.example.com"].Delay())
		}
	})

	t.Run("updates delay when resource already exists", func(t *testing.T) {
		limiter := NewConcurrentRateLimiter()

		// First, create the resource timing with initial delay
		limiter.SetResourceDelay("api.example.com", 2*time.Second)

		// Add backoff state to verify it's preserved
		limiter.Backoff(context.Background(), "api.example.com")
		initialTimings := limiter.ResourceTimings()
		initialBackoffCount := initialTimings["api.example.com"].BackoffCount()

		// Now update the delay (this tests the exists == true branch)
		limiter.SetResourceDelay("api.example.com", 5*time.Second)

		timings := limiter.ResourceTimings()
		if timings["api.example.com"].Delay() != 5*time.Second {
			t.Errorf("expected delay to be updated to 5s, got %v", timings["api.example.com"].Delay())
		}

		// Verify backoff state is preserved
		if timings["api.example.com"].BackoffCount() != initialBackoffCount {
			t.Errorf("expected backoff count to be preserved as %d, got %d", initialBackoffCount, timings["api.example.com"].BackoffCount())
		}
	})

	t.Run("preserves other timing fields when updating delay", func(t *testing.T) {
		limiter := NewConcurrentRateLimiter()

		// Create resource with initial state
		limiter.SetResourceDelay("api.example.com", 1*time.Second)
		limiter.MarkLastConsumedAsNow("api.example.com")
		limiter.Backoff(context.Background(), "api.example.com")

		initialTimings := limiter.ResourceTimings()
		initialBackoffDelay := initialTimings["api.example.com"].BackoffDelay()
		initialBackoffCount := initialTimings["api.example.com"].BackoffCount()

		// Update delay
		limiter.SetResourceDelay("api.example.com", 3*time.Second)

		timings := limiter.ResourceTimings()
		// Verify delay updated
		if timings["api.example.com"].Delay() != 3*time.Second {
			t.Errorf("expected delay 3s, got %v", timings["api.example.com"].Delay())
		}
		// Verify backoff delay preserved
		if timings["api.example.com"].BackoffDelay() != initialBackoffDelay {
			t.Errorf("expected backoff delay to be preserved")
		}
		// Verify backoff count preserved
		if timings["api.example.com"].BackoffCount() != initialBackoffCount {
			t.Errorf("expected backoff count to be preserved")
		}
	})
}

func TestResolveDelay_WithLoggerEnabled(t *testing.T) {
	t.Run("logs rate limit when logger is enabled", func(t *testing.T) {
		mockLogger := NewMockLogger(true)
		limiter := NewConcurrentRateLimiter(WithDebugLogger(mockLogger))

		// Set up resource with delay
		limiter.SetResourceDelay("api.example.com", 2*time.Second)
		limiter.MarkLastConsumedAsNow("api.example.com")

		// Resolve delay
		limiter.ResolveDelay(context.Background(), "api.example.com")

		// Verify LogRateLimit was called
		if len(mockLogger.rateLimitCalls) != 1 {
			t.Fatalf("expected 1 rate limit call, got %d", len(mockLogger.rateLimitCalls))
		}

		call := mockLogger.rateLimitCalls[0]
		if call.Resource != "api.example.com" {
			t.Errorf("expected resource 'api.example.com', got %s", call.Resource)
		}
		if call.Reason != RateLimitReasonConsumeDelay {
			t.Errorf("expected reason 'consume_delay', got %s", call.Reason)
		}
	})

	t.Run("logs with backoff reason when backoff is dominant", func(t *testing.T) {
		mockLogger := NewMockLogger(true)
		limiter := NewConcurrentRateLimiter(WithDebugLogger(mockLogger))

		// Set up resource with backoff
		limiter.SetResourceDelay("api.example.com", 1*time.Second)
		limiter.MarkLastConsumedAsNow("api.example.com")
		limiter.Backoff(context.Background(), "api.example.com")

		// Resolve delay
		limiter.ResolveDelay(context.Background(), "api.example.com")

		// Verify reason is backoff
		if len(mockLogger.rateLimitCalls) != 1 {
			t.Fatalf("expected 1 rate limit call, got %d", len(mockLogger.rateLimitCalls))
		}

		call := mockLogger.rateLimitCalls[0]
		if call.Reason != RateLimitReasonBackoff {
			t.Errorf("expected reason 'backoff', got %s", call.Reason)
		}
	})

	t.Run("does not log when logger is disabled", func(t *testing.T) {
		mockLogger := NewMockLogger(false)
		limiter := NewConcurrentRateLimiter(WithDebugLogger(mockLogger))

		// Set up resource
		limiter.SetResourceDelay("api.example.com", 2*time.Second)
		limiter.MarkLastConsumedAsNow("api.example.com")

		// Resolve delay
		limiter.ResolveDelay(context.Background(), "api.example.com")

		// Verify no logging occurred
		if len(mockLogger.rateLimitCalls) != 0 {
			t.Errorf("expected 0 rate limit calls when logger disabled, got %d", len(mockLogger.rateLimitCalls))
		}
	})
}
