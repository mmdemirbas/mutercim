package config

import (
	"testing"

	"github.com/mmdemirbas/mutercim/internal/model"
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
		Inputs: []InputSpec{{Path: "./input/book.pdf", Pages: "1-5,10"}},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("valid per-input pages should not error: %v", err)
	}
}

func TestValidatePerInputPages_Invalid(t *testing.T) {
	cfg := &Config{
		Inputs: []InputSpec{{Path: "./input/book.pdf", Pages: "abc"}},
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
					{Path: "./vol1.pdf", Pages: "1-50"},
					{Path: "./vol2.pdf", Pages: "10,20-30"},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid per-input pages",
			cfg: Config{
				Inputs: []InputSpec{
					{Path: "./vol1.pdf", Pages: "abc"},
				},
			},
			wantErr: true,
		},
		{
			name: "empty per-input pages is valid",
			cfg: Config{
				Inputs: []InputSpec{
					{Path: "./vol1.pdf", Pages: ""},
				},
			},
			wantErr: false,
		},
		{
			name: "mix of valid and invalid per-input pages",
			cfg: Config{
				Inputs: []InputSpec{
					{Path: "./vol1.pdf", Pages: "1-10"},
					{Path: "./vol2.pdf", Pages: "not-a-range"},
				},
			},
			wantErr: true,
		},
		{
			name: "valid sections combined with valid per-input pages",
			cfg: Config{
				Sections: []model.Section{
					{Name: "intro", Pages: "1-5", Type: model.SectionProse},
				},
				Inputs: []InputSpec{
					{Path: "./vol1.pdf", Pages: "1-50"},
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

func TestApplyDefaults_AllFieldsMigrated(t *testing.T) {
	cfg := &Config{}
	applyDefaults(cfg)

	// Book defaults
	if len(cfg.Book.SourceLangs) != 1 || cfg.Book.SourceLangs[0] != "ar" {
		t.Errorf("Book.SourceLangs = %v, want [ar]", cfg.Book.SourceLangs)
	}
	if len(cfg.Book.TargetLangs) != 1 || cfg.Book.TargetLangs[0] != "tr" {
		t.Errorf("Book.TargetLangs = %v, want [tr]", cfg.Book.TargetLangs)
	}

	// Inputs default
	if len(cfg.Inputs) != 1 || cfg.Inputs[0].Path != "./input" {
		t.Errorf("Inputs = %v, want [{Path: ./input}]", cfg.Inputs)
	}

	// Output & MidstateDir
	if cfg.Output != "./output" {
		t.Errorf("Output = %q, want %q", cfg.Output, "./output")
	}
	if cfg.MidstateDir != "./midstate" {
		t.Errorf("MidstateDir = %q, want %q", cfg.MidstateDir, "./midstate")
	}

	// DPI
	if cfg.DPI != 300 {
		t.Errorf("DPI = %d, want 300", cfg.DPI)
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

	// Write config
	if len(cfg.Write.Formats) != 2 || cfg.Write.Formats[0] != "md" || cfg.Write.Formats[1] != "latex" {
		t.Errorf("Write.Formats = %v, want [md latex]", cfg.Write.Formats)
	}
	if cfg.Write.LaTeXDockerImage != "mutercim/xelatex:latest" {
		t.Errorf("Write.LaTeXDockerImage = %q, want %q", cfg.Write.LaTeXDockerImage, "mutercim/xelatex:latest")
	}

	// KnowledgeDir
	if cfg.KnowledgeDir != "./knowledge" {
		t.Errorf("KnowledgeDir = %q, want %q", cfg.KnowledgeDir, "./knowledge")
	}

	// Retry
	if cfg.Retry.MaxAttempts != 3 {
		t.Errorf("Retry.MaxAttempts = %d, want 3", cfg.Retry.MaxAttempts)
	}
	if cfg.Retry.BackoffSeconds != 2 {
		t.Errorf("Retry.BackoffSeconds = %d, want 2", cfg.Retry.BackoffSeconds)
	}

	// RateLimit
	if cfg.RateLimit.RequestsPerMinute != 14 {
		t.Errorf("RateLimit.RequestsPerMinute = %d, want 14", cfg.RateLimit.RequestsPerMinute)
	}
}

func TestApplyDefaults_PreservesExistingValues(t *testing.T) {
	cfg := &Config{
		Output:       "/custom/output",
		MidstateDir:  "/custom/midstate",
		DPI:          600,
		Inputs:       []InputSpec{{Path: "./custom.pdf"}},
		Read:         ReadConfig{Models: []ModelSpec{{Provider: "claude", Model: "claude-sonnet-4-20250514"}}, Concurrency: 4},
		Translate:    TranslateConfig{Models: []ModelSpec{{Provider: "openai", Model: "gpt-4"}}, ContextWindow: 5},
		Write:        WriteConfig{Formats: []string{"docx"}, LaTeXDockerImage: "custom:latest"},
		KnowledgeDir: "/custom/knowledge",
		Retry:        RetryConfig{MaxAttempts: 5, BackoffSeconds: 10},
		RateLimit:    RateLimitConfig{RequestsPerMinute: 100},
		Book:         model.Book{SourceLangs: []string{"fa"}, TargetLangs: []string{"en"}},
	}
	applyDefaults(cfg)

	if cfg.Output != "/custom/output" {
		t.Errorf("Output was overwritten: got %q", cfg.Output)
	}
	if cfg.MidstateDir != "/custom/midstate" {
		t.Errorf("MidstateDir was overwritten: got %q", cfg.MidstateDir)
	}
	if cfg.DPI != 600 {
		t.Errorf("DPI was overwritten: got %d", cfg.DPI)
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
	if cfg.KnowledgeDir != "/custom/knowledge" {
		t.Errorf("KnowledgeDir was overwritten: got %q", cfg.KnowledgeDir)
	}
	if cfg.Retry.MaxAttempts != 5 {
		t.Errorf("Retry.MaxAttempts was overwritten: got %d", cfg.Retry.MaxAttempts)
	}
	if cfg.RateLimit.RequestsPerMinute != 100 {
		t.Errorf("RateLimit.RequestsPerMinute was overwritten: got %d", cfg.RateLimit.RequestsPerMinute)
	}
	if cfg.Book.SourceLangs[0] != "fa" {
		t.Errorf("Book.SourceLangs was overwritten: got %v", cfg.Book.SourceLangs)
	}
}

func TestApplyDefaults_LangDefaults(t *testing.T) {
	cfg := &Config{}
	applyDefaults(cfg)

	if len(cfg.Book.SourceLangs) != 1 || cfg.Book.SourceLangs[0] != "ar" {
		t.Errorf("SourceLangs = %v, want [ar]", cfg.Book.SourceLangs)
	}
	if len(cfg.Book.TargetLangs) != 1 || cfg.Book.TargetLangs[0] != "tr" {
		t.Errorf("TargetLangs = %v, want [tr]", cfg.Book.TargetLangs)
	}
}

func TestApplyDefaults_InputsDefault(t *testing.T) {
	cfg := &Config{}
	applyDefaults(cfg)

	if len(cfg.Inputs) != 1 || cfg.Inputs[0].Path != "./input" {
		t.Errorf("Inputs = %v, want [{Path: ./input}]", cfg.Inputs)
	}
}
