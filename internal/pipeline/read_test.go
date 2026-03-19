package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mmdemirbas/mutercim/internal/config"
	"github.com/mmdemirbas/mutercim/internal/model"
	"github.com/mmdemirbas/mutercim/internal/progress"
	"github.com/mmdemirbas/mutercim/internal/workspace"
)

type mockProvider struct {
	response string
}

func (m *mockProvider) Name() string         { return "mock" }
func (m *mockProvider) SupportsVision() bool { return true }
func (m *mockProvider) ReadFromImage(ctx context.Context, image []byte, systemPrompt, userPrompt string) (string, error) {
	return m.response, nil
}
func (m *mockProvider) Translate(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return m.response, nil
}

// setupReadWorkspace creates a workspace with images already in midstate/images/<stem>/.
func setupReadWorkspace(t *testing.T, stem string, pageFiles ...string) (*workspace.Workspace, *config.Config, *progress.Tracker) {
	t.Helper()
	dir := t.TempDir()

	imagesDir := filepath.Join(dir, "midstate", "images", stem)
	if err := os.MkdirAll(imagesDir, 0755); err != nil {
		t.Fatalf("mkdir images: %v", err)
	}

	for _, pf := range pageFiles {
		if err := os.WriteFile(filepath.Join(imagesDir, pf), []byte("fake-image"), 0644); err != nil {
			t.Fatalf("write image %s: %v", pf, err)
		}
	}

	if err := os.WriteFile(filepath.Join(dir, "progress.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("write progress: %v", err)
	}

	ws := &workspace.Workspace{Root: dir}
	cfg := &config.Config{
		Inputs:      []config.InputSpec{{Path: "./input/" + stem + ".pdf"}},
		MidstateDir: "./midstate",
		DPI:         300,
		Read: config.ReadConfig{
			Provider:    "mock",
			Model:       "test-model",
			Concurrency: 1,
		},
		Retry:     config.RetryConfig{MaxAttempts: 1, BackoffSeconds: 1},
		RateLimit: config.RateLimitConfig{RequestsPerMinute: 100},
	}

	tracker := progress.NewTracker(ws.ProgressPath())
	if err := tracker.Load(); err != nil {
		t.Fatalf("load tracker: %v", err)
	}

	return ws, cfg, tracker
}

func TestReadPipeline(t *testing.T) {
	ws, cfg, tracker := setupReadWorkspace(t, "testinput", "page-01.png")

	response := `{
		"page_number": 1,
		"entries": [{"number": 1, "type": "hadith", "arabic_text": "test", "is_continuation": false, "continues_on_next_page": false}],
		"footnotes": [],
		"warnings": []
	}`

	result, err := Read(context.Background(), ReadOptions{
		Workspace: ws,
		Config:    cfg,
		Provider:  &mockProvider{response: response},
		Tracker:   tracker,
	})
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	// Verify output file was created
	outputPath := filepath.Join(ws.ReadDir(), "testinput", "page_001.json")
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	var page model.ReadPage
	if err := json.Unmarshal(data, &page); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if page.PageNumber != 1 {
		t.Errorf("expected page number 1, got %d", page.PageNumber)
	}
	if len(page.Entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(page.Entries))
	}

	// Verify progress was updated
	state := tracker.State()
	phase := state.Phases["read:testinput"]
	if phase == nil {
		t.Fatal("expected read:testinput phase in progress")
	}
	if !containsInt(phase.Completed, 1) {
		t.Error("expected page 1 in completed list")
	}

	// Verify PhaseResult counts
	if result.Completed != 1 {
		t.Errorf("expected result.Completed=1, got %d", result.Completed)
	}
}

func TestReadPipelineSkipsCompleted(t *testing.T) {
	ws, cfg, tracker := setupReadWorkspace(t, "testinput", "page-01.png")

	tracker.MarkCompleted("read:testinput", 1)
	if err := tracker.Save(); err != nil {
		t.Fatalf("save tracker: %v", err)
	}

	// Create the output file so skip logic sees it as truly complete
	outputDir := filepath.Join(ws.ReadDir(), "testinput")
	os.MkdirAll(outputDir, 0755)
	outputPath := filepath.Join(outputDir, "page_001.json")
	os.WriteFile(outputPath, []byte(`{"version":"1.0","page_number":1}`), 0644)

	_, err := Read(context.Background(), ReadOptions{
		Workspace: ws,
		Config:    cfg,
		Provider:  &mockProvider{response: `{"entries":[],"footnotes":[],"warnings":[]}`},
		Tracker:   tracker,
	})
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	// Output should still contain original content (not re-processed)
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if string(data) != `{"version":"1.0","page_number":1}` {
		t.Error("output was overwritten — page should have been skipped")
	}
}

