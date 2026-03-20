package reader

import (
	"strings"
	"testing"
)

func TestBuildUserPrompt(t *testing.T) {
	prompt := BuildUserPrompt()
	if !strings.Contains(prompt, "Analyze this page image") {
		t.Errorf("BuildUserPrompt() = %q, expected to contain 'Analyze this page image'", prompt)
	}
	if !strings.Contains(prompt, "semantic roles") {
		t.Errorf("BuildUserPrompt() = %q, expected to contain 'semantic roles'", prompt)
	}
}
