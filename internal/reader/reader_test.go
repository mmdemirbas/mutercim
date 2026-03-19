package reader

import (
	"context"
	"fmt"
	"testing"
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

func TestReadPage(t *testing.T) {
	response := `{
		"page_number": 42,
		"header": {"text": "باب الألف", "type": "section_title"},
		"entries": [
			{
				"number": 100,
				"type": "hadith",
				"arabic_text": "أَبْشِرُوا",
				"is_continuation": false,
				"continues_on_next_page": false
			}
		],
		"footnotes": [
			{
				"entry_numbers": [100],
				"arabic_text": "(طب) رواه الطبراني",
				"source_codes": ["طب"]
			}
		],
		"page_footer": "- 42 -",
		"warnings": []
	}`

	mock := &mockProvider{response: response}
	r := NewReader(mock, nil)

	page, err := r.ReadPage(context.Background(), []byte("fake-image"), 42, "scholarly_entries", "test-model")
	if err != nil {
		t.Fatalf("ReadPage() error: %v", err)
	}

	if page.PageNumber != 42 {
		t.Errorf("expected page number 42, got %d", page.PageNumber)
	}
	if page.SectionType != "scholarly_entries" {
		t.Errorf("expected section type 'scholarly_entries', got %q", page.SectionType)
	}
	if page.ReadModel != "test-model" {
		t.Errorf("expected model 'test-model', got %q", page.ReadModel)
	}
	if page.Header == nil {
		t.Fatal("expected header, got nil")
	}
	if page.Header.Text != "باب الألف" {
		t.Errorf("expected header text 'باب الألف', got %q", page.Header.Text)
	}
	if len(page.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(page.Entries))
	}
	if page.Entries[0].ArabicText != "أَبْشِرُوا" {
		t.Errorf("expected arabic text 'أَبْشِرُوا', got %q", page.Entries[0].ArabicText)
	}
	if len(page.Footnotes) != 1 {
		t.Fatalf("expected 1 footnote, got %d", len(page.Footnotes))
	}
	if len(page.Footnotes[0].SourceCodes) != 1 || page.Footnotes[0].SourceCodes[0] != "طب" {
		t.Errorf("expected source code 'طب', got %v", page.Footnotes[0].SourceCodes)
	}
	if page.Version != "1.0" {
		t.Errorf("expected version '1.0', got %q", page.Version)
	}
	if page.ReadTimestamp == "" {
		t.Error("expected non-empty read timestamp")
	}
}

func TestReadPageNullPageNumber(t *testing.T) {
	// When AI returns null for page_number, use the provided pageNum
	response := `{
		"page_number": null,
		"entries": [],
		"footnotes": [],
		"warnings": []
	}`

	mock := &mockProvider{response: response}
	r := NewReader(mock, nil)

	page, err := r.ReadPage(context.Background(), []byte("fake-image"), 7, "auto", "test-model")
	if err != nil {
		t.Fatalf("ReadPage() error: %v", err)
	}
	if page.PageNumber != 7 {
		t.Errorf("expected page number 7 (from argument), got %d", page.PageNumber)
	}
}

func TestReadPageCodeBlockResponse(t *testing.T) {
	// AI wraps JSON in markdown code block
	response := "```json\n{\"page_number\": 1, \"entries\": [], \"footnotes\": [], \"warnings\": []}\n```"

	mock := &mockProvider{response: response}
	r := NewReader(mock, nil)

	page, err := r.ReadPage(context.Background(), []byte("fake-image"), 1, "auto", "test-model")
	if err != nil {
		t.Fatalf("ReadPage() error: %v", err)
	}
	if page.PageNumber != 1 {
		t.Errorf("expected page number 1, got %d", page.PageNumber)
	}
}

func TestReadPageProviderError(t *testing.T) {
	mock := &mockProvider{err: fmt.Errorf("provider error")}
	r := NewReader(mock, nil)

	_, err := r.ReadPage(context.Background(), []byte("fake-image"), 1, "auto", "test-model")
	if err == nil {
		t.Fatal("expected error when provider fails")
	}
}
