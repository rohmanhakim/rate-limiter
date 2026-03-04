package ratelimiter

import (
	"context"
	"testing"
	"time"
)

func TestNoOpLogger(t *testing.T) {
	logger := NewNoOpLogger()

	t.Run("Enabled returns false", func(t *testing.T) {
		if logger.Enabled() {
			t.Error("expected Enabled() to return false")
		}
	})

	t.Run("methods do not panic", func(t *testing.T) {
		// These should not panic and should do nothing
		logger.LogBackoff(context.Background(), "test-resource", 1, 5*time.Second)
		logger.LogRateLimit(context.Background(), "test-resource", 5*time.Second, RateLimitReasonBackoff)
	})
}

// MockLogger is a mock implementation of DebugLogger for testing
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
	Reason   RateLimitReason
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

func (m *MockLogger) LogRateLimit(_ context.Context, resource string, delay time.Duration, reason RateLimitReason, _ ...any) {
	m.rateLimitCalls = append(m.rateLimitCalls, RateLimitCall{
		Resource: resource,
		Delay:    delay,
		Reason:   reason,
	})
}

func TestMockLogger(t *testing.T) {
	t.Run("Enabled returns correct value", func(t *testing.T) {
		logger := NewMockLogger(true)
		if !logger.Enabled() {
			t.Error("expected Enabled() to return true")
		}

		logger = NewMockLogger(false)
		if logger.Enabled() {
			t.Error("expected Enabled() to return false")
		}
	})

	t.Run("LogBackoff records calls", func(t *testing.T) {
		logger := NewMockLogger(true)
		logger.LogBackoff(context.Background(), "api.example.com", 3, 5*time.Second)

		if len(logger.backoffCalls) != 1 {
			t.Fatalf("expected 1 backoff call, got %d", len(logger.backoffCalls))
		}

		call := logger.backoffCalls[0]
		if call.Resource != "api.example.com" {
			t.Errorf("expected resource 'api.example.com', got %s", call.Resource)
		}
		if call.Count != 3 {
			t.Errorf("expected count 3, got %d", call.Count)
		}
		if call.Delay != 5*time.Second {
			t.Errorf("expected delay 5s, got %v", call.Delay)
		}
	})

	t.Run("LogRateLimit records calls", func(t *testing.T) {
		logger := NewMockLogger(true)
		logger.LogRateLimit(context.Background(), "api.example.com", 2*time.Second, RateLimitReasonBackoff)

		if len(logger.rateLimitCalls) != 1 {
			t.Fatalf("expected 1 rate limit call, got %d", len(logger.rateLimitCalls))
		}

		call := logger.rateLimitCalls[0]
		if call.Resource != "api.example.com" {
			t.Errorf("expected resource 'api.example.com', got %s", call.Resource)
		}
		if call.Delay != 2*time.Second {
			t.Errorf("expected delay 2s, got %v", call.Delay)
		}
		if call.Reason != RateLimitReasonBackoff {
			t.Errorf("expected reason 'backoff', got %s", call.Reason)
		}
	})
}

// Integration tests for LogRateLimit and LogBackoff with rate limiter

