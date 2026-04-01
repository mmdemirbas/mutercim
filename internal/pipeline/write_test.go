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

	// Create a translated region page JSON
	page := &model.TranslatedRegionPage{
		Version:    "2.0",
		PageNumber: 1,
		SourceLang: "ar",
		TargetLang: "tr",
		Regions: []model.TranslatedRegion{
			{
				ID:             "r1",
				BBox:           model.BBox{400, 50, 700, 60},
				OriginalText:   "حرف الألف",
				TranslatedText: "Elif Harfi",
				Type:           model.RegionTypeHeader,
			},
			{
				ID:             "r2",
				BBox:           model.BBox{800, 150, 600, 400},
				OriginalText:   "أَبْشِرُوا",
				TranslatedText: "Müjdelenin!",
				Type:           model.RegionTypeEntry,
			},
		},
		ReadingOrder: []string{"r1", "r2"},
	}

	data, _ := json.MarshalIndent(page, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "translate/tr/TestBook/001.json"), data, 0600); err != nil {
		t.Fatalf("write translated page: %v", err)
	}

	ws := &workspace.Workspace{Root: dir}
	cfg := &config.Config{
		Inputs: []config.InputSpec{{Path: "./input", Languages: []string{"ar"}}},
		Translate: config.TranslateConfig{
			Languages: []string{"tr"},
		},
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
	trPath := filepath.Join(ws.WriteDir(), "tr", "book.md")
	data, err := os.ReadFile(trPath)
	if err != nil {
		t.Fatalf("read target markdown: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("empty target markdown")
	}

	// Check source language markdown was written
	arPath := filepath.Join(ws.WriteDir(), "ar", "book.md")
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
	texPath := filepath.Join(ws.WriteDir(), "tr", "book.tex")
	data, err := os.ReadFile(texPath)
	if err != nil {
		t.Fatalf("read latex: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("empty latex")
	}

	// Check .tex was also written to latex-build/<stem> for compilation
	buildTexPath := filepath.Join(ws.WriteDir(), "tr", "latex-build", "TestBook", "book.tex")
	buildData, err := os.ReadFile(buildTexPath)
	if err != nil {
		t.Fatalf("read latex build: %v", err)
	}
	if len(buildData) == 0 {
		t.Fatal("empty latex build")
	}
}

func TestWritePartialFailure_DocxWithoutPandoc(t *testing.T) {
	ws, cfg := setupWriteWorkspace(t)
	cfg.Write.Formats = []string{"md", "docx"}

	err := Write(context.Background(), WriteOptions{
		Workspace: ws,
		Config:    cfg,
	})
	if err != nil {
		t.Fatalf("Write() should not error on partial success, got: %v", err)
	}

	mdPath := filepath.Join(ws.WriteDir(), "tr", "book.md")
	if _, err := os.Stat(mdPath); err != nil {
		t.Error("md should have been written despite docx failure")
	}
}

func TestWriteAllFormatsFail(t *testing.T) {
	ws, cfg := setupWriteWorkspace(t)
	cfg.Write.Formats = []string{"docx"}

	err := Write(context.Background(), WriteOptions{
		Workspace: ws,
		Config:    cfg,
	})
	if err == nil {
		t.Skip("pandoc is available, cannot test all-formats-fail scenario")
	}
}

func TestWriteLatexWithoutDocker(t *testing.T) {
	ws, cfg := setupWriteWorkspace(t)
	cfg.Write.Formats = []string{"latex", "pdf"}

	err := Write(context.Background(), WriteOptions{
		Workspace: ws,
		Config:    cfg,
	})

	if err != nil {
		t.Fatalf("Write() should not error on partial success, got: %v", err)
	}

	texPath := filepath.Join(ws.WriteDir(), "tr", "book.tex")
	if _, err := os.Stat(texPath); err != nil {
		t.Error(".tex should have been written regardless of docker availability")
	}
}
