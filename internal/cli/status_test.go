package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mmdemirbas/mutercim/internal/config"
	"github.com/mmdemirbas/mutercim/internal/model"
	"github.com/mmdemirbas/mutercim/internal/workspace"
)

func TestCountJSONFiles(t *testing.T) {
	dir := t.TempDir()

	// Empty dir
	if got := countJSONFiles(dir); got != 0 {
		t.Errorf("empty dir: got %d, want 0", got)
	}

	// Mix of JSON and non-JSON files
	os.WriteFile(filepath.Join(dir, "001.json"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(dir, "002.json"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("x"), 0644)
	os.MkdirAll(filepath.Join(dir, "subdir"), 0755)

	if got := countJSONFiles(dir); got != 2 {
		t.Errorf("mixed dir: got %d, want 2", got)
	}
}

func TestCountJSONFiles_NonexistentDir(t *testing.T) {
	if got := countJSONFiles("/nonexistent"); got != 0 {
		t.Errorf("nonexistent dir: got %d, want 0", got)
	}
}

func TestDirHasFiles(t *testing.T) {
	dir := t.TempDir()

	// Empty dir
	if dirHasFiles(dir) {
		t.Error("empty dir should return false")
	}

	// Dir with only subdirs
	os.MkdirAll(filepath.Join(dir, "subdir"), 0755)
	if dirHasFiles(dir) {
		t.Error("dir with only subdirs should return false")
	}

	// Dir with file
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0644)
	if !dirHasFiles(dir) {
		t.Error("dir with file should return true")
	}
}

func TestDirHasFiles_NonexistentDir(t *testing.T) {
	if dirHasFiles("/nonexistent") {
		t.Error("nonexistent dir should return false")
	}
}

func TestCountFiles(t *testing.T) {
	dir := t.TempDir()

	if got := countFiles(dir); got != 0 {
		t.Errorf("empty: got %d, want 0", got)
	}

	os.WriteFile(filepath.Join(dir, "a.png"), []byte("x"), 0644)
	os.WriteFile(filepath.Join(dir, "b.png"), []byte("x"), 0644)
	os.MkdirAll(filepath.Join(dir, "subdir"), 0755)

	if got := countFiles(dir); got != 2 {
		t.Errorf("with files: got %d, want 2", got)
	}
}

func TestCountFiles_NonexistentDir(t *testing.T) {
	if got := countFiles("/nonexistent"); got != 0 {
		t.Errorf("nonexistent: got %d, want 0", got)
	}
}

func TestDiscoverInputs(t *testing.T) {
	dir := t.TempDir()
	ws := &workspace.Workspace{Root: dir}

	// No dirs exist yet
	stems := discoverInputs(ws)
	if len(stems) != 0 {
		t.Errorf("empty workspace: got %v", stems)
	}

	// Create cut/book1 and read/book2
	os.MkdirAll(filepath.Join(dir, "cut", "book1"), 0755)
	os.MkdirAll(filepath.Join(dir, "read", "book2"), 0755)

	stems = discoverInputs(ws)
	if len(stems) != 2 || stems[0] != "book1" || stems[1] != "book2" {
		t.Errorf("two inputs: got %v, want [book1 book2]", stems)
	}

	// Duplicate across directories
	os.MkdirAll(filepath.Join(dir, "solve", "book1"), 0755)
	stems = discoverInputs(ws)
	if len(stems) != 2 {
		t.Errorf("deduped: got %v, want 2 unique stems", stems)
	}
}

func TestDiscoverInputs_TranslateDir(t *testing.T) {
	dir := t.TempDir()
	ws := &workspace.Workspace{Root: dir}

	// Create translate/tr/book1
	os.MkdirAll(filepath.Join(dir, "translate", "tr", "book1"), 0755)

	stems := discoverInputs(ws)
	if len(stems) != 1 || stems[0] != "book1" {
		t.Errorf("translate dir: got %v, want [book1]", stems)
	}
}

func writeRegionPage(t *testing.T, dir string, pageNum int, page *model.RegionPage) {
	t.Helper()
	data, err := json.MarshalIndent(page, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	name := "001.json"
	if pageNum >= 10 {
		name = "010.json"
	}
	if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestLoadRegionPages(t *testing.T) {
	dir := t.TempDir()

	page := &model.RegionPage{
		Version:    "2.0",
		PageNumber: 1,
		Regions: []model.Region{
			{ID: "r1", Text: "text", Type: model.RegionTypeEntry},
		},
		ReadingOrder: []string{"r1"},
	}
	writeRegionPage(t, dir, 1, page)

	pages, err := loadRegionPages(dir)
	if err != nil {
		t.Fatalf("loadRegionPages: %v", err)
	}
	if len(pages) != 1 {
		t.Fatalf("got %d pages, want 1", len(pages))
	}
	if pages[0].PageNumber != 1 {
		t.Errorf("page number = %d, want 1", pages[0].PageNumber)
	}
}

func TestLoadRegionPages_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	pages, err := loadRegionPages(dir)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(pages) != 0 {
		t.Errorf("got %d pages, want 0", len(pages))
	}
}

func TestLoadRegionPages_NonexistentDir(t *testing.T) {
	_, err := loadRegionPages("/nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent dir")
	}
}

func TestLoadRegionPages_SkipsInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "001.json"), []byte("not json"), 0644)
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("text"), 0644)

	pages, err := loadRegionPages(dir)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(pages) != 0 {
		t.Errorf("got %d pages, want 0 (invalid JSON should be skipped)", len(pages))
	}
}

