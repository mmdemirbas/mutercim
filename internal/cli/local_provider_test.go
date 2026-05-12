package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mmdemirbas/mutercim/internal/config"
)

// TestLocalProvider_RunsWithoutAPIKey verifies that llamacpp and mlx
// providers can be added to a failover chain without an API key set,
// and that they run under allow_cloud=false. This is the core Phase 3
// promise: a workspace with no cloud keys can still translate.
func TestLocalProvider_RunsWithoutAPIKey(t *testing.T) {
	for _, name := range []string{"llamacpp", "mlx"} {
		t.Run(name, func(t *testing.T) {
			unsetEnv(t, strings.ToUpper(name)+"_API_KEY")
			models := []config.ModelSpec{
				{Provider: name, Model: "test-model"},
			}
			chain, err := createProviderChain(models, config.RetryConfig{MaxAttempts: 1, BackoffSeconds: 1}, false, silentLogger())
			if err != nil {
				t.Fatalf("local provider %q should build under allow_cloud=false without a key, got: %v", name, err)
			}
			defer chain.Close()
			if !strings.Contains(chain.Name(), name) {
				t.Errorf("chain should contain %q, got %q", name, chain.Name())
			}
		})
	}
}

// TestLocalProvider_NoAuthHeader verifies that requests to a local provider
// don't include an Authorization header (since llama-server / mlx_lm.server
// don't authenticate by default and shouldn't see "Bearer " with empty key).
func TestLocalProvider_NoAuthHeader(t *testing.T) {
	gotAuth := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Capture and drain.
		_, _ = io.Copy(io.Discard, r.Body)
		gotAuth <- r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"content": "ok"}},
			},
		})
	}))
	defer srv.Close()

	models := []config.ModelSpec{
		{Provider: "llamacpp", Model: "test", BaseURL: srv.URL},
	}
	chain, err := createProviderChain(models, config.RetryConfig{MaxAttempts: 1, BackoffSeconds: 1}, false, silentLogger())
	if err != nil {
		t.Fatalf("build chain: %v", err)
	}
	defer chain.Close()

	if _, err := chain.Translate(t.Context(), "sys", "user"); err != nil {
		t.Fatalf("translate: %v", err)
	}
	auth := <-gotAuth
	if auth != "" {
		t.Errorf("expected no Authorization header for local provider, got %q", auth)
	}
}
