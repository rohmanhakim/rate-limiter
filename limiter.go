package ratelimiter

import (
	"context"
	"slices"
	"sync"
	"time"
)

// RateLimiter
// Specialized component to manage rate limiting during resource consumption
// Responsibilities:
// - Bookkeep each reasource's last consumed timestamp
// - Compute the final delay for each resource given various factors
// - Make sure the consumption process respect the resource's policy
type RateLimiter interface {
	SetBaseDelay(baseDelay time.Duration)
	SetJitter(jitter time.Duration)
	SetDebugLogger(logger DebugLogger)
	SetResourceDelay(resource string, delay time.Duration)
	Backoff(ctx context.Context, resource string, serverDelay ...time.Duration)
	ResetBackoff(resource string)
	Wait(ctx context.Context, resource string) error
	ResolveDelay(ctx context.Context, resource string) time.Duration
}

type ConcurrentRateLimiter struct {
	mu              sync.RWMutex
	rngMu           sync.Mutex
	baseDelay       time.Duration
	jitter          time.Duration
	resourceTimings map[string]resourceTiming
	config          rateLimiterConfig
	debugLogger     DebugLogger
}

func NewConcurrentRateLimiter(opts ...RateLimiterOption) *ConcurrentRateLimiter {
	config := defaults()
	for _, opt := range opts {
		opt(&config)
	}
	return &ConcurrentRateLimiter{
		resourceTimings: make(map[string]resourceTiming),
		config:          config,
		debugLogger:     config.debugLogger,
		jitter:          config.jitter,
	}
}

func (r *ConcurrentRateLimiter) SetBaseDelay(baseDelay time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.baseDelay = baseDelay
}

func (r *ConcurrentRateLimiter) SetJitter(jitter time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.jitter = jitter
}

// SetDebugLogger sets the debug logger for the rate limiter.
// This allows users to provide custom logging implementation.
func (r *ConcurrentRateLimiter) SetDebugLogger(logger DebugLogger) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.debugLogger = logger
}

// Set delay to given resource, separated from global base delay
func (r *ConcurrentRateLimiter) SetResourceDelay(resource string, delay time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	currentResourceTiming, exists := r.resourceTimings[resource]
	if exists {
		currentResourceTiming.delay = delay
		r.resourceTimings[resource] = currentResourceTiming
	} else {
		r.resourceTimings[resource] = resourceTiming{
			delay: delay,
		}
	}
}

// Backoff triggers exponential backoff for the given resource.
// It increments the backoff counter and computes the delay.
// The optional serverDelay parameter allows the caller to provide a
// server-suggested delay (e.g., from Retry-After header or response body).
// The initial backoff will be max(serverDelay, initialDuration).
func (r *ConcurrentRateLimiter) Backoff(ctx context.Context, resource string, serverDelay ...time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Extract server delay if provided
	var sd time.Duration
	if len(serverDelay) > 0 {
		sd = serverDelay[0]
	}

	currentHostTiming, exists := r.resourceTimings[resource]

	r.rngMu.Lock()

	var backoffDelay time.Duration
	var backoffCount int

	if exists {
		currentHostTiming.backoffCount++
		currentHostTiming.backoffDelay = exponentialBackoffDelay(currentHostTiming.backoffCount, r.jitter, r.config.backoff, sd)
		backoffDelay = currentHostTiming.backoffDelay
		backoffCount = currentHostTiming.backoffCount
		r.resourceTimings[resource] = currentHostTiming
	} else {
		// Initialize with backoffCount=1
		backoffDelay = exponentialBackoffDelay(1, r.jitter, r.config.backoff, sd)
		backoffCount = 1
		r.resourceTimings[resource] = resourceTiming{
			backoffCount: 1,
			backoffDelay: backoffDelay,
		}
	}
	r.rngMu.Unlock()

	// Log backoff triggered if debug enabled
	if r.debugLogger.Enabled() {
		r.debugLogger.LogBackoff(ctx, resource, backoffCount, backoffDelay)
	}
}

// ResetBackoff resets the backoff counter for the given host.
// Called after a successful request to clear backoff state.
func (r *ConcurrentRateLimiter) ResetBackoff(host string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	currentHostTiming, exists := r.resourceTimings[host]
	if exists {
		currentHostTiming.backoffCount = 0
		currentHostTiming.backoffDelay = time.Duration(0)
		r.resourceTimings[host] = currentHostTiming
	}
}

