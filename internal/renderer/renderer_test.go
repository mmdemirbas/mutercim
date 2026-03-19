package renderer

import (
	"strings"
	"testing"
)

// TestRendererInterfaceSatisfied verifies that all renderers satisfy the Renderer interface.
func TestRendererInterfaceSatisfied(t *testing.T) {
	var _ Renderer = &MarkdownRenderer{}
	var _ Renderer = &ArabicMarkdownRenderer{}
	var _ Renderer = &LaTeXRenderer{}
}

func TestMarkdownExtension(t *testing.T) {
	r := &MarkdownRenderer{}
	if r.Extension() != ".md" {
		t.Errorf("expected '.md', got %q", r.Extension())
	}
}

func TestArabicMarkdownExtension(t *testing.T) {
	r := &ArabicMarkdownRenderer{}
	if r.Extension() != ".md" {
		t.Errorf("expected '.md', got %q", r.Extension())
	}
}

func TestLatexExtension(t *testing.T) {
	r := &LaTeXRenderer{}
	if r.Extension() != ".tex" {
		t.Errorf("expected '.tex', got %q", r.Extension())
	}
}

func TestLatexEscapeSpecialChars(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"normal text", "normal text"},
		{"100%", `100\%`},
		{"$5", `\$5`},
		{"a & b", `a \& b`},
		{"a_b", `a\_b`},
		{"#1", `\#1`},
		{"{x}", `\{x\}`},
		{"", ""},
	}
	for _, tt := range tests {
		got := latexEscape(tt.input)
		if got != tt.want {
			t.Errorf("latexEscape(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestCheckPandoc_ErrorMessage(t *testing.T) {
	// CheckPandoc may or may not find pandoc depending on the machine.
	// We just verify that if it returns an error, the message is informative.
	err := CheckPandoc()
	if err != nil {
		if !strings.Contains(err.Error(), "pandoc not found") {
			t.Errorf("CheckPandoc() error = %q, expected to contain 'pandoc not found'", err.Error())
		}
	}
}

func TestCheckDocker_ErrorMessage(t *testing.T) {
	// CheckDocker may or may not find docker depending on the machine.
	// We just verify that if it returns an error, the message is informative.
	err := CheckDocker()
	if err != nil {
		if !strings.Contains(err.Error(), "docker not found") {
			t.Errorf("CheckDocker() error = %q, expected to contain 'docker not found'", err.Error())
		}
	}
}
