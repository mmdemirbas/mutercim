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
	for _, name := range []string{"001.json", "003.json", "010.json", "other.txt"} {
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

func TestListPageFiles_LargePageNumbers(t *testing.T) {
	dir := t.TempDir()

	// Create page files with >999 page numbers (mixed padding widths)
	for _, name := range []string{"0001.json", "0500.json", "1000.json", "1500.json", "9999.json"} {
		os.WriteFile(filepath.Join(dir, name), []byte("{}"), 0644)
	}

	pages, err := listPageFiles(dir)
	if err != nil {
		t.Fatalf("listPageFiles() error: %v", err)
	}

	if len(pages) != 5 {
		t.Fatalf("expected 5 pages, got %d", len(pages))
	}

	expected := []int{1, 500, 1000, 1500, 9999}
	for i, pf := range pages {
		if pf.pageNum != expected[i] {
			t.Errorf("page %d: expected %d, got %d", i, expected[i], pf.pageNum)
		}
	}
}

func TestPageFilename(t *testing.T) {
	tests := []struct {
		pageNum    int
		totalPages int
		want       string
	}{
		{1, 100, "001.json"},
		{42, 999, "042.json"},
		{1, 1000, "0001.json"},
		{999, 1000, "0999.json"},
		{1000, 1000, "1000.json"},
		{1, 10000, "00001.json"},
		{12345, 99999, "12345.json"},
	}
	for _, tt := range tests {
		got := pageFilename(tt.pageNum, tt.totalPages)
		if got != tt.want {
			t.Errorf("pageFilename(%d, %d) = %q, want %q", tt.pageNum, tt.totalPages, got, tt.want)
		}
	}
}

func TestSaveRegionPageAtomicWrite(t *testing.T) {
	dir := t.TempDir()

	page := &model.RegionPage{
		Version:    "2.0",
		PageNumber: 42,
		PageSize:   model.PageSize{Width: 1500, Height: 2200},
		Regions: []model.Region{
			{ID: "r1", BBox: model.BBox{0, 0, 100, 50}, Text: "text", Type: model.RegionTypeEntry},
		},
		ReadingOrder: []string{"r1"},
	}

	if err := saveRegionPage(dir, 42, 100, page); err != nil {
		t.Fatalf("error: %v", err)
	}

	// Verify file
	data, err := os.ReadFile(filepath.Join(dir, "042.json"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var loaded model.RegionPage
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if loaded.PageNumber != 42 {
		t.Errorf("expected page 42, got %d", loaded.PageNumber)
	}

	// No tmp file
	if _, err := os.Stat(filepath.Join(dir, "042.json.tmp")); err == nil {
		t.Error("tmp file should not exist")
	}
}
