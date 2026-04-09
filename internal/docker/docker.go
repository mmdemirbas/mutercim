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
	"time"
)

// CheckAvailable returns an error if Docker is not installed or the daemon is not running.
func CheckAvailable(ctx context.Context) error {
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("docker is required but not available — install Docker and ensure the daemon is running")
	}
	out, err := exec.CommandContext(ctx, "docker", "info", "--format", "{{.ServerVersion}}").CombinedOutput() //nolint:gosec // G204: docker is a fixed binary; args are hardcoded literals
	if err != nil {
		return fmt.Errorf("docker daemon is not running — start Docker and try again: %w", err)
	}
	if strings.TrimSpace(string(out)) == "" {
		return fmt.Errorf("docker daemon returned empty version — ensure Docker is running correctly")
	}
	return nil
}

// RegistryPrefix is the container registry prefix for pre-built images.
// When non-empty, EnsureImage tries pulling from the registry before building locally.
const RegistryPrefix = "ghcr.io/mmdemirbas/mutercim"

// EnsureImage ensures a Docker image is available locally.
//
// Strategy: check local → try registry pull → build from Dockerfile.
// If dockerfileDir is empty and both local check and pull fail, returns an error.
func EnsureImage(ctx context.Context, image, dockerfileDir string) error {
	// Check if image already exists locally
	if imageExistsLocally(ctx, image) {
		return nil
	}

	// Try pulling from registry
	if tryPullFromRegistry(ctx, image) {
		return nil
	}

	// Fall back to local build
	if dockerfileDir != "" {
		return buildImage(ctx, image, dockerfileDir)
	}

	return fmt.Errorf("docker image %s not found — build it with: docker build -t %s docker/%s/",
		image, image, imageShortName(image))
}

// imageExistsLocally checks if a Docker image exists in the local daemon.
func imageExistsLocally(ctx context.Context, image string) bool {
	out, err := exec.CommandContext(ctx, "docker", "image", "inspect", image, "--format", "{{.ID}}").CombinedOutput() //nolint:gosec // G204: docker is a fixed binary; image is a trusted internal value
	return err == nil && strings.TrimSpace(string(out)) != ""
}

// dockerOpTimeout is the maximum time allowed for docker pull and build operations.
const dockerOpTimeout = 10 * time.Minute

// tryPullFromRegistry attempts to pull a pre-built image from the configured registry.
// Returns true on success, false on any failure (caller should fall back to local build).
func tryPullFromRegistry(ctx context.Context, image string) bool {
	if RegistryPrefix == "" {
		return false
	}
	name := imageShortName(image)
	if name == "" {
		return false
	}
	remoteImage := RegistryPrefix + "/" + name + ":latest"
	slog.Info("pulling docker image from registry", "image", remoteImage)
	pullCtx, cancel := context.WithTimeout(ctx, dockerOpTimeout)
	defer cancel()
	out, err := exec.CommandContext(pullCtx, "docker", "pull", remoteImage).CombinedOutput() //nolint:gosec // G204: docker is a fixed binary; remoteImage is constructed from trusted constants
	if err != nil {
		slog.Debug("registry pull failed, will build locally", "image", remoteImage, "error", err, "output", strings.TrimSpace(string(out)))
		return false
	}
	// Tag as the local name so downstream code finds it
	_, _ = exec.CommandContext(pullCtx, "docker", "tag", remoteImage, image).CombinedOutput() //nolint:gosec // G204: tagging pulled image to local name
	slog.Info("pulled docker image from registry", "image", remoteImage)
	return true
}

// buildImage builds a Docker image from a Dockerfile directory.
func buildImage(ctx context.Context, image, dockerfileDir string) error {
	slog.Debug("building docker image", "image", image, "dir", dockerfileDir)
	buildCtx, cancel := context.WithTimeout(ctx, dockerOpTimeout)
	defer cancel()
	start := time.Now()
	out, err := exec.CommandContext(buildCtx, "docker", "build", "-t", image, dockerfileDir).CombinedOutput() //nolint:gosec // G204: docker is a fixed binary; image/dir are trusted internal values
	elapsed := time.Since(start)
	if err != nil {
		slog.Error("docker build failed", "image", image, "elapsed_s", int(elapsed.Seconds()), "output", string(out))
		return fmt.Errorf("docker build %s failed: %w\n%s", image, err, truncateOutput(string(out), 2000))
	}
	slog.Debug("docker build complete", "image", image, "elapsed_s", int(elapsed.Seconds()))
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
	return exec.CommandContext(ctx, "docker", args...).CombinedOutput() //nolint:gosec // G204: docker is a fixed binary; args are constructed by internal callers
}

// FindDockerDir returns the path to docker/<tool>/ directory if it can be found.
// It walks upward from the current working directory looking for docker/<tool>/Dockerfile,
// then checks relative to the executable location. Returns empty string if not found.
func FindDockerDir(tool string) string {
	// Walk upward from cwd (works from repo root or any subdirectory)
	if cwd, err := os.Getwd(); err == nil {
		dir := cwd
		for {
			candidate := filepath.Join(dir, "docker", tool)
			if isDockerfileDir(candidate) {
				return candidate
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}

	// Check relative to executable (works for `go install` from repo)
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Join(filepath.Dir(exe), "..", "docker", tool)
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
