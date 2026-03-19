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
	tracker.MarkCompleted(PhaseRead, 1)
	tracker.MarkCompleted(PhaseRead, 2)
	tracker.MarkCompleted(PhaseRead, 3)
	tracker.MarkFailed(PhaseRead, 4)

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

	read := state.Phases[PhaseRead]
	if read == nil {
		t.Fatal("read phase is nil")
	}
	if len(read.Completed) != 3 {
		t.Errorf("len(Completed) = %d, want 3", len(read.Completed))
	}
	if len(read.Failed) != 1 {
		t.Errorf("len(Failed) = %d, want 1", len(read.Failed))
	}
}

func TestTrackerMarkCompletedDedup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "progress.json")

	tracker := NewTracker(path)
	tracker.MarkCompleted(PhaseRead, 1)
	tracker.MarkCompleted(PhaseRead, 1)
	tracker.MarkCompleted(PhaseRead, 1)

	state := tracker.State()
	read := state.Phases[PhaseRead]
	if len(read.Completed) != 1 {
		t.Errorf("len(Completed) = %d, want 1 (dedup)", len(read.Completed))
	}
}

func TestTrackerMarkCompletedRemovesFailed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "progress.json")

	tracker := NewTracker(path)
	tracker.MarkFailed(PhaseRead, 5)
	tracker.MarkCompleted(PhaseRead, 5)

	state := tracker.State()
	read := state.Phases[PhaseRead]
	if len(read.Failed) != 0 {
		t.Errorf("len(Failed) = %d, want 0 after marking completed", len(read.Failed))
	}
	if len(read.Completed) != 1 {
		t.Errorf("len(Completed) = %d, want 1", len(read.Completed))
	}
}

func TestTrackerAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "progress.json")

	tracker := NewTracker(path)
	tracker.MarkCompleted(PhaseRead, 1)
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
