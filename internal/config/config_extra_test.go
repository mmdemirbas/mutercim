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
	cfg := validBase()
	cfg.Inputs = []InputSpec{{Path: "./input/book.pdf", Pages: "1-5,10", Languages: []string{"ar"}}}
	if err := cfg.Validate(); err != nil {
		t.Errorf("valid per-input pages should not error: %v", err)
	}
}

func TestValidatePerInputPages_Invalid(t *testing.T) {
	cfg := validBase()
	cfg.Inputs = []InputSpec{{Path: "./input/book.pdf", Pages: "abc", Languages: []string{"ar"}}}
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
		inputs  []InputSpec
		wantErr bool
	}{
		{
			name: "valid per-input pages",
			inputs: []InputSpec{
				{Path: "./vol1.pdf", Pages: "1-50", Languages: []string{"ar"}},
				{Path: "./vol2.pdf", Pages: "10,20-30", Languages: []string{"ar"}},
			},
		},
		{
			name: "invalid per-input pages",
			inputs: []InputSpec{
				{Path: "./vol1.pdf", Pages: "abc", Languages: []string{"ar"}},
			},
			wantErr: true,
		},
		{
			name: "empty per-input pages is valid",
			inputs: []InputSpec{
				{Path: "./vol1.pdf", Pages: "", Languages: []string{"ar"}},
			},
		},
		{
			name: "mix of valid and invalid per-input pages",
			inputs: []InputSpec{
				{Path: "./vol1.pdf", Pages: "1-10", Languages: []string{"ar"}},
				{Path: "./vol2.pdf", Pages: "not-a-range", Languages: []string{"ar"}},
			},
			wantErr: true,
		},
		{
			name: "valid per-input pages only",
			inputs: []InputSpec{
				{Path: "./vol1.pdf", Pages: "1-50", Languages: []string{"ar"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validBase()
			cfg.Inputs = tt.inputs
			err := cfg.Validate()
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

// validBase returns a Config that passes all validations to use as a baseline in subtests.
func validBase() Config {
	return Config{
		Inputs:    []InputSpec{{Path: "./input", Languages: []string{"ar"}}},
		Read:      ReadConfig{Models: []ModelSpec{{Provider: "gemini", Model: "gemini-2.0-flash"}}},
		Translate: TranslateConfig{Models: []ModelSpec{{Provider: "gemini", Model: "gemini-2.0-flash"}}, ContextWindow: 2},
		Layout:    LayoutConfig{Tool: "doclayout-yolo"},
		OCR:       OCRConfig{Tool: ""},
		Write:     WriteConfig{Formats: []string{"md"}},
	}
}

func TestValidate_ModelSpecs(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(cfg *Config)
		wantErr bool
	}{
		{
			name:    "read model: empty provider",
			mutate:  func(c *Config) { c.Read.Models = []ModelSpec{{Provider: "", Model: "gemini-2.0-flash"}} },
			wantErr: true,
		},
		{
			name:    "read model: empty model name",
			mutate:  func(c *Config) { c.Read.Models = []ModelSpec{{Provider: "gemini", Model: ""}} },
			wantErr: true,
		},
		{
			name:    "read model: valid",
			mutate:  func(c *Config) { c.Read.Models = []ModelSpec{{Provider: "gemini", Model: "gemini-2.0-flash"}} },
			wantErr: false,
		},
		{
			name:    "translate model: empty provider",
			mutate:  func(c *Config) { c.Translate.Models = []ModelSpec{{Provider: "", Model: "gemini-2.0-flash"}} },
			wantErr: true,
		},
		{
			name:    "translate model: empty model name",
			mutate:  func(c *Config) { c.Translate.Models = []ModelSpec{{Provider: "gemini", Model: ""}} },
			wantErr: true,
		},
		{
			name:    "translate model: valid",
			mutate:  func(c *Config) { c.Translate.Models = []ModelSpec{{Provider: "claude", Model: "claude-sonnet-4-20250514"}} },
			wantErr: false,
		},
		{
			name:    "no read models: fails",
			mutate:  func(c *Config) { c.Read.Models = nil },
			wantErr: true,
		},
		{
			name:    "no translate models: fails",
			mutate:  func(c *Config) { c.Translate.Models = nil },
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validBase()
			tt.mutate(&cfg)
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidate_LayoutTool(t *testing.T) {
	tests := []struct {
		tool    string
		wantErr bool
	}{
		{"", false},
		{"doclayout-yolo", false},
		{"surya", false},
		{"unknown-tool", true},
		{"Surya", true}, // case-sensitive
	}
	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			cfg := validBase()
			cfg.Layout.Tool = tt.tool
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("layout.tool=%q: Validate() error = %v, wantErr %v", tt.tool, err, tt.wantErr)
			}
		})
	}
}

func TestValidate_OCRTool(t *testing.T) {
	tests := []struct {
		tool    string
		wantErr bool
	}{
		{"", false},
		{"qari", false},
		{"unknown-ocr", true},
		{"Qari", true}, // case-sensitive
	}
	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			cfg := validBase()
			cfg.OCR.Tool = tt.tool
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("ocr.tool=%q: Validate() error = %v, wantErr %v", tt.tool, err, tt.wantErr)
			}
		})
	}
}

