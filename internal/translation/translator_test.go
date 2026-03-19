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

func TestTranslatePage(t *testing.T) {
	response := `{
		"translated_header": {"text": "Elif Harfi"},
		"translated_entries": [
			{
				"number": 1,
				"turkish_text": "Bu bir hadîs-i şerîftir.",
				"translator_notes": ""
			}
		],
		"translated_footnotes": [
			{
				"entry_numbers": [1],
				"turkish_text": "Sahîh-i Buhârî'de rivayet edilmiştir.",
				"sources_expanded": ["Sahîh-i Buhârî"]
			}
		],
		"warnings": []
	}`

	k := &knowledge.Knowledge{
		Honorifics: []knowledge.Honorific{
			{Arabic: "صلى الله عليه وسلم", Turkish: "sallallâhu aleyhi ve sellem"},
		},
	}

	mock := &mockProvider{response: response}
	translator := NewTranslator(mock, k, true, nil)

	page := &model.SolvedPage{
		ReadPage: model.ReadPage{
			PageNumber:  1,
			SectionType: "scholarly_entries",
			Header:      &model.Header{Text: "حرف الألف", Type: "section_title"},
			Entries: []model.Entry{
				{Number: intPtr(1), Type: "hadith", ArabicText: "text"},
			},
		},
	}

	translated, err := translator.TranslatePage(context.Background(), page, nil, "test-model")
	if err != nil {
		t.Fatalf("TranslatePage() error: %v", err)
	}

	if translated.TranslationModel != "test-model" {
		t.Errorf("expected model 'test-model', got %q", translated.TranslationModel)
	}
	if translated.TranslationTimestamp == "" {
		t.Error("expected non-empty timestamp")
	}
	if translated.TranslatedHeader == nil || translated.TranslatedHeader.Text != "Elif Harfi" {
		t.Errorf("unexpected header: %+v", translated.TranslatedHeader)
	}
	if len(translated.TranslatedEntries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(translated.TranslatedEntries))
	}
	if translated.TranslatedEntries[0].TurkishText != "Bu bir hadîs-i şerîftir." {
		t.Errorf("unexpected turkish text: %q", translated.TranslatedEntries[0].TurkishText)
	}
	if len(translated.TranslatedFootnotes) != 1 {
		t.Fatalf("expected 1 footnote, got %d", len(translated.TranslatedFootnotes))
	}
}

func TestTranslatePageWithContext(t *testing.T) {
	response := `{"translated_entries": [], "warnings": []}`

	mock := &mockProvider{response: response}
	translator := NewTranslator(mock, &knowledge.Knowledge{}, true, nil)

	page := &model.SolvedPage{
		ReadPage: model.ReadPage{
			PageNumber:  5,
			SectionType: "scholarly_entries",
		},
	}

	summaries := []string{"Page 3 — Section title. Entries 100-105", "Page 4 — Entries 106-110"}
	_, err := translator.TranslatePage(context.Background(), page, summaries, "test-model")
	if err != nil {
		t.Fatalf("TranslatePage() error: %v", err)
	}
}

func TestPageSummary(t *testing.T) {
	page := &model.TranslatedPage{
		SolvedPage: model.SolvedPage{
			ReadPage: model.ReadPage{PageNumber: 42},
		},
		TranslatedHeader: &model.TranslatedHeader{Text: "Bab"},
		TranslatedEntries: []model.TranslatedEntry{
			{Number: 100, TurkishText: "text1"},
			{Number: 105, TurkishText: "text5"},
		},
	}

	s := PageSummary(page)
	if s == "" {
		t.Fatal("expected non-empty summary")
	}
	// Should contain page number, header, and entry range
	for _, want := range []string{"42", "Bab", "100", "105"} {
		if !containsStr(s, want) {
			t.Errorf("summary %q should contain %q", s, want)
		}
	}
}

func TestPageSummaryNil(t *testing.T) {
	if s := PageSummary(nil); s != "" {
		t.Errorf("expected empty for nil, got %q", s)
	}
}

func intPtr(n int) *int { return &n }

func containsStr(s, sub string) bool {
	return len(sub) > 0 && len(s) >= len(sub) && searchStr(s, sub)
}

func searchStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
