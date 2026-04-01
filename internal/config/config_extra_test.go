package config

import (
	"testing"
)

func TestResolvePath(t *testing.T) {
	cfg := &Config{}

	tests := []struct {
		base, rel string
		want      string
	}{
		{"/workspace", "./input", "/workspace/input"},
		{"/workspace", "/absolute/path", "/absolute/path"},
		{"/workspace", "relative", "/workspace/relative"},
	}
	for _, tt := range tests {
		got := cfg.ResolvePath(tt.base, tt.rel)
		if got != tt.want {
			t.Errorf("ResolvePath(%q, %q) = %q, want %q", tt.base, tt.rel, got, tt.want)
		}
	}
}

func TestValidatePerInputPages_Valid(t *testing.T) {
	cfg := &Config{
		Inputs: []InputSpec{{Path: "./input/book.pdf", Pages: "1-5,10", Languages: []string{"ar"}}},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("valid per-input pages should not error: %v", err)
	}
}

func TestValidatePerInputPages_Invalid(t *testing.T) {
	cfg := &Config{
		Inputs: []InputSpec{{Path: "./input/book.pdf", Pages: "abc", Languages: []string{"ar"}}},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for invalid per-input pages")
	}
}

func TestInputPaths(t *testing.T) {
	tests := []struct {
		name   string
		inputs []InputSpec
		want   []string
	}{
		{
			name:   "multiple inputs",
			inputs: []InputSpec{{Path: "./vol1.pdf"}, {Path: "./vol2.pdf"}, {Path: "./vol3.pdf"}},
			want:   []string{"./vol1.pdf", "./vol2.pdf", "./vol3.pdf"},
		},
		{
			name:   "single input",
			inputs: []InputSpec{{Path: "/data/book.pdf"}},
			want:   []string{"/data/book.pdf"},
		},
		{
			name:   "empty inputs",
			inputs: []InputSpec{},
			want:   []string{},
		},
		{
			name:   "inputs with pages ignored in paths",
			inputs: []InputSpec{{Path: "./a.pdf", Pages: "1-10"}, {Path: "./b.pdf", Pages: "20-30"}},
			want:   []string{"./a.pdf", "./b.pdf"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Inputs: tt.inputs}
			got := cfg.InputPaths()
			if len(got) != len(tt.want) {
				t.Fatalf("InputPaths() len = %d, want %d", len(got), len(tt.want))
			}
			for i, p := range got {
				if p != tt.want[i] {
					t.Errorf("InputPaths()[%d] = %q, want %q", i, p, tt.want[i])
				}
			}
		})
	}
}

