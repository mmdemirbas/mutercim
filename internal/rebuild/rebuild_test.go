package rebuild

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNeedsRebuild_output_missing(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.txt")
	if err := os.WriteFile(input, []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}

	if !NeedsRebuild(filepath.Join(dir, "output.txt"), input) {
		t.Error("expected rebuild when output missing")
	}
}

func TestNeedsRebuild_output_newer(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.txt")
	output := filepath.Join(dir, "output.txt")

	if err := os.WriteFile(input, []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}
	// Set input to the past
	past := time.Now().Add(-10 * time.Second)
	if err := os.Chtimes(input, past, past); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(output, []byte("result"), 0600); err != nil {
		t.Fatal(err)
	}
	// Output keeps current time (newer)

	if NeedsRebuild(output, input) {
		t.Error("expected no rebuild when output is newer than input")
	}
}

func TestNeedsRebuild_input_newer(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.txt")
	output := filepath.Join(dir, "output.txt")

	os.WriteFile(output, []byte("old"), 0644)
	// Set output to the past
	past := time.Now().Add(-10 * time.Second)
	if err := os.Chtimes(output, past, past); err != nil {
		t.Fatal(err)
	}

	os.WriteFile(input, []byte("new data"), 0644)
	// Input keeps current time (newer)

	if !NeedsRebuild(output, input) {
		t.Error("expected rebuild when input is newer than output")
	}
}

func TestNeedsRebuild_multiple_inputs_one_newer(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "old.txt")
	new := filepath.Join(dir, "new.txt")
	output := filepath.Join(dir, "output.txt")

	past := time.Now().Add(-10 * time.Second)

	os.WriteFile(old, []byte("old"), 0644)
	if err := os.Chtimes(old, past, past); err != nil {
		t.Fatal(err)
	}

	os.WriteFile(output, []byte("result"), 0644)
	if err := os.Chtimes(output, past.Add(time.Second), past.Add(time.Second)); err != nil {
		t.Fatal(err)
	}

	// new file is newer than output
	os.WriteFile(new, []byte("new"), 0644)

	if !NeedsRebuild(output, old, new) {
		t.Error("expected rebuild when one input is newer")
	}
}

func TestNeedsRebuild_directory_input(t *testing.T) {
	dir := t.TempDir()
	inputDir := filepath.Join(dir, "inputs")
	if err := os.MkdirAll(inputDir, 0750); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, "output.txt")

	past := time.Now().Add(-10 * time.Second)

	os.WriteFile(filepath.Join(inputDir, "a.txt"), []byte("a"), 0644)
	if err := os.Chtimes(filepath.Join(inputDir, "a.txt"), past, past); err != nil {
		t.Fatal(err)
	}
	os.Chtimes(inputDir, past, past)

	os.WriteFile(output, []byte("result"), 0644)
	// output is current time = newer

	if NeedsRebuild(output, inputDir) {
		t.Error("expected no rebuild when directory and its files are older")
	}

	// Now add a file — directory mtime should update
	os.WriteFile(filepath.Join(inputDir, "b.txt"), []byte("b"), 0644)

	if !NeedsRebuild(output, inputDir) {
		t.Error("expected rebuild after adding file to input directory")
	}
}

func TestNeedsRebuild_no_inputs(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "output.txt")
	os.WriteFile(output, []byte("data"), 0644)

	// No inputs → newest mtime is zero → output is newer → no rebuild
	if NeedsRebuild(output) {
		t.Error("expected no rebuild with no inputs")
	}
}

func TestNeedsRebuild_missing_input(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "output.txt")
	os.WriteFile(output, []byte("data"), 0644)

	// Missing input is skipped (not treated as error), output is newer than zero time → no rebuild
	if NeedsRebuild(output, filepath.Join(dir, "nonexistent")) {
		t.Error("expected no rebuild when input is missing (non-existent paths are skipped)")
	}
}

func TestNewestMtime_single_file(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "file.txt")
	os.WriteFile(f, []byte("data"), 0644)

	mt, err := NewestMtime(f)
	if err != nil {
		t.Fatalf("NewestMtime error: %v", err)
	}
	info, _ := os.Stat(f)
	if !mt.Equal(info.ModTime()) {
		t.Errorf("mtime = %v, want %v", mt, info.ModTime())
	}
}

func TestNewestMtime_empty_dir(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "empty")
	os.MkdirAll(sub, 0755)

	mt, err := NewestMtime(sub)
	if err != nil {
		t.Fatalf("NewestMtime error: %v", err)
	}
	info, _ := os.Stat(sub)
	if !mt.Equal(info.ModTime()) {
		t.Errorf("mtime = %v, want dir mtime %v", mt, info.ModTime())
	}
}

