package docker

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCheckAvailable_DockerInstalled(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not installed")
	}
	err := CheckAvailable(context.Background())
	// If Docker is installed but daemon not running, that's also acceptable to fail.
	// We only test that the function doesn't panic.
	_ = err
}

func TestTruncateOutput_Short(t *testing.T) {
	got := truncateOutput("hello", 10)
	if got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

func TestTruncateOutput_Long(t *testing.T) {
	got := truncateOutput("abcdefghij", 5)
	if len(got) == 0 {
		t.Error("expected non-empty output")
	}
	if got == "abcdefghij" {
		t.Error("expected truncation")
	}
}

func TestImageShortName(t *testing.T) {
	tests := []struct {
		image string
		want  string
	}{
		{"mutercim/poppler:latest", "poppler"},
		{"mutercim/doclayout-yolo:latest", "doclayout-yolo"},
		{"poppler:latest", "poppler"},
		{"poppler", "poppler"},
		{"ghcr.io/mmdemirbas/mutercim/xelatex:v1", "xelatex"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.image, func(t *testing.T) {
			got := imageShortName(tt.image)
			if got != tt.want {
				t.Errorf("imageShortName(%q) = %q, want %q", tt.image, got, tt.want)
			}
		})
	}
}

func TestIsDockerfileDir(t *testing.T) {
	dir := t.TempDir()

	if isDockerfileDir(dir) {
		t.Error("empty dir should not be a Dockerfile dir")
	}

	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM scratch\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if !isDockerfileDir(dir) {
		t.Error("dir with Dockerfile should be a Dockerfile dir")
	}
}

func TestFindDockerDir_NotFound(t *testing.T) {
	// Save and restore cwd
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(orig) }()

	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	got := FindDockerDir("nonexistent-tool")
	if got != "" {
		t.Errorf("FindDockerDir(nonexistent-tool) = %q, want empty", got)
	}
}

func TestFindDockerDir_Found(t *testing.T) {
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(orig) }()

	dir := t.TempDir()
	toolDir := filepath.Join(dir, "docker", "testtool")
	if err := os.MkdirAll(toolDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(toolDir, "Dockerfile"), []byte("FROM scratch\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	got := FindDockerDir("testtool")
	if got == "" {
		t.Error("FindDockerDir(testtool) returned empty, expected path")
	}
}

func TestTruncateOutput_Unicode(t *testing.T) {
	// 5 runes: こんにちは
	got := truncateOutput("こんにちは", 3)
	if got == "こんにちは" {
		t.Error("expected truncation of Unicode string")
	}
}

func TestTruncateOutput_ExactLength(t *testing.T) {
	got := truncateOutput("abcde", 5)
	if got != "abcde" {
		t.Errorf("got %q, want %q", got, "abcde")
	}
}
