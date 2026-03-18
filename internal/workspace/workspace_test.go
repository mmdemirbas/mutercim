package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInit(t *testing.T) {
	dir := t.TempDir()

	ws, err := Init(InitOptions{
		Dir:        dir,
		Title:      "Test Book",
		Author:     "Test Author",
		SourceLang: "ar",
		TargetLang: "tr",
	})
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	if ws.Root != dir {
		t.Errorf("Root = %q, want %q", ws.Root, dir)
	}

	// Verify directories exist
	dirs := []string{
		"input",
		"output/arabic/pages",
		"output/turkish/pages",
		"output/latex",
		"cache/images",
		"cache/extracted",
		"cache/enriched",
		"cache/translated",
		"cache/staged",
		"knowledge",
		"reports",
	}
	for _, d := range dirs {
		fullPath := filepath.Join(dir, d)
		if fi, err := os.Stat(fullPath); err != nil || !fi.IsDir() {
			t.Errorf("directory %s not created", d)
		}
	}

	// Verify config file exists
	if _, err := os.Stat(filepath.Join(dir, "mutercim.yaml")); err != nil {
		t.Error("mutercim.yaml not created")
	}

	// Verify progress.json exists
	if _, err := os.Stat(filepath.Join(dir, "progress.json")); err != nil {
		t.Error("progress.json not created")
	}

	// Verify config content
	data, err := os.ReadFile(filepath.Join(dir, "mutercim.yaml"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	content := string(data)
	if !contains(content, "Test Book") {
		t.Error("config missing title")
	}
	if !contains(content, "Test Author") {
		t.Error("config missing author")
	}
}

func TestInitDoubleInit(t *testing.T) {
	dir := t.TempDir()

	_, err := Init(InitOptions{Dir: dir})
	if err != nil {
		t.Fatalf("first Init() error = %v", err)
	}

	_, err = Init(InitOptions{Dir: dir})
	if err == nil {
		t.Error("second Init() should fail")
	}
}

func TestInitDefaults(t *testing.T) {
	dir := t.TempDir()

	_, err := Init(InitOptions{Dir: dir})
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "mutercim.yaml"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	content := string(data)

	if !contains(content, "source_lang: ar") {
		t.Error("config missing default source_lang")
	}
	if !contains(content, "target_lang: tr") {
		t.Error("config missing default target_lang")
	}
}

func TestDiscover(t *testing.T) {
	dir := t.TempDir()

	// Init workspace
	Init(InitOptions{Dir: dir})

	// Discover from root
	ws, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if ws.Root != dir {
		t.Errorf("Root = %q, want %q", ws.Root, dir)
	}

	// Discover from subdirectory
	subDir := filepath.Join(dir, "cache", "extracted")
	ws, err = Discover(subDir)
	if err != nil {
		t.Fatalf("Discover() from subdir error = %v", err)
	}
	if ws.Root != dir {
		t.Errorf("Root from subdir = %q, want %q", ws.Root, dir)
	}
}

func TestDiscoverNotFound(t *testing.T) {
	dir := t.TempDir()

	_, err := Discover(dir)
	if err == nil {
		t.Error("Discover() should fail when no workspace exists")
	}
}

func TestWorkspacePaths(t *testing.T) {
	ws := &Workspace{Root: "/tmp/test"}

	if ws.ConfigPath() != "/tmp/test/mutercim.yaml" {
		t.Errorf("ConfigPath() = %q", ws.ConfigPath())
	}
	if ws.InputDir() != "/tmp/test/input" {
		t.Errorf("InputDir() = %q", ws.InputDir())
	}
	if ws.CacheDir() != "/tmp/test/cache" {
		t.Errorf("CacheDir() = %q", ws.CacheDir())
	}
	if ws.ProgressPath() != "/tmp/test/progress.json" {
		t.Errorf("ProgressPath() = %q", ws.ProgressPath())
	}
	if ws.StagedDir() != "/tmp/test/cache/staged" {
		t.Errorf("StagedDir() = %q", ws.StagedDir())
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
