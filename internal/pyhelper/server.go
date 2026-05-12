package pyhelper

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"
)

// Server manages the lifecycle of a uv-launched Python HTTP server.
//
// Each tool that wants to use the uv backend constructs a Server with a
// project directory (containing pyproject.toml and the entrypoint script),
// a script name (e.g. "server.py"), and any extra environment. Start
// allocates a free port, spawns `uv run <script>`, and polls the configured
// /health endpoint until the server reports ready. Stop signals the
// process and waits for exit.
type Server struct {
	// ProjectDir is the absolute path to the python-tools/<tool>/ directory.
	ProjectDir string
	// Script is the python file to run (e.g. "server.py"). Resolved against
	// ProjectDir.
	Script string
	// HealthPath is the HTTP path to poll for readiness. Default "/health".
	HealthPath string
	// HealthReady is the value that http response's "status" field must
	// equal for the server to be considered ready. Default "ready".
	HealthReady string
	// StartTimeout is the maximum time to wait for /health to report ready.
	// Default 120s — long enough for model load on CPU.
	StartTimeout time.Duration
	// HealthPollInterval is the interval between health checks. Default 500ms.
	HealthPollInterval time.Duration
	// Env are extra KEY=VALUE pairs added to the subprocess environment.
	// PORT is injected automatically by Start.
	Env []string
	// HTTPClient is used for health checks. Tools should reuse this for
	// their own request paths. Default: 120s timeout.
	HTTPClient *http.Client
	// Logger is used for lifecycle events. Default: slog.Default.
	Logger *slog.Logger

	mu      sync.Mutex
	port    int
	cmd     *exec.Cmd
	stopped bool
}

// Port returns the assigned port. Zero before Start, valid after.
func (s *Server) Port() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.port
}

// Start spawns the python server, waits for /health to report ready, and
// returns. Caller must invoke Stop to clean up.
func (s *Server) Start(ctx context.Context) error {
	s.applyDefaults()

	if err := EnsureUV(ctx); err != nil {
		return err
	}
	if err := EnsureProject(ctx, s.ProjectDir); err != nil {
		return err
	}

	port, err := FreePort()
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.port = port
	s.stopped = false
	s.mu.Unlock()

	scriptPath := filepath.Join(s.ProjectDir, s.Script)
	if _, statErr := os.Stat(scriptPath); statErr != nil {
		return fmt.Errorf("script not found: %s: %w", scriptPath, statErr)
	}

	//nolint:gosec // G204: uv is a fixed binary; projectDir and script are internal
	cmd := exec.Command("uv", "run", "--project", s.ProjectDir, "python", scriptPath)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("PORT=%d", port),
	)
	cmd.Env = append(cmd.Env, s.Env...)
	// New process group so we can signal the whole tree on Stop.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if startErr := cmd.Start(); startErr != nil {
		return fmt.Errorf("start python server: %w", startErr)
	}
	s.mu.Lock()
	s.cmd = cmd
	s.mu.Unlock()

	s.Logger.Info("python server starting", "project", s.ProjectDir, "script", s.Script, "port", port, "pid", cmd.Process.Pid)

	if waitErr := s.waitReady(ctx); waitErr != nil {
		// Best-effort cleanup on startup failure.
		_ = s.Stop(context.Background())
		return waitErr
	}
	return nil
}

// Stop signals the process and waits for exit. Tolerant of already-stopped
// servers and of nil receivers.
func (s *Server) Stop(ctx context.Context) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	if s.stopped || s.cmd == nil || s.cmd.Process == nil {
		s.stopped = true
		s.mu.Unlock()
		return nil
	}
	cmd := s.cmd
	s.stopped = true
	s.mu.Unlock()

	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err == nil {
		// Send SIGTERM to the whole process group so the python server and
		// any child processes exit. Fall back to single-process kill if
		// pgid lookup fails (e.g. the process already exited).
		_ = syscall.Kill(-pgid, syscall.SIGTERM)
	} else {
		_ = cmd.Process.Signal(syscall.SIGTERM)
	}

	// Wait with a deadline so a stuck process doesn't block forever.
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()
	select {
	case waitErr := <-done:
		if waitErr != nil && !isInterruptedExit(waitErr) {
			s.Logger.Warn("python server exit error", "error", waitErr)
		}
	case <-time.After(5 * time.Second):
		// Force-kill if SIGTERM didn't take.
		if pgid != 0 {
			_ = syscall.Kill(-pgid, syscall.SIGKILL)
		} else {
			_ = cmd.Process.Kill()
		}
		<-done
	case <-ctx.Done():
		return ctx.Err()
	}
	s.Logger.Info("python server stopped", "project", s.ProjectDir, "port", s.port)
	return nil
}

// IsReady checks the configured health endpoint once. Returns true when
// the response includes the expected ready value. Safe to call after Stop.
func (s *Server) IsReady(ctx context.Context) bool {
	s.applyDefaults()
	port := s.Port()
	if port == 0 {
		return false
	}
	url := fmt.Sprintf("http://127.0.0.1:%d%s", port, s.HealthPath)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	var body struct {
		Status string `json:"status"`
	}
	if decodeErr := json.NewDecoder(resp.Body).Decode(&body); decodeErr != nil {
		return false
	}
	return body.Status == s.HealthReady
}

func (s *Server) waitReady(ctx context.Context) error {
	ticker := time.NewTicker(s.HealthPollInterval)
	defer ticker.Stop()
	deadline := time.Now().Add(s.StartTimeout)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if s.IsReady(ctx) {
				return nil
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("python server startup timed out after %v on port %d", s.StartTimeout, s.Port())
			}
		}
	}
}

func (s *Server) applyDefaults() {
	if s.HealthPath == "" {
		s.HealthPath = "/health"
	}
	if s.HealthReady == "" {
		s.HealthReady = "ready"
	}
	if s.StartTimeout == 0 {
		s.StartTimeout = 120 * time.Second
	}
	if s.HealthPollInterval == 0 {
		s.HealthPollInterval = 500 * time.Millisecond
	}
	if s.HTTPClient == nil {
		s.HTTPClient = &http.Client{Timeout: 120 * time.Second}
	}
	if s.Logger == nil {
		s.Logger = slog.Default()
	}
}

// PortFromEnv reads the PORT environment variable as an int. Returns the
// fallback when PORT is unset or unparseable. Provided as a convenience for
// test helpers and Python-server-side adapters that want to mirror the
// PORT-injection contract.
func PortFromEnv(fallback int) int {
	s := os.Getenv("PORT")
	if s == "" {
		return fallback
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return fallback
	}
	return n
}

// isInterruptedExit returns true if the wait error reflects a SIGTERM/SIGKILL
// signal we sent ourselves during Stop. Such "errors" aren't real failures
// for the lifecycle owner.
func isInterruptedExit(err error) bool {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return false
	}
	status, ok := exitErr.Sys().(syscall.WaitStatus)
	if !ok {
		return false
	}
	if status.Signaled() {
		sig := status.Signal()
		return sig == syscall.SIGTERM || sig == syscall.SIGKILL || sig == syscall.SIGINT
	}
	return false
}

// Discard is an io.Writer that discards everything written to it. Provided
// for tests that want to silence subprocess output.
var Discard io.Writer = io.Discard
