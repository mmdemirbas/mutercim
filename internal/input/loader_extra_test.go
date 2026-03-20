package input

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListImages_UppercaseExtension(t *testing.T) {
	dir := t.TempDir()

	// .PNG (uppercase) should NOT be recognized — current code checks lowercase only
	os.WriteFile(filepath.Join(dir, "page-01.PNG"), []byte("fake"), 0644)
	os.WriteFile(filepath.Join(dir, "page-02.png"), []byte("fake"), 0644)

	images, err := ListImages(dir)
	if err != nil {
		t.Fatalf("ListImages() error: %v", err)
	}

	// Only the lowercase .png should be found
	if len(images) != 1 {
		t.Errorf("expected 1 image (only .png), got %d", len(images))
	}
}

func TestListImages_MixedExtensions(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "page-01.png"), []byte("fake"), 0644)
	os.WriteFile(filepath.Join(dir, "page-02.jpg"), []byte("fake"), 0644)
	os.WriteFile(filepath.Join(dir, "page-03.jpeg"), []byte("fake"), 0644)
	os.WriteFile(filepath.Join(dir, "page-04.gif"), []byte("fake"), 0644)  // not supported
	os.WriteFile(filepath.Join(dir, "page-05.webp"), []byte("fake"), 0644) // not supported

	images, err := ListImages(dir)
	if err != nil {
		t.Fatalf("ListImages() error: %v", err)
	}
	if len(images) != 3 {
		t.Errorf("expected 3 images (png+jpg+jpeg), got %d", len(images))
	}
}

func TestListImages_SubdirIgnored(t *testing.T) {
	dir := t.TempDir()

	os.MkdirAll(filepath.Join(dir, "subdir"), 0755)
	os.WriteFile(filepath.Join(dir, "subdir", "page-01.png"), []byte("fake"), 0644)
	os.WriteFile(filepath.Join(dir, "page-01.png"), []byte("fake"), 0644)

	images, err := ListImages(dir)
	if err != nil {
		t.Fatalf("ListImages() error: %v", err)
	}
	if len(images) != 1 {
		t.Errorf("expected 1 image (subdir ignored), got %d", len(images))
	}
}

func TestParsePageNumber_VariousFormats(t *testing.T) {
	tests := []struct {
		filename string
		expected int
	}{
		{"page-0001.png", 1},   // 4-digit padding
		{"page-1.png", 1},      // no padding
		{"42.png", 42},         // bare number
		{"book_page-5.png", 5}, // multiple numbers, uses last
		{"no-number.png", -1},  // no number
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
