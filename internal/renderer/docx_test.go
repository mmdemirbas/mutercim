package renderer

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func skipWithoutDocker(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not installed, skipping")
	}
	out, err := exec.CommandContext(context.Background(), "docker", "info", "--format", "{{.ServerVersion}}").CombinedOutput()
	if err != nil || len(out) == 0 {
		t.Skip("docker daemon not running, skipping")
	}
}

func skipWithoutPandocImage(t *testing.T) {
	t.Helper()
	skipWithoutDocker(t)
	out, err := exec.CommandContext(context.Background(), "docker", "image", "inspect", DefaultPandocImage, "--format", "{{.ID}}").CombinedOutput()
	if err != nil || len(strings.TrimSpace(string(out))) == 0 {
		t.Skipf("docker image %s not available, skipping", DefaultPandocImage)
	}
}

func TestConvertMarkdownToDocx_Success(t *testing.T) {
	skipWithoutPandocImage(t)

	dir := t.TempDir()
	mdPath := filepath.Join(dir, "test.md")
	docxPath := filepath.Join(dir, "test.docx")

	if err := os.WriteFile(mdPath, []byte("# Title\n\nSome text."), 0644); err != nil {
		t.Fatalf("write md: %v", err)
	}

	err := ConvertMarkdownToDocx(context.Background(), mdPath, docxPath, "")
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
	skipWithoutPandocImage(t)

	dir := t.TempDir()
	err := ConvertMarkdownToDocx(context.Background(), filepath.Join(dir, "nonexistent.md"), filepath.Join(dir, "out.docx"), "")
	if err == nil {
		t.Error("expected error for missing input file")
	}
}

func TestConvertMarkdownToDocx_ContextCancelled(t *testing.T) {
	skipWithoutPandocImage(t)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(1 * time.Millisecond)

	dir := t.TempDir()
	mdPath := filepath.Join(dir, "test.md")
	os.WriteFile(mdPath, []byte("# Title"), 0644)

	err := ConvertMarkdownToDocx(ctx, mdPath, filepath.Join(dir, "out.docx"), "")
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}