func TestReadPipelineNoImages(t *testing.T) {
	dir := t.TempDir()

	// midstate/images/ exists but is empty (no subdirs)
	os.MkdirAll(filepath.Join(dir, "midstate", "images"), 0755)

	ws := &workspace.Workspace{Root: dir}
	cfg := &config.Config{}
	tracker := progress.NewTracker(filepath.Join(dir, "progress.json"))

	_, err := Read(context.Background(), ReadOptions{
		Workspace: ws,
		Config:    cfg,
		Provider:  &mockProvider{response: "{}"},
		Tracker:   tracker,
	})
	if err == nil {
		t.Fatal("expected error when no images found")
	}
	if got := err.Error(); got != "no page images found in "+ws.ImagesDir()+" — run 'mutercim pages' first" {
		t.Errorf("unexpected error: %q", got)
	}
}

func TestReadPipelineMissingImagesDir(t *testing.T) {
	dir := t.TempDir()

	// midstate/images/ doesn't exist at all
	ws := &workspace.Workspace{Root: dir}
	cfg := &config.Config{}
	tracker := progress.NewTracker(filepath.Join(dir, "progress.json"))

	_, err := Read(context.Background(), ReadOptions{
		Workspace: ws,
		Config:    cfg,
		Provider:  &mockProvider{response: "{}"},
		Tracker:   tracker,
	})
	if err == nil {
		t.Fatal("expected error when images dir missing")
	}
}

func TestReadPipelinePerInputPages(t *testing.T) {
	ws, cfg, tracker := setupReadWorkspace(t, "testinput", "page-01.png", "page-02.png")
	// Per-input pages: only process page 1
	cfg.Inputs = []config.InputSpec{{Path: "./input/testinput.pdf", Pages: "1"}}

	response := `{"page_number": 1, "entries": [], "footnotes": [], "warnings": []}`

	_, err := Read(context.Background(), ReadOptions{
		Workspace: ws,
		Config:    cfg,
		Provider:  &mockProvider{response: response},
		Tracker:   tracker,
	})
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	page1 := filepath.Join(ws.ReadDir(), "testinput", "page_001.json")
	if _, err := os.Stat(page1); err != nil {
		t.Errorf("expected page 1 output, got error: %v", err)
	}

	page2 := filepath.Join(ws.ReadDir(), "testinput", "page_002.json")
	if _, err := os.Stat(page2); err == nil {
		t.Error("page 2 should not be processed when per-input pages is '1'")
	}
}

func TestReadPipelineCLIPagesOverridePerInput(t *testing.T) {
	ws, cfg, tracker := setupReadWorkspace(t, "testinput", "page-01.png", "page-02.png")
	cfg.Inputs = []config.InputSpec{{Path: "./input/testinput.pdf", Pages: "1"}}

	response := `{"page_number": 2, "entries": [], "footnotes": [], "warnings": []}`

	// CLI override: process page 2 only
	_, err := Read(context.Background(), ReadOptions{
		Workspace: ws,
		Config:    cfg,
		Provider:  &mockProvider{response: response},
		Tracker:   tracker,
		Pages:     []int{2},
	})
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	page1 := filepath.Join(ws.ReadDir(), "testinput", "page_001.json")
	if _, err := os.Stat(page1); err == nil {
		t.Error("page 1 should not be processed when CLI pages override is [2]")
	}

	page2 := filepath.Join(ws.ReadDir(), "testinput", "page_002.json")
	if _, err := os.Stat(page2); err != nil {
		t.Errorf("expected page 2 output, got error: %v", err)
	}
}

