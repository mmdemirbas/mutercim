package translation

import (
	"strings"
	"testing"

	"github.com/mmdemirbas/mutercim/internal/model"
)

func TestBuildSystemPrompt(t *testing.T) {
	prompt := BuildSystemPrompt(
		"- صلى الله عليه وسلم → sallallâhu aleyhi ve sellem",
		"- أبو هريرة → Ebû Hüreyre",
		"- خ = Sahîh-i Buhârî",
		"- حديث → hadîs-i şerîf",
		"Page 1 — intro",
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
		"ar",
		"tr",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt should contain %q", want)
		}
	}
}

func TestBuildSystemPromptNoExpand(t *testing.T) {
	prompt := BuildSystemPrompt("", "", "", "", "", false, []string{"ar"}, "tr")
	if !strings.Contains(prompt, "Keep source abbreviation codes as-is") {
		t.Error("expected no-expand instruction")
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

func TestBuildRegionUserPrompt(t *testing.T) {
	page := &model.SolvedRegionPage{
		RegionPage: model.RegionPage{
			Version:    "2.0",
			PageNumber: 1,
			Regions: []model.Region{
				{ID: "r1", Text: "header text", Type: model.RegionTypeHeader},
				{ID: "r2", Text: "entry text", Type: model.RegionTypeEntry},
				{ID: "sep1", Text: "", Type: model.RegionTypeSeparator},
				{ID: "pn1", Text: "42", Type: model.RegionTypePageNumber},
			},
			ReadingOrder: []string{"r1", "r2", "sep1", "pn1"},
		},
		GlossaryContext:     []string{"حديث → hadîs-i şerîf"},
		PreviousPageSummary: "Previous page summary",
	}

	prompt := BuildRegionUserPrompt(page, []string{"ar"}, "tr")

	for _, want := range []string{
		"ar",
		"tr",
		"Region r1 (header): header text",
		"Region r2 (entry): entry text",
		"Region sep1 (separator): [separator line - do not translate]",
		"Region pn1 (page_number): 42 [do not translate]",
		"حديث → hadîs-i şerîf",
		"Previous page summary",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt should contain %q, got:\n%s", want, prompt)
		}
	}
}

func TestBuildRegionUserPrompt_NoGlossary(t *testing.T) {
	page := &model.SolvedRegionPage{
		RegionPage: model.RegionPage{
			Version:    "2.0",
			PageNumber: 1,
			Regions: []model.Region{
				{ID: "r1", Text: "text", Type: model.RegionTypeEntry},
			},
			ReadingOrder: []string{"r1"},
		},
	}

	prompt := BuildRegionUserPrompt(page, []string{"ar"}, "tr")

	if strings.Contains(prompt, "GLOSSARY") {
		t.Error("prompt should not contain GLOSSARY when no terms")
	}
}
