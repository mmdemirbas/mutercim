package provider

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mmdemirbas/mutercim/internal/apiclient"
)

// errorProvider is a mock that returns a configurable error.
type errorProvider struct {
	name   string
	vision bool
	err    error
}

func (e *errorProvider) Name() string         { return e.name }
func (e *errorProvider) SupportsVision() bool { return e.vision }
func (e *errorProvider) ReadFromImage(ctx context.Context, image []byte, systemPrompt, userPrompt string) (string, error) {
	if e.err != nil {
		return "", e.err
	}
	return "read:" + e.name, nil
}
func (e *errorProvider) Translate(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if e.err != nil {
		return "", e.err
	}
	return "translate:" + e.name, nil
}

func quota429() error {
	return fmt.Errorf("max retries (3) exceeded: %w", &apiclient.HTTPError{StatusCode: 429, Status: "429 Too Many Requests"})
}

func badRequest400() error {
	return &apiclient.HTTPError{StatusCode: 400, Status: "400 Bad Request"}
}

func TestFailoverChain_FirstProviderSucceeds(t *testing.T) {
	chain := NewFailoverChain(
		[]Provider{
			&errorProvider{name: "primary", vision: true},
			&errorProvider{name: "secondary", vision: true},
		},
		nil, 60*time.Second, nil,
	)

	result, err := chain.Translate(context.Background(), "sys", "user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "translate:primary" {
		t.Errorf("expected primary, got %q", result)
	}
}

func TestFailoverChain_FirstExhausted_FallsToSecond(t *testing.T) {
	chain := NewFailoverChain(
		[]Provider{
			&errorProvider{name: "primary", vision: true, err: quota429()},
			&errorProvider{name: "secondary", vision: true},
		},
		nil, 60*time.Second, nil,
	)

	result, err := chain.Translate(context.Background(), "sys", "user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "translate:secondary" {
		t.Errorf("expected secondary, got %q", result)
	}
}

func TestFailoverChain_AllExhausted(t *testing.T) {
	chain := NewFailoverChain(
		[]Provider{
			&errorProvider{name: "a", vision: true, err: quota429()},
			&errorProvider{name: "b", vision: true, err: quota429()},
		},
		nil, 60*time.Second, nil,
	)

	_, err := chain.Translate(context.Background(), "sys", "user")
	if err == nil {
		t.Fatal("expected error when all exhausted")
	}
	if !strings.Contains(err.Error(), "all providers exhausted") {
		t.Errorf("expected 'all providers exhausted', got: %v", err)
	}
}

func TestFailoverChain_NonQuotaError_TriesFallover(t *testing.T) {
	chain := NewFailoverChain(
		[]Provider{
			&errorProvider{name: "primary", vision: true, err: badRequest400()},
			&errorProvider{name: "secondary", vision: true},
		},
		nil, 60*time.Second, nil,
	)

	// 400 on primary should try secondary (which succeeds with empty response)
	result, err := chain.Translate(context.Background(), "sys", "user")
	if err != nil {
		t.Fatalf("expected secondary to succeed, got error: %v", err)
	}
	_ = result
}

func TestFailoverChain_RecoveryWindow(t *testing.T) {
	now := time.Unix(1000, 0)
	chain := NewFailoverChain(
		[]Provider{
			&errorProvider{name: "primary", vision: true, err: quota429()},
			&errorProvider{name: "secondary", vision: true},
		},
		nil, 60*time.Second, nil,
	)
	chain.now = func() time.Time { return now }

	// First call: primary exhausted, falls to secondary
	result, err := chain.Translate(context.Background(), "sys", "user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "translate:secondary" {
		t.Errorf("expected secondary, got %q", result)
	}

	// Advance time past recovery window
	now = now.Add(61 * time.Second)

	// Make primary succeed now
	chain.entries[0].provider = &errorProvider{name: "primary", vision: true}

	result, err = chain.Translate(context.Background(), "sys", "user")
	if err != nil {
		t.Fatalf("unexpected error after recovery: %v", err)
	}
	if result != "translate:primary" {
		t.Errorf("expected primary after recovery, got %q", result)
	}
}

func TestFailoverChain_SkipsNonVisionForRead(t *testing.T) {
	chain := NewFailoverChain(
		[]Provider{
			&errorProvider{name: "text-only", vision: false},
			&errorProvider{name: "vision-model", vision: true},
		},
		nil, 60*time.Second, nil,
	)

	result, err := chain.ReadFromImage(context.Background(), []byte("img"), "sys", "user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "read:vision-model" {
		t.Errorf("expected vision-model, got %q", result)
	}
}

func TestFailoverChain_NonVisionUsedForTranslate(t *testing.T) {
	chain := NewFailoverChain(
		[]Provider{
			&errorProvider{name: "text-only", vision: false},
		},
		nil, 60*time.Second, nil,
	)

	result, err := chain.Translate(context.Background(), "sys", "user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "translate:text-only" {
		t.Errorf("expected text-only, got %q", result)
	}
}

func TestFailoverChain_Name(t *testing.T) {
	chain := NewFailoverChain(
		[]Provider{
			&errorProvider{name: "gemini"},
			&errorProvider{name: "groq"},
		},
		nil, 60*time.Second, nil,
	)
	if got := chain.Name(); got != "failover(gemini,groq)" {
		t.Errorf("Name() = %q, want %q", got, "failover(gemini,groq)")
	}
}

func TestFailoverChain_SupportsVision_AnyVision(t *testing.T) {
	chain := NewFailoverChain(
		[]Provider{
			&errorProvider{name: "text", vision: false},
			&errorProvider{name: "vision", vision: true},
		},
		nil, 60*time.Second, nil,
	)
	if !chain.SupportsVision() {
		t.Error("should support vision when any provider does")
	}
}

func TestFailoverChain_SupportsVision_NoneVision(t *testing.T) {
	chain := NewFailoverChain(
		[]Provider{
			&errorProvider{name: "a", vision: false},
			&errorProvider{name: "b", vision: false},
		},
		nil, 60*time.Second, nil,
	)
	if chain.SupportsVision() {
		t.Error("should not support vision when no provider does")
	}
}

func TestFailoverChain_SingleProvider(t *testing.T) {
	chain := NewFailoverChain(
		[]Provider{&errorProvider{name: "only", vision: true}},
		nil, 60*time.Second, nil,
	)

	result, err := chain.ReadFromImage(context.Background(), []byte("img"), "sys", "user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "read:only" {
		t.Errorf("got %q", result)
	}
}

func TestFailoverChain_ActiveProvider(t *testing.T) {
	now := time.Unix(1000, 0)
	chain := NewFailoverChain(
		[]Provider{
			&errorProvider{name: "primary", vision: true, err: quota429()},
			&errorProvider{name: "secondary", vision: true},
		},
		nil, 60*time.Second, nil,
	)
	chain.now = func() time.Time { return now }

	if got := chain.ActiveProvider(false); got != "primary" {
		t.Errorf("before call: active = %q, want primary", got)
	}

	_, _ = chain.Translate(context.Background(), "sys", "user")

	if got := chain.ActiveProvider(false); got != "secondary" {
		t.Errorf("after exhaust: active = %q, want secondary", got)
	}
}
