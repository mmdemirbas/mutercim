package knowledge

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEmbeddedDefaults(t *testing.T) {
	k, err := Load("", "")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if len(k.Honorifics) == 0 {
		t.Error("expected embedded honorifics, got none")
	}
	if len(k.Companions) == 0 {
		t.Error("expected embedded companions, got none")
	}
	if len(k.Terminology) == 0 {
		t.Error("expected embedded terminology, got none")
	}
	if len(k.Places) == 0 {
		t.Error("expected embedded places, got none")
	}
}

func TestLoadWorkspaceOverrides(t *testing.T) {
	dir := t.TempDir()

	// Write a workspace sources file
	yaml := `entries:
  - code: "خ"
    name_ar: "صحيح البخاري"
    name_tr: "Sahîh-i Buhârî"
    author_tr: "İmam Buhârî"
  - code: "م"
    name_ar: "صحيح مسلم"
    name_tr: "Sahîh-i Müslim"
`
	if err := os.WriteFile(filepath.Join(dir, "sources.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatalf("write sources: %v", err)
	}

	k, err := Load(dir, "")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if len(k.Sources) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(k.Sources))
	}
	if k.Sources[0].Layer != "workspace" {
		t.Errorf("expected layer 'workspace', got %q", k.Sources[0].Layer)
	}
}

func TestLoadStagedOverrides(t *testing.T) {
	workDir := t.TempDir()
	stagedDir := t.TempDir()

	// Workspace source
	wsYaml := `entries:
  - code: "خ"
    name_ar: "صحيح البخاري"
    name_tr: "Sahîh-i Buhârî (workspace)"
`
	if err := os.WriteFile(filepath.Join(workDir, "sources.yaml"), []byte(wsYaml), 0644); err != nil {
		t.Fatalf("write workspace sources: %v", err)
	}

	// Staged source overrides workspace
	stagedYaml := `entries:
  - code: "خ"
    name_ar: "صحيح البخاري"
    name_tr: "Sahîh-i Buhârî (staged)"
`
	if err := os.WriteFile(filepath.Join(stagedDir, "sources.yaml"), []byte(stagedYaml), 0644); err != nil {
		t.Fatalf("write staged sources: %v", err)
	}

	k, err := Load(workDir, stagedDir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	src, ok := k.LookupSource("خ")
	if !ok {
		t.Fatal("expected source 'خ' to exist")
	}
	// Staged should override workspace
	if src.NameTr != "Sahîh-i Buhârî (staged)" {
		t.Errorf("expected staged override, got %q", src.NameTr)
	}
	if src.Layer != "staged" {
		t.Errorf("expected layer 'staged', got %q", src.Layer)
	}
}

func TestLoadNonexistentDirs(t *testing.T) {
	k, err := Load("/nonexistent/knowledge", "/nonexistent/staged")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	// Should still have embedded defaults
	if len(k.Honorifics) == 0 {
		t.Error("expected embedded honorifics even with missing dirs")
	}
}

func TestLookupSource(t *testing.T) {
	k := &Knowledge{
		Sources: []Source{
			{Code: "خ", NameAr: "صحيح البخاري", NameTr: "Sahîh-i Buhârî"},
		},
	}

	src, ok := k.LookupSource("خ")
	if !ok {
		t.Fatal("expected source to be found")
	}
	if src.NameTr != "Sahîh-i Buhârî" {
		t.Errorf("expected 'Sahîh-i Buhârî', got %q", src.NameTr)
	}

	_, ok = k.LookupSource("nonexistent")
	if ok {
		t.Error("expected source not to be found")
	}
}
