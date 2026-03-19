package renderer

import "testing"

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
