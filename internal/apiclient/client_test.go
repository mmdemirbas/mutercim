package apiclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
