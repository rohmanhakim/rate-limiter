package ratelimiter

import (
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
