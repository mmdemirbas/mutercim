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

// setupTranslateWorkspace creates a workspace with solved region pages in solve/<stem>/.
func setupTranslateWorkspace(t *testing.T, stem string, pages map[int]*model.SolvedRegionPage) (*workspace.Workspace, *config.Config) {
	t.Helper()
	dir := t.TempDir()

	solvedDir := filepath.Join(dir, "solve", stem)
	if err := os.MkdirAll(solvedDir, 0750); err != nil {
		t.Fatalf("mkdir solved: %v", err)
	}

	for pageNum, page := range pages {
		data, err := json.MarshalIndent(page, "", "  ")
		if err != nil {
			t.Fatalf("marshal solved page %d: %v", pageNum, err)
		}
		filename := filepath.Join(solvedDir, pageName(pageNum))
		if err := os.WriteFile(filename, data, 0600); err != nil {
			t.Fatalf("write solved page %d: %v", pageNum, err)
		}
	}

	ws := &workspace.Workspace{Root: dir}
	cfg := &config.Config{
		Inputs: []config.InputSpec{{Path: "./input", Languages: []string{"ar"}}},
		Translate: config.TranslateConfig{
			Languages:     []string{"tr"},
			Models:        []config.ModelSpec{{Provider: "mock", Model: "test-model"}},
			ContextWindow: 2,
		},
		Write: config.WriteConfig{
			ExpandSources: true,
		},
	}

	return ws, cfg
}

// makeSolvedRegionPage creates a minimal SolvedRegionPage for testing.
func makeSolvedRegionPage(pageNum int) *model.SolvedRegionPage {
	return &model.SolvedRegionPage{
		RegionPage: model.RegionPage{
			Version:    "2.0",
			PageNumber: pageNum,
			PageSize:   model.PageSize{Width: 1500, Height: 2200},
			Regions: []model.Region{
				{ID: "r1", BBox: model.BBox{400, 50, 700, 60}, Text: "header", Type: model.RegionTypeHeader},
				{ID: "r2", BBox: model.BBox{800, 150, 600, 400}, Text: "entry text", Type: model.RegionTypeEntry},
			},
			ReadingOrder: []string{"r1", "r2"},
		},
	}
}

// regionTranslateResponseJSON returns a mock region translation response JSON string.
func regionTranslateResponseJSON() string {
	return `{"regions":[{"id":"r1","translated_text":"translated header"},{"id":"r2","translated_text":"translated entry"}],"warnings":[]}`
}

func TestTranslatePipeline(t *testing.T) {
	stem := "testbook"
	pages := map[int]*model.SolvedRegionPage{
		1: makeSolvedRegionPage(1),
	}

	ws, cfg := setupTranslateWorkspace(t, stem, pages)

	_, err := Translate(context.Background(), TranslateOptions{
		Workspace: ws,
		Config:    cfg,
		Provider:  &mockProvider{response: regionTranslateResponseJSON()},
		Knowledge: &knowledge.Knowledge{},
	})
	if err != nil {
		t.Fatalf("Translate() error: %v", err)
	}

	// Verify translated JSON was created
	translatedPath := filepath.Join(ws.TranslateDir(), "tr", stem, "001.json")
	data, err := os.ReadFile(translatedPath)
	if err != nil {
		t.Fatalf("read translated output: %v", err)
	}

	var translated model.TranslatedRegionPage
	if err := json.Unmarshal(data, &translated); err != nil {
		t.Fatalf("unmarshal translated page: %v", err)
	}
	if translated.PageNumber != 1 {
		t.Errorf("expected page number 1, got %d", translated.PageNumber)
	}
	if translated.Version != "2.0" {
		t.Errorf("expected version 2.0, got %q", translated.Version)
	}
	if len(translated.Regions) != 2 {
		t.Errorf("expected 2 translated regions, got %d", len(translated.Regions))
	}
	if translated.Regions[0].TranslatedText != "translated header" {
		t.Errorf("expected translated text %q, got %q", "translated header", translated.Regions[0].TranslatedText)
	}
	if translated.TranslateModel != "mock/test-model" {
		t.Errorf("expected translate model %q, got %q", "mock/test-model", translated.TranslateModel)
	}
	if translated.SourceLang != "ar" {
		t.Errorf("expected source lang %q, got %q", "ar", translated.SourceLang)
	}
	if translated.TargetLang != "tr" {
		t.Errorf("expected target lang %q, got %q", "tr", translated.TargetLang)
	}
}

func TestTranslatePipelineNoSolvedPages(t *testing.T) {
	dir := t.TempDir()

	// Create solve/ but leave it empty (no subdirs)
	os.MkdirAll(filepath.Join(dir, "solve"), 0755)

	ws := &workspace.Workspace{Root: dir}
	cfg := &config.Config{
		Inputs: []config.InputSpec{{Path: "./input", Languages: []string{"ar"}}},
		Translate: config.TranslateConfig{
			Languages:     []string{"tr"},
			ContextWindow: 2,
		},
	}

	_, err := Translate(context.Background(), TranslateOptions{
		Workspace: ws,
		Config:    cfg,
		Provider:  &mockProvider{response: regionTranslateResponseJSON()},
		Knowledge: &knowledge.Knowledge{},
	})
	if err == nil {
		t.Fatal("expected error when no solved pages found")
	}
	expectedMsg := "no solved pages found in " + ws.SolveDir() + " (run solve first)"
	if err.Error() != expectedMsg {
		t.Errorf("unexpected error message:\n  got:  %q\n  want: %q", err.Error(), expectedMsg)
	}
}

func TestTranslatePipelineMultiLang(t *testing.T) {
	stem := "testbook"
	pages := map[int]*model.SolvedRegionPage{
		1: makeSolvedRegionPage(1),
	}

	ws, cfg := setupTranslateWorkspace(t, stem, pages)
	cfg.Translate.Languages = []string{"tr", "en"}

	_, err := Translate(context.Background(), TranslateOptions{
		Workspace: ws,
		Config:    cfg,
		Provider:  &mockProvider{response: regionTranslateResponseJSON()},
		Knowledge: &knowledge.Knowledge{},
	})
	if err != nil {
		t.Fatalf("Translate() error: %v", err)
	}

	// Verify translated JSON exists for both languages
	for _, lang := range []string{"tr", "en"} {
		translatedPath := filepath.Join(ws.TranslateDir(), lang, stem, "001.json")
		if _, err := os.Stat(translatedPath); err != nil {
			t.Errorf("expected translated output for lang %q at %s: %v", lang, translatedPath, err)
		}
	}
}
