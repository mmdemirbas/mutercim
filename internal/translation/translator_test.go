package translation

import (
	"context"
	"testing"

	"github.com/mmdemirbas/mutercim/internal/knowledge"
	"github.com/mmdemirbas/mutercim/internal/model"
)

type mockProvider struct {
	response string
	err      error
}

func (m *mockProvider) Name() string         { return "mock" }
func (m *mockProvider) SupportsVision() bool { return true }
func (m *mockProvider) ReadFromImage(ctx context.Context, image []byte, systemPrompt, userPrompt string) (string, error) {
	return m.response, m.err
}
func (m *mockProvider) Translate(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return m.response, m.err
}

//nolint:cyclop // integration test with detailed translated page assertions
func TestTranslatePage(t *testing.T) {
	response := `{
		"regions": [
			{"id": "r1", "translated_text": "Elif Harfi"},
			{"id": "r2", "translated_text": "1060) Git ve aileni besle."}
		],
		"warnings": []
	}`

	k := &knowledge.Knowledge{
		Entries: []knowledge.Entry{
			{
				Forms: map[string][]string{
					"ar": {"صلى الله عليه وسلم"},
					"tr": {"sallallâhu aleyhi ve sellem"},
				},
			},
		},
	}

	mock := &mockProvider{response: response}
	translator := NewTranslator(mock, k, true, []string{"ar"}, "tr", nil)

	page := &model.SolvedRegionPage{
		RegionPage: model.RegionPage{
			Version:    "2.0",
			PageNumber: 1,
			Regions: []model.Region{
				{ID: "r1", BBox: model.BBox{400, 50, 700, 60}, Text: "حرف الألف", Type: model.RegionTypeHeader},
				{ID: "r2", BBox: model.BBox{800, 150, 600, 400}, Text: "١٠٦٠) اذْهَبِي", Type: model.RegionTypeEntry},
			},
			ReadingOrder: []string{"r1", "r2"},
		},
	}

	translated, err := translator.TranslatePage(context.Background(), page, nil, "test-model")
	if err != nil {
		t.Fatalf("TranslatePage() error: %v", err)
	}

	if translated.TranslateModel != "test-model" {
		t.Errorf("TranslateModel = %q, want %q", translated.TranslateModel, "test-model")
	}
	if translated.TranslateTimestamp == "" {
		t.Error("expected non-empty timestamp")
	}
	if translated.Version != "2.0" {
		t.Errorf("Version = %q, want %q", translated.Version, "2.0")
	}
	if translated.SourceLang != "ar" {
		t.Errorf("SourceLang = %q, want %q", translated.SourceLang, "ar")
	}
	if translated.TargetLang != "tr" {
		t.Errorf("TargetLang = %q, want %q", translated.TargetLang, "tr")
	}

	if len(translated.Regions) != 2 {
		t.Fatalf("len(Regions) = %d, want 2", len(translated.Regions))
	}

	r1 := translated.Regions[0]
	if r1.ID != "r1" {
		t.Errorf("Regions[0].ID = %q", r1.ID)
	}
	if r1.OriginalText != "حرف الألف" {
		t.Errorf("Regions[0].OriginalText = %q", r1.OriginalText)
	}
	if r1.TranslatedText != "Elif Harfi" {
		t.Errorf("Regions[0].TranslatedText = %q", r1.TranslatedText)
	}
	if r1.Type != model.RegionTypeHeader {
		t.Errorf("Regions[0].Type = %q", r1.Type)
	}

	r2 := translated.Regions[1]
	if r2.TranslatedText != "1060) Git ve aileni besle." {
		t.Errorf("Regions[1].TranslatedText = %q", r2.TranslatedText)
	}
	if r2.BBox != (model.BBox{800, 150, 600, 400}) {
		t.Errorf("BBox should be preserved, got %v", r2.BBox)
	}

	if len(translated.ReadingOrder) != 2 {
		t.Errorf("len(ReadingOrder) = %d, want 2", len(translated.ReadingOrder))
	}
}

