package reader

import (
	"strings"
	"testing"
)

func TestBuildUserPrompt(t *testing.T) {
	tests := []struct {
		sectionType string
		contains    string
	}{
		{"scholarly_entries", "numbered scholarly entries"},
		{"prose", "prose section"},
		{"reference_table", "reference table"},
		{"toc", "table of contents"},
		{"index", "alphabetical index"},
		{"auto", "Read all text"},
		{"", "Read all text"},
	}

	for _, tt := range tests {
		t.Run(tt.sectionType, func(t *testing.T) {
			prompt := BuildUserPrompt(tt.sectionType)
			if !strings.Contains(prompt, tt.contains) {
				t.Errorf("BuildUserPrompt(%q) = %q, expected to contain %q", tt.sectionType, prompt, tt.contains)
			}
		})
	}
}

func TestSectionHint(t *testing.T) {
	// auto and unknown types should return empty hint
	if hint := SectionHint("auto"); hint != "" {
		t.Errorf("SectionHint('auto') = %q, expected empty", hint)
	}
	if hint := SectionHint("unknown"); hint != "" {
		t.Errorf("SectionHint('unknown') = %q, expected empty", hint)
	}

	// known types should return non-empty hint
	for _, st := range []string{"scholarly_entries", "prose", "reference_table", "toc", "index"} {
		if hint := SectionHint(st); hint == "" {
			t.Errorf("SectionHint(%q) returned empty, expected non-empty", st)
		}
	}
}
