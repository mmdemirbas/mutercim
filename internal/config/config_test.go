package config

import (
	"os"
	"path/filepath"
	"testing"
)

//nolint:cyclop // exhaustive defaults coverage test
func TestLoadDefaults(t *testing.T) {
	// Load with minimal config file (inputs[].languages is required)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "mutercim.yaml"), []byte("inputs:\n  - path: ./input\n    languages: [ar]\n"), 0600); err != nil {
		t.Fatal(err)
	}
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Cut.DPI != 300 {
		t.Errorf("Cut.DPI = %d, want 300", cfg.Cut.DPI)
	}
	if len(cfg.Read.Models) != 1 || cfg.Read.Models[0].Provider != "gemini" {
		t.Errorf("Read.Models = %+v, want [{gemini gemini-2.0-flash}]", cfg.Read.Models)
	}
	if cfg.Translate.ContextWindow != 2 {
		t.Errorf("Translate.ContextWindow = %d, want 2", cfg.Translate.ContextWindow)
	}
	if cfg.Read.Retry.MaxAttempts != 3 {
		t.Errorf("Read.Retry.MaxAttempts = %d, want 3", cfg.Read.Retry.MaxAttempts)
	}
	if cfg.PrimarySourceLang() != "ar" {
		t.Errorf("PrimarySourceLang() = %q, want %q", cfg.PrimarySourceLang(), "ar")
	}
	if len(cfg.Translate.Models) != 1 {
		t.Fatalf("Translate.Models len = %d, want 1", len(cfg.Translate.Models))
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()

	yaml := `
inputs:
  - path: ./input
    languages: [ar]
cut:
  dpi: 600
read:
  models:
    - provider: claude
      model: claude-sonnet-4-20250514
translate:
  languages: [tr]
`
	configPath := filepath.Join(dir, "mutercim.yaml")
	if err := os.WriteFile(configPath, []byte(yaml), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Cut.DPI != 600 {
		t.Errorf("Cut.DPI = %d, want 600", cfg.Cut.DPI)
	}
	if len(cfg.Read.Models) != 1 || cfg.Read.Models[0].Provider != "claude" || cfg.Read.Models[0].Model != "claude-sonnet-4-20250514" {
		t.Errorf("Read.Models = %+v, want [{Provider:claude Model:claude-sonnet-4-20250514}]", cfg.Read.Models)
	}
	// Default should still apply for unset fields
	if cfg.Read.Concurrency != 1 {
		t.Errorf("Read.Concurrency = %d, want 1 (default)", cfg.Read.Concurrency)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name:    "empty config fails (languages required)",
			cfg:     Config{},
			wantErr: true,
		},
		{
			name: "config with input languages is valid",
			cfg: Config{
				Inputs: []InputSpec{{Path: "./input", Languages: []string{"ar"}}},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIsPDF(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"./input/book.pdf", true},
		{"./input", false},
		{"book.PDF", false}, // case sensitive
		{"", false},
	}
	for _, tt := range tests {
		if got := IsPDF(tt.path); got != tt.want {
			t.Errorf("IsPDF(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestInputsList(t *testing.T) {
	dir := t.TempDir()
	yaml := `
inputs:
  - path: ./input/vol1.pdf
    languages: [ar]
  - path: ./input/vol2.pdf
    languages: [ar]
`
	configPath := filepath.Join(dir, "mutercim.yaml")
	if err := os.WriteFile(configPath, []byte(yaml), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(cfg.Inputs) != 2 {
		t.Fatalf("len(Inputs) = %d, want 2", len(cfg.Inputs))
	}
	if cfg.Inputs[0].Path != "./input/vol1.pdf" {
		t.Errorf("Inputs[0].Path = %q, want %q", cfg.Inputs[0].Path, "./input/vol1.pdf")
	}
	if cfg.Inputs[1].Path != "./input/vol2.pdf" {
		t.Errorf("Inputs[1].Path = %q, want %q", cfg.Inputs[1].Path, "./input/vol2.pdf")
	}
}

func TestInputsWithPerInputPages(t *testing.T) {
	dir := t.TempDir()
	yaml := `
inputs:
  - path: ./input/vol1.pdf
    pages: "1-50"
    languages: [ar]
  - path: ./input/vol2.pdf
    pages: "10-20"
    languages: [ar]
  - path: ./input/vol3.pdf
    languages: [ar]
`
	configPath := filepath.Join(dir, "mutercim.yaml")
	if err := os.WriteFile(configPath, []byte(yaml), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(cfg.Inputs) != 3 {
		t.Fatalf("len(Inputs) = %d, want 3", len(cfg.Inputs))
	}
	if cfg.Inputs[0].Path != "./input/vol1.pdf" || cfg.Inputs[0].Pages != "1-50" {
		t.Errorf("Inputs[0] = %+v, want {Path: ./input/vol1.pdf, Pages: 1-50}", cfg.Inputs[0])
	}
	if cfg.Inputs[1].Path != "./input/vol2.pdf" || cfg.Inputs[1].Pages != "10-20" {
		t.Errorf("Inputs[1] = %+v, want {Path: ./input/vol2.pdf, Pages: 10-20}", cfg.Inputs[1])
	}
	if cfg.Inputs[2].Path != "./input/vol3.pdf" || cfg.Inputs[2].Pages != "" {
		t.Errorf("Inputs[2] = %+v, want {Path: ./input/vol3.pdf, Pages: }", cfg.Inputs[2])
	}
}

//nolint:cyclop // exhaustive model chain config test
func TestModelsFailoverChainConfig(t *testing.T) {
	dir := t.TempDir()
	yaml := `
inputs:
  - path: ./input
    languages: [ar]
read:
  models:
    - provider: gemini
      model: gemini-2.5-flash-lite
      rpm: 10
    - provider: groq
      model: llama-3.3-70b-versatile
      rpm: 30
      vision: false
translate:
  models:
    - provider: gemini
      model: gemini-2.5-flash-lite
    - provider: mistral
      model: mistral-small-latest
      rpm: 60
`
	configPath := filepath.Join(dir, "mutercim.yaml")
	if err := os.WriteFile(configPath, []byte(yaml), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Read models
	if len(cfg.Read.Models) != 2 {
		t.Fatalf("Read.Models len = %d, want 2", len(cfg.Read.Models))
	}
	if cfg.Read.Models[0].Provider != "gemini" || cfg.Read.Models[0].RPM != 10 {
		t.Errorf("Read.Models[0] = %+v", cfg.Read.Models[0])
	}
	if cfg.Read.Models[1].Provider != "groq" || cfg.Read.Models[1].RPM != 30 {
		t.Errorf("Read.Models[1] = %+v", cfg.Read.Models[1])
	}
	if cfg.Read.Models[1].Vision == nil || *cfg.Read.Models[1].Vision != false {
		t.Errorf("Read.Models[1].Vision should be false, got %v", cfg.Read.Models[1].Vision)
	}

	// Translate models
	if len(cfg.Translate.Models) != 2 {
		t.Fatalf("Translate.Models len = %d, want 2", len(cfg.Translate.Models))
	}
	if cfg.Translate.Models[1].Provider != "mistral" || cfg.Translate.Models[1].RPM != 60 {
		t.Errorf("Translate.Models[1] = %+v", cfg.Translate.Models[1])
	}
}

func TestModelsDefaultWhenOmitted(t *testing.T) {
	dir := t.TempDir()
	yaml := `
inputs:
  - path: ./input
    languages: [ar]
cut:
  dpi: 300
`
	configPath := filepath.Join(dir, "mutercim.yaml")
	if err := os.WriteFile(configPath, []byte(yaml), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// When models not specified, defaults to gemini
	if len(cfg.Read.Models) != 1 {
		t.Fatalf("Read.Models len = %d, want 1", len(cfg.Read.Models))
	}
	if cfg.Read.Models[0].Provider != "gemini" || cfg.Read.Models[0].Model != "gemini-2.0-flash" {
		t.Errorf("Read.Models[0] = %+v, want gemini/gemini-2.0-flash", cfg.Read.Models[0])
	}
	if len(cfg.Translate.Models) != 1 {
		t.Fatalf("Translate.Models len = %d, want 1", len(cfg.Translate.Models))
	}
	if cfg.Translate.Models[0].Provider != "gemini" || cfg.Translate.Models[0].Model != "gemini-2.0-flash" {
		t.Errorf("Translate.Models[0] = %+v, want gemini/gemini-2.0-flash", cfg.Translate.Models[0])
	}
}
