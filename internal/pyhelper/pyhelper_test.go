package pyhelper

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// uvAvailable reports whether the test environment has uv installed.
// Returns the resolved uv path on success. Tests that need uv use
// t.Skip when it's absent.
func uvAvailable() (string, bool) {
	p, err := exec.LookPath("uv")
	if err != nil {
		return "", false
	}
	return p, true
}

func TestEnsureUV_PresentSucceeds(t *testing.T) {
	if _, ok := uvAvailable(); !ok {
		t.Skip("uv not installed on test host")
	}
	if err := EnsureUV(context.Background()); err != nil {
		t.Errorf("EnsureUV with uv on PATH should succeed, got: %v", err)
	}
}

func TestEnsureUV_AbsentFailsHelpfully(t *testing.T) {
	// Temporarily hide uv by setting PATH to an empty dir.
	origPath := os.Getenv("PATH")
	defer func() { _ = os.Setenv("PATH", origPath) }()
	if err := os.Setenv("PATH", "/nonexistent-dir-for-pyhelper-test"); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	err := EnsureUV(context.Background())
	if err == nil {
		t.Fatal("expected error when uv is not on PATH")
	}
	if !strings.Contains(err.Error(), "uv is required") {
		t.Errorf("error should mention uv, got: %v", err)
	}
	if !strings.Contains(err.Error(), "brew install uv") {
		t.Errorf("error should hint at install command, got: %v", err)
	}
}

func TestFreePort_AssignsValidPort(t *testing.T) {
	for i := range 5 {
		port, err := FreePort()
		if err != nil {
			t.Fatalf("FreePort attempt %d: %v", i, err)
		}
		if port < 1024 || port > 65535 {
			t.Errorf("port %d outside expected ephemeral range", port)
		}
	}
}

func TestPortFromEnv(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		setEnv   bool
		fallback int
		want     int
	}{
		{"unset", "", false, 8000, 8000},
		{"empty", "", true, 8000, 8000},
		{"valid", "9001", true, 8000, 9001},
		{"invalid", "not-a-number", true, 8000, 8000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setEnv {
				if err := os.Setenv("PORT", tt.envValue); err != nil {
					t.Fatalf("setenv: %v", err)
				}
			} else {
				_ = os.Unsetenv("PORT")
			}
			defer func() { _ = os.Unsetenv("PORT") }()
			got := PortFromEnv(tt.fallback)
			if got != tt.want {
				t.Errorf("PortFromEnv(%d) with PORT=%q = %d, want %d", tt.fallback, tt.envValue, got, tt.want)
			}
		})
	}
}

func TestFindProjectDir(t *testing.T) {
	// Create a temp tree with a python-tools/myhelper/pyproject.toml file
	// and chdir into it. FindProjectDir should locate the directory.
	tmp := t.TempDir()
	projectDir := filepath.Join(tmp, "python-tools", "myhelper")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "pyproject.toml"), []byte("[project]\nname='x'\nversion='0'\n"), 0o644); err != nil {
		t.Fatalf("write pyproject: %v", err)
	}
	origWD, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWD) }()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	got := FindProjectDir("myhelper")
	if got == "" {
		t.Fatal("FindProjectDir should locate the project directory")
	}
	wantAbs, _ := filepath.Abs(projectDir)
	// Use filepath.EvalSymlinks for macOS /private/var vs /var quirks.
	gotResolved, _ := filepath.EvalSymlinks(got)
	wantResolved, _ := filepath.EvalSymlinks(wantAbs)
	if gotResolved != wantResolved {
		t.Errorf("FindProjectDir = %q, want %q", gotResolved, wantResolved)
	}
}

func TestFindProjectDir_AbsentReturnsEmpty(t *testing.T) {
	tmp := t.TempDir()
	origWD, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWD) }()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	if got := FindProjectDir("absent-tool"); got != "" {
		t.Errorf("FindProjectDir for missing tool should return empty, got %q", got)
	}
}

func TestServer_StartStop_StdlibOnly(t *testing.T) {
	if _, ok := uvAvailable(); !ok {
		t.Skip("uv not installed on test host")
	}
	// Locate the bundled mini test server.
	_, thisFile, _, _ := runtimeCaller()
	mini := filepath.Join(filepath.Dir(thisFile), "testdata", "mini")
	if _, err := os.Stat(filepath.Join(mini, "pyproject.toml")); err != nil {
		t.Fatalf("test fixture missing: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	srv := &Server{
		ProjectDir:         mini,
		Script:             "server.py",
		StartTimeout:       30 * time.Second,
		HealthPollInterval: 100 * time.Millisecond,
		Logger:             slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError})),
	}
	if err := srv.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = srv.Stop(context.Background()) }()

	if !srv.IsReady(ctx) {
		t.Fatal("server should be ready after Start returns")
	}

	// Verify the /echo endpoint also works — proves it's our server.
	resp, err := srv.HTTPClient.Get(fmt.Sprintf("http://127.0.0.1:%d/echo?msg=phase2", srv.Port()))
	if err != nil {
		t.Fatalf("GET /echo: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/echo returned status %d", resp.StatusCode)
	}
	var body struct {
		Echo string `json:"echo"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Echo != "phase2" {
		t.Errorf("echo body = %q, want phase2", body.Echo)
	}

	if err := srv.Stop(context.Background()); err != nil {
		t.Errorf("Stop: %v", err)
	}
	// Stop is idempotent.
	if err := srv.Stop(context.Background()); err != nil {
		t.Errorf("second Stop: %v", err)
	}
	if srv.IsReady(ctx) {
		t.Error("IsReady should return false after Stop")
	}
}

// runtimeCaller wraps runtime.Caller to avoid importing it at file top.
// Returning the test file's path lets us locate testdata/ deterministically
// regardless of where `go test` was invoked from.
func runtimeCaller() (uintptr, string, int, bool) {
	return runtimeCallerImpl()
}