func TestNewestMtime_includes_dir_own_mtime(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "data")
	os.MkdirAll(sub, 0755)

	// Create old file
	past := time.Now().Add(-1 * time.Hour)
	f := filepath.Join(sub, "old.txt")
	os.WriteFile(f, []byte("old"), 0644)
	os.Chtimes(f, past, past)

	// Set dir mtime to recent (simulates file deletion updating dir mtime)
	recent := time.Now()
	os.Chtimes(sub, recent, recent)

	mt, err := NewestMtime(sub)
	if err != nil {
		t.Fatalf("NewestMtime error: %v", err)
	}
	// Dir mtime (recent) should be newer than file mtime (past)
	if mt.Before(recent.Add(-time.Second)) {
		t.Errorf("mtime %v should be >= dir mtime %v", mt, recent)
	}
}

func TestNewestMtime_nested_directory(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "a", "b")
	os.MkdirAll(nested, 0755)

	past := time.Now().Add(-1 * time.Hour)
	os.Chtimes(filepath.Join(dir, "a"), past, past)
	os.Chtimes(nested, past, past)

	// Put a newer file deep inside
	f := filepath.Join(nested, "file.txt")
	os.WriteFile(f, []byte("data"), 0644)

	mt, err := NewestMtime(filepath.Join(dir, "a"))
	if err != nil {
		t.Fatalf("NewestMtime error: %v", err)
	}
	info, _ := os.Stat(f)
	if !mt.Equal(info.ModTime()) {
		t.Errorf("mtime = %v, want nested file mtime %v", mt, info.ModTime())
	}
}

func TestNewestMtime_multiple_paths(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, "old.txt")
	f2 := filepath.Join(dir, "new.txt")

	past := time.Now().Add(-1 * time.Hour)
	os.WriteFile(f1, []byte("old"), 0644)
	os.Chtimes(f1, past, past)

	os.WriteFile(f2, []byte("new"), 0644)

	mt, err := NewestMtime(f1, f2)
	if err != nil {
		t.Fatalf("NewestMtime error: %v", err)
	}
	info, _ := os.Stat(f2)
	if !mt.Equal(info.ModTime()) {
		t.Errorf("mtime = %v, want newer file mtime %v", mt, info.ModTime())
	}
}

func TestNewestMtime_nonexistent_path(t *testing.T) {
	// Non-existent paths are silently skipped, returning zero time
	mt, err := NewestMtime("/nonexistent/path")
	if err != nil {
		t.Fatalf("NewestMtime error: %v (non-existent paths should be skipped)", err)
	}
	if !mt.IsZero() {
		t.Errorf("expected zero time for non-existent path, got %v", mt)
	}
}

func TestNewestMtime_no_paths(t *testing.T) {
	mt, err := NewestMtime()
	if err != nil {
		t.Fatalf("NewestMtime error: %v", err)
	}
	if !mt.IsZero() {
		t.Errorf("expected zero time, got %v", mt)
	}
}

func TestNeedsRebuild_nonexistent_input_dir_skipped(t *testing.T) {
	dir := t.TempDir()
	inputFile := filepath.Join(dir, "input.txt")
	output := filepath.Join(dir, "output.txt")

	// Create input file in the past
	past := time.Now().Add(-10 * time.Second)
	os.WriteFile(inputFile, []byte("data"), 0644)
	os.Chtimes(inputFile, past, past)

	// Output is newer
	os.WriteFile(output, []byte("result"), 0644)

	// A non-existent directory as one of the inputs should NOT force rebuild
	nonexistent := filepath.Join(dir, "memory-dir-not-created-yet")
	if NeedsRebuild(output, inputFile, nonexistent) {
		t.Error("expected no rebuild when non-existent dir is an input and output is newer than existing inputs")
	}
}

func TestNewestMtime_nonexistent_path_skipped(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "file.txt")
	os.WriteFile(f, []byte("data"), 0644)

	mt, err := NewestMtime(f, filepath.Join(dir, "nonexistent"))
	if err != nil {
		t.Fatalf("NewestMtime error: %v", err)
	}
	info, _ := os.Stat(f)
	if !mt.Equal(info.ModTime()) {
		t.Errorf("mtime = %v, want %v", mt, info.ModTime())
	}
}

func TestNeedsRebuild_dir_mtime_on_deletion(t *testing.T) {
	dir := t.TempDir()
	inputDir := filepath.Join(dir, "inputs")
	if err := os.MkdirAll(inputDir, 0750); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, "output.txt")

	// Create a file, then create output newer
	f := filepath.Join(inputDir, "a.txt")
	os.WriteFile(f, []byte("a"), 0644)
	past := time.Now().Add(-10 * time.Second)
	os.Chtimes(f, past, past)
	os.Chtimes(inputDir, past, past)

	os.WriteFile(output, []byte("result"), 0644)

	if NeedsRebuild(output, inputDir) {
		t.Error("expected no rebuild before deletion")
	}

	// Delete the file — directory mtime should update
	if err := os.Remove(f); err != nil {
		t.Fatal(err)
	}

	if !NeedsRebuild(output, inputDir) {
		t.Error("expected rebuild after file deletion (dir mtime updated)")
	}
}
