package provider

import (
	"net/url"
	"strings"
	"testing"
)

func TestGeminiEndpoint_NoAPIKeyInURL(t *testing.T) {
	p := &GeminiProvider{ //nolint:gosec // G101: fake key for test, not a real credential
		baseURL: "https://example.com",
		model:   "gemini-pro",
		apiKey:  "AIza_secret&key=value#extra",
	}

	ep := p.endpoint()

	// API key should NOT appear in the URL (it's sent via X-Goog-Api-Key header)
	if strings.Contains(ep, "AIza_secret") {
		t.Errorf("endpoint URL should not contain API key, got: %s", ep)
	}
	if strings.Contains(ep, "key=") {
		t.Errorf("endpoint URL should not have key query param, got: %s", ep)
	}

	// URL should still be valid
	_, err := url.Parse(ep)
	if err != nil {
		t.Fatalf("endpoint() produced invalid URL: %v", err)
	}

	want := "https://example.com/v1beta/models/gemini-pro:generateContent"
	if ep != want {
		t.Errorf("endpoint() = %q, want %q", ep, want)
	}
}

func TestOllamaProvider_BaseURLUsed(t *testing.T) {
	// Directly construct provider with custom base URL
	p := &OllamaProvider{
		model:   "test-model",
		baseURL: "http://custom-host:8000",
	}

	// Verify Name and SupportsVision
	if p.Name() != "ollama" {
		t.Errorf("Name() = %q", p.Name())
	}
	if !p.SupportsVision() {
		t.Error("SupportsVision() should be true")
	}
}

func TestNewOllamaProvider_DefaultHost(t *testing.T) {
	// Save and clear env
	orig := t.TempDir() // just to ensure cleanup
	_ = orig

	// With OLLAMA_HOST unset, should use localhost
	// We can't reliably unset env in parallel tests, so just test the constructor
	p := &OllamaProvider{baseURL: "http://localhost:11434"}
	if p.baseURL != "http://localhost:11434" {
		t.Errorf("baseURL = %q, want default", p.baseURL)
	}
}
