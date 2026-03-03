// Package simple_logger provides a simple implementation of the ratelimiter.DebugLogger interface
// that prints retry information to stdout.
package simple_logger

import (
	"context"
	"fmt"
	"time"

	ratelimiter "github.com/rohmanhakim/rate-limiter"
)

// SimpleLogger implements ratelimiter.DebugLogger interface.
// It prints retry attempts to stdout with timestamps.
type SimpleLogger struct{}

// NewSimpleLogger creates a new SimpleLogger instance.
func NewSimpleLogger() *SimpleLogger {
	return &SimpleLogger{}
}

// Enabled returns true to enable debug logging.
func (l *SimpleLogger) Enabled() bool {
	return true
}

// LogBackoff logs backoff events when the rate limiter triggers exponential backoff.
func (l *SimpleLogger) LogBackoff(ctx context.Context, resource string, count int, delay time.Duration, attrs ...any) {
	timestamp := time.Now().Format("15:04:05.000")
	fmt.Printf("[%s] 🔄 BACKOFF | resource: %s | count: %d | delay: %s\n",
		timestamp, resource, count, delay.Round(time.Millisecond))
}

// LogRateLimit logs rate limit decisions when delay is applied before a request.
func (l *SimpleLogger) LogRateLimit(ctx context.Context, resource string, delay time.Duration, reason ratelimiter.RateLimitReason, attrs ...any) {
	timestamp := time.Now().Format("15:04:05.000")
	fmt.Printf("[%s] ⏳ RATE LIMIT | resource: %s | delay: %s | reason: %s\n",
		timestamp, resource, delay.Round(time.Millisecond), reason)
}

// Interface assertion to ensure SimpleLogger implements DebugLogger
var _ ratelimiter.DebugLogger = (*SimpleLogger)(nil)
