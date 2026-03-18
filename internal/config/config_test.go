package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mmdemirbas/mutercim/internal/model"
)

func TestLoadDefaults(t *testing.T) {
	// Load with no config file — should get defaults
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.DPI != 300 {
		t.Errorf("DPI = %d, want 300", cfg.DPI)
	}
	if cfg.Extract.Provider != "gemini" {
		t.Errorf("Extract.Provider = %q, want %q", cfg.Extract.Provider, "gemini")
	}
	if cfg.Extract.Model != "gemini-2.0-flash" {
		t.Errorf("Extract.Model = %q, want %q", cfg.Extract.Model, "gemini-2.0-flash")
	}
	if cfg.Translate.ContextWindow != 2 {
		t.Errorf("Translate.ContextWindow = %d, want 2", cfg.Translate.ContextWindow)
	}
	if cfg.Retry.MaxAttempts != 3 {
		t.Errorf("Retry.MaxAttempts = %d, want 3", cfg.Retry.MaxAttempts)
	}
	if cfg.RateLimit.RequestsPerMinute != 14 {
		t.Errorf("RateLimit.RequestsPerMinute = %d, want 14", cfg.RateLimit.RequestsPerMinute)
	}
	if cfg.Book.SourceLang != "ar" {
		t.Errorf("Book.SourceLang = %q, want %q", cfg.Book.SourceLang, "ar")
	}
	if cfg.Book.TargetLang != "tr" {
		t.Errorf("Book.TargetLang = %q, want %q", cfg.Book.TargetLang, "tr")
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()

	yaml := `
book:
  title: "Test Book"
  author: "Test Author"
  source_lang: ar
  target_lang: tr
dpi: 600
sections:
  - name: intro
    pages: "1-5"
    type: prose
    translate: true
  - name: hadith
    pages: "6-100"
    type: scholarly_entries
    translate: true
extract:
  provider: claude
  model: claude-sonnet-4-20250514
rate_limit:
  requests_per_minute: 50
`
	configPath := filepath.Join(dir, "mutercim.yaml")
	os.WriteFile(configPath, []byte(yaml), 0644)

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Book.Title != "Test Book" {
		t.Errorf("Book.Title = %q, want %q", cfg.Book.Title, "Test Book")
	}
	if cfg.Book.Author != "Test Author" {
		t.Errorf("Book.Author = %q, want %q", cfg.Book.Author, "Test Author")
	}
	if cfg.DPI != 600 {
		t.Errorf("DPI = %d, want 600", cfg.DPI)
	}
	if len(cfg.Sections) != 2 {
		t.Fatalf("len(Sections) = %d, want 2", len(cfg.Sections))
	}
	if cfg.Sections[0].Name != "intro" {
		t.Errorf("Sections[0].Name = %q, want %q", cfg.Sections[0].Name, "intro")
	}
	if cfg.Sections[0].Type != model.SectionProse {
		t.Errorf("Sections[0].Type = %q, want %q", cfg.Sections[0].Type, model.SectionProse)
	}
	if cfg.Extract.Provider != "claude" {
		t.Errorf("Extract.Provider = %q, want %q", cfg.Extract.Provider, "claude")
	}
	if cfg.Extract.Model != "claude-sonnet-4-20250514" {
		t.Errorf("Extract.Model = %q, want %q", cfg.Extract.Model, "claude-sonnet-4-20250514")
	}
	// Default should still apply for unset fields
	if cfg.Extract.Concurrency != 1 {
		t.Errorf("Extract.Concurrency = %d, want 1 (default)", cfg.Extract.Concurrency)
	}
	if cfg.RateLimit.RequestsPerMinute != 50 {
		t.Errorf("RateLimit.RequestsPerMinute = %d, want 50", cfg.RateLimit.RequestsPerMinute)
	}
}

func TestSectionForPage(t *testing.T) {
	cfg := &Config{
		Sections: []model.Section{
			{Name: "front_matter", Pages: "1-2", Type: model.SectionSkip},
			{Name: "intro", Pages: "3-5", Type: model.SectionProse, Translate: true},
			{Name: "hadith", Pages: "6-100", Type: model.SectionScholarlyEntries, Translate: true},
		},
	}

	tests := []struct {
		page     int
		wantName string
		wantType model.SectionType
	}{
		{1, "front_matter", model.SectionSkip},
		{2, "front_matter", model.SectionSkip},
		{3, "intro", model.SectionProse},
		{5, "intro", model.SectionProse},
		{6, "hadith", model.SectionScholarlyEntries},
		{50, "hadith", model.SectionScholarlyEntries},
		{100, "hadith", model.SectionScholarlyEntries},
		{101, "auto", model.SectionAuto}, // not in any section
	}

	for _, tt := range tests {
		s := cfg.SectionForPage(tt.page)
		if s.Name != tt.wantName {
			t.Errorf("SectionForPage(%d).Name = %q, want %q", tt.page, s.Name, tt.wantName)
		}
		if s.Type != tt.wantType {
			t.Errorf("SectionForPage(%d).Type = %q, want %q", tt.page, s.Type, tt.wantType)
		}
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: Config{
				Sections: []model.Section{
					{Name: "test", Pages: "1-10", Type: model.SectionProse},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid section type",
			cfg: Config{
				Sections: []model.Section{
					{Name: "test", Pages: "1-10", Type: "invalid_type"},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid page range",
			cfg: Config{
				Sections: []model.Section{
					{Name: "test", Pages: "abc", Type: model.SectionProse},
				},
			},
			wantErr: true,
		},
		{
			name:    "empty sections is valid",
			cfg:     Config{},
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

func TestInputsMigration(t *testing.T) {
	// When only Input (singular) is set, Inputs should be populated
	dir := t.TempDir()
	yaml := `input: ./input/book.pdf`
	configPath := filepath.Join(dir, "mutercim.yaml")
	os.WriteFile(configPath, []byte(yaml), 0644)

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(cfg.Inputs) != 1 || cfg.Inputs[0] != "./input/book.pdf" {
		t.Errorf("Inputs = %v, want [./input/book.pdf]", cfg.Inputs)
	}
}

func TestInputsList(t *testing.T) {
	dir := t.TempDir()
	yaml := `
inputs:
  - ./input/vol1.pdf
  - ./input/vol2.pdf
pages: "1-5"
`
	configPath := filepath.Join(dir, "mutercim.yaml")
	os.WriteFile(configPath, []byte(yaml), 0644)

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(cfg.Inputs) != 2 {
		t.Fatalf("len(Inputs) = %d, want 2", len(cfg.Inputs))
	}
	if cfg.Inputs[0] != "./input/vol1.pdf" {
		t.Errorf("Inputs[0] = %q, want %q", cfg.Inputs[0], "./input/vol1.pdf")
	}
	if cfg.Inputs[1] != "./input/vol2.pdf" {
		t.Errorf("Inputs[1] = %q, want %q", cfg.Inputs[1], "./input/vol2.pdf")
	}
	if cfg.Pages != "1-5" {
		t.Errorf("Pages = %q, want %q", cfg.Pages, "1-5")
	}
}
