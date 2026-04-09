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

//nolint:cyclop // table-driven test complexity is inherent
func TestHasPhaseOutput_with_data(t *testing.T) {
	dir := t.TempDir()
	ws := &workspace.Workspace{Root: dir}
	cfg := &config.Config{
		Translate: config.TranslateConfig{Languages: []string{"tr"}},
	}

	// Create cut output with completion marker
	imgDir := filepath.Join(dir, "cut", "TestBook")
	if err := os.MkdirAll(imgDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(imgDir, "001.png"), []byte("fake"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(imgDir, "report.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if !hasPhaseOutput(phaseCut, ws, cfg) {
		t.Error("hasPhaseOutput(cut) = false after creating cut output with report.json")
	}

	// Create read output with report.json
	readDir := filepath.Join(dir, "read", "TestBook")
	if err := os.MkdirAll(readDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(readDir, "001.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(readDir, "report.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if !hasPhaseOutput(phaseRead, ws, cfg) {
		t.Error("hasPhaseOutput(read) = false after creating read output with report.json")
	}

	// Create solve output with report.json
	solvedDir := filepath.Join(dir, "solve", "TestBook")
	if err := os.MkdirAll(solvedDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(solvedDir, "001.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(solvedDir, "report.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if !hasPhaseOutput(phaseSolve, ws, cfg) {
		t.Error("hasPhaseOutput(solve) = false after creating solve output with report.json")
	}

	// Translate not yet present
	if hasPhaseOutput(phaseTranslate, ws, cfg) {
		t.Error("hasPhaseOutput(translate) = true before creating translate output")
	}

	// Create translate output with report.json
	translatedDir := filepath.Join(dir, "translate", "tr", "TestBook")
	if err := os.MkdirAll(translatedDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(translatedDir, "001.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(translatedDir, "report.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if !hasPhaseOutput(phaseTranslate, ws, cfg) {
		t.Error("hasPhaseOutput(translate) = false after creating translate output with report.json")
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

	// Output for one language with report.json is sufficient
	translatedDir := filepath.Join(dir, "translate", "en", "TestBook")
	if err := os.MkdirAll(translatedDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(translatedDir, "001.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(translatedDir, "report.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if !hasPhaseOutput(phaseTranslate, ws, cfg) {
		t.Error("hasPhaseOutput(translate) = false when one target lang has output with report.json")
	}
}

func TestHasReport(t *testing.T) {
	dir := t.TempDir()

	// Non-existent dir
	if hasReport(filepath.Join(dir, "nope")) {
		t.Error("hasReport returns true for non-existent dir")
	}

	// Empty dir
	emptyDir := filepath.Join(dir, "empty")
	if err := os.MkdirAll(emptyDir, 0750); err != nil {
		t.Fatal(err)
	}
	if hasReport(emptyDir) {
		t.Error("hasReport returns true for empty dir")
	}

	// Dir with files but no report.json (simulates interrupted phase)
	subDir := filepath.Join(emptyDir, "input1")
	if err := os.MkdirAll(subDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "001.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if hasReport(emptyDir) {
		t.Error("hasReport returns true for dir without report.json")
	}

	// Dir with report.json in subdirectory (complete phase)
	if err := os.WriteFile(filepath.Join(subDir, "report.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if !hasReport(emptyDir) {
		t.Error("hasReport returns false for dir with report.json in subdirectory")
	}

	// Dir with report.json directly (cut phase pattern)
	cutDir := filepath.Join(dir, "cut")
	if err := os.MkdirAll(cutDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cutDir, "report.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if !hasReport(cutDir) {
		t.Error("hasReport returns false for dir with report.json directly")
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
