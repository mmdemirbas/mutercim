package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/mmdemirbas/mutercim/internal/config"
	"github.com/mmdemirbas/mutercim/internal/knowledge"
	"github.com/mmdemirbas/mutercim/internal/model"
)

// countingProvider counts calls and cancels context after N successes.
type countingProvider struct {
	response  string
	calls     atomic.Int32
	cancelAt  int32
	cancelFn  context.CancelFunc
	failAfter int32 // if >0, fail after this many calls
}

func (c *countingProvider) Name() string         { return "counting" }
func (c *countingProvider) SupportsVision() bool { return true }
func (c *countingProvider) ReadFromImage(_ context.Context, _ []byte, _, _ string) (string, error) {
	n := c.calls.Add(1)
	if c.failAfter > 0 && n > c.failAfter {
		return "", fmt.Errorf("mock failure after %d calls", c.failAfter)
	}
	if c.cancelAt > 0 && n >= c.cancelAt && c.cancelFn != nil {
		c.cancelFn()
	}
	return c.response, nil
}
func (c *countingProvider) Translate(_ context.Context, _, _ string) (string, error) {
	return c.response, nil
}

func TestReadPipeline_ContextCancelledAfterFirstPage(t *testing.T) {
	ws, cfg := setupReadWorkspace(t, "testinput", "001.png", "002.png", "003.png")

	response := `{"regions": [{"id": "r1", "bbox": [0,0,100,100], "text": "text", "type": "entry"}], "reading_order": ["r1"], "warnings": []}`

	ctx, cancel := context.WithCancel(context.Background())
	prov := &countingProvider{response: response, cancelAt: 1, cancelFn: cancel}

	result, err := Read(ctx, ReadOptions{
		Workspace: ws,
		Config:    cfg,
		Provider:  prov,
	})
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	// Should have processed exactly 1 page before context was cancelled
	if result.Completed != 1 {
		t.Errorf("expected 1 completed, got %d", result.Completed)
	}
	calls := int(prov.calls.Load())
	if calls != 1 {
		t.Errorf("expected 1 provider call, got %d", calls)
	}
}

func TestReadPipeline_ContextCancelledPreLoop(t *testing.T) {
	ws, cfg := setupReadWorkspace(t, "testinput", "001.png", "002.png")

	response := `{"regions": [], "reading_order": [], "warnings": []}`

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
	if result.Completed != 0 {
		t.Errorf("expected 0 completed with pre-cancelled context, got %d", result.Completed)
	}
}

func TestReadPipeline_ForceReprocesses(t *testing.T) {
	ws, cfg := setupReadWorkspace(t, "testinput", "001.png")

	response := `{"regions": [{"id": "r1", "bbox": [0,0,100,100], "text": "new", "type": "entry"}], "reading_order": ["r1"], "warnings": []}`

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
			{Path: "./input/book3.pdf"},
		},
	}

	m := buildInputPageMap(cfg)

	if pages, ok := m["book1"]; !ok || len(pages) != 5 {
		t.Errorf("book1 pages = %v, want 5 pages", m["book1"])
	}
	if pages, ok := m["book2"]; !ok || len(pages) != 2 {
		t.Errorf("book2 pages = %v, want 2 pages", m["book2"])
	}
	if _, ok := m["book3"]; ok {
		t.Error("book3 should not be in map (no pages)")
	}
}

func TestBuildInputPageMap_InvalidPages(t *testing.T) {
	m := buildInputPageMap(&config.Config{
		Inputs: []config.InputSpec{{Path: "./input/book.pdf", Pages: "invalid"}},
	})
	if _, ok := m["book"]; ok {
		t.Error("invalid pages should not create map entry")
	}
}

func TestBuildInputPageMap_Empty(t *testing.T) {
	m := buildInputPageMap(&config.Config{})
	if len(m) != 0 {
		t.Errorf("expected empty map, got %v", m)
	}
}

func TestReadPipeline_PageNotInImageSet(t *testing.T) {
	ws, cfg := setupReadWorkspace(t, "testinput", "001.png")

	result, err := Read(context.Background(), ReadOptions{
		Workspace: ws,
		Config:    cfg,
		Provider:  &mockProvider{response: `{"regions":[],"reading_order":[],"warnings":[]}`},
		Pages:     []int{99},
	})
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
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
		t.Errorf("expected 0 completed, got %d", result.Completed)
	}
}
