package pipeline

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mmdemirbas/mutercim/internal/config"
	"github.com/mmdemirbas/mutercim/internal/model"
	"github.com/mmdemirbas/mutercim/internal/progress"
	"github.com/mmdemirbas/mutercim/internal/workspace"
)

func setupCompileWorkspace(t *testing.T) (*workspace.Workspace, *config.Config, *progress.Tracker) {
	t.Helper()
	dir := t.TempDir()

	// Create workspace structure
	for _, d := range []string{
		"cache/translated/TestBook",
		"output/turkish",
		"output/arabic",
		"output/latex",
	} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}

	// Create a translated page JSON
	page := &model.TranslatedPage{
		EnrichedPage: model.EnrichedPage{
			ExtractedPage: model.ExtractedPage{
				PageNumber:  1,
				SectionType: "scholarly_entries",
				Header:      &model.Header{Text: "حرف الألف", Type: "section_title"},
				Entries: []model.Entry{
					{Number: intPtr(1), Type: "hadith", ArabicText: "أَبْشِرُوا"},
				},
			},
		},
		TranslatedHeader:  &model.TranslatedHeader{Text: "Elif Harfi"},
		TranslatedEntries: []model.TranslatedEntry{{Number: 1, TurkishText: "Müjdelenin!"}},
	}

	data, _ := json.MarshalIndent(page, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "cache/translated/TestBook/page_001.json"), data, 0644); err != nil {
		t.Fatalf("write translated page: %v", err)
	}

	// Progress
	if err := os.WriteFile(filepath.Join(dir, "progress.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("write progress: %v", err)
	}

	ws := &workspace.Workspace{Root: dir}
	cfg := &config.Config{
		Compile: config.CompileConfig{
			Formats:   []string{"md"},
			SkipPDF:   true,
			ExpandSources: true,
		},
	}
	tracker := progress.NewTracker(ws.ProgressPath())
	if err := tracker.Load(); err != nil {
		t.Fatalf("load tracker: %v", err)
	}

	return ws, cfg, tracker
}

func TestCompileMarkdown(t *testing.T) {
	ws, cfg, tracker := setupCompileWorkspace(t)

	err := Compile(context.Background(), CompileOptions{
		Workspace: ws,
		Config:    cfg,
		Tracker:   tracker,
	})
	if err != nil {
		t.Fatalf("Compile() error: %v", err)
	}

	// Check Turkish markdown was written
	turkishPath := filepath.Join(ws.OutputDir(), "turkish", "TestBook.md")
	data, err := os.ReadFile(turkishPath)
	if err != nil {
		t.Fatalf("read turkish markdown: %v", err)
	}
	content := string(data)
	if len(content) == 0 {
		t.Fatal("empty turkish markdown")
	}

	// Check Arabic markdown was written
	arabicPath := filepath.Join(ws.OutputDir(), "arabic", "TestBook.md")
	data, err = os.ReadFile(arabicPath)
	if err != nil {
		t.Fatalf("read arabic markdown: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("empty arabic markdown")
	}

	// Check progress was updated
	state := tracker.State()
	phase := state.Phases["compile:TestBook"]
	if phase == nil {
		t.Fatal("expected compile:TestBook phase in progress")
	}
}

func TestCompileLatexSkipPDF(t *testing.T) {
	ws, cfg, tracker := setupCompileWorkspace(t)
	cfg.Compile.Formats = []string{"latex"}
	cfg.Compile.SkipPDF = true

	err := Compile(context.Background(), CompileOptions{
		Workspace: ws,
		Config:    cfg,
		Tracker:   tracker,
	})
	if err != nil {
		t.Fatalf("Compile() error: %v", err)
	}

	texPath := filepath.Join(ws.OutputDir(), "latex", "book.tex")
	data, err := os.ReadFile(texPath)
	if err != nil {
		t.Fatalf("read latex: %v", err)
	}
	content := string(data)
	if len(content) == 0 {
		t.Fatal("empty latex")
	}
}

func intPtr(n int) *int { return &n }
