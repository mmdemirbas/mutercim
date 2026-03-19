package apiclient

import (
	"context"
	"testing"
	"time"
)

func TestRateLimiterFirstRequestImmediate(t *testing.T) {
	rl := NewRateLimiter(60) // 60 RPM
	defer rl.Close()

	// First request should succeed immediately (1 initial token)
	ctx := context.Background()
	if err := rl.Wait(ctx); err != nil {
		t.Fatalf("Wait() returned error: %v", err)
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

	// Consume the initial token
	ctx := context.Background()
	if err := rl.Wait(ctx); err != nil {
		t.Fatalf("Wait() error consuming initial token: %v", err)
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

func TestRateLimiterPacing(t *testing.T) {
	// 600 RPM = 1 per 100ms
	rl := NewRateLimiter(600)
	defer rl.Close()

	ctx := context.Background()

	// First request immediate
	start := time.Now()
	if err := rl.Wait(ctx); err != nil {
		t.Fatalf("Wait() error: %v", err)
	}

	// Second request should wait ~100ms for refill
	if err := rl.Wait(ctx); err != nil {
		t.Fatalf("Wait() error: %v", err)
	}
	elapsed := time.Since(start)

	if elapsed < 80*time.Millisecond {
		t.Errorf("expected pacing delay, but only %v elapsed", elapsed)
	}
}
