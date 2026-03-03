package ratelimiter

import (
	"context"
	"time"
)

// DebugLogger provides structured debug logging capabilities for the rate limiter package.
// Users can implement this interface to provide custom logging behavior.
// When debug mode is disabled, use NoOpLogger for zero overhead.
type DebugLogger interface {
	// Enabled returns true if debug logging is enabled.
	// When false, the retry handler will skip logging entirely for efficiency.
	Enabled() bool

	LogBackoff(ctx context.Context, resource string, count int, delay time.Duration, attrs ...any)

	LogRateLimit(ctx context.Context, resource string, delay time.Duration, reason RateLimitReason, attrs ...any)
}

// NoOpLogger is a no-operation implementation of DebugLogger.
// It provides zero overhead when debug mode is disabled.
// All methods are empty and Enabled() always returns false.
type NoOpLogger struct{}

// NewNoOpLogger creates a new NoOpLogger instance.
func NewNoOpLogger() *NoOpLogger {
	return &NoOpLogger{}
}

// Enabled returns false - debug logging is disabled.
func (n *NoOpLogger) Enabled() bool { return false }

// LogRetry is a no-op.
func (n *NoOpLogger) LogRateLimit(_ context.Context, _ string, _ time.Duration, _ RateLimitReason, _ ...any) {
}

// LogBackoff is no-op.
func (n *NoOpLogger) LogBackoff(_ context.Context, _ string, _ int, _ time.Duration, _ ...any) {}
