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

// setupTranslateWorkspace creates a workspace with solved page JSONs in solve/<stem>/.
func setupTranslateWorkspace(t *testing.T, stem string, pages map[int]*model.SolvedPage) (*workspace.Workspace, *config.Config) {
	t.Helper()
	dir := t.TempDir()

	solvedDir := filepath.Join(dir, "solve", stem)
	if err := os.MkdirAll(solvedDir, 0755); err != nil {
		t.Fatalf("mkdir solved: %v", err)
	}

	for pageNum, page := range pages {
		data, err := json.MarshalIndent(page, "", "  ")
		if err != nil {
			t.Fatalf("marshal solved page %d: %v", pageNum, err)
		}
		filename := filepath.Join(solvedDir, pageName(pageNum))
		if err := os.WriteFile(filename, data, 0644); err != nil {
			t.Fatalf("write solved page %d: %v", pageNum, err)
		}
	}

	ws := &workspace.Workspace{Root: dir}
	cfg := &config.Config{
		Book: model.Book{
			SourceLangs: []string{"ar"},
			TargetLangs: []string{"tr"},
		},
		Translate: config.TranslateConfig{
			Models:        []config.ModelSpec{{Provider: "mock", Model: "test-model"}},
			ContextWindow: 2,
		},
		Write: config.WriteConfig{
			ExpandSources: true,
		},
	}

	return ws, cfg
}

// makeSolvedPage creates a minimal SolvedPage for testing.
func makeSolvedPage(pageNum int) *model.SolvedPage {
	entryNum := 1
	return &model.SolvedPage{
		ReadPage: model.ReadPage{
			Version:     "1.0",
			PageNumber:  pageNum,
			SectionType: "scholarly_entries",
			Entries: []model.Entry{
				{
					Number:     &entryNum,
					Type:       "hadith",
					ArabicText: "test arabic text",
				},
			},
		},
	}
}

// translateResponseJSON returns a mock translation response JSON string.
func translateResponseJSON() string {
	return `{"translated_entries":[{"number":1,"translated_text":"test translation","translator_notes":""}],"warnings":[]}`
}

func TestTranslatePipeline(t *testing.T) {
	stem := "testbook"
	pages := map[int]*model.SolvedPage{
		1: makeSolvedPage(1),
	}

	ws, cfg := setupTranslateWorkspace(t, stem, pages)

	_, err := Translate(context.Background(), TranslateOptions{
		Workspace: ws,
		Config:    cfg,
		Provider:  &mockProvider{response: translateResponseJSON()},
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

	var translated model.TranslatedPage
	if err := json.Unmarshal(data, &translated); err != nil {
		t.Fatalf("unmarshal translated page: %v", err)
	}
	if translated.PageNumber != 1 {
		t.Errorf("expected page number 1, got %d", translated.PageNumber)
	}
	if len(translated.TranslatedEntries) != 1 {
		t.Errorf("expected 1 translated entry, got %d", len(translated.TranslatedEntries))
	}
	if translated.TranslatedEntries[0].TranslatedText != "test translation" {
		t.Errorf("expected translated text %q, got %q", "test translation", translated.TranslatedEntries[0].TranslatedText)
	}
	if translated.TranslationModel != "mock/test-model" {
		t.Errorf("expected translation model %q, got %q", "mock/test-model", translated.TranslationModel)
	}
}

func TestTranslatePipelineNoSolvedPages(t *testing.T) {
	dir := t.TempDir()

	// Create solve/ but leave it empty (no subdirs)
	os.MkdirAll(filepath.Join(dir, "solve"), 0755)

	ws := &workspace.Workspace{Root: dir}
	cfg := &config.Config{
		Book: model.Book{
			SourceLangs: []string{"ar"},
			TargetLangs: []string{"tr"},
		},
		Translate: config.TranslateConfig{
			ContextWindow: 2,
		},
	}

	_, err := Translate(context.Background(), TranslateOptions{
		Workspace: ws,
		Config:    cfg,
		Provider:  &mockProvider{response: translateResponseJSON()},
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
	pages := map[int]*model.SolvedPage{
		1: makeSolvedPage(1),
	}

	ws, cfg := setupTranslateWorkspace(t, stem, pages)
	cfg.Book.TargetLangs = []string{"tr", "en"}

	_, err := Translate(context.Background(), TranslateOptions{
		Workspace: ws,
		Config:    cfg,
		Provider:  &mockProvider{response: translateResponseJSON()},
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
