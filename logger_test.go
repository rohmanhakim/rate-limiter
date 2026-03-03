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
