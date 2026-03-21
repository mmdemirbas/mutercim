package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mutercim.yaml")
	os.WriteFile(path, []byte("invalid: yaml: [broken"), 0644)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoad_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mutercim.yaml")
	os.WriteFile(path, []byte(""), 0644)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for empty file (source_langs required)")
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
	os.WriteFile(path, []byte("book:\n  title: \"Test\"\n  source_langs: [ar]\ninputs:\n  - path: ./input\n"), 0644)

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
	os.WriteFile(path, []byte("book:\n  title: X\n  source_langs: [ar]\n"), 0644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Retry.MaxAttempts != 3 {
		t.Errorf("Retry.MaxAttempts = %d, want 3", cfg.Retry.MaxAttempts)
	}
	if cfg.DPI != 300 {
		t.Errorf("DPI = %d, want 300", cfg.DPI)
	}
}

func TestLoad_ValidateCalledWithInvalidPages(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mutercim.yaml")

	yaml := `book:
  title: "Test"
  source_langs: [ar]
inputs:
  - path: ./input
    pages: "not-a-range"
`
	os.WriteFile(path, []byte(yaml), 0644)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid pages in config")
	}
	if !strings.Contains(err.Error(), "validate") {
		t.Errorf("error should mention validation: %v", err)
	}
}

func TestLoad_ValidateCalledWithValidPages(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mutercim.yaml")

	yaml := `book:
  title: "Test"
  source_langs: [ar]
inputs:
  - path: ./input
    pages: "1-50"
`
	os.WriteFile(path, []byte(yaml), 0644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Inputs[0].Pages != "1-50" {
		t.Errorf("pages = %q", cfg.Inputs[0].Pages)
	}
}

func TestLoad_MissingSourceLangs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mutercim.yaml")
	os.WriteFile(path, []byte("book:\n  title: X\n"), 0644)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing source_langs")
	}
	if !strings.Contains(err.Error(), "source_langs") {
		t.Errorf("error should mention source_langs: %v", err)
	}
}
