// Package docker provides shared helpers for running tools inside Docker containers.
// All external tool dependencies (pdftoppm, pandoc, DocLayout-YOLO, XeLaTeX, Surya)
// run in containers. Docker is the single external runtime dependency.
package docker

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CheckAvailable returns an error if Docker is not installed or the daemon is not running.
func CheckAvailable(ctx context.Context) error {
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("docker is required but not available — install Docker and ensure the daemon is running")
	}
	out, err := exec.CommandContext(ctx, "docker", "info", "--format", "{{.ServerVersion}}").CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker daemon is not running — start Docker and try again: %w", err)
	}
	if strings.TrimSpace(string(out)) == "" {
		return fmt.Errorf("docker daemon returned empty version — ensure Docker is running correctly")
	}
	return nil
}

// EnsureImage checks if a Docker image exists locally. If not, it builds it from
// the given Dockerfile directory. Logs the build at INFO level.
// If dockerfileDir is empty, auto-build is skipped and an informative error is returned
// when the image is missing.
func EnsureImage(ctx context.Context, image, dockerfileDir string) error {
	// Check if image already exists
	out, err := exec.CommandContext(ctx, "docker", "image", "inspect", image, "--format", "{{.ID}}").CombinedOutput()
	if err == nil && strings.TrimSpace(string(out)) != "" {
		return nil // image exists
	}

	// Image missing — try to build
	if dockerfileDir == "" {
		return fmt.Errorf("docker image %s not found — build it with: docker build -t %s docker/%s/",
			image, image, imageShortName(image))
	}

	slog.Info("building docker image", "image", image, "dir", dockerfileDir)
	buildOut, err := exec.CommandContext(ctx, "docker", "build", "-t", image, dockerfileDir).CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker build %s failed: %w\n%s", image, err, truncateOutput(string(buildOut), 500))
	}
	return nil
}

// imageShortName extracts the short name from an image like "mutercim/poppler:latest" → "poppler".
func imageShortName(image string) string {
	// Strip tag
	if idx := strings.LastIndex(image, ":"); idx >= 0 {
		image = image[:idx]
	}
	// Strip registry/org prefix
	if idx := strings.LastIndex(image, "/"); idx >= 0 {
		image = image[idx+1:]
	}
	return image
}

// Run executes a docker command with the given arguments and returns its combined output.
// The first element of args should be the docker subcommand (e.g. "run").
func Run(ctx context.Context, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, "docker", args...).CombinedOutput()
}

// FindDockerDir returns the path to docker/<tool>/ directory if it can be found.
// It checks relative to the current working directory first, then relative to the
// executable location. Returns empty string if not found.
func FindDockerDir(tool string) string {
	// Check relative to cwd (works when running from repo root)
	dir := filepath.Join("docker", tool)
	if isDockerfileDir(dir) {
		return dir
	}

	// Check relative to executable (works for `go install` from repo)
	if exe, err := os.Executable(); err == nil {
		dir = filepath.Join(filepath.Dir(exe), "..", "docker", tool)
		if isDockerfileDir(dir) {
			return dir
		}
	}

	return ""
}

func isDockerfileDir(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, "Dockerfile"))
	return err == nil && !info.IsDir()
}

func truncateOutput(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[len(runes)-maxLen:]) + "\n... (truncated)"
}
