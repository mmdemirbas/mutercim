package translation

import (
	"strings"
	"testing"
)

func TestBuildSystemPrompt(t *testing.T) {
	prompt := BuildSystemPrompt(
		"- صلى الله عليه وسلم → sallallâhu aleyhi ve sellem",
		"- أبو هريرة → Ebû Hüreyre",
		"- خ = Sahîh-i Buhârî",
		"- حديث → hadîs-i şerîf",
		"Page 1 — intro",
		"scholarly_entries",
		true,
		[]string{"ar"},
		"tr",
	)

	for _, want := range []string{
		"sallallâhu aleyhi ve sellem",
		"Ebû Hüreyre",
		"Sahîh-i Buhârî",
		"hadîs-i şerîf",
		"Page 1",
		"expand all source abbreviation codes",
		"scholarly entries",
		"ar",
		"tr",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt should contain %q", want)
		}
	}
}

func TestBuildSystemPromptNoExpand(t *testing.T) {
	prompt := BuildSystemPrompt("", "", "", "", "", "auto", false, []string{"ar"}, "tr")
	if !strings.Contains(prompt, "Keep source abbreviation codes as-is") {
		t.Error("expected no-expand instruction")
	}
}

func TestSectionHint(t *testing.T) {
	tests := []struct {
		sectionType string
		contains    string
	}{
		{"scholarly_entries", "scholarly entries"},
		{"prose", "continuous prose"},
		{"toc", "table of contents"},
		{"index", "alphabetical index"},
		{"auto", ""},
	}

	for _, tt := range tests {
		hint := SectionHint(tt.sectionType)
		if tt.contains == "" {
			if hint != "" {
				t.Errorf("SectionHint(%q) = %q, expected empty", tt.sectionType, hint)
			}
		} else if !strings.Contains(hint, tt.contains) {
			t.Errorf("SectionHint(%q) = %q, expected to contain %q", tt.sectionType, hint, tt.contains)
		}
	}
}

func TestBuildLanguageInstruction(t *testing.T) {
	tests := []struct {
		name     string
		sources  []string
		target   string
		contains []string
	}{
		{"single source", []string{"ar"}, "tr", []string{"ar", "tr"}},
		{"multi source", []string{"ar", "fa"}, "tr", []string{"primarily ar", "fa fragments", "tr"}},
		{"empty source", nil, "en", []string{"en"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildLanguageInstruction(tt.sources, tt.target)
			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("buildLanguageInstruction(%v, %q) = %q, should contain %q", tt.sources, tt.target, result, want)
				}
			}
		})
	}
}

func TestBuildContextSection(t *testing.T) {
	empty := BuildContextSection(nil)
	if !strings.Contains(empty, "No previous context") {
		t.Errorf("expected no-context message, got %q", empty)
	}

	ctx := BuildContextSection([]string{"Page 1 — intro", "Page 2 — entries"})
	if !strings.Contains(ctx, "Page 1") || !strings.Contains(ctx, "Page 2") {
		t.Errorf("expected both pages in context, got %q", ctx)
	}
}
