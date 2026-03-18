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

type mockProvider struct {
	response string
}

func (m *mockProvider) Name() string         { return "mock" }
func (m *mockProvider) SupportsVision() bool { return true }
func (m *mockProvider) ExtractFromImage(ctx context.Context, image []byte, systemPrompt, userPrompt string) (string, error) {
	return m.response, nil
}
func (m *mockProvider) Translate(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return m.response, nil
}

func setupTestWorkspace(t *testing.T) (*workspace.Workspace, *config.Config, *progress.Tracker) {
	t.Helper()
	dir := t.TempDir()

	// Create workspace directories
	for _, d := range []string{"input", "cache/images", "cache/extracted"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}

	// Create config file
	cfgContent := "book:\n  title: Test\ninput: ./input\n"
	if err := os.WriteFile(filepath.Join(dir, "mutercim.yaml"), []byte(cfgContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// Create a test image in a per-input subdir
	imagesDir := filepath.Join(dir, "cache/images/testinput")
	if err := os.MkdirAll(imagesDir, 0755); err != nil {
		t.Fatalf("mkdir images: %v", err)
	}
	if err := os.WriteFile(filepath.Join(imagesDir, "page-01.png"), []byte("fake-image"), 0644); err != nil {
		t.Fatalf("write image: %v", err)
	}

	// Create progress file
	if err := os.WriteFile(filepath.Join(dir, "progress.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("write progress: %v", err)
	}

	ws := &workspace.Workspace{Root: dir}
	cfg := &config.Config{
		Inputs:   []string{imagesDir},
		CacheDir: "./cache",
		DPI:      300,
		Extract: config.ExtractConfig{
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

func TestExtractPipeline(t *testing.T) {
	ws, cfg, tracker := setupTestWorkspace(t)

	response := `{
		"page_number": 1,
		"entries": [{"number": 1, "type": "hadith", "arabic_text": "test", "is_continuation": false, "continues_on_next_page": false}],
		"footnotes": [],
		"warnings": []
	}`

	err := Extract(context.Background(), ExtractOptions{
		Workspace: ws,
		Config:    cfg,
		Provider:  &mockProvider{response: response},
		Tracker:   tracker,
		Logger:    nil,
	})
	if err != nil {
		t.Fatalf("Extract() error: %v", err)
	}

	// Verify output file was created in per-input subdir
	outputPath := filepath.Join(ws.ExtractedDir(), "testinput", "page_001.json")
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	var page model.ExtractedPage
	if err := json.Unmarshal(data, &page); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if page.PageNumber != 1 {
		t.Errorf("expected page number 1, got %d", page.PageNumber)
	}
	if len(page.Entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(page.Entries))
	}

	// Verify progress was updated with compound phase name
	state := tracker.State()
	phase := state.Phases["extract:testinput"]
	if phase == nil {
		t.Fatal("expected extract:testinput phase in progress")
	}
	if !containsInt(phase.Completed, 1) {
		t.Error("expected page 1 in completed list")
	}
}

func TestExtractPipelineSkipsCompleted(t *testing.T) {
	ws, cfg, tracker := setupTestWorkspace(t)

	// Mark page as already completed using compound phase name
	tracker.MarkCompleted("extract:testinput", 1)
	if err := tracker.Save(); err != nil {
		t.Fatalf("save tracker: %v", err)
	}

	err := Extract(context.Background(), ExtractOptions{
		Workspace: ws,
		Config:    cfg,
		Provider:  &mockProvider{response: "{}"},
		Tracker:   tracker,
		Logger:    nil,
	})
	if err != nil {
		t.Fatalf("Extract() error: %v", err)
	}

	// Output file should NOT exist since we skipped it
	outputPath := filepath.Join(ws.ExtractedDir(), "testinput", "page_001.json")
	if _, err := os.Stat(outputPath); err == nil {
		t.Error("expected no output file for already completed page")
	}
}

func TestExtractPipelineNoImages(t *testing.T) {
	dir := t.TempDir()

	// Create empty images dir
	emptyDir := filepath.Join(dir, "cache/images/empty")
	if err := os.MkdirAll(emptyDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	ws := &workspace.Workspace{Root: dir}
	cfg := &config.Config{Inputs: []string{emptyDir}}
	tracker := progress.NewTracker(filepath.Join(dir, "progress.json"))

	err := Extract(context.Background(), ExtractOptions{
		Workspace: ws,
		Config:    cfg,
		Provider:  &mockProvider{response: "{}"},
		Tracker:   tracker,
	})
	// Extract logs per-input errors but doesn't fail the overall run
	if err != nil {
		t.Fatalf("Extract() unexpected error: %v", err)
	}

	// No output files should exist
	outputPath := filepath.Join(ws.ExtractedDir(), "empty", "page_001.json")
	if _, err := os.Stat(outputPath); err == nil {
		t.Error("expected no output files for empty input")
	}
}

func TestSaveExtractedPage(t *testing.T) {
	dir := t.TempDir()

	page := &model.ExtractedPage{
		Version:    "1.0",
		PageNumber: 5,
		Entries:    []model.Entry{{Type: "hadith", ArabicText: "test"}},
	}

	if err := saveExtractedPage(dir, 5, page); err != nil {
		t.Fatalf("saveExtractedPage() error: %v", err)
	}

	// Verify file exists with correct name
	path := filepath.Join(dir, "page_005.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	var loaded model.ExtractedPage
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
