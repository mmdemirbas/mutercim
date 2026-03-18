package apiclient

import (
	"context"
	"time"
)

// RateLimiter implements a token bucket rate limiter.
type RateLimiter struct {
	tokens     chan struct{}
	refillStop chan struct{}
}

// NewRateLimiter creates a rate limiter that allows n requests per minute.
func NewRateLimiter(requestsPerMinute int) *RateLimiter {
	rl := &RateLimiter{
		tokens:     make(chan struct{}, requestsPerMinute),
		refillStop: make(chan struct{}),
	}
	// Fill initial tokens
	for i := 0; i < requestsPerMinute; i++ {
		rl.tokens <- struct{}{}
	}
	// Start refill goroutine
	interval := time.Minute / time.Duration(requestsPerMinute)
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				select {
				case rl.tokens <- struct{}{}:
				default:
					// Bucket full, discard token
				}
			case <-rl.refillStop:
				return
			}
		}
	}()
	return rl
}

// Wait blocks until a token is available or ctx is cancelled.
func (rl *RateLimiter) Wait(ctx context.Context) error {
	select {
	case <-rl.tokens:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Close stops the refill goroutine.
func (rl *RateLimiter) Close() {
	close(rl.refillStop)
}
