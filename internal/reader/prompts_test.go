package reader

import (
	"strings"
	"testing"
)

func TestBuildUserPrompt(t *testing.T) {
	prompt := BuildUserPrompt()
	if !strings.Contains(prompt, "Read this page image") {
		t.Errorf("BuildUserPrompt() = %q, expected to contain 'Read this page image'", prompt)
	}
	if !strings.Contains(prompt, "layout structure") {
		t.Errorf("BuildUserPrompt() = %q, expected to contain 'layout structure'", prompt)
	}
}

func TestSystemPromptContainsKeyRules(t *testing.T) {
	for _, want := range []string{
		"MULTIPLE COLUMNS",
		"SEPARATOR LINES",
		"FOOTNOTE RULES",
		"COMMON ERRORS TO AVOID",
		"tashkeel",
	} {
		if !strings.Contains(systemPrompt, want) {
			t.Errorf("systemPrompt should contain %q", want)
		}
	}
}
