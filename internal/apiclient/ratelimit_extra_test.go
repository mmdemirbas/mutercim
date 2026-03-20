package apiclient

import (
	"context"
	"testing"
)

func TestRateLimiterMultipleClose(t *testing.T) {
	rl := NewRateLimiter(60)

	// Multiple Close calls should not panic
	rl.Close()
	rl.Close()
	rl.Close()
}

func TestRateLimiterWaitAfterClose(t *testing.T) {
	rl := NewRateLimiter(60)

	// Consume initial token
	if err := rl.Wait(context.Background()); err != nil {
		t.Fatalf("first Wait: %v", err)
	}

	rl.Close()

	// Wait after Close — should fail via context since no more tokens will be produced
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // immediately cancelled
	err := rl.Wait(ctx)
	if err == nil {
		t.Error("expected error after Close with cancelled context")
	}
}

func TestRateLimiterPreCancelledContext(t *testing.T) {
	rl := NewRateLimiter(60)
	defer rl.Close()

	// Consume the initial token
	if err := rl.Wait(context.Background()); err != nil {
		t.Fatalf("consume token: %v", err)
	}

	// Pre-cancelled context should return immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := rl.Wait(ctx)
	if err == nil {
		t.Error("expected error for pre-cancelled context")
	}
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
}
