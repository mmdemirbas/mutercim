package docker

import (
	"context"
	"os/exec"
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
