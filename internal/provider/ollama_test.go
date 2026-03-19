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
		model:   "qwen2.5-vl:7b",
		baseURL: serverURL,
	}
}

func TestOllamaProviderExtractFromImage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ollamaRequest
		json.NewDecoder(r.Body).Decode(&req)

		if len(req.Images) == 0 {
			t.Error("expected images in request")
		}
		if req.Stream {
			t.Error("expected stream=false")
		}

		resp := ollamaResponse{Response: `{"page_number": 1}`}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := newTestOllamaProvider(t, server.URL)
	result, err := p.ExtractFromImage(context.Background(), []byte("image"), "system", "user")
	if err != nil {
		t.Fatalf("ExtractFromImage() error: %v", err)
	}
	if result != `{"page_number": 1}` {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestOllamaProviderTranslate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ollamaRequest
		json.NewDecoder(r.Body).Decode(&req)

		if len(req.Images) != 0 {
			t.Error("translation should not have images")
		}

		resp := ollamaResponse{Response: `{"translated_entries": []}`}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
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
