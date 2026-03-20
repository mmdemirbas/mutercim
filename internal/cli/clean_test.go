package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mmdemirbas/mutercim/internal/progress"
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
		{"pages+", []string{"pages", "read", "solve", "translate", "write"}},
		{"read+", []string{"read", "solve", "translate", "write"}},
		{"solve+", []string{"solve", "translate", "write"}},
		{"translate+", []string{"translate", "write"}},
		{"write+", []string{"write"}},
		{"log+", []string{"log", "memory", "pages", "read", "solve", "translate", "write"}},
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
	os.MkdirAll(ws.InputDir(), 0755)
	os.MkdirAll(ws.KnowledgeDir(), 0755)
	os.WriteFile(filepath.Join(ws.InputDir(), "test.pdf"), []byte("pdf"), 0644)
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

func TestClean_resets_progress(t *testing.T) {
	dir := t.TempDir()
	ws := &workspace.Workspace{Root: dir}

	// Create progress file with some data
	tracker := progress.NewTracker(ws.ProgressPath())
	tracker.MarkCompleted("read:TestBook", 1)
	tracker.MarkCompleted("read:TestBook", 2)
	tracker.MarkCompleted("solve:TestBook", 1)
	tracker.MarkCompleted("translate:tr:TestBook", 1)
	if err := tracker.Save(); err != nil {
		t.Fatalf("save progress: %v", err)
	}

	// Simulate cleaning read+ using the same approach as the real clean command
	phases, _ := expandTargets([]string{"read+"})

	tracker2 := progress.NewTracker(ws.ProgressPath())
	if err := tracker2.Load(); err != nil {
		t.Fatalf("reload progress: %v", err)
	}

	for _, phase := range phases {
		for _, prefix := range progressPrefixes(phase) {
			for _, name := range tracker2.PhaseNames() {
				if strings.HasPrefix(string(name), prefix) {
					tracker2.DeletePhase(name)
				}
			}
		}
	}
	if err := tracker2.Save(); err != nil {
		t.Fatalf("save progress: %v", err)
	}

	// Reload and verify
	tracker3 := progress.NewTracker(ws.ProgressPath())
	if err := tracker3.Load(); err != nil {
		t.Fatalf("reload progress: %v", err)
	}
	state := tracker3.State()

	if _, ok := state.Phases["read:TestBook"]; ok {
		t.Error("read:TestBook should be deleted from progress")
	}
	if _, ok := state.Phases["solve:TestBook"]; ok {
		t.Error("solve:TestBook should be deleted from progress")
	}
	if _, ok := state.Phases["translate:tr:TestBook"]; ok {
		t.Error("translate:tr:TestBook should be deleted from progress")
	}
}

func TestPhaseIndex(t *testing.T) {
	if phaseIndex("pages") != 2 {
		t.Errorf("phaseIndex(pages) = %d, want 2", phaseIndex("pages"))
	}
	if phaseIndex("write") != 6 {
		t.Errorf("phaseIndex(write) = %d, want 6", phaseIndex("write"))
	}
	if phaseIndex("bogus") != -1 {
		t.Errorf("phaseIndex(bogus) = %d, want -1", phaseIndex("bogus"))
	}
}
