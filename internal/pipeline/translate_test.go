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
	"github.com/mmdemirbas/mutercim/internal/progress"
	"github.com/mmdemirbas/mutercim/internal/workspace"
)

// setupTranslateWorkspace creates a workspace with solved page JSONs in midstate/solved/<stem>/.
func setupTranslateWorkspace(t *testing.T, stem string, pages map[int]*model.SolvedPage) (*workspace.Workspace, *config.Config, *progress.Tracker) {
	t.Helper()
	dir := t.TempDir()

	solvedDir := filepath.Join(dir, "midstate", "solved", stem)
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

	if err := os.WriteFile(filepath.Join(dir, "progress.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("write progress: %v", err)
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

	tracker := progress.NewTracker(ws.ProgressPath())
	if err := tracker.Load(); err != nil {
		t.Fatalf("load tracker: %v", err)
	}

	return ws, cfg, tracker
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

	ws, cfg, tracker := setupTranslateWorkspace(t, stem, pages)

	_, err := Translate(context.Background(), TranslateOptions{
		Workspace: ws,
		Config:    cfg,
		Provider:  &mockProvider{response: translateResponseJSON()},
		Knowledge: &knowledge.Knowledge{},
		Tracker:   tracker,
	})
	if err != nil {
		t.Fatalf("Translate() error: %v", err)
	}

	// Verify translated JSON was created
	translatedPath := filepath.Join(ws.TranslatedDir(), "tr", stem, "page_001.json")
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

	// Verify incremental markdown output was written
	mdPath := filepath.Join(ws.OutputDir(), "tr", "pages", stem, "page_001.md")
	if _, err := os.Stat(mdPath); err != nil {
		t.Errorf("expected markdown output at %s: %v", mdPath, err)
	}

	// Verify progress was updated
	state := tracker.State()
	phaseName := progress.PhaseName("translate:tr:" + stem)
	phase := state.Phases[phaseName]
	if phase == nil {
		t.Fatal("expected translate:tr:testbook phase in progress")
	}
	if !containsInt(phase.Completed, 1) {
		t.Error("expected page 1 in completed list")
	}
}

func TestTranslatePipelineNoSolvedPages(t *testing.T) {
	dir := t.TempDir()

	// Create midstate/solved/ but leave it empty (no subdirs)
	os.MkdirAll(filepath.Join(dir, "midstate", "solved"), 0755)

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
	tracker := progress.NewTracker(filepath.Join(dir, "progress.json"))

	_, err := Translate(context.Background(), TranslateOptions{
		Workspace: ws,
		Config:    cfg,
		Provider:  &mockProvider{response: translateResponseJSON()},
		Knowledge: &knowledge.Knowledge{},
		Tracker:   tracker,
	})
	if err == nil {
		t.Fatal("expected error when no solved pages found")
	}
	expectedMsg := "no solved pages found in " + ws.SolvedDir() + " (run solve first)"
	if err.Error() != expectedMsg {
		t.Errorf("unexpected error message:\n  got:  %q\n  want: %q", err.Error(), expectedMsg)
	}
}

func TestTranslatePipelineMultiLang(t *testing.T) {
	stem := "testbook"
	pages := map[int]*model.SolvedPage{
		1: makeSolvedPage(1),
	}

	ws, cfg, tracker := setupTranslateWorkspace(t, stem, pages)
	cfg.Book.TargetLangs = []string{"tr", "en"}

	_, err := Translate(context.Background(), TranslateOptions{
		Workspace: ws,
		Config:    cfg,
		Provider:  &mockProvider{response: translateResponseJSON()},
		Knowledge: &knowledge.Knowledge{},
		Tracker:   tracker,
	})
	if err != nil {
		t.Fatalf("Translate() error: %v", err)
	}

	// Verify translated JSON exists for both languages
	for _, lang := range []string{"tr", "en"} {
		translatedPath := filepath.Join(ws.TranslatedDir(), lang, stem, "page_001.json")
		if _, err := os.Stat(translatedPath); err != nil {
			t.Errorf("expected translated output for lang %q at %s: %v", lang, translatedPath, err)
		}

		// Verify markdown output for each language
		mdPath := filepath.Join(ws.OutputDir(), lang, "pages", stem, "page_001.md")
		if _, err := os.Stat(mdPath); err != nil {
			t.Errorf("expected markdown output for lang %q at %s: %v", lang, mdPath, err)
		}

		// Verify progress for each language
		phaseName := progress.PhaseName("translate:" + lang + ":" + stem)
		state := tracker.State()
		phase := state.Phases[phaseName]
		if phase == nil {
			t.Errorf("expected progress phase %q", phaseName)
			continue
		}
		if !containsInt(phase.Completed, 1) {
			t.Errorf("expected page 1 in completed list for lang %q", lang)
		}
	}
}
