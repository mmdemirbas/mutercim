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
	if err := os.WriteFile(path, []byte("invalid: yaml: [broken"), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoad_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mutercim.yaml")
	if err := os.WriteFile(path, []byte(""), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for empty file (languages required)")
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
	if err := os.WriteFile(path, []byte("inputs:\n  - path: ./input\n    languages: [ar]\n"), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.PrimarySourceLang() != "ar" {
		t.Errorf("PrimarySourceLang() = %q, want %q", cfg.PrimarySourceLang(), "ar")
	}
}

func TestLoad_DefaultsApplied(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mutercim.yaml")
	if err := os.WriteFile(path, []byte("inputs:\n  - path: ./input\n    languages: [ar]\n"), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Read.Retry.MaxAttempts != 3 {
		t.Errorf("Read.Retry.MaxAttempts = %d, want 3", cfg.Read.Retry.MaxAttempts)
	}
	if cfg.Cut.DPI != 300 {
		t.Errorf("Cut.DPI = %d, want 300", cfg.Cut.DPI)
	}
}

func TestLoad_ValidateCalledWithInvalidPages(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mutercim.yaml")

	yaml := `inputs:
  - path: ./input
    languages: [ar]
    pages: "not-a-range"
`
	if err := os.WriteFile(path, []byte(yaml), 0600); err != nil {
		t.Fatal(err)
	}

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

	yaml := `inputs:
  - path: ./input
    languages: [ar]
    pages: "1-50"
`
	if err := os.WriteFile(path, []byte(yaml), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Inputs[0].Pages != "1-50" {
		t.Errorf("pages = %q", cfg.Inputs[0].Pages)
	}
}

func TestLoad_MissingLanguages(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mutercim.yaml")
	if err := os.WriteFile(path, []byte("inputs:\n  - path: ./input\n"), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing languages")
	}
	if !strings.Contains(err.Error(), "languages") {
		t.Errorf("error should mention languages: %v", err)
	}
}
