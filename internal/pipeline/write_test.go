package pipeline

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mmdemirbas/mutercim/internal/config"
	"github.com/mmdemirbas/mutercim/internal/model"
	"github.com/mmdemirbas/mutercim/internal/workspace"
)

func setupWriteWorkspace(t *testing.T) (*workspace.Workspace, *config.Config) {
	t.Helper()
	dir := t.TempDir()

	// Create workspace structure with per-lang translated dir
	for _, d := range []string{
		"translate/tr/TestBook",
		"write/tr",
		"write/ar",
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
		TranslatedEntries: []model.TranslatedEntry{{Number: 1, TranslatedText: "M\u00fcjdelenin!"}},
	}

	data, _ := json.MarshalIndent(page, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "translate/tr/TestBook/001.json"), data, 0644); err != nil {
		t.Fatalf("write translated page: %v", err)
	}

	ws := &workspace.Workspace{Root: dir}
	cfg := &config.Config{
		Book: model.Book{Title: "TestBook", SourceLangs: []string{"ar"}, TargetLangs: []string{"tr"}},
		Write: config.WriteConfig{
			Formats:       []string{"md"},
			ExpandSources: true,
		},
	}

	return ws, cfg
}

func TestWriteMarkdown(t *testing.T) {
	ws, cfg := setupWriteWorkspace(t)

	err := Write(context.Background(), WriteOptions{
		Workspace: ws,
		Config:    cfg,
	})
	if err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	// Check target language markdown was written
	trPath := filepath.Join(ws.WriteDir(), "tr", "TestBook.md")
	data, err := os.ReadFile(trPath)
	if err != nil {
		t.Fatalf("read target markdown: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("empty target markdown")
	}

	// Check source language markdown was written
	arPath := filepath.Join(ws.WriteDir(), "ar", "TestBook.md")
	data, err = os.ReadFile(arPath)
	if err != nil {
		t.Fatalf("read source markdown: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("empty source markdown")
	}
}

func TestWriteLatex(t *testing.T) {
	ws, cfg := setupWriteWorkspace(t)
	cfg.Write.Formats = []string{"latex"}

	err := Write(context.Background(), WriteOptions{
		Workspace: ws,
		Config:    cfg,
	})
	if err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	// Check .tex was written to lang root with title-based name
	texPath := filepath.Join(ws.WriteDir(), "tr", "TestBook.tex")
	data, err := os.ReadFile(texPath)
	if err != nil {
		t.Fatalf("read latex: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("empty latex")
	}

	// Check .tex was also written to latex-build for compilation
	buildTexPath := filepath.Join(ws.WriteDir(), "tr", "latex-build", "book.tex")
	buildData, err := os.ReadFile(buildTexPath)
	if err != nil {
		t.Fatalf("read latex build: %v", err)
	}
	if len(buildData) == 0 {
		t.Fatal("empty latex build")
	}
}

func intPtr(n int) *int { return &n }

func TestWritePartialFailure_DocxWithoutPandoc(t *testing.T) {
	ws, cfg := setupWriteWorkspace(t)
	// Request md + docx. Pandoc is almost certainly not in PATH during tests,
	// but md should succeed regardless.
	cfg.Write.Formats = []string{"md", "docx"}

	err := Write(context.Background(), WriteOptions{
		Workspace: ws,
		Config:    cfg,
	})
	// Should NOT return error — md succeeded, only docx failed (partial success)
	if err != nil {
		t.Fatalf("Write() should not error on partial success, got: %v", err)
	}

	// md should be written
	mdPath := filepath.Join(ws.WriteDir(), "tr", "TestBook.md")
	if _, err := os.Stat(mdPath); err != nil {
		t.Error("md should have been written despite docx failure")
	}
}

func TestWriteAllFormatsFail(t *testing.T) {
	ws, cfg := setupWriteWorkspace(t)
	// Request only docx (requires pandoc, likely missing in tests)
	cfg.Write.Formats = []string{"docx"}

	// First write md so docx has input (docx requires md to exist)
	// Actually docx will fail at pandoc check before even looking for md
	err := Write(context.Background(), WriteOptions{
		Workspace: ws,
		Config:    cfg,
	})
	// Should return error since ALL formats failed
	if err == nil {
		// pandoc might be installed; skip in that case
		t.Skip("pandoc is available, cannot test all-formats-fail scenario")
	}
}

func TestWriteLatexWithoutDocker(t *testing.T) {
	ws, cfg := setupWriteWorkspace(t)
	// Request latex + pdf. Docker may not be available,
	// but latex (.tex generation) should succeed regardless.
	cfg.Write.Formats = []string{"latex", "pdf"}

	err := Write(context.Background(), WriteOptions{
		Workspace: ws,
		Config:    cfg,
	})

	// latex should succeed, pdf may fail (if no docker) — partial success = no error
	if err != nil {
		t.Fatalf("Write() should not error on partial success, got: %v", err)
	}

	// .tex file should exist
	texPath := filepath.Join(ws.WriteDir(), "tr", "TestBook.tex")
	if _, err := os.Stat(texPath); err != nil {
		t.Error(".tex should have been written regardless of docker availability")
	}
}