func TestReadPipelineMultiInput(t *testing.T) {
	dir := t.TempDir()

	// Create image directories for two stems
	for _, stem := range []string{"stem1", "stem2"} {
		imagesDir := filepath.Join(dir, "midstate", "images", stem)
		if err := os.MkdirAll(imagesDir, 0755); err != nil {
			t.Fatalf("mkdir images for %s: %v", stem, err)
		}
		if err := os.WriteFile(filepath.Join(imagesDir, "page-01.png"), []byte("fake-image"), 0644); err != nil {
			t.Fatalf("write image for %s: %v", stem, err)
		}
	}

	if err := os.WriteFile(filepath.Join(dir, "progress.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("write progress: %v", err)
	}

	ws := &workspace.Workspace{Root: dir}
	cfg := &config.Config{
		Inputs: []config.InputSpec{
			{Path: "./input/stem1.pdf"},
			{Path: "./input/stem2.pdf"},
		},
		MidstateDir: "./midstate",
		DPI:         300,
		Read: config.ReadConfig{
			Provider:    "mock",
			Model:       "test-model",
			Concurrency: 1,
		},
		Retry:     config.RetryConfig{MaxAttempts: 1, BackoffSeconds: 1},
		RateLimit: config.RateLimitConfig{RequestsPerMinute: 100},
	}

	tracker := progress.NewTracker(ws.ProgressPath())
	if err := tracker.Load(); err != nil {
		t.Fatalf("load tracker: %v", err)
	}

	response := `{
		"page_number": 1,
		"entries": [{"number": 1, "type": "hadith", "arabic_text": "test", "is_continuation": false, "continues_on_next_page": false}],
		"footnotes": [],
		"warnings": []
	}`

	_, err := Read(context.Background(), ReadOptions{
		Workspace: ws,
		Config:    cfg,
		Provider:  &mockProvider{response: response},
		Tracker:   tracker,
	})
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	// Verify output files exist and have correct page numbers for both stems
	for _, stem := range []string{"stem1", "stem2"} {
		outputPath := filepath.Join(ws.ReadDir(), stem, "page_001.json")
		data, err := os.ReadFile(outputPath)
		if err != nil {
			t.Fatalf("read output for %s: %v", stem, err)
		}

		var page model.ReadPage
		if err := json.Unmarshal(data, &page); err != nil {
			t.Fatalf("unmarshal output for %s: %v", stem, err)
		}
		if page.PageNumber != 1 {
			t.Errorf("%s: expected page number 1, got %d", stem, page.PageNumber)
		}
	}

	// Verify progress has both "read:stem1" and "read:stem2" phases
	state := tracker.State()
	for _, stem := range []string{"stem1", "stem2"} {
		phaseName := progress.PhaseName("read:" + stem)
		phase := state.Phases[phaseName]
		if phase == nil {
			t.Fatalf("expected %s phase in progress", phaseName)
		}
		if !containsInt(phase.Completed, 1) {
			t.Errorf("expected page 1 in completed list for %s", phaseName)
		}
	}
}

type failingProvider struct{}

func (m *failingProvider) Name() string         { return "failing" }
func (m *failingProvider) SupportsVision() bool { return true }
func (m *failingProvider) ReadFromImage(ctx context.Context, image []byte, systemPrompt, userPrompt string) (string, error) {
	return "", fmt.Errorf("mock API failure")
}
func (m *failingProvider) Translate(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return "", fmt.Errorf("mock API failure")
}

func TestReadPipeline_AllPagesFail_ReturnsZeroCompleted(t *testing.T) {
	ws, cfg, tracker := setupReadWorkspace(t, "testinput", "page-01.png", "page-02.png")

	// Provider that always fails
	result, err := Read(context.Background(), ReadOptions{
		Workspace: ws,
		Config:    cfg,
		Provider:  &failingProvider{},
		Tracker:   tracker,
	})
	if err != nil {
		t.Fatalf("Read() should not return error (individual pages fail gracefully), got: %v", err)
	}
	if result.Completed != 0 {
		t.Errorf("expected 0 completed, got %d", result.Completed)
	}
	if result.Failed != 2 {
		t.Errorf("expected 2 failed, got %d", result.Failed)
	}
}

func TestSaveReadPage(t *testing.T) {
	dir := t.TempDir()

	page := &model.ReadPage{
		Version:    "1.0",
		PageNumber: 5,
		Entries:    []model.Entry{{Type: "hadith", ArabicText: "test"}},
	}

	if err := saveReadPage(dir, 5, page); err != nil {
		t.Fatalf("saveReadPage() error: %v", err)
	}

	path := filepath.Join(dir, "page_005.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	var loaded model.ReadPage
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if loaded.PageNumber != 5 {
		t.Errorf("expected page 5, got %d", loaded.PageNumber)
	}

	// Verify no .tmp file left behind
	tmpPath := path + ".tmp"
	if _, err := os.Stat(tmpPath); err == nil {
		t.Error("tmp file should not exist after successful save")
	}
}
