package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mmdemirbas/mutercim/internal/apiclient"
)

func newTestGeminiProvider(t *testing.T, serverURL string) *GeminiProvider {
	t.Helper()
	cfg := apiclient.DefaultClientConfig()
	cfg.RequestsPerMinute = 100
	client := apiclient.NewClient(cfg, nil)
	t.Cleanup(client.Close)

	p := NewGeminiProvider(client, "test-key", "test-model")
	p.baseURL = serverURL
	return p
}

func TestGeminiProviderName(t *testing.T) {
	p := &GeminiProvider{}
	if p.Name() != "gemini" {
		t.Errorf("expected 'gemini', got %q", p.Name())
	}
}

func TestGeminiProviderSupportsVision(t *testing.T) {
	p := &GeminiProvider{}
	if !p.SupportsVision() {
		t.Error("expected SupportsVision() = true")
	}
}

func TestGeminiProviderExtractFromImage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request structure
		var req geminiRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if len(req.Contents) == 0 || len(req.Contents[0].Parts) < 2 {
			t.Error("expected image and text parts")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if req.Contents[0].Parts[0].InlineData == nil {
			t.Error("expected inline data for image")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if req.SystemInstruction == nil {
			t.Error("expected system instruction")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if req.GenerationConfig.Temperature != 0 {
			t.Errorf("expected temperature 0, got %f", req.GenerationConfig.Temperature)
		}

		if req.GenerationConfig.ResponseMIMEType != "application/json" {
			t.Errorf("expected responseMimeType 'application/json', got %q", req.GenerationConfig.ResponseMIMEType)
		}

		resp := geminiResponse{
			Candidates: []geminiCandidate{{
				Content: geminiContent{
					Parts: []geminiPart{{Text: `{"page_number": 1, "entries": []}`}},
				},
			}},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := newTestGeminiProvider(t, server.URL)

	result, err := p.ExtractFromImage(context.Background(), []byte("fake-image"), "system prompt", "user prompt")
	if err != nil {
		t.Fatalf("ExtractFromImage() error: %v", err)
	}
	if result != `{"page_number": 1, "entries": []}` {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestGeminiProviderTranslate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req geminiRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Translation should NOT have inline data
		if len(req.Contents) == 0 || len(req.Contents[0].Parts) == 0 {
			t.Error("expected text parts")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if req.Contents[0].Parts[0].InlineData != nil {
			t.Error("translation should not have inline data")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		resp := geminiResponse{
			Candidates: []geminiCandidate{{
				Content: geminiContent{
					Parts: []geminiPart{{Text: `{"translated_entries": []}`}},
				},
			}},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := newTestGeminiProvider(t, server.URL)

	result, err := p.Translate(context.Background(), "system prompt", "user prompt")
	if err != nil {
		t.Fatalf("Translate() error: %v", err)
	}
	if result != `{"translated_entries": []}` {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestGeminiProviderEmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := geminiResponse{
			Candidates: []geminiCandidate{},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := newTestGeminiProvider(t, server.URL)

	_, err := p.ExtractFromImage(context.Background(), []byte("fake-image"), "system", "user")
	if err == nil {
		t.Fatal("expected error for empty response")
	}
}

func TestGeminiProviderHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "invalid api key"}`))
	}))
	defer server.Close()

	cfg := apiclient.DefaultClientConfig()
	cfg.RequestsPerMinute = 100
	cfg.MaxRetries = 0 // No retries for faster test
	client := apiclient.NewClient(cfg, nil)
	defer client.Close()

	p := NewGeminiProvider(client, "bad-key", "test-model")
	p.baseURL = server.URL

	_, err := p.ExtractFromImage(context.Background(), []byte("fake-image"), "system", "user")
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
}
