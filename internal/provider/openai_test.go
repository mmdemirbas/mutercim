package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mmdemirbas/mutercim/internal/apiclient"
)

func newTestOpenAIProvider(t *testing.T, serverURL string) *OpenAIProvider {
	t.Helper()
	cfg := apiclient.DefaultClientConfig()
	cfg.RequestsPerMinute = 100
	client := apiclient.NewClient(cfg, nil)
	t.Cleanup(client.Close)

	p := NewOpenAIProvider(client, "test-key", "gpt-4o")
	p.baseURL = serverURL
	return p
}

func TestOpenAIProviderExtractFromImage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("expected Bearer auth header")
		}

		resp := openaiResponse{
			Choices: []openaiChoice{{
				Message: openaiChoiceMessage{Content: `{"page_number": 1}`},
			}},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := newTestOpenAIProvider(t, server.URL)
	result, err := p.ExtractFromImage(context.Background(), []byte("image"), "system", "user")
	if err != nil {
		t.Fatalf("ExtractFromImage() error: %v", err)
	}
	if result != `{"page_number": 1}` {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestOpenAIProviderTranslate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openaiResponse{
			Choices: []openaiChoice{{
				Message: openaiChoiceMessage{Content: `{"translated_entries": []}`},
			}},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := newTestOpenAIProvider(t, server.URL)
	result, err := p.Translate(context.Background(), "system", "user")
	if err != nil {
		t.Fatalf("Translate() error: %v", err)
	}
	if result != `{"translated_entries": []}` {
		t.Errorf("unexpected result: %q", result)
	}
}