func TestTranslatePage_SeparatorsNotTranslated(t *testing.T) {
	response := `{
		"regions": [
			{"id": "r1", "translated_text": "translated entry"}
		],
		"warnings": []
	}`

	mock := &mockProvider{response: response}
	translator := NewTranslator(mock, &knowledge.Knowledge{}, true, []string{"ar"}, "tr", nil)

	page := &model.SolvedRegionPage{
		RegionPage: model.RegionPage{
			Version:    "2.0",
			PageNumber: 1,
			Regions: []model.Region{
				{ID: "r1", Text: "entry", Type: model.RegionTypeEntry},
				{ID: "sep1", Text: "---", Type: model.RegionTypeSeparator},
				{ID: "pn1", Text: "42", Type: model.RegionTypePageNumber},
			},
			ReadingOrder: []string{"r1", "sep1", "pn1"},
		},
	}

	translated, err := translator.TranslatePage(context.Background(), page, nil, "test-model")
	if err != nil {
		t.Fatalf("TranslatePage() error: %v", err)
	}

	// Separator and page_number should keep original text
	if translated.Regions[1].TranslatedText != "---" {
		t.Errorf("separator TranslatedText = %q, want %q", translated.Regions[1].TranslatedText, "---")
	}
	if translated.Regions[2].TranslatedText != "42" {
		t.Errorf("page_number TranslatedText = %q, want %q", translated.Regions[2].TranslatedText, "42")
	}
}

func TestTranslatePageWithContext(t *testing.T) {
	response := `{"regions": [], "warnings": []}`

	mock := &mockProvider{response: response}
	translator := NewTranslator(mock, &knowledge.Knowledge{}, true, []string{"ar"}, "tr", nil)

	page := &model.SolvedRegionPage{
		RegionPage: model.RegionPage{
			Version:    "2.0",
			PageNumber: 5,
		},
	}

	summaries := []string{"Page 3 — Section title", "Page 4 — Entries"}
	_, err := translator.TranslatePage(context.Background(), page, summaries, "test-model")
	if err != nil {
		t.Fatalf("TranslatePage() error: %v", err)
	}
}

func TestTranslatePage_FormatsGlossaryContext(t *testing.T) {
	response := `{"regions": [{"id": "r1", "translated_text": "text"}], "warnings": []}`

	k := &knowledge.Knowledge{
		Entries: []knowledge.Entry{
			{
				Forms: map[string][]string{
					"ar": {"حديث"},
					"tr": {"hadîs-i şerîf"},
				},
				Note: "Prophetic tradition",
			},
		},
	}

	mock := &mockProvider{response: response}
	translator := NewTranslator(mock, k, true, []string{"ar"}, "tr", nil)

	page := &model.SolvedRegionPage{
		RegionPage: model.RegionPage{
			Version:    "2.0",
			PageNumber: 1,
			Regions: []model.Region{
				{ID: "r1", Text: "هذا حديث", Type: model.RegionTypeEntry},
			},
			ReadingOrder: []string{"r1"},
		},
		GlossaryContext: []string{"حديث"}, // source canonical form from solver
	}

	// The translator should format this using knowledge for the target language
	glossary := translator.formatGlossaryContext(page.GlossaryContext, "ar")
	if len(glossary) != 1 {
		t.Fatalf("expected 1 glossary line, got %d", len(glossary))
	}
	if glossary[0] != "حديث → hadîs-i şerîf — Prophetic tradition" {
		t.Errorf("glossary line = %q", glossary[0])
	}
}

func TestPageSummary(t *testing.T) {
	page := &model.TranslatedRegionPage{
		PageNumber: 42,
		Regions: []model.TranslatedRegion{
			{ID: "r1", Type: model.RegionTypeHeader, TranslatedText: "Elif Harfi"},
			{ID: "r2", Type: model.RegionTypeEntry, TranslatedText: "entry text"},
		},
	}

	s := PageSummary(page)
	if s != "Elif Harfi" {
		t.Errorf("PageSummary = %q, want %q", s, "Elif Harfi")
	}
}

func TestPageSummaryNil(t *testing.T) {
	if s := PageSummary(nil); s != "" {
		t.Errorf("expected empty for nil, got %q", s)
	}
}

func TestPageSummary_NoHeader(t *testing.T) {
	page := &model.TranslatedRegionPage{
		PageNumber: 1,
		Regions: []model.TranslatedRegion{
			{ID: "r1", Type: model.RegionTypeEntry, TranslatedText: "text"},
		},
	}
	if s := PageSummary(page); s != "" {
		t.Errorf("expected empty for page without header, got %q", s)
	}
}
