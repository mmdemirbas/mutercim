package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mutercim.yaml")

	if err := os.WriteFile(path, []byte("invalid: yaml: [broken"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoad_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mutercim.yaml")

	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// Should get defaults
	if cfg.Book.PrimarySourceLang() != "ar" {
		t.Errorf("expected default source lang 'ar', got %q", cfg.Book.PrimarySourceLang())
	}
}

func TestLoad_NonexistentFile(t *testing.T) {
	_, err := Load("/nonexistent/mutercim.yaml")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestLoad_MinimalValidConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mutercim.yaml")

	yaml := `book:
  title: "Test"
inputs:
  - path: ./input
`
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Book.Title != "Test" {
		t.Errorf("title = %q, want %q", cfg.Book.Title, "Test")
	}
}

func TestLoad_DefaultsApplied(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mutercim.yaml")

	// Minimal config, rely on defaults
	if err := os.WriteFile(path, []byte("book:\n  title: X\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Retry.MaxAttempts != 3 {
		t.Errorf("Retry.MaxAttempts = %d, want 3", cfg.Retry.MaxAttempts)
	}
	if cfg.Retry.BackoffSeconds != 2 {
		t.Errorf("Retry.BackoffSeconds = %d, want 2", cfg.Retry.BackoffSeconds)
	}
	if cfg.DPI != 300 {
		t.Errorf("DPI = %d, want 300", cfg.DPI)
	}
}