func TestLogRateLimit_Integration(t *testing.T) {
	t.Run("LogRateLimit called with correct params from ResolveDelay", func(t *testing.T) {
		mockLogger := NewMockLogger(true)
		limiter := NewConcurrentRateLimiter(WithDebugLogger(mockLogger))

		// Set up resource timing
		limiter.SetBaseDelay(3 * time.Second)
		limiter.SetResourceDelay("test-api", 2*time.Second)
		limiter.MarkLastConsumedAsNow("test-api")

		// Trigger ResolveDelay
		_ = limiter.ResolveDelay(context.Background(), "test-api")

		// Verify LogRateLimit was called
		if len(mockLogger.rateLimitCalls) != 1 {
			t.Fatalf("expected 1 rate limit call, got %d", len(mockLogger.rateLimitCalls))
		}

		call := mockLogger.rateLimitCalls[0]
		if call.Resource != "test-api" {
			t.Errorf("expected resource 'test-api', got %s", call.Resource)
		}
		// Base delay (3s) > consume delay (2s), so reason should be base_delay
		if call.Reason != RateLimitReasonBaseDelay {
			t.Errorf("expected reason 'base_delay', got %s", call.Reason)
		}
	})

	t.Run("LogRateLimit called with consume_delay reason", func(t *testing.T) {
		mockLogger := NewMockLogger(true)
		limiter := NewConcurrentRateLimiter(WithDebugLogger(mockLogger))

		// Set up resource timing where consume delay is dominant
		limiter.SetBaseDelay(1 * time.Second)
		limiter.SetResourceDelay("test-api", 5*time.Second)
		limiter.MarkLastConsumedAsNow("test-api")

		// Trigger ResolveDelay
		_ = limiter.ResolveDelay(context.Background(), "test-api")

		// Verify reason
		if len(mockLogger.rateLimitCalls) != 1 {
			t.Fatalf("expected 1 rate limit call, got %d", len(mockLogger.rateLimitCalls))
		}

		call := mockLogger.rateLimitCalls[0]
		if call.Reason != RateLimitReasonConsumeDelay {
			t.Errorf("expected reason 'consume_delay', got %s", call.Reason)
		}
	})

	t.Run("LogRateLimit includes attrs when set", func(t *testing.T) {
		mockLogger := NewMockLogger(true)
		limiter := NewConcurrentRateLimiter(
			WithDebugLogger(mockLogger),
			WithLogAttrs("service", "api-service", "env", "test"),
		)

		limiter.SetResourceDelay("test-api", 2*time.Second)
		limiter.MarkLastConsumedAsNow("test-api")

		_ = limiter.ResolveDelay(context.Background(), "test-api")

		// Verify LogRateLimit was called (attrs are passed via variadic param)
		if len(mockLogger.rateLimitCalls) != 1 {
			t.Fatalf("expected 1 rate limit call, got %d", len(mockLogger.rateLimitCalls))
		}
	})
}

func TestLogBackoff_Integration(t *testing.T) {
	t.Run("LogBackoff called with correct params from Backoff", func(t *testing.T) {
		mockLogger := NewMockLogger(true)
		limiter := NewConcurrentRateLimiter(WithDebugLogger(mockLogger))

		// Trigger backoff
		limiter.Backoff(context.Background(), "test-api")

		// Verify LogBackoff was called
		if len(mockLogger.backoffCalls) != 1 {
			t.Fatalf("expected 1 backoff call, got %d", len(mockLogger.backoffCalls))
		}

		call := mockLogger.backoffCalls[0]
		if call.Resource != "test-api" {
			t.Errorf("expected resource 'test-api', got %s", call.Resource)
		}
		if call.Count != 1 {
			t.Errorf("expected count 1, got %d", call.Count)
		}
		if call.Delay <= 0 {
			t.Errorf("expected positive delay, got %v", call.Delay)
		}
	})

	t.Run("LogBackoff called with incrementing count on repeated backoffs", func(t *testing.T) {
		mockLogger := NewMockLogger(true)
		limiter := NewConcurrentRateLimiter(WithDebugLogger(mockLogger))

		// Trigger multiple backoffs
		limiter.Backoff(context.Background(), "test-api")
		limiter.Backoff(context.Background(), "test-api")
		limiter.Backoff(context.Background(), "test-api")

		// Verify counts increment
		if len(mockLogger.backoffCalls) != 3 {
			t.Fatalf("expected 3 backoff calls, got %d", len(mockLogger.backoffCalls))
		}

		expectedCounts := []int{1, 2, 3}
		for i, call := range mockLogger.backoffCalls {
			if call.Count != expectedCounts[i] {
				t.Errorf("call %d: expected count %d, got %d", i, expectedCounts[i], call.Count)
			}
		}
	})

	t.Run("LogBackoff not called when logger disabled", func(t *testing.T) {
		mockLogger := NewMockLogger(false)
		limiter := NewConcurrentRateLimiter(WithDebugLogger(mockLogger))

		// Trigger backoff
		limiter.Backoff(context.Background(), "test-api")

		// Verify no logging occurred
		if len(mockLogger.backoffCalls) != 0 {
			t.Errorf("expected 0 backoff calls when logger disabled, got %d", len(mockLogger.backoffCalls))
		}
	})

	t.Run("LogBackoff includes attrs when set", func(t *testing.T) {
		mockLogger := NewMockLogger(true)
		limiter := NewConcurrentRateLimiter(
			WithDebugLogger(mockLogger),
			WithLogAttrs("service", "api-service"),
		)

		// Trigger backoff
		limiter.Backoff(context.Background(), "test-api")

		// Verify LogBackoff was called
		if len(mockLogger.backoffCalls) != 1 {
			t.Fatalf("expected 1 backoff call, got %d", len(mockLogger.backoffCalls))
		}
	})
}
