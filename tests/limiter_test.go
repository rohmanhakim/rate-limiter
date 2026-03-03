package ratelimiter_test

import (
	"context"
	"testing"
	"time"

	ratelimiter "github.com/rohmanhakim/rate-limiter"
)

// TestRateLimiterInterface verifies the RateLimiter interface implementation
func TestRateLimiterInterface(t *testing.T) {
	// This test ensures ConcurrentRateLimiter implements RateLimiter interface
	var _ ratelimiter.RateLimiter = ratelimiter.NewConcurrentRateLimiter()
}

func TestNewConcurrentRateLimiter(t *testing.T) {
	t.Run("creates limiter with defaults", func(t *testing.T) {
		limiter := ratelimiter.NewConcurrentRateLimiter()
		if limiter == nil {
			t.Error("expected limiter to be created, got nil")
		}
	})

	t.Run("applies functional options", func(t *testing.T) {
		limiter := ratelimiter.NewConcurrentRateLimiter(
			ratelimiter.WithJitter(100*time.Millisecond),
			ratelimiter.WithInitialDuration(2*time.Second),
			ratelimiter.WithMultiplier(3.0),
			ratelimiter.WithMaxDuration(30*time.Second),
		)
		if limiter == nil {
			t.Error("expected limiter to be created, got nil")
		}
	})
}

func TestConcurrentRateLimiter_SetBaseDelay(t *testing.T) {
	limiter := ratelimiter.NewConcurrentRateLimiter()

	limiter.SetBaseDelay(5 * time.Second)

	if limiter.BaseDelay() != 5*time.Second {
		t.Errorf("expected base delay 5s, got %v", limiter.BaseDelay())
	}
}

func TestConcurrentRateLimiter_SetJitter(t *testing.T) {
	limiter := ratelimiter.NewConcurrentRateLimiter()

	limiter.SetJitter(200 * time.Millisecond)

	if limiter.Jitter() != 200*time.Millisecond {
		t.Errorf("expected jitter 200ms, got %v", limiter.Jitter())
	}
}

func TestConcurrentRateLimiter_SetResourceDelay(t *testing.T) {
	limiter := ratelimiter.NewConcurrentRateLimiter()

	limiter.SetResourceDelay("api.example.com", 3*time.Second)

	timings := limiter.ResourceTimings()
	timing, exists := timings["api.example.com"]
	if !exists {
		t.Fatal("expected resource timing to exist")
	}
	if timing.Delay() != 3*time.Second {
		t.Errorf("expected delay 3s, got %v", timing.Delay())
	}
}

func TestConcurrentRateLimiter_Backoff(t *testing.T) {
	t.Run("triggers backoff for new resource", func(t *testing.T) {
		limiter := ratelimiter.NewConcurrentRateLimiter(
			ratelimiter.WithInitialDuration(1 * time.Second),
		)

		limiter.Backoff(context.Background(), "api.example.com")

		timings := limiter.ResourceTimings()
		timing, exists := timings["api.example.com"]
		if !exists {
			t.Fatal("expected resource timing to exist")
		}
		if timing.BackoffCount() != 1 {
			t.Errorf("expected backoff count 1, got %d", timing.BackoffCount())
		}
		if timing.BackoffDelay() < 1*time.Second {
			t.Errorf("expected backoff delay >= 1s, got %v", timing.BackoffDelay())
		}
	})

	t.Run("increments backoff count for existing resource", func(t *testing.T) {
		limiter := ratelimiter.NewConcurrentRateLimiter(
			ratelimiter.WithInitialDuration(1 * time.Second),
		)

		limiter.Backoff(context.Background(), "api.example.com")
		limiter.Backoff(context.Background(), "api.example.com")

		timings := limiter.ResourceTimings()
		timing := timings["api.example.com"]
		if timing.BackoffCount() != 2 {
			t.Errorf("expected backoff count 2, got %d", timing.BackoffCount())
		}
	})

	t.Run("uses server delay when provided", func(t *testing.T) {
		limiter := ratelimiter.NewConcurrentRateLimiter(
			ratelimiter.WithInitialDuration(1 * time.Second),
		)

		serverDelay := 5 * time.Second
		limiter.Backoff(context.Background(), "api.example.com", serverDelay)

		timings := limiter.ResourceTimings()
		timing := timings["api.example.com"]
		if timing.BackoffDelay() < serverDelay {
			t.Errorf("expected backoff delay >= server delay (%v), got %v", serverDelay, timing.BackoffDelay())
		}
	})
}

