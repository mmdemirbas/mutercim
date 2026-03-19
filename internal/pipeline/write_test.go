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

func setupWriteWorkspace(t *testing.T) (*workspace.Workspace, *config.Config, *progress.Tracker) {
	t.Helper()
	dir := t.TempDir()

	// Create workspace structure
	for _, d := range []string{
		"midstate/translated/TestBook",
		"output/tr",
		"output/ar",
		"output/tr/latex",
	} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}

	// Create a translated page JSON
	page := &model.TranslatedPage{
		SolvedPage: model.SolvedPage{
			ReadPage: model.ReadPage{
				PageNumber:  1,
				SectionType: "scholarly_entries",
				Header:      &model.Header{Text: "\u062d\u0631\u0641 \u0627\u0644\u0623\u0644\u0641", Type: "section_title"},
				Entries: []model.Entry{
					{Number: intPtr(1), Type: "hadith", ArabicText: "\u0623\u064e\u0628\u0652\u0634\u0650\u0631\u064f\u0648\u0627"},
				},
			},
		},
		TranslatedHeader:  &model.TranslatedHeader{Text: "Elif Harfi"},
		TranslatedEntries: []model.TranslatedEntry{{Number: 1, TurkishText: "M\u00fcjdelenin!"}},
	}

	data, _ := json.MarshalIndent(page, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "midstate/translated/TestBook/page_001.json"), data, 0644); err != nil {
		t.Fatalf("write translated page: %v", err)
	}

	// Progress
	if err := os.WriteFile(filepath.Join(dir, "progress.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("write progress: %v", err)
	}

	ws := &workspace.Workspace{Root: dir}
	cfg := &config.Config{
		Book: model.Book{SourceLang: "ar", TargetLang: "tr"},
		Write: config.WriteConfig{
			Formats:       []string{"md"},
			SkipPDF:       true,
			ExpandSources: true,
		},
	}
	tracker := progress.NewTracker(ws.ProgressPath())
	if err := tracker.Load(); err != nil {
		t.Fatalf("load tracker: %v", err)
	}

	return ws, cfg, tracker
}

func TestWriteMarkdown(t *testing.T) {
	ws, cfg, tracker := setupWriteWorkspace(t)

	err := Write(context.Background(), WriteOptions{
		Workspace: ws,
		Config:    cfg,
		Tracker:   tracker,
	})
	if err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	// Check Turkish markdown was written
	turkishPath := filepath.Join(ws.OutputDir(), "tr", "TestBook.md")
	data, err := os.ReadFile(turkishPath)
	if err != nil {
		t.Fatalf("read turkish markdown: %v", err)
	}
	content := string(data)
	if len(content) == 0 {
		t.Fatal("empty turkish markdown")
	}

	// Check Arabic markdown was written
	arabicPath := filepath.Join(ws.OutputDir(), "ar", "TestBook.md")
	data, err = os.ReadFile(arabicPath)
	if err != nil {
		t.Fatalf("read arabic markdown: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("empty arabic markdown")
	}

	// Check progress was updated
	state := tracker.State()
	phase := state.Phases["write:TestBook"]
	if phase == nil {
		t.Fatal("expected write:TestBook phase in progress")
	}
}

func TestWriteLatexSkipPDF(t *testing.T) {
	ws, cfg, tracker := setupWriteWorkspace(t)
	cfg.Write.Formats = []string{"latex"}
	cfg.Write.SkipPDF = true

	err := Write(context.Background(), WriteOptions{
		Workspace: ws,
		Config:    cfg,
		Tracker:   tracker,
	})
	if err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	texPath := filepath.Join(ws.OutputDir(), "tr", "latex", "book.tex")
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
