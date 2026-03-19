package apiclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestClientDo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	cfg := DefaultClientConfig()
	cfg.RequestsPerMinute = 100
	client := NewClient(cfg, nil)
	defer client.Close()

	body, err := client.Do(context.Background(), Request{
		Method:  "POST",
		URL:     server.URL,
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    map[string]string{"test": "data"},
	})
	if err != nil {
		t.Fatalf("Do() error: %v", err)
	}
	if len(body) == 0 {
		t.Fatal("Do() returned empty body")
	}
}

func TestClientDoRetries(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	}))
	defer server.Close()

	cfg := DefaultClientConfig()
	cfg.RequestsPerMinute = 100
	cfg.MaxRetries = 3
	cfg.BaseBackoff = 10 * time.Millisecond
	client := NewClient(cfg, nil)
	defer client.Close()

	body, err := client.Do(context.Background(), Request{
		Method: "POST",
		URL:    server.URL,
	})
	if err != nil {
		t.Fatalf("Do() error: %v", err)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
	if len(body) == 0 {
		t.Fatal("Do() returned empty body")
	}
}

func TestClientDoRetriesWithRetryAfter(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 2 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	}))
	defer server.Close()

	cfg := DefaultClientConfig()
	cfg.RequestsPerMinute = 100
	cfg.MaxRetries = 3
	cfg.BaseBackoff = 10 * time.Millisecond
	client := NewClient(cfg, nil)
	defer client.Close()

	start := time.Now()
	_, err := client.Do(context.Background(), Request{
		Method: "POST",
		URL:    server.URL,
	})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("Do() error: %v", err)
	}
	// Retry-After: 1 means 1 second backoff
	if elapsed < 900*time.Millisecond {
		t.Errorf("expected at least ~1s backoff from Retry-After, got %v", elapsed)
	}
}

func TestClientDoNonRetryable(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"bad request", http.StatusBadRequest},
		{"unauthorized", http.StatusUnauthorized},
		{"forbidden", http.StatusForbidden},
		{"not found", http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(`{"error": "test error"}`))
			}))
			defer server.Close()

			cfg := DefaultClientConfig()
			cfg.RequestsPerMinute = 100
			cfg.MaxRetries = 3
			client := NewClient(cfg, nil)
			defer client.Close()

			_, err := client.Do(context.Background(), Request{
				Method: "POST",
				URL:    server.URL,
			})
			if err == nil {
				t.Fatalf("Do() expected error for %d response", tt.statusCode)
			}
			var httpErr *HTTPError
			if !errors.As(err, &httpErr) {
				t.Fatalf("expected HTTPError, got %T: %v", err, err)
			}
			if httpErr.StatusCode != tt.statusCode {
				t.Errorf("expected status %d, got %d", tt.statusCode, httpErr.StatusCode)
			}
		})
	}
}

