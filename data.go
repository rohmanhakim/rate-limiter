package ratelimiter

import "time"

// timing-related data used to track when to consume resource
type resourceTiming struct {
	lastConsumedAt time.Time
	backoffDelay   time.Duration
	delay          time.Duration
	backoffCount   int
}

func (h *resourceTiming) Delay() time.Duration {
	return h.delay
}

func (h *resourceTiming) BackoffDelay() time.Duration {
	return h.backoffDelay
}

func (h *resourceTiming) LastConsumedAt() time.Time {
	return h.lastConsumedAt
}

func (h *resourceTiming) BackoffCount() int {
	return h.backoffCount
}

// rateLimiterConfig holds the internal configuration for rate limiting logic.
// It is populated via functional options.
type rateLimiterConfig struct {
	jitter      time.Duration
	backoff     backoffConfig
	debugLogger DebugLogger
	attrs       []any
}

// backoffConfig holds the internal configuration for exponential backoff.
type backoffConfig struct {
	initialDuration time.Duration
	multiplier      float64
	maxDuration     time.Duration
}

// defaults returns a retryConfig with sensible default values.
func defaults() rateLimiterConfig {
	return rateLimiterConfig{
		jitter: 0,
		backoff: backoffConfig{
			initialDuration: 1 * time.Second,
			multiplier:      2.0,
			maxDuration:     1 * time.Minute,
		},
		debugLogger: NewNoOpLogger(),
		attrs:       []any{},
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
		c.backoff.initialDuration = d
	}
}

// WithMultiplier sets the backoff multiplier.
// Each subsequent delay is multiplied by this value. Default is 2.0.
func WithMultiplier(m float64) RateLimiterOption {
	return func(c *rateLimiterConfig) {
		c.backoff.multiplier = m
	}
}

// WithMaxDuration sets the maximum backoff duration.
// Default is 1 minute.
func WithMaxDuration(d time.Duration) RateLimiterOption {
	return func(c *rateLimiterConfig) {
		c.backoff.maxDuration = d
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
