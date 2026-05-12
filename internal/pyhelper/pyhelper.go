// Package pyhelper provides uv-managed Python tool lifecycle helpers.
//
// Each Python-based external tool (qari-ocr, doclayout-yolo, surya) can be
// run either via Docker (the existing default path) or via a uv-managed
// virtualenv directly on the host (this package). The uv path eliminates
// the Docker dependency for Python tools — they run as native subprocesses
// against pinned wheels under python-tools/<tool>/.
//
// Lifecycle mirrors the docker package's container model: EnsureUV +
// EnsureProject set up the environment; Server.Start spawns a long-running
// HTTP server, polls /health until ready, and exposes the assigned port.
// Server.Stop terminates the process and waits for exit. The HTTP request
// surface itself stays in each tool's own package (qari.go, etc.) — this
// helper only owns process lifecycle.
package pyhelper

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// MinUVVersion is the minimum supported uv version. Pinned to match what
// the spike used and what's been verified to work on macOS arm64.
const MinUVVersion = "0.11.0"

// EnsureUV returns an error if the uv binary is not on PATH. It does not
// install uv automatically — that's a host concern the user must handle
// (brew install uv / curl install script / etc.). Producing a clear,
// non-actionable failure here is better than silently installing.
func EnsureUV(ctx context.Context) error {
	path, err := exec.LookPath("uv")
	if err != nil {
		return fmt.Errorf("uv is required but not on PATH — install with 'brew install uv' or see https://docs.astral.sh/uv/#installation")
	}
	out, err := exec.CommandContext(ctx, path, "--version").CombinedOutput() //nolint:gosec // G204: uv is a fixed binary on PATH; no user input
	if err != nil {
		return fmt.Errorf("uv --version failed: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}
	if !strings.HasPrefix(strings.TrimSpace(string(out)), "uv ") {
		return fmt.Errorf("uv --version returned unexpected output: %q", strings.TrimSpace(string(out)))
	}
	return nil
}

// EnsureProject runs `uv sync` in the given project directory if the
// environment is stale (pyproject.toml or uv.lock newer than the venv
// marker). Idempotent — safe to call on every Start.
//
// projectDir must contain a pyproject.toml. Returns an error if not.
func EnsureProject(ctx context.Context, projectDir string) error {
	abs, err := filepath.Abs(projectDir)
	if err != nil {
		return fmt.Errorf("resolve project dir: %w", err)
	}
	pyproject := filepath.Join(abs, "pyproject.toml")
	if _, err := os.Stat(pyproject); err != nil {
		return fmt.Errorf("pyproject.toml not found in %s: %w", abs, err)
	}
	venvMarker := filepath.Join(abs, ".venv", "pyvenv.cfg")
	if upToDate(pyproject, venvMarker) {
		// pyproject is older than the venv marker → already synced.
		// Still check lockfile mtime to catch lock-only updates.
		lockfile := filepath.Join(abs, "uv.lock")
		if _, err := os.Stat(lockfile); err == nil && upToDate(lockfile, venvMarker) {
			return nil
		}
	}
	cmd := exec.CommandContext(ctx, "uv", "sync", "--project", abs) //nolint:gosec // G204: uv is a fixed binary; projectDir is an internal value
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("uv sync in %s: %w", abs, err)
	}
	return nil
}

// upToDate returns true when target exists and is at least as new as
// source. Returns false when either side is missing or when source is
// newer (which means a re-sync is needed).
func upToDate(source, target string) bool {
	src, err := os.Stat(source)
	if err != nil {
		return false
	}
	tgt, err := os.Stat(target)
	if err != nil {
		return false
	}
	return !src.ModTime().After(tgt.ModTime())
}

// FindProjectDir locates a python-tools/<tool>/ directory by walking up
// from the current working directory and the executable's directory.
// Returns absolute path on success.
//
// Search order: ./python-tools/<tool> → <exe-dir>/python-tools/<tool> →
// <exe-dir>/../python-tools/<tool>. Mirrors docker.FindDockerDir.
func FindProjectDir(tool string) string {
	candidates := []string{filepath.Join("python-tools", tool)}
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(exeDir, "python-tools", tool),
			filepath.Join(exeDir, "..", "python-tools", tool),
		)
	}
	for _, c := range candidates {
		abs, err := filepath.Abs(c)
		if err != nil {
			continue
		}
		if _, err := os.Stat(filepath.Join(abs, "pyproject.toml")); err == nil {
			return abs
		}
	}
	return ""
}

// FreePort returns an OS-assigned available TCP port on localhost. The
// listener is closed before returning, leaving a brief TOCTOU window —
// callers must handle "address in use" errors on subsequent bind and
// retry. Matches the freePort helper used by the docker package.
func FreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("allocate free port: %w", err)
	}
	defer func() {
		_ = l.Close()
	}()
	addr, ok := l.Addr().(*net.TCPAddr)
	if !ok {
		return 0, errors.New("listener address is not TCPAddr")
	}
	return addr.Port, nil
}