func TestDoJSON(t *testing.T) {
	type testResponse struct {
		Status string `json:"status"`
		Count  int    `json:"count"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(testResponse{Status: "ok", Count: 42})
	}))
	defer server.Close()

	cfg := DefaultClientConfig()
	cfg.RequestsPerMinute = 100
	client := NewClient(cfg, nil)
	defer client.Close()

	resp, err := DoJSON[testResponse](client, context.Background(), Request{
		Method: "POST",
		URL:    server.URL,
	})
	if err != nil {
		t.Fatalf("DoJSON() error: %v", err)
	}
	if resp.Status != "ok" {
		t.Errorf("expected status 'ok', got %q", resp.Status)
	}
	if resp.Count != 42 {
		t.Errorf("expected count 42, got %d", resp.Count)
	}
}

func TestEncodeImageBase64(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name         string
		fileName     string
		content      []byte
		expectedMIME string
	}{
		{
			name:         "PNG file",
			fileName:     "test.png",
			content:      []byte{0x89, 0x50, 0x4E, 0x47},
			expectedMIME: "image/png",
		},
		{
			name:         "JPEG file",
			fileName:     "test.jpg",
			content:      []byte{0xFF, 0xD8, 0xFF, 0xE0},
			expectedMIME: "image/jpeg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			imgPath := filepath.Join(tmpDir, tt.fileName)
			if err := os.WriteFile(imgPath, tt.content, 0644); err != nil {
				t.Fatalf("write test file: %v", err)
			}

			data, mimeType, err := EncodeImageBase64(imgPath)
			if err != nil {
				t.Fatalf("EncodeImageBase64() error: %v", err)
			}
			if data == "" {
				t.Fatal("EncodeImageBase64() returned empty data")
			}
			if mimeType != tt.expectedMIME {
				t.Errorf("expected mime type %q, got %q", tt.expectedMIME, mimeType)
			}
		})
	}
}

func TestEncodeImageBase64FileNotFound(t *testing.T) {
	_, _, err := EncodeImageBase64("/nonexistent/file.png")
	if err == nil {
		t.Fatal("EncodeImageBase64() expected error for missing file")
	}
}

func TestCalculateBackoff_ExponentialGrowth(t *testing.T) {
	cfg := DefaultClientConfig()
	cfg.BaseBackoff = 2 * time.Second
	cfg.RequestsPerMinute = 100
	client := NewClient(cfg, nil)
	defer client.Close()

	tests := []struct {
		attempt  int
		wantBase time.Duration
	}{
		{1, 2 * time.Second},
		{2, 4 * time.Second},
		{3, 8 * time.Second},
	}

	for _, tt := range tests {
		backoff := client.calculateBackoff(tt.attempt, errors.New("server error"))
		// With jitter (0.5x to 1.5x), backoff should be in [base*0.5, base*1.5]
		low := time.Duration(float64(tt.wantBase) * 0.5)
		high := time.Duration(float64(tt.wantBase) * 1.5)
		if backoff < low || backoff > high {
			t.Errorf("attempt %d: backoff %v not in [%v, %v]", tt.attempt, backoff, low, high)
		}
	}
}

func TestCalculateBackoff_RetryAfterRespected(t *testing.T) {
	cfg := DefaultClientConfig()
	cfg.RequestsPerMinute = 100
	client := NewClient(cfg, nil)
	defer client.Close()

	// Retry-After of 5s should be respected as-is
	err := &HTTPError{StatusCode: 429, RetryAfter: 5 * time.Second}
	backoff := client.calculateBackoff(1, err)
	if backoff != 5*time.Second {
		t.Errorf("expected 5s from Retry-After, got %v", backoff)
	}
}

func TestCalculateBackoff_RetryAfterCapped(t *testing.T) {
	cfg := DefaultClientConfig()
	cfg.RequestsPerMinute = 100
	client := NewClient(cfg, nil)
	defer client.Close()

	// Retry-After of 60s should be capped to 30s
	err := &HTTPError{StatusCode: 429, RetryAfter: 60 * time.Second}
	backoff := client.calculateBackoff(1, err)
	if backoff != 30*time.Second {
		t.Errorf("expected 30s cap, got %v", backoff)
	}

	// Retry-After of 120s should also be capped
	err = &HTTPError{StatusCode: 503, RetryAfter: 120 * time.Second}
	backoff = client.calculateBackoff(1, err)
	if backoff != 30*time.Second {
		t.Errorf("expected 30s cap for 120s Retry-After, got %v", backoff)
	}
}

func TestCalculateBackoff_429WithoutRetryAfter_UsesExponential(t *testing.T) {
	cfg := DefaultClientConfig()
	cfg.BaseBackoff = 2 * time.Second
	cfg.RequestsPerMinute = 100
	client := NewClient(cfg, nil)
	defer client.Close()

	// 429 without Retry-After should use standard exponential backoff, not 60s minimum
	err := &HTTPError{StatusCode: 429}
	backoff := client.calculateBackoff(1, err)
	// Should be ~2s (with jitter 0.5-1.5x), NOT 30-90s
	if backoff > 4*time.Second {
		t.Errorf("429 without Retry-After should use exponential backoff (~2s), got %v", backoff)
	}
}

func TestRedactURL_Gemini(t *testing.T) {
	raw := "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash-lite:generateContent?key=AIza_secret123"
	got := RedactURL(raw)

	if !strings.Contains(got, "key=REDACTED") {
		t.Errorf("Gemini URL key should be redacted, got: %s", got)
	}
	if strings.Contains(got, "AIza_secret123") {
		t.Errorf("API key should not appear in redacted URL, got: %s", got)
	}
	// Should preserve the rest of the URL
	if !strings.Contains(got, "generativelanguage.googleapis.com") {
		t.Errorf("should preserve host, got: %s", got)
	}
}

func TestRedactURL_Claude(t *testing.T) {
	// Claude uses Authorization header, not URL params. URL should be unchanged.
	raw := "https://api.anthropic.com/v1/messages"
	got := RedactURL(raw)
	if got != raw {
		t.Errorf("Claude URL (no query params) should be unchanged, got: %s", got)
	}
}

func TestRedactURL_OpenAI(t *testing.T) {
	// OpenAI uses Authorization header, not URL params. URL should be unchanged.
	raw := "https://api.openai.com/v1/chat/completions"
	got := RedactURL(raw)
	if got != raw {
		t.Errorf("OpenAI URL (no query params) should be unchanged, got: %s", got)
	}
}

func TestRedactURL_MultipleParams(t *testing.T) {
	raw := "https://example.com/api?key=secret1&model=test&api_key=secret2&apikey=secret3"
	got := RedactURL(raw)

	if strings.Contains(got, "secret1") || strings.Contains(got, "secret2") || strings.Contains(got, "secret3") {
		t.Errorf("all key params should be redacted, got: %s", got)
	}
	if !strings.Contains(got, "model=test") {
		t.Errorf("non-sensitive params should be preserved, got: %s", got)
	}
}

func TestRedactURL_TokenAndSecret(t *testing.T) {
	raw := "https://example.com/api?access_token=abc123&client_secret=xyz789&format=json"
	got := RedactURL(raw)

	if strings.Contains(got, "abc123") || strings.Contains(got, "xyz789") {
		t.Errorf("token/secret params should be redacted, got: %s", got)
	}
	if !strings.Contains(got, "format=json") {
		t.Errorf("non-sensitive params should be preserved, got: %s", got)
	}
}

func TestRedactURL_InvalidURL(t *testing.T) {
	raw := "not a valid url %%"
	got := RedactURL(raw)
	if got != raw {
		t.Errorf("invalid URL should be returned as-is, got: %s", got)
	}
}
