package apiclient

import (
	"context"
	"sync"
	"time"
)

// RateLimiter implements a token bucket rate limiter.
type RateLimiter struct {
	tokens     chan struct{}
	refillStop chan struct{}
	closeOnce  sync.Once
}

// NewRateLimiter creates a rate limiter that allows n requests per minute.
func NewRateLimiter(requestsPerMinute int) *RateLimiter {
	rl := &RateLimiter{
		tokens:     make(chan struct{}, requestsPerMinute),
		refillStop: make(chan struct{}),
	}
	// Start with one token so the first request goes immediately,
	// then pace subsequent requests via the refill ticker.
	rl.tokens <- struct{}{}
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

// Close stops the refill goroutine. Safe to call multiple times.
func (rl *RateLimiter) Close() {
	rl.closeOnce.Do(func() {
		close(rl.refillStop)
	})
}
