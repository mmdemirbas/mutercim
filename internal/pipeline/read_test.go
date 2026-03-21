package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mmdemirbas/mutercim/internal/config"
	"github.com/mmdemirbas/mutercim/internal/model"
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

// setupReadWorkspace creates a workspace with images already in pages/<stem>/.
func setupReadWorkspace(t *testing.T, stem string, pageFiles ...string) (*workspace.Workspace, *config.Config) {
	t.Helper()
	dir := t.TempDir()

	imagesDir := filepath.Join(dir, "pages", stem)
	if err := os.MkdirAll(imagesDir, 0755); err != nil {
		t.Fatalf("mkdir images: %v", err)
	}

	for _, pf := range pageFiles {
		if err := os.WriteFile(filepath.Join(imagesDir, pf), []byte("fake-image"), 0644); err != nil {
			t.Fatalf("write image %s: %v", pf, err)
		}
	}

	// Create a minimal config file so mtime checks can reference it
	os.WriteFile(filepath.Join(dir, "mutercim.yaml"), []byte("book:\n  title: test\n"), 0644)
	os.MkdirAll(filepath.Join(dir, "knowledge"), 0755)

	ws := &workspace.Workspace{Root: dir}
	cfg := &config.Config{
		Inputs: []config.InputSpec{{Path: "./input/" + stem + ".pdf"}},
		Pages:  config.PagesConfig{DPI: 300},
		Read: config.ReadConfig{
			Models:      []config.ModelSpec{{Provider: "mock", Model: "test-model"}},
			Concurrency: 1,
			Retry:       config.RetryConfig{MaxAttempts: 1, BackoffSeconds: 1},
		},
	}

	return ws, cfg
}

func TestReadPipeline(t *testing.T) {
	ws, cfg := setupReadWorkspace(t, "testinput", "001.png")

	// Provider returns region-based response (new format)
	response := `{
		"regions": [
			{"id": "r1", "bbox": [400, 50, 700, 60], "text": "باب الألف", "type": "header"},
			{"id": "r2", "bbox": [800, 150, 600, 400], "text": "test hadith", "type": "entry"}
		],
		"reading_order": ["r1", "r2"],
		"warnings": []
	}`

	result, err := Read(context.Background(), ReadOptions{
		Workspace: ws,
		Config:    cfg,
		Provider:  &mockProvider{response: response},
	})
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	// Verify output file was created with v2.0 format
	outputPath := filepath.Join(ws.ReadDir(), "testinput", "001.json")
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	var page model.RegionPage
	if err := json.Unmarshal(data, &page); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if page.Version != "2.0" {
		t.Errorf("expected version 2.0, got %q", page.Version)
	}
	if page.PageNumber != 1 {
		t.Errorf("expected page number 1, got %d", page.PageNumber)
	}
	if len(page.Regions) != 2 {
		t.Errorf("expected 2 regions, got %d", len(page.Regions))
	}

	// Verify PhaseResult counts
	if result.Completed != 1 {
		t.Errorf("expected result.Completed=1, got %d", result.Completed)
	}
}

