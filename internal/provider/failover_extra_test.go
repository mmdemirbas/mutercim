package provider

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mmdemirbas/mutercim/internal/apiclient"
)

func TestFailoverChain_NoVisionProviders_ReadReturnsError(t *testing.T) {
	chain := NewFailoverChain(
		[]Provider{
			&errorProvider{name: "text1", vision: false},
			&errorProvider{name: "text2", vision: false},
		},
		nil, 60*time.Second, nil,
	)

	_, err := chain.ReadFromImage(context.Background(), []byte("img"), "sys", "user")
	if err == nil {
		t.Fatal("expected error when no vision providers")
	}
	if !strings.Contains(err.Error(), "no eligible providers") {
		t.Errorf("expected 'no eligible providers', got: %v", err)
	}
}

func TestFailoverChain_OnFailoverCallback(t *testing.T) {
	var calledFrom, calledTo string
	chain := NewFailoverChain(
		[]Provider{
			&errorProvider{name: "primary", vision: true, err: quota429()},
			&errorProvider{name: "secondary", vision: true},
		},
		nil, 60*time.Second, nil,
	)
	chain.OnFailover = func(from, to string) {
		calledFrom = from
		calledTo = to
	}

	_, err := chain.Translate(context.Background(), "sys", "user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if calledFrom != "primary" || calledTo != "secondary" {
		t.Errorf("OnFailover(%q, %q), want (primary, secondary)", calledFrom, calledTo)
	}
}

func TestFailoverChain_OnFailoverNotCalledForLastProvider(t *testing.T) {
	called := false
	chain := NewFailoverChain(
		[]Provider{
			&errorProvider{name: "only", vision: true, err: quota429()},
		},
		nil, 60*time.Second, nil,
	)
	chain.OnFailover = func(from, to string) {
		called = true
	}

	_, err := chain.Translate(context.Background(), "sys", "user")
	if err == nil {
		t.Fatal("expected error when single provider exhausted")
	}
	if called {
		t.Error("OnFailover should not be called when there's no next provider")
	}
}

func TestFailoverChain_SetRetryCallback_NilClients(t *testing.T) {
	chain := NewFailoverChain(
		[]Provider{
			&errorProvider{name: "a", vision: true},
		},
		nil, // no clients
		60*time.Second, nil,
	)

	// Should not panic
	chain.SetRetryCallback(func(attempt, maxRetries, statusCode int, backoff time.Duration) {})
}

func TestFailoverChain_Close_NilClients(t *testing.T) {
	chain := NewFailoverChain(
		[]Provider{
			&errorProvider{name: "a", vision: true},
		},
		nil,
		60*time.Second, nil,
	)

	// Should not panic
	chain.Close()
}

func TestFailoverChain_MixedVisionAndExhaustion(t *testing.T) {
	// text-only, vision1 (429), vision2 (ok)
	chain := NewFailoverChain(
		[]Provider{
			&errorProvider{name: "text-only", vision: false},
			&errorProvider{name: "vision1", vision: true, err: quota429()},
			&errorProvider{name: "vision2", vision: true},
		},
		nil, 60*time.Second, nil,
	)

	result, err := chain.ReadFromImage(context.Background(), []byte("img"), "sys", "user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "read:vision2" {
		t.Errorf("expected vision2, got %q", result)
	}
}

func TestFailoverChain_ActiveProvider_NoEligible(t *testing.T) {
	now := time.Unix(1000, 0)
	chain := NewFailoverChain(
		[]Provider{
			&errorProvider{name: "primary", vision: true, err: quota429()},
		},
		nil, 60*time.Second, nil,
	)
	chain.now = func() time.Time { return now }

	// Exhaust the only provider
	chain.Translate(context.Background(), "sys", "user")

	if got := chain.ActiveProvider(false); got != "" {
		t.Errorf("expected empty string when all exhausted, got %q", got)
	}
}

func TestIsQuotaError_Wrapped(t *testing.T) {
	base := &apiclient.HTTPError{StatusCode: 429, Status: "429 Too Many Requests"}
	wrapped := fmt.Errorf("layer1: %w", fmt.Errorf("layer2: %w", base))

	if !isQuotaError(wrapped) {
		t.Error("should detect 429 through multiple wrapping layers")
	}
}

func TestIsQuotaError_NonHTTPError(t *testing.T) {
	err := fmt.Errorf("network timeout")
	if isQuotaError(err) {
		t.Error("network error should not be a quota error")
	}
}

func TestIsQuotaError_Non429HTTPError(t *testing.T) {
	err := &apiclient.HTTPError{StatusCode: 500, Status: "500 Internal Server Error"}
	if isQuotaError(err) {
		t.Error("500 should not be a quota error")
	}
}

func TestFailoverChain_SecondCallUsesRecoveredProvider(t *testing.T) {
	now := time.Unix(1000, 0)
	callCount := 0

	// Provider that fails on first call, succeeds on second
	dynamic := &errorProvider{name: "primary", vision: true, err: quota429()}

	chain := NewFailoverChain(
		[]Provider{
			dynamic,
			&errorProvider{name: "secondary", vision: true},
		},
		nil, 10*time.Second, nil,
	)
	chain.now = func() time.Time {
		callCount++
		return now
	}

	// First call: primary exhausted → secondary
	result, _ := chain.Translate(context.Background(), "sys", "user")
	if result != "translate:secondary" {
		t.Fatalf("first call: expected secondary, got %q", result)
	}

	// Advance past recovery and fix primary
	now = now.Add(11 * time.Second)
	dynamic.err = nil

	result, _ = chain.Translate(context.Background(), "sys", "user")
	if result != "translate:primary" {
		t.Errorf("after recovery: expected primary, got %q", result)
	}
}

func TestFailoverChain_AllExhausted_AccumulatesErrors(t *testing.T) {
	chain := NewFailoverChain(
		[]Provider{
			&errorProvider{name: "alpha", vision: true, err: quota429()},
			&errorProvider{name: "beta", vision: true, err: quota429()},
		},
		nil, 60*time.Second, nil,
	)

	_, err := chain.Translate(context.Background(), "sys", "user")
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	// Should contain both provider names in the accumulated error
	if !strings.Contains(msg, "alpha") {
		t.Errorf("error should mention alpha: %v", err)
	}
	if !strings.Contains(msg, "beta") {
		t.Errorf("error should mention beta: %v", err)
	}
	if !strings.Contains(msg, "all providers exhausted") {
		t.Errorf("error should say 'all providers exhausted': %v", err)
	}
}
