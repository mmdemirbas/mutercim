package cli

import (
	"log/slog"
	"os"
	"testing"
)

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input string
		want  slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"error", slog.LevelError},
		{"", slog.LevelInfo},
		{"unknown", slog.LevelInfo},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseLogLevel(tt.input)
			if got != tt.want {
				t.Errorf("parseLogLevel(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestSanitizeLogValue(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"clean", "read --pages 1-10", "read --pages 1-10"},
		{"newline injection", "read\nfake log line", "read fake log line"},
		{"carriage return", "read\rfake", "read fake"},
		{"control chars", "read\x00\x01\x1b[31m", "read   [31m"},
		{"tabs preserved", "read\t--pages", "read\t--pages"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeLogValue(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeLogValue(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNewRootCmd(t *testing.T) {
	cmd := NewRootCmd()
	if cmd.Use != "mutercim" {
		t.Errorf("Use = %q, want mutercim", cmd.Use)
	}

	// Verify all expected subcommands are registered
	names := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		names[sub.Name()] = true
	}
	for _, want := range []string{"all", "cut", "read", "solve", "translate", "write", "init", "status", "config", "clean"} {
		if !names[want] {
			t.Errorf("missing subcommand %q", want)
		}
	}
}

func TestExecute_InvalidFlag(t *testing.T) {
	// Override os.Args to simulate an invalid flag
	origArgs := os.Args
	defer func() { os.Args = origArgs }()
	os.Args = []string{"mutercim", "--invalid-flag-xyz"}

	err := Execute()
	if err == nil {
		t.Error("Execute() should return error for invalid flag")
	}
}