func TestIsPDFFunction(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"./book.pdf", true},
		{"./images", false},
		{"./vol1.PDF", false}, // case-sensitive
		{"", false},
	}
	for _, tt := range tests {
		if got := IsPDF(tt.path); got != tt.want {
			t.Errorf("IsPDF(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestValidate_PerInputPages(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid per-input pages",
			cfg: Config{
				Inputs: []InputSpec{
					{Path: "./vol1.pdf", Pages: "1-50", Languages: []string{"ar"}},
					{Path: "./vol2.pdf", Pages: "10,20-30", Languages: []string{"ar"}},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid per-input pages",
			cfg: Config{
				Inputs: []InputSpec{
					{Path: "./vol1.pdf", Pages: "abc", Languages: []string{"ar"}},
				},
			},
			wantErr: true,
		},
		{
			name: "empty per-input pages is valid",
			cfg: Config{
				Inputs: []InputSpec{
					{Path: "./vol1.pdf", Pages: "", Languages: []string{"ar"}},
				},
			},
			wantErr: false,
		},
		{
			name: "mix of valid and invalid per-input pages",
			cfg: Config{
				Inputs: []InputSpec{
					{Path: "./vol1.pdf", Pages: "1-10", Languages: []string{"ar"}},
					{Path: "./vol2.pdf", Pages: "not-a-range", Languages: []string{"ar"}},
				},
			},
			wantErr: true,
		},
		{
			name: "valid per-input pages only",
			cfg: Config{
				Inputs: []InputSpec{
					{Path: "./vol1.pdf", Pages: "1-50", Languages: []string{"ar"}},
				},
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

//nolint:cyclop,gocognit // exhaustive field-by-field validation inherently complex
func TestApplyDefaults_AllFieldsMigrated(t *testing.T) {
	cfg := &Config{}
	applyDefaults(cfg)

	// Inputs default
	if len(cfg.Inputs) != 1 || cfg.Inputs[0].Path != "./input" {
		t.Errorf("Inputs = %v, want [{Path: ./input}]", cfg.Inputs)
	}

	// Translate languages default
	if len(cfg.Translate.Languages) != 1 || cfg.Translate.Languages[0] != "tr" {
		t.Errorf("Translate.Languages = %v, want [tr]", cfg.Translate.Languages)
	}

	// Cut.DPI
	if cfg.Cut.DPI != 300 {
		t.Errorf("Cut.DPI = %d, want 300", cfg.Cut.DPI)
	}

	// Read config — legacy provider/model migrated to models list
	if cfg.Read.Concurrency != 1 {
		t.Errorf("Read.Concurrency = %d, want 1", cfg.Read.Concurrency)
	}
	if len(cfg.Read.Models) != 1 {
		t.Fatalf("Read.Models len = %d, want 1", len(cfg.Read.Models))
	}
	if cfg.Read.Models[0].Provider != "gemini" {
		t.Errorf("Read.Models[0].Provider = %q, want %q", cfg.Read.Models[0].Provider, "gemini")
	}
	if cfg.Read.Models[0].Model != "gemini-2.0-flash" {
		t.Errorf("Read.Models[0].Model = %q, want %q", cfg.Read.Models[0].Model, "gemini-2.0-flash")
	}

	// Read retry
	if cfg.Read.Retry.MaxAttempts != 3 {
		t.Errorf("Read.Retry.MaxAttempts = %d, want 3", cfg.Read.Retry.MaxAttempts)
	}
	if cfg.Read.Retry.BackoffSeconds != 2 {
		t.Errorf("Read.Retry.BackoffSeconds = %d, want 2", cfg.Read.Retry.BackoffSeconds)
	}

	// Solve retry
	if cfg.Solve.Retry.MaxAttempts != 3 {
		t.Errorf("Solve.Retry.MaxAttempts = %d, want 3", cfg.Solve.Retry.MaxAttempts)
	}

	// Translate config
	if len(cfg.Translate.Models) != 1 {
		t.Fatalf("Translate.Models len = %d, want 1", len(cfg.Translate.Models))
	}
	if cfg.Translate.Models[0].Provider != "gemini" {
		t.Errorf("Translate.Models[0].Provider = %q, want %q", cfg.Translate.Models[0].Provider, "gemini")
	}
	if cfg.Translate.ContextWindow != 2 {
		t.Errorf("Translate.ContextWindow = %d, want 2", cfg.Translate.ContextWindow)
	}

	// Translate retry
	if cfg.Translate.Retry.MaxAttempts != 3 {
		t.Errorf("Translate.Retry.MaxAttempts = %d, want 3", cfg.Translate.Retry.MaxAttempts)
	}

	// Write config
	if len(cfg.Write.Formats) != 4 || cfg.Write.Formats[0] != "md" || cfg.Write.Formats[1] != "latex" || cfg.Write.Formats[2] != "docx" || cfg.Write.Formats[3] != "pdf" {
		t.Errorf("Write.Formats = %v, want [md latex docx pdf]", cfg.Write.Formats)
	}

	// Knowledge
	if len(cfg.Knowledge) != 1 || cfg.Knowledge[0] != "./knowledge" {
		t.Errorf("Knowledge = %v, want [./knowledge]", cfg.Knowledge)
	}
}

//nolint:cyclop // exhaustive field-by-field validation inherently complex
func TestApplyDefaults_PreservesExistingValues(t *testing.T) {
	cfg := &Config{
		Cut:       CutConfig{DPI: 600},
		Inputs:    []InputSpec{{Path: "./custom.pdf", Languages: []string{"fa"}}},
		Read:      ReadConfig{Models: []ModelSpec{{Provider: "claude", Model: "claude-sonnet-4-20250514"}}, Concurrency: 4, Retry: RetryConfig{MaxAttempts: 5, BackoffSeconds: 10, MaxFailPercent: 10}},
		Translate: TranslateConfig{Languages: []string{"en"}, Models: []ModelSpec{{Provider: "openai", Model: "gpt-4"}}, ContextWindow: 5, Retry: RetryConfig{MaxAttempts: 5, BackoffSeconds: 10, MaxFailPercent: 10}},
		Write:     WriteConfig{Formats: []string{"docx"}},
		Knowledge: []string{"/custom/knowledge"},
	}
	applyDefaults(cfg)

	if cfg.Cut.DPI != 600 {
		t.Errorf("Cut.DPI was overwritten: got %d", cfg.Cut.DPI)
	}
	if len(cfg.Read.Models) != 1 || cfg.Read.Models[0].Provider != "claude" {
		t.Errorf("Read.Models was overwritten: got %+v", cfg.Read.Models)
	}
	if cfg.Read.Concurrency != 4 {
		t.Errorf("Read.Concurrency was overwritten: got %d", cfg.Read.Concurrency)
	}
	if len(cfg.Translate.Models) != 1 || cfg.Translate.Models[0].Provider != "openai" {
		t.Errorf("Translate.Models was overwritten: got %+v", cfg.Translate.Models)
	}
	if cfg.Translate.ContextWindow != 5 {
		t.Errorf("Translate.ContextWindow was overwritten: got %d", cfg.Translate.ContextWindow)
	}
	if len(cfg.Write.Formats) != 1 || cfg.Write.Formats[0] != "docx" {
		t.Errorf("Write.Formats was overwritten: got %v", cfg.Write.Formats)
	}
	if len(cfg.Knowledge) != 1 || cfg.Knowledge[0] != "/custom/knowledge" {
		t.Errorf("Knowledge was overwritten: got %v", cfg.Knowledge)
	}
	if cfg.Read.Retry.MaxAttempts != 5 {
		t.Errorf("Read.Retry.MaxAttempts was overwritten: got %d", cfg.Read.Retry.MaxAttempts)
	}
	if cfg.Translate.Languages[0] != "en" {
		t.Errorf("Translate.Languages was overwritten: got %v", cfg.Translate.Languages)
	}
	if cfg.Inputs[0].Languages[0] != "fa" {
		t.Errorf("Inputs[0].Languages was overwritten: got %v", cfg.Inputs[0].Languages)
	}
}

func TestApplyDefaults_LangDefaults(t *testing.T) {
	cfg := &Config{}
	applyDefaults(cfg)

	// Inputs get default but no languages (languages is required, no default)
	if len(cfg.Inputs) != 1 {
		t.Fatalf("Inputs len = %d, want 1", len(cfg.Inputs))
	}
	if len(cfg.Inputs[0].Languages) != 0 {
		t.Errorf("Inputs[0].Languages = %v, want [] (no default)", cfg.Inputs[0].Languages)
	}
	if len(cfg.Translate.Languages) != 1 || cfg.Translate.Languages[0] != "tr" {
		t.Errorf("Translate.Languages = %v, want [tr]", cfg.Translate.Languages)
	}
}

func TestApplyDefaults_InputsDefault(t *testing.T) {
	cfg := &Config{}
	applyDefaults(cfg)

	if len(cfg.Inputs) != 1 || cfg.Inputs[0].Path != "./input" {
		t.Errorf("Inputs = %v, want [{Path: ./input}]", cfg.Inputs)
	}
}
