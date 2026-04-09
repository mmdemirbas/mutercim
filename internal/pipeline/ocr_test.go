package pipeline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mmdemirbas/mutercim/internal/model"
)

func TestLoadOCRPage_RoundTrip(t *testing.T) {
	page := &model.OCRPage{
		Version:    "1.0",
		PageNumber: 42,
		Tool:       "qari",
		FullText:   "بسم الله الرحمن الرحيم",
		Regions: []model.OCRRegion{
			{ID: "r1", Text: "header text", ElapsedMs: 100},
			{ID: "r2", Text: "entry text", ElapsedMs: 200},
		},
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "042.json")

	data, err := json.MarshalIndent(page, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	loaded, err := loadOCRPage(path)
	if err != nil {
		t.Fatalf("loadOCRPage: %v", err)
	}

	if loaded.PageNumber != 42 {
		t.Errorf("PageNumber = %d, want 42", loaded.PageNumber)
	}
	if loaded.Tool != "qari" {
		t.Errorf("Tool = %q, want qari", loaded.Tool)
	}
	if loaded.FullText != page.FullText {
		t.Errorf("FullText mismatch")
	}
	if len(loaded.Regions) != 2 {
		t.Fatalf("len(Regions) = %d, want 2", len(loaded.Regions))
	}
	if loaded.Regions[0].ID != "r1" || loaded.Regions[1].Text != "entry text" {
		t.Errorf("region data mismatch: %+v", loaded.Regions)
	}
}

func TestLoadOCRPage_NotFound(t *testing.T) {
	_, err := loadOCRPage(filepath.Join(t.TempDir(), "nonexistent.json"))
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestLoadOCRPage_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := loadOCRPage(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLoadLayoutPage_RoundTrip(t *testing.T) {
	page := &model.LayoutPage{
		Version:    "1.0",
		PageNumber: 7,
		Tool:       "doclayout-yolo",
		Regions: []model.LayoutRegion{
			{ID: "r1", BBox: model.BBox{10, 20, 100, 50}, Type: "header", Confidence: 0.95},
		},
		ReadingOrder: []string{"r1"},
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "007.json")

	data, err := json.MarshalIndent(page, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	loaded, err := loadLayoutPage(path)
	if err != nil {
		t.Fatalf("loadLayoutPage: %v", err)
	}

	if loaded.PageNumber != 7 {
		t.Errorf("PageNumber = %d, want 7", loaded.PageNumber)
	}
	if loaded.Tool != "doclayout-yolo" {
		t.Errorf("Tool = %q, want doclayout-yolo", loaded.Tool)
	}
	if len(loaded.Regions) != 1 {
		t.Fatalf("len(Regions) = %d, want 1", len(loaded.Regions))
	}
	if loaded.Regions[0].Confidence != 0.95 {
		t.Errorf("Confidence = %v, want 0.95", loaded.Regions[0].Confidence)
	}
}
