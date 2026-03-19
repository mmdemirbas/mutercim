package pipeline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mmdemirbas/mutercim/internal/model"
)

func TestListPageFiles(t *testing.T) {
	dir := t.TempDir()

	// Create page files
	for _, name := range []string{"page_001.json", "page_003.json", "page_010.json", "other.txt"} {
		os.WriteFile(filepath.Join(dir, name), []byte("{}"), 0644)
	}

	pages, err := listPageFiles(dir)
	if err != nil {
		t.Fatalf("listPageFiles() error: %v", err)
	}

	if len(pages) != 3 {
		t.Fatalf("expected 3 pages, got %d", len(pages))
	}

	// Should be sorted
	expected := []int{1, 3, 10}
	for i, pf := range pages {
		if pf.pageNum != expected[i] {
			t.Errorf("page %d: expected %d, got %d", i, expected[i], pf.pageNum)
		}
	}
}

func TestListPageFilesEmpty(t *testing.T) {
	dir := t.TempDir()

	pages, err := listPageFiles(dir)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(pages) != 0 {
		t.Errorf("expected 0 pages, got %d", len(pages))
	}
}

func TestFilterPages(t *testing.T) {
	pages := []pageFile{
		{pageNum: 1, path: "a"},
		{pageNum: 2, path: "b"},
		{pageNum: 3, path: "c"},
		{pageNum: 5, path: "d"},
	}

	filtered := filterPages(pages, []int{2, 5})
	if len(filtered) != 2 {
		t.Fatalf("expected 2, got %d", len(filtered))
	}
	if filtered[0].pageNum != 2 || filtered[1].pageNum != 5 {
		t.Errorf("unexpected pages: %v", filtered)
	}
}

func TestFilterPagesNoMatch(t *testing.T) {
	pages := []pageFile{{pageNum: 1, path: "a"}}
	filtered := filterPages(pages, []int{99})
	if len(filtered) != 0 {
		t.Errorf("expected 0, got %d", len(filtered))
	}
}

func TestDiscoverSubdirs(t *testing.T) {
	dir := t.TempDir()

	os.MkdirAll(filepath.Join(dir, "Anfas1"), 0755)
	os.MkdirAll(filepath.Join(dir, "Anfas2"), 0755)
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0644)

	stems, err := discoverSubdirs(dir)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(stems) != 2 {
		t.Fatalf("expected 2 subdirs, got %d", len(stems))
	}
	if stems[0] != "Anfas1" || stems[1] != "Anfas2" {
		t.Errorf("unexpected stems: %v", stems)
	}
}

func TestDiscoverSubdirsNonexistent(t *testing.T) {
	stems, err := discoverSubdirs("/nonexistent")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if stems != nil {
		t.Errorf("expected nil, got %v", stems)
	}
}

func TestContainsInt(t *testing.T) {
	if !containsInt([]int{1, 2, 3}, 2) {
		t.Error("expected true for 2 in [1,2,3]")
	}
	if containsInt([]int{1, 2, 3}, 4) {
		t.Error("expected false for 4 in [1,2,3]")
	}
	if containsInt(nil, 1) {
		t.Error("expected false for nil slice")
	}
}

func TestFileStem(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"./input/Anfas1.pdf", "Anfas1"},
		{"book.pdf", "book"},
		{"./dir/file.tar.gz", "file.tar"},
		{"noext", "noext"},
	}
	for _, tt := range tests {
		got := fileStem(tt.input)
		if got != tt.want {
			t.Errorf("fileStem(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFileExists(t *testing.T) {
	dir := t.TempDir()

	// File exists
	path := filepath.Join(dir, "test.json")
	os.WriteFile(path, []byte("{}"), 0644)
	if !fileExists(path) {
		t.Error("expected true for existing file")
	}

	// File doesn't exist
	if fileExists(filepath.Join(dir, "nonexistent")) {
		t.Error("expected false for missing file")
	}

	// Directory — not a file
	if fileExists(dir) {
		t.Error("expected false for directory")
	}
}

func TestSaveExtractedPageAtomicWrite(t *testing.T) {
	dir := t.TempDir()

	page := &model.ExtractedPage{
		Version:    "1.0",
		PageNumber: 42,
		Entries:    []model.Entry{{Type: "hadith", ArabicText: "text"}},
	}

	if err := saveExtractedPage(dir, 42, page); err != nil {
		t.Fatalf("error: %v", err)
	}

	// Verify file
	data, err := os.ReadFile(filepath.Join(dir, "page_042.json"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var loaded model.ExtractedPage
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if loaded.PageNumber != 42 {
		t.Errorf("expected page 42, got %d", loaded.PageNumber)
	}

	// No tmp file
	if _, err := os.Stat(filepath.Join(dir, "page_042.json.tmp")); err == nil {
		t.Error("tmp file should not exist")
	}
}
