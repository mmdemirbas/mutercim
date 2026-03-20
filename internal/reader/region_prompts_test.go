package reader

import (
	"strings"
	"testing"

	"github.com/mmdemirbas/mutercim/internal/model"
)

func TestBuildRegionUserPrompt(t *testing.T) {
	prompt := BuildRegionUserPrompt()
	if prompt == "" {
		t.Error("empty prompt")
	}
	if !strings.Contains(prompt, "region") {
		t.Error("prompt should mention regions")
	}
}

func TestBuildRegionUserPromptWithLayout_EmptyRegions(t *testing.T) {
	prompt := BuildRegionUserPromptWithLayout(nil)
	// Should fall back to the basic prompt
	if prompt != BuildRegionUserPrompt() {
		t.Errorf("empty regions should return basic prompt, got %q", prompt)
	}
}

func TestBuildRegionUserPromptWithLayout_WithRegions(t *testing.T) {
	regions := []model.Region{
		{
			ID:   "r1",
			BBox: model.BBox{400, 50, 700, 60},
			Text: "باب الألف",
		},
		{
			ID:   "r2",
			BBox: model.BBox{800, 150, 600, 400},
			Text: "",
		},
	}

	prompt := BuildRegionUserPromptWithLayout(regions)

	if !strings.Contains(prompt, "r1: bbox=[400, 50, 700, 60]") {
		t.Error("prompt should contain r1 bbox")
	}
	if !strings.Contains(prompt, `preliminary_text="باب الألف"`) {
		t.Error("prompt should contain r1 preliminary text")
	}
	if !strings.Contains(prompt, "r2: bbox=[800, 150, 600, 400]") {
		t.Error("prompt should contain r2 bbox")
	}
	// r2 has no text, should not have preliminary_text
	if strings.Contains(prompt, "r2: bbox=[800, 150, 600, 400], preliminary_text") {
		t.Error("prompt should not contain preliminary_text for empty text region")
	}
	if !strings.Contains(prompt, "layout analysis tool") {
		t.Error("prompt should mention layout analysis tool")
	}
}

func TestRegionSystemPrompts_ContainKeyElements(t *testing.T) {
	tests := []struct {
		name     string
		prompt   string
		mustHave []string
	}{
		{
			"ai-only system prompt",
			regionSystemPromptAIOnly,
			[]string{
				"diacritical marks",
				"bbox",
				"reading_order",
				"multi-column",
				"separator",
				"RTL",
				"regions",
				"ONLY JSON",
			},
		},
		{
			"with-layout system prompt",
			regionSystemPromptWithLayout,
			[]string{
				"diacritical marks",
				"bbox",
				"reading_order",
				"layout detection tool",
				"ACCURATE text",
				"split or merged",
				"missed any text regions",
				"ONLY JSON",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, s := range tt.mustHave {
				if !strings.Contains(tt.prompt, s) {
					t.Errorf("prompt missing %q", s)
				}
			}
		})
	}
}