func TestCollectValidationWarnings_EmptyText(t *testing.T) {
	dir := t.TempDir()
	ws := &workspace.Workspace{Root: dir}
	stem := "testbook"

	readDir := filepath.Join(dir, "read", stem)
	os.MkdirAll(readDir, 0755)

	page := &model.RegionPage{
		Version:    "2.0",
		PageNumber: 1,
		Regions: []model.Region{
			{ID: "r1", Text: "", Type: model.RegionTypeEntry},       // empty text — warning
			{ID: "sep1", Text: "", Type: model.RegionTypeSeparator}, // ok for separator
		},
		ReadingOrder: []string{"r1", "sep1"},
	}
	writeRegionPage(t, readDir, 1, page)

	warnings := collectValidationWarnings(ws, []string{stem})
	if len(warnings) != 1 {
		t.Fatalf("got %d warnings, want 1: %v", len(warnings), warnings)
	}
	if got := warnings[0]; got == "" || !contains(got, "r1") || !contains(got, "empty text") {
		t.Errorf("unexpected warning: %q", got)
	}
}

func TestCollectValidationWarnings_BadReadingOrder(t *testing.T) {
	dir := t.TempDir()
	ws := &workspace.Workspace{Root: dir}
	stem := "testbook"

	readDir := filepath.Join(dir, "read", stem)
	os.MkdirAll(readDir, 0755)

	page := &model.RegionPage{
		Version:    "2.0",
		PageNumber: 1,
		Regions: []model.Region{
			{ID: "r1", Text: "text", Type: model.RegionTypeEntry},
		},
		ReadingOrder: []string{"r1", "r99"},
	}
	writeRegionPage(t, readDir, 1, page)

	warnings := collectValidationWarnings(ws, []string{stem})
	if len(warnings) != 1 {
		t.Fatalf("got %d warnings, want 1: %v", len(warnings), warnings)
	}
	if got := warnings[0]; !contains(got, "r99") {
		t.Errorf("warning should mention r99: %q", got)
	}
}

func TestCollectValidationWarnings_NoWarnings(t *testing.T) {
	dir := t.TempDir()
	ws := &workspace.Workspace{Root: dir}
	stem := "testbook"

	readDir := filepath.Join(dir, "read", stem)
	os.MkdirAll(readDir, 0755)

	page := &model.RegionPage{
		Version:    "2.0",
		PageNumber: 1,
		Regions: []model.Region{
			{ID: "r1", Text: "text", Type: model.RegionTypeEntry},
		},
		ReadingOrder: []string{"r1"},
	}
	writeRegionPage(t, readDir, 1, page)

	warnings := collectValidationWarnings(ws, []string{stem})
	if len(warnings) != 0 {
		t.Errorf("got warnings, want none: %v", warnings)
	}
}

