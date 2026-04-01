package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mmdemirbas/mutercim/internal/config"
	"github.com/mmdemirbas/mutercim/internal/workspace"
)

func TestHasPhaseOutput_empty_workspace(t *testing.T) {
	dir := t.TempDir()
	ws := &workspace.Workspace{Root: dir}
	cfg := &config.Config{
		Translate: config.TranslateConfig{Languages: []string{"tr"}},
	}

	for _, p := range []phase{phaseCut, phaseRead, phaseSolve, phaseTranslate} {
		if hasPhaseOutput(p, ws, cfg) {
			t.Errorf("hasPhaseOutput(%s) = true for empty workspace", phaseName(p))
		}
	}
}

func TestHasPhaseOutput_with_data(t *testing.T) {
	dir := t.TempDir()
	ws := &workspace.Workspace{Root: dir}
	cfg := &config.Config{
		Translate: config.TranslateConfig{Languages: []string{"tr"}},
	}

	// Create cut output
	imgDir := filepath.Join(dir, "cut", "TestBook")
	if err := os.MkdirAll(imgDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(imgDir, "001.png"), []byte("fake"), 0600); err != nil {
		t.Fatal(err)
	}
	if !hasPhaseOutput(phaseCut, ws, cfg) {
		t.Error("hasPhaseOutput(cut) = false after creating cut output")
	}

	// Create read output
	readDir := filepath.Join(dir, "read", "TestBook")
	if err := os.MkdirAll(readDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(readDir, "001.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if !hasPhaseOutput(phaseRead, ws, cfg) {
		t.Error("hasPhaseOutput(read) = false after creating read output")
	}

	// Create solve output
	solvedDir := filepath.Join(dir, "solve", "TestBook")
	if err := os.MkdirAll(solvedDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(solvedDir, "001.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if !hasPhaseOutput(phaseSolve, ws, cfg) {
		t.Error("hasPhaseOutput(solve) = false after creating solve output")
	}

	// Translate not yet present
	if hasPhaseOutput(phaseTranslate, ws, cfg) {
		t.Error("hasPhaseOutput(translate) = true before creating translate output")
	}

	// Create translate output
	translatedDir := filepath.Join(dir, "translate", "tr", "TestBook")
	os.MkdirAll(translatedDir, 0755)
	os.WriteFile(filepath.Join(translatedDir, "001.json"), []byte("{}"), 0644)
	if !hasPhaseOutput(phaseTranslate, ws, cfg) {
		t.Error("hasPhaseOutput(translate) = false after creating translate output")
	}
}

func TestHasPhaseOutput_translate_multiple_langs(t *testing.T) {
	dir := t.TempDir()
	ws := &workspace.Workspace{Root: dir}
	cfg := &config.Config{
		Translate: config.TranslateConfig{Languages: []string{"tr", "en"}},
	}

	// No output for any language
	if hasPhaseOutput(phaseTranslate, ws, cfg) {
		t.Error("hasPhaseOutput(translate) = true with no translated output")
	}

	// Output for one language is sufficient
	translatedDir := filepath.Join(dir, "translate", "en", "TestBook")
	os.MkdirAll(translatedDir, 0755)
	os.WriteFile(filepath.Join(translatedDir, "001.json"), []byte("{}"), 0644)
	if !hasPhaseOutput(phaseTranslate, ws, cfg) {
		t.Error("hasPhaseOutput(translate) = false when one target lang has output")
	}
}

func TestDirHasEntries(t *testing.T) {
	dir := t.TempDir()

	// Non-existent dir
	if dirHasEntries(filepath.Join(dir, "nope")) {
		t.Error("dirHasEntries returns true for non-existent dir")
	}

	// Empty dir
	emptyDir := filepath.Join(dir, "empty")
	os.MkdirAll(emptyDir, 0755)
	if dirHasEntries(emptyDir) {
		t.Error("dirHasEntries returns true for empty dir")
	}

	// Dir with file
	os.WriteFile(filepath.Join(emptyDir, "file.txt"), []byte("hi"), 0644)
	if !dirHasEntries(emptyDir) {
		t.Error("dirHasEntries returns false for dir with file")
	}
}

func TestPhaseName(t *testing.T) {
	tests := []struct {
		p    phase
		want string
	}{
		{phaseCut, "cut"},
		{phaseRead, "read"},
		{phaseSolve, "solve"},
		{phaseTranslate, "translate"},
		{phaseWrite, "write"},
		{phase(99), "unknown"},
	}
	for _, tt := range tests {
		if got := phaseName(tt.p); got != tt.want {
			t.Errorf("phaseName(%d) = %q, want %q", tt.p, got, tt.want)
		}
	}
}
