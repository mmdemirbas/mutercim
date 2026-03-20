package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mmdemirbas/mutercim/internal/config"
	"github.com/mmdemirbas/mutercim/internal/knowledge"
	"github.com/mmdemirbas/mutercim/internal/model"
)

func TestReadPipeline_ContextCancelled(t *testing.T) {
	ws, cfg := setupReadWorkspace(t, "testinput", "page-01.png", "page-02.png", "page-03.png")

	response := `{"regions": [{"id": "r1", "bbox": [0,0,100,100], "text": "text", "type": "entry"}], "reading_order": ["r1"], "warnings": []}`

	// Cancel context immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := Read(ctx, ReadOptions{
		Workspace: ws,
		Config:    cfg,
		Provider:  &mockProvider{response: response},
	})
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	// With cancelled context, should process zero pages
	if result.Completed != 0 {
		t.Errorf("expected 0 completed with cancelled context, got %d", result.Completed)
	}
}

func TestReadPipeline_ForceReprocesses(t *testing.T) {
	ws, cfg := setupReadWorkspace(t, "testinput", "page-01.png")

	response := `{"regions": [{"id": "r1", "bbox": [0,0,100,100], "text": "new text", "type": "entry"}], "reading_order": ["r1"], "warnings": []}`

	// Create existing output
	outputDir := filepath.Join(ws.ReadDir(), "testinput")
	os.MkdirAll(outputDir, 0755)
	os.WriteFile(filepath.Join(outputDir, "001.json"), []byte(`{"version":"1.0"}`), 0644)

	result, err := Read(context.Background(), ReadOptions{
		Workspace: ws,
		Config:    cfg,
		Provider:  &mockProvider{response: response},
		Force:     true,
	})
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	if result.Completed != 1 {
		t.Errorf("expected 1 completed with --force, got %d", result.Completed)
	}
}

func TestBuildInputPageMap(t *testing.T) {
	cfg := &config.Config{
		Inputs: []config.InputSpec{
			{Path: "./input/book1.pdf", Pages: "1-5"},
			{Path: "./input/book2.pdf", Pages: "10,20"},
			{Path: "./input/book3.pdf"}, // no pages restriction
		},
	}

	m := buildInputPageMap(cfg)

	if pages, ok := m["book1"]; !ok {
		t.Error("expected book1 in map")
	} else if len(pages) != 5 {
		t.Errorf("book1 pages = %v, want 5 pages", pages)
	}

	if pages, ok := m["book2"]; !ok {
		t.Error("expected book2 in map")
	} else if len(pages) != 2 {
		t.Errorf("book2 pages = %v, want 2 pages", pages)
	}

	if _, ok := m["book3"]; ok {
		t.Error("book3 should not be in map (no pages restriction)")
	}
}

func TestBuildInputPageMap_InvalidPages(t *testing.T) {
	cfg := &config.Config{
		Inputs: []config.InputSpec{
			{Path: "./input/book.pdf", Pages: "invalid"},
		},
	}

	m := buildInputPageMap(cfg)
	if _, ok := m["book"]; ok {
		t.Error("invalid pages should not create map entry")
	}
}

func TestBuildInputPageMap_Empty(t *testing.T) {
	cfg := &config.Config{}
	m := buildInputPageMap(cfg)
	if len(m) != 0 {
		t.Errorf("expected empty map, got %v", m)
	}
}

func TestReadPipeline_PageNotInImageSet(t *testing.T) {
	ws, cfg := setupReadWorkspace(t, "testinput", "page-01.png")

	response := `{"regions": [], "reading_order": [], "warnings": []}`

	// Request page 99 which doesn't have an image
	result, err := Read(context.Background(), ReadOptions{
		Workspace: ws,
		Config:    cfg,
		Provider:  &mockProvider{response: response},
		Pages:     []int{99},
	})
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	// Page 99 has no image, should be skipped (not counted as failed)
	if result.Completed != 0 {
		t.Errorf("expected 0 completed, got %d", result.Completed)
	}
}

func TestSolvePipeline_ContextCancelled(t *testing.T) {
	ws := setupSolveWorkspace(t, "testbook", map[int]*model.RegionPage{
		1: makeRegionPage(1),
		2: makeRegionPage(2),
		3: makeRegionPage(3),
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := Solve(ctx, SolveOptions{
		Workspace: ws,
		Knowledge: &knowledge.Knowledge{},
	})
	if err != nil {
		t.Fatalf("Solve() error: %v", err)
	}

	if result.Completed != 0 {
		t.Errorf("expected 0 completed with cancelled context, got %d", result.Completed)
	}
}
