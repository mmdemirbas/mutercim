package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListStagedFiles(t *testing.T) {
	dir := t.TempDir()
	ws := &Workspace{Root: dir}

	// No staged dir yet
	files, err := ws.ListStagedFiles()
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0, got %d", len(files))
	}

	// Create staged dir with files
	stagedDir := ws.MemoryDir()
	os.MkdirAll(stagedDir, 0755)
	os.WriteFile(filepath.Join(stagedDir, "sources.yaml"), []byte("entries: []"), 0644)
	os.WriteFile(filepath.Join(stagedDir, "other.txt"), []byte("x"), 0644)
	os.MkdirAll(filepath.Join(stagedDir, "subdir"), 0755)

	files, err = ws.ListStagedFiles()
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 yaml file, got %d: %v", len(files), files)
	}
	if files[0] != "sources.yaml" {
		t.Errorf("expected 'sources.yaml', got %q", files[0])
	}
}

func TestPromoteStagedFile(t *testing.T) {
	dir := t.TempDir()
	ws := &Workspace{Root: dir}

	// Create staged and knowledge dirs
	os.MkdirAll(ws.MemoryDir(), 0755)
	os.MkdirAll(ws.KnowledgeDir(), 0755)

	// Create a staged file
	content := []byte("entries:\n  - code: test\n")
	os.WriteFile(filepath.Join(ws.MemoryDir(), "sources.yaml"), content, 0644)

	err := ws.PromoteStagedFile("sources.yaml", false)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	// Verify promoted file exists
	promoted, err := os.ReadFile(filepath.Join(ws.KnowledgeDir(), "sources.yaml"))
	if err != nil {
		t.Fatalf("read promoted: %v", err)
	}
	if string(promoted) != string(content) {
		t.Errorf("content mismatch")
	}
}

func TestPromoteStagedFileAlreadyExists(t *testing.T) {
	dir := t.TempDir()
	ws := &Workspace{Root: dir}

	os.MkdirAll(ws.MemoryDir(), 0755)
	os.MkdirAll(ws.KnowledgeDir(), 0755)

	os.WriteFile(filepath.Join(ws.MemoryDir(), "sources.yaml"), []byte("new"), 0644)
	os.WriteFile(filepath.Join(ws.KnowledgeDir(), "sources.yaml"), []byte("existing"), 0644)

	// Without replace — should error
	err := ws.PromoteStagedFile("sources.yaml", false)
	if err == nil {
		t.Fatal("expected error when file exists without --replace")
	}

	// With replace — should succeed
	err = ws.PromoteStagedFile("sources.yaml", true)
	if err != nil {
		t.Fatalf("error with replace: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(ws.KnowledgeDir(), "sources.yaml"))
	if string(data) != "new" {
		t.Errorf("expected replaced content 'new', got %q", string(data))
	}
}

func TestPromoteStagedFileNotFound(t *testing.T) {
	dir := t.TempDir()
	ws := &Workspace{Root: dir}
	os.MkdirAll(ws.MemoryDir(), 0755)

	err := ws.PromoteStagedFile("nonexistent.yaml", false)
	if err == nil {
		t.Error("expected error for missing staged file")
	}
}
