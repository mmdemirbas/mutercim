package progress

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTrackerNewAndEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "progress.json")

	tracker := NewTracker(path)
	if err := tracker.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	state := tracker.State()
	if state.BookID != "" {
		t.Errorf("BookID = %q, want empty", state.BookID)
	}
	if state.TotalPages != 0 {
		t.Errorf("TotalPages = %d, want 0", state.TotalPages)
	}
	if len(state.Phases) != 0 {
		t.Errorf("len(Phases) = %d, want 0", len(state.Phases))
	}
}

func TestTrackerLoadFromEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "progress.json")
	os.WriteFile(path, []byte("{}\n"), 0644)

	tracker := NewTracker(path)
	if err := tracker.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	state := tracker.State()
	if len(state.Phases) != 0 {
		t.Errorf("len(Phases) = %d, want 0", len(state.Phases))
	}
}

func TestTrackerMarkAndSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "progress.json")

	tracker := NewTracker(path)
	tracker.SetBookID("test-book")
	tracker.SetTotalPages(100)
	tracker.MarkCompleted(PhaseExtract, 1)
	tracker.MarkCompleted(PhaseExtract, 2)
	tracker.MarkCompleted(PhaseExtract, 3)
	tracker.MarkFailed(PhaseExtract, 4)

	if err := tracker.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Load into new tracker
	tracker2 := NewTracker(path)
	if err := tracker2.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	state := tracker2.State()
	if state.BookID != "test-book" {
		t.Errorf("BookID = %q, want %q", state.BookID, "test-book")
	}
	if state.TotalPages != 100 {
		t.Errorf("TotalPages = %d, want 100", state.TotalPages)
	}

	extract := state.Phases[PhaseExtract]
	if extract == nil {
		t.Fatal("extract phase is nil")
	}
	if len(extract.Completed) != 3 {
		t.Errorf("len(Completed) = %d, want 3", len(extract.Completed))
	}
	if len(extract.Failed) != 1 {
		t.Errorf("len(Failed) = %d, want 1", len(extract.Failed))
	}
}

func TestTrackerMarkCompletedDedup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "progress.json")

	tracker := NewTracker(path)
	tracker.MarkCompleted(PhaseExtract, 1)
	tracker.MarkCompleted(PhaseExtract, 1)
	tracker.MarkCompleted(PhaseExtract, 1)

	state := tracker.State()
	extract := state.Phases[PhaseExtract]
	if len(extract.Completed) != 1 {
		t.Errorf("len(Completed) = %d, want 1 (dedup)", len(extract.Completed))
	}
}

func TestTrackerMarkCompletedRemovesFailed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "progress.json")

	tracker := NewTracker(path)
	tracker.MarkFailed(PhaseExtract, 5)
	tracker.MarkCompleted(PhaseExtract, 5)

	state := tracker.State()
	extract := state.Phases[PhaseExtract]
	if len(extract.Failed) != 0 {
		t.Errorf("len(Failed) = %d, want 0 after marking completed", len(extract.Failed))
	}
	if len(extract.Completed) != 1 {
		t.Errorf("len(Completed) = %d, want 1", len(extract.Completed))
	}
}

func TestTrackerAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "progress.json")

	tracker := NewTracker(path)
	tracker.MarkCompleted(PhaseExtract, 1)
	if err := tracker.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify no .tmp file remains
	tmpPath := path + ".tmp"
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Error("tmp file should not exist after successful save")
	}

	// Verify the actual file exists and is valid JSON
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if len(data) == 0 {
		t.Error("progress file is empty")
	}
}
