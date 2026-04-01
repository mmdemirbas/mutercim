package pipeline

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mmdemirbas/mutercim/internal/knowledge"
	"github.com/mmdemirbas/mutercim/internal/model"
	"github.com/mmdemirbas/mutercim/internal/workspace"
)

// setupSolveWorkspace creates a workspace with region pages in read/<stem>/.
func setupSolveWorkspace(t *testing.T, stem string, pages map[int]*model.RegionPage) *workspace.Workspace {
	t.Helper()
	dir := t.TempDir()

	readDir := filepath.Join(dir, "read", stem)
	if err := os.MkdirAll(readDir, 0750); err != nil {
		t.Fatalf("mkdir read dir: %v", err)
	}

	for pageNum, page := range pages {
		data, err := json.MarshalIndent(page, "", "  ")
		if err != nil {
			t.Fatalf("marshal page %d: %v", pageNum, err)
		}
		filename := filepath.Join(readDir, pageName(pageNum))
		if err := os.WriteFile(filename, data, 0600); err != nil {
			t.Fatalf("write page %d: %v", pageNum, err)
		}
	}

	return &workspace.Workspace{Root: dir}
}

// pageName returns the filename for a page number (e.g. "001.json").
func pageName(pageNum int) string {
	return padPageNum(pageNum) + ".json"
}

// padPageNum zero-pads a page number to 3 digits.
func padPageNum(n int) string {
	s := ""
	if n < 100 {
		s += "0"
	}
	if n < 10 {
		s += "0"
	}
	s += itoa(n)
	return s
}

// itoa converts an int to string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}

func makeRegionPage(pageNum int) *model.RegionPage {
	return &model.RegionPage{
		Version:    "2.0",
		PageNumber: pageNum,
		PageSize:   model.PageSize{Width: 1500, Height: 2200},
		Regions: []model.Region{
			{ID: "r1", BBox: model.BBox{400, 50, 700, 60}, Text: "header text", Type: model.RegionTypeHeader},
			{ID: "r2", BBox: model.BBox{800, 150, 600, 400}, Text: "entry text", Type: model.RegionTypeEntry},
		},
		ReadingOrder: []string{"r1", "r2"},
	}
}

func TestSolvePipeline(t *testing.T) {
	ws := setupSolveWorkspace(t, "testbook", map[int]*model.RegionPage{
		1: makeRegionPage(1),
	})

	_, err := Solve(context.Background(), SolveOptions{
		Workspace: ws,
		Knowledge: &knowledge.Knowledge{},
	})
	if err != nil {
		t.Fatalf("Solve() error: %v", err)
	}

	// Verify output file was created with SolvedRegionPage format
	outputPath := filepath.Join(ws.SolveDir(), "testbook", "001.json")
	data, err := os.ReadFile(outputPath) //nolint:gosec // G304: path is internal workspace path, not user input
	if err != nil {
		t.Fatalf("read solved output: %v", err)
	}

	var solved model.SolvedRegionPage
	if err := json.Unmarshal(data, &solved); err != nil {
		t.Fatalf("unmarshal solved page: %v", err)
	}
	if solved.PageNumber != 1 {
		t.Errorf("expected page number 1, got %d", solved.PageNumber)
	}
	if solved.Version != "2.0" {
		t.Errorf("expected version 2.0, got %q", solved.Version)
	}
	if len(solved.Regions) != 2 {
		t.Errorf("expected 2 regions, got %d", len(solved.Regions))
	}
}

func TestSolvePipelineSkipsCompleted(t *testing.T) {
	ws := setupSolveWorkspace(t, "testbook", map[int]*model.RegionPage{
		1: makeRegionPage(1),
	})

	// Set input mtime to the past so output appears newer
	past := time.Now().Add(-10 * time.Second)
	readDir := filepath.Join(ws.ReadDir(), "testbook")
	if err := os.Chtimes(filepath.Join(readDir, "001.json"), past, past); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(readDir, past, past); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(ws.KnowledgeDir(), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(ws.KnowledgeDir(), past, past); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(ws.MemoryDir(), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(ws.MemoryDir(), past, past); err != nil {
		t.Fatal(err)
	}

	// Create the output file so skip logic sees it as up-to-date
	solvedDir := filepath.Join(ws.SolveDir(), "testbook")
	if err := os.MkdirAll(solvedDir, 0750); err != nil {
		t.Fatalf("mkdir solved dir: %v", err)
	}
	outputPath := filepath.Join(solvedDir, "001.json")
	originalContent := `{"version":"2.0","page_number":1}`
	if err := os.WriteFile(outputPath, []byte(originalContent), 0600); err != nil {
		t.Fatalf("write existing solved page: %v", err)
	}

	_, err := Solve(context.Background(), SolveOptions{
		Workspace: ws,
		Knowledge: &knowledge.Knowledge{},
	})
	if err != nil {
		t.Fatalf("Solve() error: %v", err)
	}

	// Output should still contain original content (not re-processed)
	data, err := os.ReadFile(outputPath) //nolint:gosec // G304: path is internal workspace path, not user input
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if string(data) != originalContent {
		t.Error("output was overwritten — page should have been skipped")
	}
}

func TestSolvePipelineNoReadPages(t *testing.T) {
	dir := t.TempDir()

	// read/ exists but is empty (no subdirs)
	if err := os.MkdirAll(filepath.Join(dir, "read"), 0750); err != nil {
		t.Fatalf("mkdir read dir: %v", err)
	}

	ws := &workspace.Workspace{Root: dir}

	_, err := Solve(context.Background(), SolveOptions{
		Workspace: ws,
		Knowledge: &knowledge.Knowledge{},
	})
	if err == nil {
		t.Fatal("expected error when no read pages found")
	}

	expectedMsg := "no read pages found in " + ws.ReadDir() + " (run read first)"
	if got := err.Error(); got != expectedMsg {
		t.Errorf("unexpected error message:\n got: %q\nwant: %q", got, expectedMsg)
	}
}

func TestSolvePipelineMissingReadDir(t *testing.T) {
	dir := t.TempDir()

	// read/ doesn't exist at all
	ws := &workspace.Workspace{Root: dir}

	_, err := Solve(context.Background(), SolveOptions{
		Workspace: ws,
		Knowledge: &knowledge.Knowledge{},
	})
	if err == nil {
		t.Fatal("expected error when read dir missing")
	}
}
