package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInit(t *testing.T) {
	dir := t.TempDir()

	ws, err := Init(InitOptions{
		Dir:         dir,
		SourceLangs: "ar",
		TargetLangs: "tr",
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
		"knowledge",
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

	// Verify config content
	data, err := os.ReadFile(filepath.Join(dir, "mutercim.yaml")) //nolint:gosec // G304: reading from t.TempDir(), not user input
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	content := string(data)
	if !contains(content, "languages: [ar]") {
		t.Error("config missing source language")
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

	data, err := os.ReadFile(filepath.Join(dir, "mutercim.yaml")) //nolint:gosec // G304: reading from t.TempDir(), not user input
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	content := string(data)

	if !contains(content, "languages: [ar]") {
		t.Error("config missing default source language")
	}
	if !contains(content, "languages: [tr]") {
		t.Error("config missing default target language")
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
	subDir := filepath.Join(dir, "midstate", "read")
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
	if ws.MemoryDir() != "/tmp/test/memory" {
		t.Errorf("MemoryDir() = %q", ws.MemoryDir())
	}
	if ws.LogPath() != "/tmp/test/mutercim.log" {
		t.Errorf("LogPath() = %q", ws.LogPath())
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
