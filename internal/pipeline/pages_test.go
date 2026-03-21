package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mmdemirbas/mutercim/internal/config"
	"github.com/mmdemirbas/mutercim/internal/workspace"
)

func TestPagesSkipsImageDirectory(t *testing.T) {
	dir := t.TempDir()

	// Input is a directory (not a PDF) — should be a no-op
	imagesDir := filepath.Join(dir, "input", "scanned")
	os.MkdirAll(imagesDir, 0755)
	os.WriteFile(filepath.Join(imagesDir, "001.png"), []byte("fake"), 0644)

	ws := &workspace.Workspace{Root: dir}
	cfg := &config.Config{
		Inputs: []config.InputSpec{{Path: imagesDir}},
		DPI:    300,
	}

	err := Pages(context.Background(), PagesOptions{
		Workspace: ws,
		Config:    cfg,
	})
	if err != nil {
		t.Fatalf("Pages() error: %v", err)
	}

	// pages/ should NOT have been created since input isn't a PDF
	stemDir := filepath.Join(ws.PagesDir(), filepath.Base(imagesDir))
	if _, err := os.Stat(stemDir); err == nil {
		t.Error("expected no pages/<stem> dir for non-PDF input")
	}
}

func TestPagesNoInputs(t *testing.T) {
	dir := t.TempDir()

	ws := &workspace.Workspace{Root: dir}
	cfg := &config.Config{
		Inputs: nil,
	}

	err := Pages(context.Background(), PagesOptions{
		Workspace: ws,
		Config:    cfg,
	})
	if err == nil {
		t.Fatal("expected error for empty inputs")
	}
}

func TestContiguousRanges(t *testing.T) {
	tests := []struct {
		name  string
		pages []int
		want  [][2]int
	}{
		{"nil pages", nil, [][2]int{{0, 0}}},
		{"empty pages", []int{}, [][2]int{{0, 0}}},
		{"single page", []int{5}, [][2]int{{5, 5}}},
		{"contiguous", []int{1, 2, 3}, [][2]int{{1, 3}}},
		{"two ranges", []int{1, 2, 3, 10, 11}, [][2]int{{1, 3}, {10, 11}}},
		{"three ranges", []int{1, 2, 3, 10, 11, 500}, [][2]int{{1, 3}, {10, 11}, {500, 500}}},
		{"unsorted", []int{11, 1, 3, 10, 2, 500}, [][2]int{{1, 3}, {10, 11}, {500, 500}}},
		{"all separate", []int{1, 5, 9}, [][2]int{{1, 1}, {5, 5}, {9, 9}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := contiguousRanges(tt.pages)
			if len(got) != len(tt.want) {
				t.Fatalf("contiguousRanges(%v) = %v, want %v", tt.pages, got, tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("contiguousRanges(%v)[%d] = %v, want %v", tt.pages, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestPagesPerInputPages(t *testing.T) {
	// We can't test actual PDF conversion without pdftoppm, but we can test
	// that non-PDF inputs with per-input pages still get the no-op treatment.
	dir := t.TempDir()

	imagesDir := filepath.Join(dir, "input", "scans")
	os.MkdirAll(imagesDir, 0755)
	os.WriteFile(filepath.Join(imagesDir, "001.png"), []byte("fake"), 0644)

	ws := &workspace.Workspace{Root: dir}
	cfg := &config.Config{
		Inputs: []config.InputSpec{{Path: imagesDir, Pages: "1-5"}},
		DPI:    300,
	}

	err := Pages(context.Background(), PagesOptions{
		Workspace: ws,
		Config:    cfg,
	})
	if err != nil {
		t.Fatalf("Pages() error: %v", err)
	}
}