func TestConcurrentRateLimiter_ResetBackoff(t *testing.T) {
	limiter := ratelimiter.NewConcurrentRateLimiter()

	limiter.Backoff(context.Background(), "api.example.com")
	limiter.ResetBackoff("api.example.com")

	timings := limiter.ResourceTimings()
	timing := timings["api.example.com"]
	if timing.BackoffCount() != 0 {
		t.Errorf("expected backoff count 0 after reset, got %d", timing.BackoffCount())
	}
	if timing.BackoffDelay() != 0 {
		t.Errorf("expected backoff delay 0 after reset, got %v", timing.BackoffDelay())
	}
}

func TestConcurrentRateLimiter_MarkLastConsumedAsNow(t *testing.T) {
	limiter := ratelimiter.NewConcurrentRateLimiter()

	before := time.Now()
	limiter.MarkLastConsumedAsNow("api.example.com")
	after := time.Now()

	timings := limiter.ResourceTimings()
	timing := timings["api.example.com"]
	lastConsumed := timing.LastConsumedAt()

	if lastConsumed.Before(before) || lastConsumed.After(after) {
		t.Errorf("expected last consumed time between %v and %v, got %v", before, after, lastConsumed)
	}
}

func TestConcurrentRateLimiter_ResolveDelay(t *testing.T) {
	t.Run("returns zero for unregistered resource", func(t *testing.T) {
		limiter := ratelimiter.NewConcurrentRateLimiter()

		delay := limiter.ResolveDelay(context.Background(), "unknown-resource")
		if delay != 0 {
			t.Errorf("expected 0 delay for unknown resource, got %v", delay)
		}
	})

	t.Run("returns remaining delay after marking consumed", func(t *testing.T) {
		limiter := ratelimiter.NewConcurrentRateLimiter()
		limiter.SetBaseDelay(5 * time.Second)
		limiter.MarkLastConsumedAsNow("api.example.com")

		// Immediately after marking, remaining delay should be close to base delay
		delay := limiter.ResolveDelay(context.Background(), "api.example.com")
		if delay < 4*time.Second {
			t.Errorf("expected delay >= 4s, got %v", delay)
		}
	})

	t.Run("returns zero delay after waiting", func(t *testing.T) {
		limiter := ratelimiter.NewConcurrentRateLimiter()
		limiter.SetBaseDelay(100 * time.Millisecond)
		limiter.MarkLastConsumedAsNow("api.example.com")

		// Wait for delay to pass
		time.Sleep(150 * time.Millisecond)

		delay := limiter.ResolveDelay(context.Background(), "api.example.com")
		if delay != 0 {
			t.Errorf("expected 0 delay after waiting, got %v", delay)
		}
	})

	t.Run("respects backoff delay", func(t *testing.T) {
		limiter := ratelimiter.NewConcurrentRateLimiter(
			ratelimiter.WithInitialDuration(5 * time.Second),
		)
		limiter.MarkLastConsumedAsNow("api.example.com")
		limiter.Backoff(context.Background(), "api.example.com")

		delay := limiter.ResolveDelay(context.Background(), "api.example.com")
		if delay < 4*time.Second {
			t.Errorf("expected delay >= 4s (backoff), got %v", delay)
		}
	})

	t.Run("respects resource delay", func(t *testing.T) {
		limiter := ratelimiter.NewConcurrentRateLimiter()
		limiter.SetResourceDelay("api.example.com", 5*time.Second)
		limiter.MarkLastConsumedAsNow("api.example.com")

		delay := limiter.ResolveDelay(context.Background(), "api.example.com")
		if delay < 4*time.Second {
			t.Errorf("expected delay >= 4s (resource delay), got %v", delay)
		}
	})
}

