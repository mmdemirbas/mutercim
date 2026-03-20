package apiclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientDo_InvalidRetryAfter(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 2 {
			w.Header().Set("Retry-After", "not-a-number")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	cfg := DefaultClientConfig()
	cfg.RequestsPerMinute = 100
	cfg.MaxRetries = 3
	cfg.BaseBackoff = 10 * time.Millisecond
	client := NewClient(cfg, nil)
	defer client.Close()

	// Should succeed — invalid Retry-After falls back to exponential backoff
	_, err := client.Do(context.Background(), Request{
		Method: "POST",
		URL:    server.URL,
	})
	if err != nil {
		t.Fatalf("Do() error: %v", err)
	}
	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}

func TestClientDo_MarshalFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := DefaultClientConfig()
	cfg.RequestsPerMinute = 100
	client := NewClient(cfg, nil)
	defer client.Close()

	// Channel is not JSON-serializable
	_, err := client.Do(context.Background(), Request{
		Method: "POST",
		URL:    server.URL,
		Body:   make(chan int),
	})
	if err == nil {
		t.Fatal("expected error for non-marshalable body")
	}
}

func TestClientDo_ContextCancelledDuringBackoff(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	cfg := DefaultClientConfig()
	cfg.RequestsPerMinute = 100
	cfg.MaxRetries = 5
	cfg.BaseBackoff = 5 * time.Second // long backoff
	client := NewClient(cfg, nil)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_, err := client.Do(ctx, Request{
		Method: "POST",
		URL:    server.URL,
	})
	if err == nil {
		t.Fatal("expected error for cancelled context during backoff")
	}
	// Should have attempted at most 2 times (first + one retry before timeout)
	if attempts > 2 {
		t.Errorf("expected at most 2 attempts, got %d", attempts)
	}
}

func TestClientDo_OnRetryCallback(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	cfg := DefaultClientConfig()
	cfg.RequestsPerMinute = 100
	cfg.MaxRetries = 5
	cfg.BaseBackoff = 10 * time.Millisecond
	client := NewClient(cfg, nil)
	defer client.Close()

	var retryCalls []int
	client.OnRetry = func(attempt, maxRetries, statusCode int, backoff time.Duration) {
		retryCalls = append(retryCalls, statusCode)
	}

	_, err := client.Do(context.Background(), Request{
		Method: "POST",
		URL:    server.URL,
	})
	if err != nil {
		t.Fatalf("Do() error: %v", err)
	}

	// Should have called OnRetry for each retry attempt
	if len(retryCalls) != 2 {
		t.Errorf("expected 2 retry callbacks, got %d", len(retryCalls))
	}
	for _, sc := range retryCalls {
		if sc != 429 {
			t.Errorf("expected status code 429 in callback, got %d", sc)
		}
	}
}

func TestClientDo_NonRetryableReturnsImmediately(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer server.Close()

	cfg := DefaultClientConfig()
	cfg.RequestsPerMinute = 100
	cfg.MaxRetries = 5
	client := NewClient(cfg, nil)
	defer client.Close()

	_, err := client.Do(context.Background(), Request{
		Method: "POST",
		URL:    server.URL,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	// Should only attempt once for non-retryable errors
	if attempts != 1 {
		t.Errorf("expected 1 attempt for 401, got %d", attempts)
	}
}

func TestClientDo_NetworkErrorRetries(t *testing.T) {
	cfg := DefaultClientConfig()
	cfg.RequestsPerMinute = 100
	cfg.MaxRetries = 2
	cfg.BaseBackoff = 10 * time.Millisecond
	client := NewClient(cfg, nil)
	defer client.Close()

	// URL that will fail (connection refused)
	_, err := client.Do(context.Background(), Request{
		Method: "POST",
		URL:    "http://127.0.0.1:1", // port 1 should be refused
	})
	if err == nil {
		t.Fatal("expected error for connection refused")
	}
	// Error should mention max retries
	if !contains(err.Error(), "max retries") {
		t.Errorf("expected max retries error, got: %v", err)
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
