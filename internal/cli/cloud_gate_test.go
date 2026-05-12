package cli

import (
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/mmdemirbas/mutercim/internal/config"
)

// silentLogger returns a logger that discards all output. Tests don't need
// to see warn-line side output, only assert behavior on returned values.
func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}

// withEnv sets an env var for the duration of the test and restores
// (or unsets) the prior value on cleanup.
func withEnv(t *testing.T, key, value string) {
	t.Helper()
	prev, hadPrev := os.LookupEnv(key)
	if err := os.Setenv(key, value); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	t.Cleanup(func() {
		if hadPrev {
			_ = os.Setenv(key, prev)
		} else {
			_ = os.Unsetenv(key)
		}
	})
}

// unsetEnv clears an env var for the duration of the test and restores
// the prior value on cleanup.
func unsetEnv(t *testing.T, key string) {
	t.Helper()
	prev, hadPrev := os.LookupEnv(key)
	_ = os.Unsetenv(key)
	t.Cleanup(func() {
		if hadPrev {
			_ = os.Setenv(key, prev)
		}
	})
}

func TestCreateProviderChain_BlocksCloudWhenAllowCloudFalse(t *testing.T) {
	// Set a key so the cloud filter (not the key check) is what blocks.
	withEnv(t, "GEMINI_API_KEY", "test-key")

	models := []config.ModelSpec{
		{Provider: "gemini", Model: "gemini-2.0-flash"},
	}
	chain, err := createProviderChain(models, config.RetryConfig{MaxAttempts: 1, BackoffSeconds: 1}, false, silentLogger())
	if err == nil {
		if chain != nil {
			chain.Close()
		}
		t.Fatal("expected error when only-cloud chain is built with allow_cloud=false, got nil")
	}
	if !strings.Contains(err.Error(), "allow_cloud") {
		t.Errorf("error should mention allow_cloud, got: %v", err)
	}
	if !strings.Contains(err.Error(), "ollama") {
		t.Errorf("error should suggest local provider (ollama), got: %v", err)
	}
}

func TestCreateProviderChain_AllowsCloudWhenAllowCloudTrue(t *testing.T) {
	withEnv(t, "GEMINI_API_KEY", "test-key")

	models := []config.ModelSpec{
		{Provider: "gemini", Model: "gemini-2.0-flash"},
	}
	chain, err := createProviderChain(models, config.RetryConfig{MaxAttempts: 1, BackoffSeconds: 1}, true, silentLogger())
	if err != nil {
		t.Fatalf("chain build with allow_cloud=true and key set should succeed, got: %v", err)
	}
	defer chain.Close()
	if chain.Name() == "" {
		t.Error("chain.Name() should be non-empty")
	}
}

func TestCreateProviderChain_LocalProviderAlwaysAllowed(t *testing.T) {
	// Ollama is local; should be in chain even with allow_cloud=false.
	models := []config.ModelSpec{
		{Provider: "ollama", Model: "qwen3:14b"},
	}
	chain, err := createProviderChain(models, config.RetryConfig{MaxAttempts: 1, BackoffSeconds: 1}, false, silentLogger())
	if err != nil {
		t.Fatalf("local provider should run with allow_cloud=false, got: %v", err)
	}
	defer chain.Close()
}

func TestCreateProviderChain_UnknownProviderTreatedAsCloud(t *testing.T) {
	// Custom unknown provider — under default-deny we must filter it
	// when allow_cloud=false, NOT silently let it through.
	withEnv(t, "MYWEIRD_API_KEY", "test-key")

	models := []config.ModelSpec{
		{Provider: "myweird", Model: "x"},
	}
	_, err := createProviderChain(models, config.RetryConfig{MaxAttempts: 1, BackoffSeconds: 1}, false, silentLogger())
	if err == nil {
		t.Fatal("unknown-class provider must be filtered under allow_cloud=false")
	}
}

func TestCreateProviderChain_MixedChain_KeepsOnlyLocalWhenBlocked(t *testing.T) {
	// Chain has [cloud, local]. With allow_cloud=false the cloud entry is
	// filtered; the chain still builds with just the local entry.
	withEnv(t, "GEMINI_API_KEY", "test-key")

	models := []config.ModelSpec{
		{Provider: "gemini", Model: "gemini-2.0-flash"},
		{Provider: "ollama", Model: "qwen3:14b"},
	}
	chain, err := createProviderChain(models, config.RetryConfig{MaxAttempts: 1, BackoffSeconds: 1}, false, silentLogger())
	if err != nil {
		t.Fatalf("chain should build with local fallback even when cloud is blocked, got: %v", err)
	}
	defer chain.Close()
	// Chain name should list the local provider only (gemini was filtered).
	if !strings.Contains(chain.Name(), "ollama") {
		t.Errorf("chain should contain ollama, got: %s", chain.Name())
	}
	if strings.Contains(chain.Name(), "gemini") {
		t.Errorf("gemini should have been filtered, got: %s", chain.Name())
	}
}

func TestCreateProviderChain_CloudKeyMissing_StillReportsCloudBlock(t *testing.T) {
	// Cloud filter runs BEFORE the API-key check; missing key shouldn't
	// shadow the local-first error.
	unsetEnv(t, "GEMINI_API_KEY")
	unsetEnv(t, "GOOGLE_API_KEY")

	models := []config.ModelSpec{
		{Provider: "gemini", Model: "gemini-2.0-flash"},
	}
	_, err := createProviderChain(models, config.RetryConfig{MaxAttempts: 1, BackoffSeconds: 1}, false, silentLogger())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "allow_cloud") {
		t.Errorf("with allow_cloud=false the cloud-block error should win over missing-key, got: %v", err)
	}
}

func TestModelLabels_AnnotatesCloudWhenBlocked(t *testing.T) {
	models := []config.ModelSpec{
		{Provider: "gemini", Model: "gemini-2.0-flash"},
		{Provider: "ollama", Model: "qwen3:14b"},
		{Provider: "groq", Model: "llama-3.3-70b"},
	}
	labels := modelLabels(models, false)
	if len(labels) != 3 {
		t.Fatalf("expected 3 labels, got %d", len(labels))
	}
	if !strings.Contains(labels[0], "gemini/gemini-2.0-flash") || !strings.Contains(labels[0], "blocked") {
		t.Errorf("gemini label should be marked blocked, got: %s", labels[0])
	}
	if strings.Contains(labels[1], "blocked") {
		t.Errorf("ollama label should not be marked blocked, got: %s", labels[1])
	}
	if !strings.Contains(labels[2], "groq/llama-3.3-70b") || !strings.Contains(labels[2], "blocked") {
		t.Errorf("groq label should be marked blocked, got: %s", labels[2])
	}
}

func TestModelLabels_NoAnnotationWhenCloudAllowed(t *testing.T) {
	models := []config.ModelSpec{
		{Provider: "gemini", Model: "gemini-2.0-flash"},
		{Provider: "ollama", Model: "qwen3:14b"},
	}
	labels := modelLabels(models, true)
	for _, l := range labels {
		if strings.Contains(l, "blocked") {
			t.Errorf("no label should be marked blocked when allow_cloud=true, got: %s", l)
		}
	}
}

// errCheck is a tiny sentinel-check helper so the tests above can stay terse.
var _ = errors.Is // keep errors import used if tests below grow
