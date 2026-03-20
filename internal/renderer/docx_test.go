package renderer

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestCheckPandoc_Available(t *testing.T) {
	if _, err := exec.LookPath("pandoc"); err != nil {
		t.Skip("pandoc not installed, skipping")
	}
	if err := CheckPandoc(); err != nil {
		t.Errorf("CheckPandoc() error: %v", err)
	}
}

func TestConvertMarkdownToDocx_Success(t *testing.T) {
	if _, err := exec.LookPath("pandoc"); err != nil {
		t.Skip("pandoc not installed, skipping")
	}

	dir := t.TempDir()
	mdPath := filepath.Join(dir, "test.md")
	docxPath := filepath.Join(dir, "test.docx")

	if err := os.WriteFile(mdPath, []byte("# Title\n\nSome text."), 0644); err != nil {
		t.Fatalf("write md: %v", err)
	}

	err := ConvertMarkdownToDocx(context.Background(), mdPath, docxPath)
	if err != nil {
		t.Fatalf("ConvertMarkdownToDocx() error: %v", err)
	}

	info, err := os.Stat(docxPath)
	if err != nil {
		t.Fatalf("docx file missing: %v", err)
	}
	if info.Size() == 0 {
		t.Error("docx file is empty")
	}
}

func TestConvertMarkdownToDocx_MissingInput(t *testing.T) {
	if _, err := exec.LookPath("pandoc"); err != nil {
		t.Skip("pandoc not installed, skipping")
	}

	dir := t.TempDir()
	err := ConvertMarkdownToDocx(context.Background(), "/nonexistent/file.md", filepath.Join(dir, "out.docx"))
	if err == nil {
		t.Error("expected error for missing input file")
	}
}

func TestConvertMarkdownToDocx_ContextCancelled(t *testing.T) {
	if _, err := exec.LookPath("pandoc"); err != nil {
		t.Skip("pandoc not installed, skipping")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	// Give the context time to expire
	time.Sleep(1 * time.Millisecond)

	dir := t.TempDir()
	mdPath := filepath.Join(dir, "test.md")
	os.WriteFile(mdPath, []byte("# Title"), 0644)

	err := ConvertMarkdownToDocx(ctx, mdPath, filepath.Join(dir, "out.docx"))
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}
