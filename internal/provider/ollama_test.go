package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mmdemirbas/mutercim/internal/apiclient"
)

func newTestOllamaProvider(t *testing.T, serverURL string) *OllamaProvider {
	t.Helper()
	cfg := apiclient.DefaultClientConfig()
	cfg.RequestsPerMinute = 100
	client := apiclient.NewClient(cfg, nil)
	t.Cleanup(client.Close)

	return &OllamaProvider{
		client:  client,
		model:   "qwen2.5vl:7b",
		baseURL: serverURL,
	}
}

func TestOllamaProviderReadFromImage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify endpoint
		if r.URL.Path != "/api/chat" {
			t.Errorf("expected /api/chat, got %s", r.URL.Path)
		}

		var req ollamaChatRequest
		_ = json.NewDecoder(r.Body).Decode(&req)

		// Verify message structure
		if len(req.Messages) < 2 {
			t.Fatalf("expected at least 2 messages, got %d", len(req.Messages))
		}
		if req.Messages[0].Role != "system" {
			t.Errorf("expected system message first, got %s", req.Messages[0].Role)
		}
		if req.Messages[1].Role != "user" {
			t.Errorf("expected user message second, got %s", req.Messages[1].Role)
		}
		// Images should be a sibling of content in the user message
		if len(req.Messages[1].Images) == 0 {
			t.Error("expected images in user message")
		}
		if req.Stream {
			t.Error("expected stream=false")
		}

		resp := ollamaChatResponse{
			Message: ollamaMessage{Role: "assistant", Content: `{"page_number": 1}`},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := newTestOllamaProvider(t, server.URL)
	result, err := p.ReadFromImage(context.Background(), []byte("image"), "system", "user")
	if err != nil {
		t.Fatalf("ReadFromImage() error: %v", err)
	}
	if result != `{"page_number": 1}` {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestOllamaProviderTranslate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Errorf("expected /api/chat, got %s", r.URL.Path)
		}

		var req ollamaChatRequest
		_ = json.NewDecoder(r.Body).Decode(&req)

		// Translation should not have images
		for _, msg := range req.Messages {
			if len(msg.Images) != 0 {
				t.Error("translation should not have images")
			}
		}

		resp := ollamaChatResponse{
			Message: ollamaMessage{Role: "assistant", Content: `{"translated_entries": []}`},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := newTestOllamaProvider(t, server.URL)
	result, err := p.Translate(context.Background(), "system", "user")
	if err != nil {
		t.Fatalf("Translate() error: %v", err)
	}
	if result != `{"translated_entries": []}` {
		t.Errorf("unexpected result: %q", result)
	}
}
