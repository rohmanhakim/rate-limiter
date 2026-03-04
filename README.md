# rate-limiter

[![codecov](https://codecov.io/github/rohmanhakim/rate-limiter/graph/badge.svg?token=ENEK67CwCY)](https://codecov.io/github/rohmanhakim/rate-limiter)
[![Go Reference](https://pkg.go.dev/badge/github.com/rohmanhakim/rate-limiter.svg)](https://pkg.go.dev/github.com/rohmanhakim/rate-limiter)

A highly concurrent, thread-safe Go package for rate limiting resource consumption. Features granular delays, exponential backoff, and jitter support. Zero external dependencies.

## Features

- **Concurrent Engine**: Read/Write locks engineered to handle high throughput without unnecessary lock contention
- **Granular Tuning**: Set separate delays per specific resource alongside global base delays
- **Exponential Backoff**: Dynamically slow down resource firing using proven backoff math
- **Jitter Support**: Add randomness to delays to help avoid thundering herd and collision problems
- **Clean Configuration**: Intuitive functional options API with sensible defaults
- **Context Awareness**: Full `context.Context` integration for graceful cancellation, eliminating goroutine leaks
- **Debug Logging**: Optional logging interface to trace rate limiting activity
- **Zero Dependencies**: Relies solely on the Go standard library

## Installation

```bash
go get github.com/rohmanhakim/rate-limiter
```

## Quick Start

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/rohmanhakim/rate-limiter"
)

func main() {
	// 1. Create a concurrent rate limiter with sensible defaults
	rl := ratelimiter.NewConcurrentRateLimiter(
		ratelimiter.WithJitter(100*time.Millisecond),
		ratelimiter.WithInitialDuration(1*time.Second),
		ratelimiter.WithMultiplier(2.0),
		ratelimiter.WithMaxDuration(30*time.Second),
	)

	// Set a global base delay (all requests will wait AT LEAST this long)
	rl.SetBaseDelay(500 * time.Millisecond)

	ctx := context.Background()
	resource := "api.example.com"

	// 2. Wrap your operations by waiting
	err := rl.Wait(ctx, resource)
	if err != nil { // handles context deadline/cancellation
		fmt.Printf("Rate limit waiter cancelled: %v\n", err)
		return
	}

	fmt.Printf("Executing request for %s...\n", resource)
	// Output: Executing request for api.example.com...
}
```

## Configuration Options

### Functional Options

The `NewConcurrentRateLimiter` function accepts functional options for configuration:

| Option | Description | Default |
|--------|-------------|---------|
| `WithJitter(d time.Duration)` | Maximum random delay added to compute jitter | 0 (no jitter) |
| `WithInitialDuration(d time.Duration)` | Initial backoff duration | 1 second |
| `WithMultiplier(m float64)` | Exponential backoff multiplier | 2.0 |
| `WithMaxDuration(d time.Duration)` | Cap for maximum backoff duration | 1 minute |
| `WithDebugLogger(l DebugLogger, ...)` | Sets custom logging implementation | NoOpLogger |
| `WithLogAttrs(attrs ...any)` | Additional attributes passed to the logger | none |

### Additional Limits & Overrides

You can manually adjust delays for specific resources overriding the base settings dynamically:

```go
rl := ratelimiter.NewConcurrentRateLimiter()

// Global rate (e.g. 1 req/s)
rl.SetBaseDelay(1 * time.Second) 

// Specific resource override (e.g. 1 req / 5s for stricter endpoints)
rl.SetResourceDelay("strict.example.com", 5 * time.Second) 
```

## Backoff Mechanism

The rate limiter supports explicit signal-driven exponential backoffs. If a remote server sends a `429 Too Many Requests` or `5xx` error, you can instruct the limiter to back off:

```go
// Trigger an exponential backoff state specifically for "api.example.com"
rl.Backoff(ctx, "api.example.com")

// Optionally define a server-suggested delay (e.g., from Retry-After header)
rl.Backoff(ctx, "api.example.com", ratelimiter.BackoffOptions{
    ServerDelay: 5 * time.Second,
})

// Once the remote service recovers, reset the backoff state
rl.ResetBackoff("api.example.com")
```

The computed delay resolution logic takes the highest active duration among: `BaseDelay`, `ResourceDelay`, and `BackoffDelay` before adding randomized `Jitter`.

## Context Cancellation

The rate limiter handles context cancellation gracefully across delays, ensuring there are no hidden memory leaks during high concurrency and long backoff bounds:

```go
ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
defer cancel()

// If the delay is 5s, it will terminate after 2s without leaking resources
err := rl.Wait(ctx, "slow.api.com")
if err != nil {
    // Expected: context deadline exceeded
}
```

## Debug Logging

Adding observability to rate limits and backoffs is simple via the `DebugLogger` interface. 

```go
type MyLogger struct{}

func (l *MyLogger) Enabled() bool { return true }

func (l *MyLogger) LogBackoff(ctx context.Context, resource string, count int, delay time.Duration, attrs ...any) {
    log.Printf("Backoff triggered [%s]: attempt=%d, delay=%v", resource, count, delay)
}

func (l *MyLogger) LogRateLimit(ctx context.Context, resource string, delay time.Duration, reason ratelimiter.RateLimitReason, attrs ...any) {
    log.Printf("Rate limit applied [%s]: delay=%v, reason=%s", resource, delay, reason)
}

// Attach it to the rate limiter!
rl := ratelimiter.NewConcurrentRateLimiter(
    ratelimiter.WithDebugLogger(&MyLogger{}),
)
```

## API Reference

### Key Types & Methods

```go
// Core Interface for Mocking
type RateLimiter interface { ... }

// The primary implementation
type ConcurrentRateLimiter struct { ... }

func NewConcurrentRateLimiter(opts ...RateLimiterOption) *ConcurrentRateLimiter

// Setting Global & Specific Rules
func (r *ConcurrentRateLimiter) SetBaseDelay(baseDelay time.Duration)
func (r *ConcurrentRateLimiter) SetJitter(jitter time.Duration)
func (r *ConcurrentRateLimiter) SetResourceDelay(resource string, delay time.Duration)

// Execution
func (r *ConcurrentRateLimiter) Wait(ctx context.Context, resource string) error
func (r *ConcurrentRateLimiter) Backoff(ctx context.Context, resource string, opts ...BackoffOptions)
func (r *ConcurrentRateLimiter) ResetBackoff(resource string)
func (r *ConcurrentRateLimiter) ResolveDelay(ctx context.Context, resource string) time.Duration

type BackoffOptions struct {
	ServerDelay time.Duration
}
```

## Examples

Complete, runnable examples are provided in the `example/` directory demonstrating patterns such as:
- **`multi_service_fetch/`**: A complex, concurrent worker pool crawling multiple domains applying domain-specific delays and respecting backoff rules.
- **`simple_fetch/`**: A straightforward single pipeline demonstrating basic throttling.

## License

MIT License - see [LICENSE](LICENSE) for details.
