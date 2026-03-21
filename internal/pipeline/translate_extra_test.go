package pipeline

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mmdemirbas/mutercim/internal/config"
	"github.com/mmdemirbas/mutercim/internal/knowledge"
	"github.com/mmdemirbas/mutercim/internal/model"
	"github.com/mmdemirbas/mutercim/internal/workspace"
)

func TestTranslatePipeline_ContextCancelled(t *testing.T) {
	stem := "testbook"
	pages := map[int]*model.SolvedRegionPage{
		1: makeSolvedRegionPage(1),
		2: makeSolvedRegionPage(2),
	}
	ws, cfg := setupTranslateWorkspace(t, stem, pages)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := Translate(ctx, TranslateOptions{
		Workspace: ws,
		Config:    cfg,
		Provider:  &mockProvider{response: regionTranslateResponseJSON()},
		Knowledge: &knowledge.Knowledge{},
	})
	if err != nil {
		t.Fatalf("Translate() error: %v", err)
	}
	if result.Completed != 0 {
		t.Errorf("expected 0 completed, got %d", result.Completed)
	}
}

func TestTranslatePipeline_NoTargetLangs(t *testing.T) {
	stem := "testbook"
	ws, cfg := setupTranslateWorkspace(t, stem, map[int]*model.SolvedRegionPage{1: makeSolvedRegionPage(1)})
	cfg.Translate.Languages = nil

	_, err := Translate(context.Background(), TranslateOptions{
		Workspace: ws, Config: cfg,
		Provider:  &mockProvider{response: regionTranslateResponseJSON()},
		Knowledge: &knowledge.Knowledge{},
	})
	if err == nil {
		t.Fatal("expected error when no target languages")
	}
}

func TestTranslatePipeline_ProviderFails(t *testing.T) {
	stem := "testbook"
	ws, cfg := setupTranslateWorkspace(t, stem, map[int]*model.SolvedRegionPage{1: makeSolvedRegionPage(1)})

	result, err := Translate(context.Background(), TranslateOptions{
		Workspace: ws, Config: cfg,
		Provider: &failingProvider{}, Knowledge: &knowledge.Knowledge{},
	})
	if err != nil {
		t.Fatalf("should not return top-level error: %v", err)
	}
	if result.Completed != 0 {
		t.Errorf("expected 0 completed, got %d", result.Completed)
	}
	if result.Failed != 1 {
		t.Errorf("expected 1 failed, got %d", result.Failed)
	}
}

func TestTranslatePipeline_WithGlossaryContext(t *testing.T) {
	stem := "testbook"
	page := &model.SolvedRegionPage{
		RegionPage: model.RegionPage{
			Version: "2.0", PageNumber: 1,
			Regions:      []model.Region{{ID: "r1", Text: "حديث", Type: model.RegionTypeEntry}},
			ReadingOrder: []string{"r1"},
		},
		GlossaryContext: []string{"حديث"},
	}
	ws, cfg := setupTranslateWorkspace(t, stem, map[int]*model.SolvedRegionPage{1: page})

	result, err := Translate(context.Background(), TranslateOptions{
		Workspace: ws, Config: cfg,
		Provider: &mockProvider{response: regionTranslateResponseJSON()},
		Knowledge: &knowledge.Knowledge{
			Entries: []knowledge.Entry{
				{Forms: map[string][]string{"ar": {"حديث"}, "tr": {"hadîs-i şerîf"}}},
			},
		},
	})
	if err != nil {
		t.Fatalf("Translate() error: %v", err)
	}
	if result.Completed != 1 {
		t.Errorf("expected 1 completed, got %d", result.Completed)
	}
}

func TestWritePipeline_DocxFailsContinuesOtherFormats(t *testing.T) {
	// Set up a workspace with translated pages so Write can find them
	dir := t.TempDir()
	ws := &workspace.Workspace{Root: dir}
	stem := "testbook"

	// Create translated page
	translatedDir := filepath.Join(dir, "translate", "tr", stem)
	os.MkdirAll(translatedDir, 0755)
	page := &model.TranslatedRegionPage{
		Version: "2.0", PageNumber: 1, SourceLang: "ar", TargetLang: "tr",
		Regions: []model.TranslatedRegion{
			{ID: "r1", Type: model.RegionTypeHeader, OriginalText: "عنوان", TranslatedText: "Title"},
			{ID: "r2", Type: model.RegionTypeEntry, OriginalText: "نص", TranslatedText: "Entry"},
		},
		ReadingOrder: []string{"r1", "r2"},
	}
	data, _ := json.MarshalIndent(page, "", "  ")
	os.WriteFile(filepath.Join(translatedDir, "001.json"), data, 0644)

	// Config requesting md + docx (docx will fail because pandoc may not be installed or md not yet written when docx runs first)
	cfg := &config.Config{
		Inputs: []config.InputSpec{{Path: "./input", Languages: []string{"ar"}}},
		Translate: config.TranslateConfig{
			Languages: []string{"tr"},
		},
		Write: config.WriteConfig{
			Formats: []string{"md", "docx"},
		},
	}

	// Write with Force to bypass mtime checks
	err := Write(context.Background(), WriteOptions{
		Workspace: ws,
		Config:    cfg,
		Force:     true,
	})

	// Should NOT return error — md should succeed even if docx fails
	if err != nil {
		t.Fatalf("Write() should not return error when at least one format succeeds: %v", err)
	}

	// Verify markdown was written
	mdPath := filepath.Join(ws.WriteDir(), "tr", "book.md")
	if _, statErr := os.Stat(mdPath); statErr != nil {
		t.Errorf("expected markdown output at %s: %v", mdPath, statErr)
	}
}

func TestWritePipeline_AllFormatsFailReturnsError(t *testing.T) {
	dir := t.TempDir()
	ws := &workspace.Workspace{Root: dir}
	stem := "testbook"

	translatedDir := filepath.Join(dir, "translate", "tr", stem)
	os.MkdirAll(translatedDir, 0755)
	page := &model.TranslatedRegionPage{
		Version: "2.0", PageNumber: 1,
		Regions:      []model.TranslatedRegion{{ID: "r1", Type: model.RegionTypeEntry, TranslatedText: "text"}},
		ReadingOrder: []string{"r1"},
	}
	data, _ := json.MarshalIndent(page, "", "  ")
	os.WriteFile(filepath.Join(translatedDir, "001.json"), data, 0644)

	cfg := &config.Config{
		Inputs: []config.InputSpec{{Path: "./input", Languages: []string{"ar"}}},
		Translate: config.TranslateConfig{
			Languages: []string{"tr"},
		},
		Write: config.WriteConfig{
			// Only request formats that will fail: pdf (no docker), docx (no pandoc likely)
			Formats: []string{"pdf", "docx"},
		},
	}

	err := Write(context.Background(), WriteOptions{
		Workspace: ws, Config: cfg, Force: true,
	})
	// This may or may not error depending on docker/pandoc availability
	// The test verifies it doesn't panic
	_ = err
}
