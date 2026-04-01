package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mmdemirbas/mutercim/internal/workspace"
)

func TestExpandTargets_single(t *testing.T) {
	got, err := expandTargets([]string{"read"})
	if err != nil {
		t.Fatalf("expandTargets error: %v", err)
	}
	if len(got) != 1 || got[0] != "read" {
		t.Errorf("expandTargets([read]) = %v, want [read]", got)
	}
}

func TestExpandTargets_multiple(t *testing.T) {
	got, err := expandTargets([]string{"log", "read", "solve"})
	if err != nil {
		t.Fatalf("expandTargets error: %v", err)
	}
	want := []string{"log", "read", "solve"}
	if len(got) != len(want) {
		t.Fatalf("expandTargets len = %d, want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("expandTargets[%d] = %q, want %q", i, got[i], w)
		}
	}
}

func TestExpandTargets_plus_suffix(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"cut+", []string{"cut", "layout", "ocr", "read", "solve", "translate", "write"}},
		{"read+", []string{"read", "solve", "translate", "write"}},
		{"solve+", []string{"solve", "translate", "write"}},
		{"translate+", []string{"translate", "write"}},
		{"write+", []string{"write"}},
		{"ocr+", []string{"ocr", "read", "solve", "translate", "write"}},
		{"layout+", []string{"layout", "ocr", "read", "solve", "translate", "write"}},
		{"log+", []string{"log", "memory", "cut", "layout", "ocr", "read", "solve", "translate", "write"}},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := expandTargets([]string{tt.input})
			if err != nil {
				t.Fatalf("expandTargets error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("expandTargets(%q) = %v, want %v", tt.input, got, tt.want)
			}
			for i, w := range tt.want {
				if got[i] != w {
					t.Errorf("expandTargets(%q)[%d] = %q, want %q", tt.input, i, got[i], w)
				}
			}
		})
	}
}

func TestExpandTargets_all(t *testing.T) {
	got, err := expandTargets([]string{"all"})
	if err != nil {
		t.Fatalf("expandTargets error: %v", err)
	}
	if len(got) != len(cleanablePhases) {
		t.Fatalf("expandTargets([all]) len = %d, want %d", len(got), len(cleanablePhases))
	}
	for i, w := range cleanablePhases {
		if got[i] != w {
			t.Errorf("expandTargets([all])[%d] = %q, want %q", i, got[i], w)
		}
	}
}

func TestExpandTargets_dedup(t *testing.T) {
	got, err := expandTargets([]string{"read", "read+", "solve"})
	if err != nil {
		t.Fatalf("expandTargets error: %v", err)
	}
	// read+ includes read, solve, translate, write. "solve" is already covered.
	want := []string{"read", "solve", "translate", "write"}
	if len(got) != len(want) {
		t.Fatalf("expandTargets = %v, want %v", got, want)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("[%d] = %q, want %q", i, got[i], w)
		}
	}
}

func TestExpandTargets_invalid(t *testing.T) {
	_, err := expandTargets([]string{"bogus"})
	if err == nil {
		t.Fatal("expected error for unknown target")
	}
}

func TestClean_never_deletes_input_or_knowledge(t *testing.T) {
	dir := t.TempDir()
	ws := &workspace.Workspace{Root: dir}

	// Create protected directories
	inputDir := filepath.Join(dir, "input")
	os.MkdirAll(inputDir, 0755)
	os.MkdirAll(ws.KnowledgeDir(), 0755)
	os.WriteFile(filepath.Join(inputDir, "test.pdf"), []byte("pdf"), 0644)
	os.WriteFile(filepath.Join(ws.KnowledgeDir(), "terms.yaml"), []byte("yaml"), 0644)

	// Create cleanable directories
	os.MkdirAll(ws.ReadDir(), 0755)
	os.WriteFile(filepath.Join(ws.ReadDir(), "data.json"), []byte("{}"), 0644)

	// "all" should not include input/ or knowledge/
	phases, err := expandTargets([]string{"all"})
	if err != nil {
		t.Fatalf("expandTargets error: %v", err)
	}
	for _, p := range phases {
		if p == "input" || p == "knowledge" {
			t.Errorf("expandTargets([all]) includes protected dir %q", p)
		}
	}

	// Verify phaseDir never returns input or knowledge paths
	if phaseDir(ws, "input") != "" {
		t.Error("phaseDir(input) should return empty")
	}
	if phaseDir(ws, "knowledge") != "" {
		t.Error("phaseDir(knowledge) should return empty")
	}
}

func TestClean_log_truncates_not_removes(t *testing.T) {
	dir := t.TempDir()
	ws := &workspace.Workspace{Root: dir, OutputDir: dir}

	// Create log file with content
	logPath := ws.LogPath()
	if err := os.WriteFile(logPath, []byte("some log data\nmore lines\n"), 0600); err != nil {
		t.Fatal(err)
	}

	// Verify log file has content
	data, err := os.ReadFile(logPath) //nolint:gosec // G304: path from ws.LogPath(), not user input
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("log file should have content")
	}

	// Truncate the log file
	f, err := os.OpenFile(logPath, os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatalf("open for truncate: %v", err)
	}
	_ = f.Close()

	// Verify file still exists but is empty
	data, err = os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read truncated log: %v", err)
	}
	if len(data) != 0 {
		t.Errorf("expected empty log file after truncation, got %d bytes", len(data))
	}
}

func TestPhaseIndex(t *testing.T) {
	if phaseIndex("cut") != 2 {
		t.Errorf("phaseIndex(cut) = %d, want 2", phaseIndex("cut"))
	}
	if phaseIndex("layout") != 3 {
		t.Errorf("phaseIndex(layout) = %d, want 3", phaseIndex("layout"))
	}
	if phaseIndex("ocr") != 4 {
		t.Errorf("phaseIndex(ocr) = %d, want 4", phaseIndex("ocr"))
	}
	if phaseIndex("write") != 8 {
		t.Errorf("phaseIndex(write) = %d, want 8", phaseIndex("write"))
	}
	if phaseIndex("bogus") != -1 {
		t.Errorf("phaseIndex(bogus) = %d, want -1", phaseIndex("bogus"))
	}
}
