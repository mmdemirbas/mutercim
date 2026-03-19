package solver

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mmdemirbas/mutercim/internal/model"
)

func TestStageFromReferenceTable(t *testing.T) {
	dir := t.TempDir()

	page := &model.ReadPage{
		PageNumber:  7,
		SectionType: "reference_table",
		Entries: []model.Entry{
			{ArabicText: "خ - صحيح البخاري"},
			{ArabicText: "م - صحيح مسلم"},
		},
	}

	err := StageFromReferenceTable(page, dir)
	if err != nil {
		t.Fatalf("StageFromReferenceTable() error: %v", err)
	}

	// Verify file was created
	path := filepath.Join(dir, "sources_page_007.yaml")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected staged file at %s", path)
	}

	// Verify no .tmp file left behind
	tmpPath := path + ".tmp"
	if _, err := os.Stat(tmpPath); err == nil {
		t.Error("tmp file should not exist after successful save")
	}
}

func TestStageFromReferenceTableSkipsNonRefTable(t *testing.T) {
	dir := t.TempDir()

	page := &model.ReadPage{
		PageNumber:  1,
		SectionType: "scholarly_entries",
		Entries:     []model.Entry{{ArabicText: "text"}},
	}

	err := StageFromReferenceTable(page, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// No file should be created
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Errorf("expected no staged files, got %d", len(entries))
	}
}

func TestStageFromReferenceTableEmptyEntries(t *testing.T) {
	dir := t.TempDir()

	page := &model.ReadPage{
		PageNumber:  7,
		SectionType: "reference_table",
		Entries:     []model.Entry{},
	}

	err := StageFromReferenceTable(page, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Errorf("expected no staged files for empty entries, got %d", len(entries))
	}
}