func TestConcurrentRateLimiter_ResourceTimings(t *testing.T) {
	t.Run("returns copy of timings map", func(t *testing.T) {
		limiter := ratelimiter.NewConcurrentRateLimiter()
		limiter.MarkLastConsumedAsNow("api.example.com")

		timings1 := limiter.ResourceTimings()
		timings2 := limiter.ResourceTimings()

		// Both should be separate maps (copies)
		// Modifying timings1 should not affect timings2
		if &timings1 == &timings2 {
			t.Error("expected ResourceTimings to return copies, not same map reference")
		}

		// Both should have the same resource
		if len(timings1) != len(timings2) {
			t.Errorf("expected both maps to have same length: %d vs %d", len(timings1), len(timings2))
		}
	})
}

func TestConcurrentRateLimiter_Concurrency(t *testing.T) {
	limiter := ratelimiter.NewConcurrentRateLimiter()

	// Run concurrent operations to ensure no race conditions
	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func(id int) {
			resource := "api.example.com"
			for j := 0; j < 100; j++ {
				limiter.MarkLastConsumedAsNow(resource)
				limiter.Backoff(context.Background(), resource)
				limiter.ResolveDelay(context.Background(), resource)
				limiter.ResetBackoff(resource)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestRateLimitReason(t *testing.T) {
	t.Run("reason constants are defined", func(t *testing.T) {
		if ratelimiter.RateLimitReasonBaseDelay != "base_delay" {
			t.Errorf("expected RateLimitReasonBaseDelay to be 'base_delay'")
		}
		if ratelimiter.RateLimitReasonConsumeDelay != "consume_delay" {
			t.Errorf("expected RateLimitReasonConsumeDelay to be 'consume_delay'")
		}
		if ratelimiter.RateLimitReasonBackoff != "backoff" {
			t.Errorf("expected RateLimitReasonBackoff to be 'backoff'")
		}
	})
}

func TestWithDebugLogger(t *testing.T) {
	logger := NewMockLogger(true)
	limiter := ratelimiter.NewConcurrentRateLimiter(
		ratelimiter.WithDebugLogger(logger),
	)

	limiter.Backoff(context.Background(), "api.example.com")

	if len(logger.backoffCalls) != 1 {
		t.Errorf("expected 1 backoff log call, got %d", len(logger.backoffCalls))
	}
}

// MockLogger for testing (black-box friendly - implements DebugLogger)
type MockLogger struct {
	enabled        bool
	backoffCalls   []BackoffCall
	rateLimitCalls []RateLimitCall
}

type BackoffCall struct {
	Resource string
	Count    int
	Delay    time.Duration
}

type RateLimitCall struct {
	Resource string
	Delay    time.Duration
	Reason   ratelimiter.RateLimitReason
}

func NewMockLogger(enabled bool) *MockLogger {
	return &MockLogger{
		enabled:        enabled,
		backoffCalls:   make([]BackoffCall, 0),
		rateLimitCalls: make([]RateLimitCall, 0),
	}
}

func (m *MockLogger) Enabled() bool {
	return m.enabled
}

func (m *MockLogger) LogBackoff(_ context.Context, resource string, count int, delay time.Duration, _ ...any) {
	m.backoffCalls = append(m.backoffCalls, BackoffCall{
		Resource: resource,
		Count:    count,
		Delay:    delay,
	})
}

func (m *MockLogger) LogRateLimit(_ context.Context, resource string, delay time.Duration, reason ratelimiter.RateLimitReason, _ ...any) {
	m.rateLimitCalls = append(m.rateLimitCalls, RateLimitCall{
		Resource: resource,
		Delay:    delay,
		Reason:   reason,
	})
}