func TestReadPipelineSkipsCompleted(t *testing.T) {
	ws, cfg := setupReadWorkspace(t, "testinput", "001.png")

	// Set all inputs to the past so output appears newer
	past := time.Now().Add(-10 * time.Second)
	imgPath := filepath.Join(ws.PagesDir(), "testinput", "001.png")
	os.Chtimes(imgPath, past, past)
	os.MkdirAll(ws.KnowledgeDir(), 0755)
	os.Chtimes(ws.KnowledgeDir(), past, past)
	os.Chtimes(ws.ConfigPath(), past, past)

	// Create the output file so skip logic sees it as up-to-date
	outputDir := filepath.Join(ws.ReadDir(), "testinput")
	os.MkdirAll(outputDir, 0755)
	outputPath := filepath.Join(outputDir, "001.json")
	os.WriteFile(outputPath, []byte(`{"version":"1.0","page_number":1}`), 0644)

	_, err := Read(context.Background(), ReadOptions{
		Workspace: ws,
		Config:    cfg,
		Provider:  &mockProvider{response: `{"regions":[],"reading_order":[],"warnings":[]}`},
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

	// pages/ exists but is empty (no subdirs)
	os.MkdirAll(filepath.Join(dir, "pages"), 0755)

	ws := &workspace.Workspace{Root: dir}
	cfg := &config.Config{}

	_, err := Read(context.Background(), ReadOptions{
		Workspace: ws,
		Config:    cfg,
		Provider:  &mockProvider{response: "{}"},
	})
	if err == nil {
		t.Fatal("expected error when no images found")
	}
	if got := err.Error(); got != "no page images found in "+ws.PagesDir()+" — run 'mutercim pages' first" {
		t.Errorf("unexpected error: %q", got)
	}
}

func TestReadPipelineMissingImagesDir(t *testing.T) {
	dir := t.TempDir()

	// pages/ doesn't exist at all
	ws := &workspace.Workspace{Root: dir}
	cfg := &config.Config{}

	_, err := Read(context.Background(), ReadOptions{
		Workspace: ws,
		Config:    cfg,
		Provider:  &mockProvider{response: "{}"},
	})
	if err == nil {
		t.Fatal("expected error when images dir missing")
	}
}

func TestReadPipelinePerInputPages(t *testing.T) {
	ws, cfg := setupReadWorkspace(t, "testinput", "001.png", "002.png")
	// Per-input pages: only process page 1
	cfg.Inputs = []config.InputSpec{{Path: "./input/testinput.pdf", Pages: "1"}}

	response := `{"regions": [], "reading_order": [], "warnings": []}`

	_, err := Read(context.Background(), ReadOptions{
		Workspace: ws,
		Config:    cfg,
		Provider:  &mockProvider{response: response},
	})
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	page1 := filepath.Join(ws.ReadDir(), "testinput", "001.json")
	if _, err := os.Stat(page1); err != nil {
		t.Errorf("expected page 1 output, got error: %v", err)
	}

	page2 := filepath.Join(ws.ReadDir(), "testinput", "002.json")
	if _, err := os.Stat(page2); err == nil {
		t.Error("page 2 should not be processed when per-input pages is '1'")
	}
}

func TestReadPipelineCLIPagesOverridePerInput(t *testing.T) {
	ws, cfg := setupReadWorkspace(t, "testinput", "001.png", "002.png")
	cfg.Inputs = []config.InputSpec{{Path: "./input/testinput.pdf", Pages: "1"}}

	response := `{"regions": [], "reading_order": [], "warnings": []}`

	// CLI override: process page 2 only
	_, err := Read(context.Background(), ReadOptions{
		Workspace: ws,
		Config:    cfg,
		Provider:  &mockProvider{response: response},
		Pages:     []int{2},
	})
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	page1 := filepath.Join(ws.ReadDir(), "testinput", "001.json")
	if _, err := os.Stat(page1); err == nil {
		t.Error("page 1 should not be processed when CLI pages override is [2]")
	}

	page2 := filepath.Join(ws.ReadDir(), "testinput", "002.json")
	if _, err := os.Stat(page2); err != nil {
		t.Errorf("expected page 2 output, got error: %v", err)
	}
}

func TestReadPipelineMultiInput(t *testing.T) {
	dir := t.TempDir()

	// Create image directories for two stems
	for _, stem := range []string{"stem1", "stem2"} {
		imagesDir := filepath.Join(dir, "pages", stem)
		if err := os.MkdirAll(imagesDir, 0755); err != nil {
			t.Fatalf("mkdir images for %s: %v", stem, err)
		}
		if err := os.WriteFile(filepath.Join(imagesDir, "001.png"), []byte("fake-image"), 0644); err != nil {
			t.Fatalf("write image for %s: %v", stem, err)
		}
	}

	ws := &workspace.Workspace{Root: dir}
	cfg := &config.Config{
		Inputs: []config.InputSpec{
			{Path: "./input/stem1.pdf"},
			{Path: "./input/stem2.pdf"},
		},
		Pages: config.PagesConfig{DPI: 300},
		Read: config.ReadConfig{
			Models:      []config.ModelSpec{{Provider: "mock", Model: "test-model"}},
			Concurrency: 1,
			Retry:       config.RetryConfig{MaxAttempts: 1, BackoffSeconds: 1},
		},
	}

	response := `{
		"regions": [
			{"id": "r1", "bbox": [0, 0, 100, 50], "text": "test", "type": "entry"}
		],
		"reading_order": ["r1"],
		"warnings": []
	}`

	_, err := Read(context.Background(), ReadOptions{
		Workspace: ws,
		Config:    cfg,
		Provider:  &mockProvider{response: response},
	})
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	// Verify output files exist with v2.0 format for both stems
	for _, stem := range []string{"stem1", "stem2"} {
		outputPath := filepath.Join(ws.ReadDir(), stem, "001.json")
		data, err := os.ReadFile(outputPath)
		if err != nil {
			t.Fatalf("read output for %s: %v", stem, err)
		}

		var page model.RegionPage
		if err := json.Unmarshal(data, &page); err != nil {
			t.Fatalf("unmarshal output for %s: %v", stem, err)
		}
		if page.PageNumber != 1 {
			t.Errorf("%s: expected page number 1, got %d", stem, page.PageNumber)
		}
		if page.Version != "2.0" {
			t.Errorf("%s: expected version 2.0, got %q", stem, page.Version)
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
	ws, cfg := setupReadWorkspace(t, "testinput", "001.png", "002.png")

	// Provider that always fails
	result, err := Read(context.Background(), ReadOptions{
		Workspace: ws,
		Config:    cfg,
		Provider:  &failingProvider{},
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

func TestSaveRegionPage(t *testing.T) {
	dir := t.TempDir()

	page := &model.RegionPage{
		Version:    "2.0",
		PageNumber: 5,
		PageSize:   model.PageSize{Width: 1500, Height: 2200},
		Regions: []model.Region{
			{ID: "r1", BBox: model.BBox{0, 0, 100, 50}, Text: "test", Type: model.RegionTypeEntry},
		},
		ReadingOrder: []string{"r1"},
	}

	if err := saveRegionPage(dir, 5, 100, page); err != nil {
		t.Fatalf("saveRegionPage() error: %v", err)
	}

	path := filepath.Join(dir, "005.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	var loaded model.RegionPage
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if loaded.PageNumber != 5 {
		t.Errorf("expected page 5, got %d", loaded.PageNumber)
	}
	if loaded.Version != "2.0" {
		t.Errorf("expected version 2.0, got %q", loaded.Version)
	}
	if len(loaded.Regions) != 1 {
		t.Errorf("expected 1 region, got %d", len(loaded.Regions))
	}

	// Verify no .tmp file left behind
	tmpPath := path + ".tmp"
	if _, err := os.Stat(tmpPath); err == nil {
		t.Error("tmp file should not exist after successful save")
	}
}

func TestCountRegionTypes(t *testing.T) {
	regions := []model.Region{
		{Type: model.RegionTypeEntry},
		{Type: model.RegionTypeEntry},
		{Type: model.RegionTypeHeader},
		{Type: model.RegionTypeFootnote},
		{Type: model.RegionTypeSeparator},
		{Type: model.RegionTypeFootnote},
		{Type: model.RegionTypeEntry},
	}

	entries, footnotes := countRegionTypes(regions)
	if entries != 3 {
		t.Errorf("entries = %d, want 3", entries)
	}
	if footnotes != 2 {
		t.Errorf("footnotes = %d, want 2", footnotes)
	}

	// Empty list
	e, f := countRegionTypes(nil)
	if e != 0 || f != 0 {
		t.Errorf("empty: entries=%d, footnotes=%d, want 0,0", e, f)
	}
}
