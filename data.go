package ratelimiter

import (
	"time"

	exponentialbackoff "github.com/rohmanhakim/exponential-backoff"
)

// timing-related data used to track when to consume resource
type resourceTiming struct {
	lastConsumedAt time.Time
	backoffDelay   time.Duration
	delay          time.Duration
	backoffCount   int
}

func (h resourceTiming) Delay() time.Duration {
	return h.delay
}

func (h resourceTiming) BackoffDelay() time.Duration {
	return h.backoffDelay
}

func (h resourceTiming) LastConsumedAt() time.Time {
	return h.lastConsumedAt
}

func (h resourceTiming) BackoffCount() int {
	return h.backoffCount
}

// rateLimiterConfig holds the internal configuration for rate limiting logic.
// It is populated via functional options.
type rateLimiterConfig struct {
	jitter         time.Duration
	backoffInitial time.Duration
	backoffMult    float64
	backoffMax     time.Duration
	debugLogger    DebugLogger
	attrs          []any
}

// backoffConfig returns an exponentialbackoff.Config for use with the external library.
func (c rateLimiterConfig) backoffConfig() exponentialbackoff.Config {
	return exponentialbackoff.MustConfig(c.backoffInitial, c.backoffMax, c.backoffMult)
}

// defaults returns a retryConfig with sensible default values.
func defaults() rateLimiterConfig {
	return rateLimiterConfig{
		jitter:         0,
		backoffInitial: 1 * time.Second,
		backoffMult:    2.0,
		backoffMax:     1 * time.Minute,
		debugLogger:    NewNoOpLogger(),
		attrs:          []any{},
	}
}

// RateLimiterOption is a functional option for configuring retry behavior.
type RateLimiterOption func(*rateLimiterConfig)

// WithJitter sets the maximum random duration added to backoff delays.
// This helps avoid thundering herd problems. Default is 0 (no jitter).
func WithJitter(d time.Duration) RateLimiterOption {
	return func(c *rateLimiterConfig) {
		c.jitter = d
	}
}

// WithInitialDuration sets the initial backoff duration.
// Default is 1 second.
func WithInitialDuration(d time.Duration) RateLimiterOption {
	return func(c *rateLimiterConfig) {
		c.backoffInitial = d
	}
}

// WithMultiplier sets the backoff multiplier.
// Each subsequent delay is multiplied by this value. Default is 2.0.
func WithMultiplier(m float64) RateLimiterOption {
	return func(c *rateLimiterConfig) {
		c.backoffMult = m
	}
}

// WithMaxDuration sets the maximum backoff duration.
// Default is 1 minute.
func WithMaxDuration(d time.Duration) RateLimiterOption {
	return func(c *rateLimiterConfig) {
		c.backoffMax = d
	}
}

// WithLogAttrs sets additional attributes to be passed to the logger.
// Attributes follow Go's slog convention for structured logging - alternating
// key-value pairs (string, any, string, any, ...).
// These attributes are passed to LogRetry calls for structured logging context.
func WithLogAttrs(attrs ...any) RateLimiterOption {
	return func(c *rateLimiterConfig) {
		c.attrs = attrs
	}
}

func WithDebugLogger(debugLogger DebugLogger, attrs ...any) RateLimiterOption {
	return func(c *rateLimiterConfig) {
		c.debugLogger = debugLogger
	}
}

// RateLimitReason describes why a rate limit was applied.
type RateLimitReason string

const (
	// RateLimitReasonBaseDelay indicates the base delay was applied.
	RateLimitReasonBaseDelay RateLimitReason = "base_delay"
	// RateLimitReasonCrawlDelay indicates crawl-delay from robots.txt was applied.
	RateLimitReasonConsumeDelay RateLimitReason = "consume_delay"
	// RateLimitReasonBackoff indicates exponential backoff was applied.
	RateLimitReasonBackoff RateLimitReason = "backoff"
)
