package apiclient

import (
	"context"
	"testing"
	"time"
)

func TestRateLimiterWait(t *testing.T) {
	rl := NewRateLimiter(60) // 60 RPM = 1 per second
	defer rl.Close()

	// Should be able to consume all initial tokens without blocking
	ctx := context.Background()
	for i := 0; i < 60; i++ {
		if err := rl.Wait(ctx); err != nil {
			t.Fatalf("Wait() returned error on token %d: %v", i, err)
		}
	}
}

func TestRateLimiterContextCancelled(t *testing.T) {
	rl := NewRateLimiter(1) // 1 RPM
	defer rl.Close()

	// Consume the only token
	ctx := context.Background()
	if err := rl.Wait(ctx); err != nil {
		t.Fatalf("Wait() returned error: %v", err)
	}

	// Now try with a cancelled context — should fail
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := rl.Wait(ctx)
	if err == nil {
		t.Fatal("Wait() should have returned error for cancelled context")
	}
}

func TestRateLimiterRefill(t *testing.T) {
	// Use a high RPM so refill happens quickly (every ~16ms for 3600 RPM)
	rl := NewRateLimiter(3600)
	defer rl.Close()

	// Drain all tokens
	ctx := context.Background()
	for i := 0; i < 3600; i++ {
		if err := rl.Wait(ctx); err != nil {
			t.Fatalf("Wait() error draining token %d: %v", i, err)
		}
	}

	// Wait for a refill (at 3600 RPM = 1 per ~16ms)
	time.Sleep(50 * time.Millisecond)

	// Should be able to get at least one token after refill
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := rl.Wait(ctx); err != nil {
		t.Fatalf("Wait() after refill returned error: %v", err)
	}
}