func TestCollectValidationWarnings_NoReadDir(t *testing.T) {
	dir := t.TempDir()
	ws := &workspace.Workspace{Root: dir}

	warnings := collectValidationWarnings(ws, []string{"nonexistent"})
	if len(warnings) != 0 {
		t.Errorf("got warnings for missing dir: %v", warnings)
	}
}

func TestBuildPhaseRows(t *testing.T) {
	dir := t.TempDir()
	ws := &workspace.Workspace{Root: dir}
	stem := "testbook"

	// Create cut output with 3 images
	cutDir := filepath.Join(dir, "cut", stem)
	os.MkdirAll(cutDir, 0755)
	for _, name := range []string{"001.png", "002.png", "003.png"} {
		os.WriteFile(filepath.Join(cutDir, name), []byte("img"), 0644)
	}

	// Create read with 2 JSON files
	readDir := filepath.Join(dir, "read", stem)
	os.MkdirAll(readDir, 0755)
	os.WriteFile(filepath.Join(readDir, "001.json"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(readDir, "002.json"), []byte("{}"), 0644)

	cfg := &config.Config{
		Layout:    config.LayoutConfig{Tool: "doclayout-yolo"},
		Translate: config.TranslateConfig{Languages: []string{"tr"}},
	}
	rows := buildPhaseRows(ws, cfg, []string{stem}, 3)

	// cut, layout, ocr, read, solve, translate(tr), write(tr) = 7 rows
	if len(rows) != 7 {
		t.Fatalf("got %d rows, want 7", len(rows))
	}

	// Cut: 3/3, done
	if rows[0].Completed != 3 || rows[0].Total != 3 || !rows[0].Done {
		t.Errorf("cut row: completed=%d total=%d done=%v", rows[0].Completed, rows[0].Total, rows[0].Done)
	}

	// Layout: 0/3, not done (tool enabled)
	if rows[1].Completed != 0 || rows[1].Total != 3 || rows[1].Skipped {
		t.Errorf("layout row: completed=%d total=%d skipped=%v", rows[1].Completed, rows[1].Total, rows[1].Skipped)
	}

	// OCR: skipped (no tool configured)
	if !rows[2].Skipped {
		t.Errorf("ocr row: expected skipped when tool is empty")
	}

	// Read: 2/3, not done
	if rows[3].Completed != 2 || rows[3].Total != 3 || rows[3].Done {
		t.Errorf("read row: completed=%d total=%d done=%v", rows[3].Completed, rows[3].Total, rows[3].Done)
	}

	// Solve: 0/2, not done
	if rows[4].Completed != 0 || rows[4].Total != 2 {
		t.Errorf("solve row: completed=%d total=%d", rows[4].Completed, rows[4].Total)
	}
}

func TestBuildPhaseRows_NoInputs(t *testing.T) {
	dir := t.TempDir()
	ws := &workspace.Workspace{Root: dir}

	cfg := &config.Config{
		Layout:    config.LayoutConfig{Tool: ""},
		Translate: config.TranslateConfig{Languages: []string{"tr"}},
	}
	rows := buildPhaseRows(ws, cfg, nil, 0)

	// cut, layout, ocr, read, solve, translate(tr), write(tr) = 7 rows
	if len(rows) != 7 {
		t.Fatalf("got %d rows, want 7", len(rows))
	}

	// Layout should be skipped when tool is empty
	if !rows[1].Skipped {
		t.Errorf("layout row should be skipped when tool is empty")
	}

	// OCR should be skipped when tool is empty
	if !rows[2].Skipped {
		t.Errorf("ocr row should be skipped when tool is empty")
	}

	// All zeroes
	for _, row := range rows {
		if row.Completed != 0 {
			t.Errorf("phase %s: completed=%d, want 0", row.Phase, row.Completed)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
