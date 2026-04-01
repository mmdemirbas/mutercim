package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mmdemirbas/mutercim/internal/apiclient"
)

func newTestClaudeProvider(t *testing.T, serverURL string) *ClaudeProvider {
	t.Helper()
	cfg := apiclient.DefaultClientConfig()
	cfg.RequestsPerMinute = 100
	client := apiclient.NewClient(cfg, nil)
	t.Cleanup(client.Close)

	p := NewClaudeProvider(client, "test-key", "test-model")
	p.baseURL = serverURL
	return p
}

func TestClaudeProviderReadFromImage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify headers
		if r.Header.Get("x-api-key") != "test-key" {
			t.Error("expected x-api-key header")
		}
		if r.Header.Get("anthropic-version") != "2023-06-01" {
			t.Error("expected anthropic-version header")
		}

		resp := claudeResponse{
			Content: []claudeResponseContent{{Text: `{"page_number": 1}`}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := newTestClaudeProvider(t, server.URL)
	result, err := p.ReadFromImage(context.Background(), []byte("image"), "system", "user")
	if err != nil {
		t.Fatalf("ReadFromImage() error: %v", err)
	}
	if result != `{"page_number": 1}` {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestClaudeProviderTranslate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := claudeResponse{
			Content: []claudeResponseContent{{Text: `{"translated_entries": []}`}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := newTestClaudeProvider(t, server.URL)
	result, err := p.Translate(context.Background(), "system", "user")
	if err != nil {
		t.Fatalf("Translate() error: %v", err)
	}
	if result != `{"translated_entries": []}` {
		t.Errorf("unexpected result: %q", result)
	}
}
