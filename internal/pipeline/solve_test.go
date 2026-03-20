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

// setupSolveWorkspace creates a workspace with read pages already in read/<stem>/.
func setupSolveWorkspace(t *testing.T, stem string, pages map[int]*model.ReadPage) *workspace.Workspace {
	t.Helper()
	dir := t.TempDir()

	readDir := filepath.Join(dir, "read", stem)
	if err := os.MkdirAll(readDir, 0755); err != nil {
		t.Fatalf("mkdir read dir: %v", err)
	}

	for pageNum, page := range pages {
		data, err := json.MarshalIndent(page, "", "  ")
		if err != nil {
			t.Fatalf("marshal page %d: %v", pageNum, err)
		}
		filename := filepath.Join(readDir, pageName(pageNum))
		if err := os.WriteFile(filename, data, 0644); err != nil {
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

func TestSolvePipeline(t *testing.T) {
	entryNum := 1
	readPage := &model.ReadPage{
		Version:    "1.0",
		PageNumber: 1,
		Entries: []model.Entry{
			{
				Number:     &entryNum,
				Type:       "hadith",
				ArabicText: "test arabic text",
			},
		},
		Footnotes:    []model.Footnote{},
		ReadWarnings: []string{},
	}

	ws := setupSolveWorkspace(t, "testbook", map[int]*model.ReadPage{1: readPage})

	_, err := Solve(context.Background(), SolveOptions{
		Workspace: ws,
		Knowledge: &knowledge.Knowledge{},
	})
	if err != nil {
		t.Fatalf("Solve() error: %v", err)
	}

	// Verify output file was created
	outputPath := filepath.Join(ws.SolveDir(), "testbook", "001.json")
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read solved output: %v", err)
	}

	var solved model.SolvedPage
	if err := json.Unmarshal(data, &solved); err != nil {
		t.Fatalf("unmarshal solved page: %v", err)
	}
	if solved.PageNumber != 1 {
		t.Errorf("expected page number 1, got %d", solved.PageNumber)
	}
	if len(solved.Entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(solved.Entries))
	}
	if solved.Validation == nil {
		t.Error("expected validation to be set")
	}
	// ContinuationInfo is nil when there is no continuation detected (single page, no flags set)
	if solved.TranslationContext == nil {
		t.Error("expected translation_context to be set")
	}
}

func TestSolvePipelineSkipsCompleted(t *testing.T) {
	entryNum := 1
	readPage := &model.ReadPage{
		Version:    "1.0",
		PageNumber: 1,
		Entries: []model.Entry{
			{
				Number:     &entryNum,
				Type:       "hadith",
				ArabicText: "test",
			},
		},
		Footnotes:    []model.Footnote{},
		ReadWarnings: []string{},
	}

	ws := setupSolveWorkspace(t, "testbook", map[int]*model.ReadPage{1: readPage})

	// Set input mtime to the past so output appears newer
	past := time.Now().Add(-10 * time.Second)
	readDir := filepath.Join(ws.ReadDir(), "testbook")
	os.Chtimes(filepath.Join(readDir, "001.json"), past, past)
	os.Chtimes(readDir, past, past)
	os.MkdirAll(ws.KnowledgeDir(), 0755)
	os.Chtimes(ws.KnowledgeDir(), past, past)
	os.MkdirAll(ws.MemoryDir(), 0755)
	os.Chtimes(ws.MemoryDir(), past, past)

	// Create the output file so skip logic sees it as up-to-date
	solvedDir := filepath.Join(ws.SolveDir(), "testbook")
	if err := os.MkdirAll(solvedDir, 0755); err != nil {
		t.Fatalf("mkdir solved dir: %v", err)
	}
	outputPath := filepath.Join(solvedDir, "001.json")
	originalContent := `{"version":"1.0","page_number":1,"entries":[]}`
	if err := os.WriteFile(outputPath, []byte(originalContent), 0644); err != nil {
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
	data, err := os.ReadFile(outputPath)
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
	if err := os.MkdirAll(filepath.Join(dir, "read"), 0755); err != nil {
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
