package input

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListImages_UppercaseExtension(t *testing.T) {
	dir := t.TempDir()

	// .PNG (uppercase) should NOT be recognized — current code checks lowercase only
	os.WriteFile(filepath.Join(dir, "001.PNG"), []byte("fake"), 0644)
	os.WriteFile(filepath.Join(dir, "002.png"), []byte("fake"), 0644)

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

	os.WriteFile(filepath.Join(dir, "001.png"), []byte("fake"), 0644)
	os.WriteFile(filepath.Join(dir, "002.jpg"), []byte("fake"), 0644)
	os.WriteFile(filepath.Join(dir, "003.jpeg"), []byte("fake"), 0644)
	os.WriteFile(filepath.Join(dir, "004.gif"), []byte("fake"), 0644)  // not supported
	os.WriteFile(filepath.Join(dir, "005.webp"), []byte("fake"), 0644) // not supported

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
	os.WriteFile(filepath.Join(dir, "subdir", "001.png"), []byte("fake"), 0644)
	os.WriteFile(filepath.Join(dir, "001.png"), []byte("fake"), 0644)

	images, err := ListImages(dir)
	if err != nil {
		t.Fatalf("ListImages() error: %v", err)
	}
	if len(images) != 1 {
		t.Errorf("expected 1 image (subdir ignored), got %d", len(images))
	}
}

func TestListImages_LegacyPagePrefix(t *testing.T) {
	// Backward compat: old pdftoppm output with page- prefix should still work
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "page-001.png"), []byte("fake"), 0644)
	os.WriteFile(filepath.Join(dir, "page-002.png"), []byte("fake"), 0644)

	images, err := ListImages(dir)
	if err != nil {
		t.Fatalf("ListImages() error: %v", err)
	}
	if len(images) != 2 {
		t.Errorf("expected 2 images (legacy prefix), got %d", len(images))
	}
	if images[0].PageNumber != 1 || images[1].PageNumber != 2 {
		t.Errorf("page numbers: %d, %d", images[0].PageNumber, images[1].PageNumber)
	}
}

func TestParsePageNumber_VariousFormats(t *testing.T) {
	tests := []struct {
		filename string
		expected int
	}{
		{"001.png", 1},         // new naming (bare number)
		{"010.png", 10},        // new naming
		{"1000.png", 1000},     // 4+ digits
		{"page-0001.png", 1},   // legacy 4-digit padding
		{"page-1.png", 1},      // legacy no padding
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

func TestRenamePdftoppmOutput(t *testing.T) {
	dir := t.TempDir()

	// Simulate pdftoppm output
	os.WriteFile(filepath.Join(dir, "page-001.png"), []byte("img1"), 0644)
	os.WriteFile(filepath.Join(dir, "page-002.png"), []byte("img2"), 0644)
	os.WriteFile(filepath.Join(dir, "page-010.png"), []byte("img10"), 0644)

	if err := renamePdftoppmOutput(dir); err != nil {
		t.Fatalf("renamePdftoppmOutput: %v", err)
	}

	// Verify renamed files exist
	for _, name := range []string{"001.png", "002.png", "010.png"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("expected %s to exist: %v", name, err)
		}
	}

	// Verify old files are gone
	for _, name := range []string{"page-001.png", "page-002.png", "page-010.png"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			t.Errorf("expected %s to be renamed away", name)
		}
	}
}