// Wait blocks until the resource is ready to be consumed.
// It resolves the delay, waits if necessary, and marks the last consumed time.
// Returns nil when ready to proceed, or error if context is cancelled.
func (r *ConcurrentRateLimiter) Wait(ctx context.Context, resource string) error {
	delay := r.ResolveDelay(ctx, resource)

	if delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}
	}

	// Mark internally - no need for user to call
	r.markLastConsumedAsNow(resource)
	return nil
}

// markLastConsumedAsNow marks the given host's lastConsumedAt to time.Now()
func (r *ConcurrentRateLimiter) markLastConsumedAsNow(host string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	currentHostTiming, exists := r.resourceTimings[host]
	if exists {
		currentHostTiming.lastConsumedAt = time.Now()
		r.resourceTimings[host] = currentHostTiming
	} else {
		r.resourceTimings[host] = resourceTiming{
			lastConsumedAt: time.Now(),
		}
	}
}

// Compute the final delay resolution for given host
// FinalDelay = max(BaseDelay, crawlDelay, BackoffDelay) + Jitter
func (r *ConcurrentRateLimiter) ResolveDelay(ctx context.Context, host string) time.Duration {
	// copy needed state under read lock, then compute without holding r.mu
	r.mu.RLock()
	currentHostTiming, exists := r.resourceTimings[host]
	base := r.baseDelay
	jitter := r.jitter
	logger := r.debugLogger
	r.mu.RUnlock()

	// return no delay if the host not registered yet
	if !exists {
		return time.Duration(0)
	}

	delays := []time.Duration{base, currentHostTiming.delay, currentHostTiming.backoffDelay}

	// compute the highest delay between BaseDelay, crawlDelay, and BackoffDelay
	finalDelay := maxDuration(delays)

	// Determine the rate limit reason based on which delay factor is dominant
	reason := determineRateLimitReason(base, currentHostTiming.delay, currentHostTiming.backoffDelay)

	r.rngMu.Lock()
	// add jitter to the final delay (computeJitter protects rng)
	jitterAmount := computeJitter(jitter)
	finalDelay += jitterAmount
	r.rngMu.Unlock()

	elapsed := time.Since(currentHostTiming.lastConsumedAt)

	// Calculate the remaining delay
	var remainingDelay time.Duration
	if elapsed < finalDelay {
		remainingDelay = finalDelay - elapsed
	}

	// Log rate limit decision if debug enabled
	if logger.Enabled() {
		logger.LogRateLimit(ctx, host, remainingDelay, reason)
	}

	return remainingDelay
}

// determineRateLimitReason determines which delay factor is dominant
func determineRateLimitReason(baseDelay, consumeDelay, backoffDelay time.Duration) RateLimitReason {
	// Determine the reason based on which delay factor is dominant
	if backoffDelay >= baseDelay && backoffDelay >= consumeDelay {
		return RateLimitReasonBackoff
	}
	if consumeDelay >= baseDelay && consumeDelay >= backoffDelay {
		return RateLimitReasonConsumeDelay
	}
	return RateLimitReasonBaseDelay
}

func (r *ConcurrentRateLimiter) BaseDelay() time.Duration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.baseDelay
}

func (r *ConcurrentRateLimiter) Jitter() time.Duration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.jitter
}

func (r *ConcurrentRateLimiter) ResourceTimings() map[string]resourceTiming {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// return a shallow copy to avoid exposing internal map for mutation
	copyMap := make(map[string]resourceTiming, len(r.resourceTimings))
	for k, v := range r.resourceTimings {
		copyMap[k] = v
	}
	return copyMap
}

// handy function to sort a slice of time.Duration and return the highest one
func maxDuration(durations []time.Duration) time.Duration {
	// guard clause: return 0 for empty slice
	if len(durations) == 0 {
		return 0
	}

	// copy the inputs to not mutate it
	d := make([]time.Duration, len(durations))
	copy(d, durations)

	// comparison function for string time.Duration
	comparison := func(a, b time.Duration) int {
		// a > b returns -1
		// a < b returns 1
		// a == b returns 0
		if a > b {
			return -1
		} else if a < b {
			return 1
		}
		return 0
	}

	// sort descending, we don't care about sorting stability
	slices.SortFunc(d, comparison)

	// return the highest (first) one
	return d[0]
}