func TestValidate_WriteFormats(t *testing.T) {
	tests := []struct {
		formats []string
		wantErr bool
	}{
		{[]string{"md"}, false},
		{[]string{"latex"}, false},
		{[]string{"docx"}, false},
		{[]string{"pdf"}, false},
		{[]string{"md", "latex", "docx", "pdf"}, false},
		{[]string{"html"}, true},
		{[]string{"md", "unknown"}, true},
		{[]string{"MD"}, true}, // case-sensitive
		{nil, true},            // empty formats
	}
	for _, tt := range tests {
		name := "empty"
		if len(tt.formats) > 0 {
			name = tt.formats[0]
		}
		t.Run(name, func(t *testing.T) {
			cfg := validBase()
			cfg.Write.Formats = tt.formats
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("formats=%v: Validate() error = %v, wantErr %v", tt.formats, err, tt.wantErr)
			}
		})
	}
}

func TestSourceLanguages(t *testing.T) {
	tests := []struct {
		name   string
		inputs []InputSpec
		want   []string
	}{
		{
			name:   "no inputs",
			inputs: nil,
			want:   nil,
		},
		{
			name:   "single input single language",
			inputs: []InputSpec{{Path: "./vol1.pdf", Languages: []string{"ar"}}},
			want:   []string{"ar"},
		},
		{
			name:   "single input multiple languages",
			inputs: []InputSpec{{Path: "./vol1.pdf", Languages: []string{"ar", "fa"}}},
			want:   []string{"ar", "fa"},
		},
		{
			name: "multiple inputs with duplicates",
			inputs: []InputSpec{
				{Path: "./vol1.pdf", Languages: []string{"ar", "fa"}},
				{Path: "./vol2.pdf", Languages: []string{"fa", "he"}},
			},
			want: []string{"ar", "fa", "he"},
		},
		{
			name: "input with no languages",
			inputs: []InputSpec{
				{Path: "./vol1.pdf", Languages: nil},
				{Path: "./vol2.pdf", Languages: []string{"ar"}},
			},
			want: []string{"ar"},
		},
		{
			name:   "all inputs with empty languages",
			inputs: []InputSpec{{Path: "./vol1.pdf", Languages: []string{}}},
			want:   nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Inputs: tt.inputs}
			got := cfg.SourceLanguages()
			if len(got) != len(tt.want) {
				t.Fatalf("SourceLanguages() = %v, want %v", got, tt.want)
			}
			for i, lang := range got {
				if lang != tt.want[i] {
					t.Errorf("SourceLanguages()[%d] = %q, want %q", i, lang, tt.want[i])
				}
			}
		})
	}
}

func TestSourceLanguagesForStem(t *testing.T) {
	tests := []struct {
		name   string
		inputs []InputSpec
		stem   string
		want   []string
	}{
		{
			name:   "no inputs",
			inputs: nil,
			stem:   "vol1",
			want:   nil,
		},
		{
			name: "matching stem",
			inputs: []InputSpec{
				{Path: "./vol1.pdf", Languages: []string{"ar"}},
				{Path: "./vol2.pdf", Languages: []string{"fa"}},
			},
			stem: "vol2",
			want: []string{"fa"},
		},
		{
			name: "non-matching stem falls back to first input",
			inputs: []InputSpec{
				{Path: "./vol1.pdf", Languages: []string{"ar"}},
				{Path: "./vol2.pdf", Languages: []string{"fa"}},
			},
			stem: "vol3",
			want: []string{"ar"},
		},
		{
			name: "directory path stem matches",
			inputs: []InputSpec{
				{Path: "/data/books/mushaf.pdf", Languages: []string{"ar", "fa"}},
			},
			stem: "mushaf",
			want: []string{"ar", "fa"},
		},
		{
			name: "first input has no languages",
			inputs: []InputSpec{
				{Path: "./vol1.pdf", Languages: nil},
			},
			stem: "missing",
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Inputs: tt.inputs}
			got := cfg.SourceLanguagesForStem(tt.stem)
			if len(got) != len(tt.want) {
				t.Fatalf("SourceLanguagesForStem(%q) = %v, want %v", tt.stem, got, tt.want)
			}
			for i, lang := range got {
				if lang != tt.want[i] {
					t.Errorf("SourceLanguagesForStem(%q)[%d] = %q, want %q", tt.stem, i, lang, tt.want[i])
				}
			}
		})
	}
}

func TestValidate_ContextWindow(t *testing.T) {
	tests := []struct {
		window  int
		wantErr bool
	}{
		{0, false},
		{2, false},
		{100, false},
		{-1, true},
	}
	for _, tt := range tests {
		cfg := validBase()
		cfg.Translate.ContextWindow = tt.window
		err := cfg.Validate()
		if (err != nil) != tt.wantErr {
			t.Errorf("context_window=%d: Validate() error = %v, wantErr %v", tt.window, err, tt.wantErr)
		}
	}
}
