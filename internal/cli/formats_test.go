package cli

import (
	"strings"
	"testing"
)

func TestNormalizeFormats_valid(t *testing.T) {
	tests := []struct {
		input []string
		want  string // joined with ","
	}{
		{[]string{"md"}, "md"},
		{[]string{"pdf"}, "pdf"},
		{[]string{"latex"}, "latex"},
		{[]string{"tex"}, "latex"},
		{[]string{"docx"}, "docx"},
		{[]string{"md", "pdf"}, "md,pdf"},
		{[]string{"tex", "docx"}, "latex,docx"},
		{[]string{"md", "latex", "pdf", "docx"}, "md,latex,pdf,docx"},
	}
	for _, tt := range tests {
		got, err := normalizeFormats(tt.input)
		if err != nil {
			t.Errorf("normalizeFormats(%v) error: %v", tt.input, err)
			continue
		}
		if joined := strings.Join(got, ","); joined != tt.want {
			t.Errorf("normalizeFormats(%v) = %v, want %v", tt.input, joined, tt.want)
		}
	}
}

func TestNormalizeFormats_dedup(t *testing.T) {
	got, err := normalizeFormats([]string{"tex", "latex"})
	if err != nil {
		t.Fatalf("normalizeFormats error: %v", err)
	}
	if len(got) != 1 || got[0] != "latex" {
		t.Errorf("normalizeFormats([tex, latex]) = %v, want [latex]", got)
	}
}

func TestNormalizeFormats_invalid(t *testing.T) {
	_, err := normalizeFormats([]string{"html"})
	if err == nil {
		t.Fatal("normalizeFormats([html]) should error")
	}
	if !strings.Contains(err.Error(), "unknown output format") {
		t.Errorf("error = %q, want to contain 'unknown output format'", err)
	}
}

func TestNormalizeFormats_empty(t *testing.T) {
	got, err := normalizeFormats([]string{})
	if err != nil {
		t.Fatalf("normalizeFormats([]) error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("normalizeFormats([]) = %v, want empty", got)
	}
}
