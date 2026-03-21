package input

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListImages(t *testing.T) {
	dir := t.TempDir()

	// Create test image files with various naming patterns
	files := []string{"001.png", "002.png", "010.png", "readme.txt"}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("fake"), 0644); err != nil {
			t.Fatalf("create file %s: %v", f, err)
		}
	}

	images, err := ListImages(dir)
	if err != nil {
		t.Fatalf("ListImages() error: %v", err)
	}
	if len(images) != 3 {
		t.Fatalf("expected 3 images, got %d", len(images))
	}

	// Verify sorted order
	expected := []int{1, 2, 10}
	for i, img := range images {
		if img.PageNumber != expected[i] {
			t.Errorf("image %d: expected page %d, got %d", i, expected[i], img.PageNumber)
		}
	}
}

func TestListImagesEmptyDir(t *testing.T) {
	dir := t.TempDir()

	images, err := ListImages(dir)
	if err != nil {
		t.Fatalf("ListImages() error: %v", err)
	}
	if len(images) != 0 {
		t.Errorf("expected 0 images, got %d", len(images))
	}
}

func TestListImagesNonexistent(t *testing.T) {
	images, err := ListImages("/nonexistent/dir")
	if err != nil {
		t.Fatalf("ListImages() error: %v", err)
	}
	if images != nil {
		t.Errorf("expected nil for nonexistent dir, got %v", images)
	}
}

func TestListImagesJPEG(t *testing.T) {
	dir := t.TempDir()

	files := []string{"001.jpg", "002.jpeg", "003.png"}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("fake"), 0644); err != nil {
			t.Fatalf("create file %s: %v", f, err)
		}
	}

	images, err := ListImages(dir)
	if err != nil {
		t.Fatalf("ListImages() error: %v", err)
	}
	if len(images) != 3 {
		t.Fatalf("expected 3 images, got %d", len(images))
	}
}

func TestParsePageNumber(t *testing.T) {
	tests := []struct {
		filename string
		expected int
	}{
		{"page-001.png", 1},
		{"page-010.png", 10},
		{"page-100.png", 100},
		{"page_001.png", 1},
		{"001.png", 1},
		{"page.png", -1},
		{"readme.txt", -1},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got := parsePageNumber(tt.filename)
			if got != tt.expected {
				t.Errorf("parsePageNumber(%q) = %d, want %d", tt.filename, got, tt.expected)
			}
		})
	}
}

func TestLoadImage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.png")
	content := []byte{0x89, 0x50, 0x4E, 0x47}
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	data, err := LoadImage(path)
	if err != nil {
		t.Fatalf("LoadImage() error: %v", err)
	}
	if len(data) != len(content) {
		t.Errorf("expected %d bytes, got %d", len(content), len(data))
	}
}

func TestLoadImageNotFound(t *testing.T) {
	_, err := LoadImage("/nonexistent/file.png")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}
