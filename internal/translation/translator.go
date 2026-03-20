package translation

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/mmdemirbas/mutercim/internal/apiclient"
	"github.com/mmdemirbas/mutercim/internal/knowledge"
	"github.com/mmdemirbas/mutercim/internal/model"
	"github.com/mmdemirbas/mutercim/internal/provider"
)

// translationResponse matches the JSON schema returned by the AI model.
type translationResponse struct {
	TranslatedHeader    *model.TranslatedHeader  `json:"translated_header"`
	TranslatedEntries   []model.TranslatedEntry  `json:"translated_entries"`
	TranslatedFootnotes []translatedFootnoteResp `json:"translated_footnotes"`
	Warnings            []string                 `json:"warnings"`
}

// translatedFootnoteResp matches the footnote format in the translation prompt.
type translatedFootnoteResp struct {
	EntryNumbers    []int    `json:"entry_numbers"`
	TranslatedText  string   `json:"translated_text"`
	SourcesExpanded []string `json:"sources_expanded"`
}

// Translator translates solved pages using an AI provider.
type Translator struct {
	provider      provider.Provider
	knowledge     *knowledge.Knowledge
	expandSources bool
	sourceLangs   []string
	targetLang    string
	logger        *slog.Logger
}

// NewTranslator creates a new Translator for a specific target language.
func NewTranslator(p provider.Provider, k *knowledge.Knowledge, expandSources bool, sourceLangs []string, targetLang string, logger *slog.Logger) *Translator {
	if logger == nil {
		logger = slog.Default()
	}
	return &Translator{
		provider:      p,
		knowledge:     k,
		expandSources: expandSources,
		sourceLangs:   sourceLangs,
		targetLang:    targetLang,
		logger:        logger,
	}
}

// TranslatePage translates a single solved page. contextPages provides
// summaries from previous pages for the sliding context window.
func (t *Translator) TranslatePage(ctx context.Context, page *model.SolvedPage, contextSummaries []string, modelName string) (*model.TranslatedPage, error) {
	// Build the system prompt with knowledge injected
	systemPrompt := BuildSystemPrompt(
		t.knowledge.HonorificsSection(),
		t.knowledge.PeopleSection(),
		t.knowledge.SourcesSection(),
		t.knowledge.TerminologySection(),
		BuildContextSection(contextSummaries),
		t.expandSources,
		t.sourceLangs,
		t.targetLang,
	)

	// Build user prompt with page JSON
	pageJSON, err := json.Marshal(page)
	if err != nil {
		return nil, fmt.Errorf("marshal page %d: %w", page.PageNumber, err)
	}
	userPrompt := BuildUserPrompt(string(pageJSON))

	t.logger.Info("translating page", "page", page.PageNumber)

	rawResponse, err := t.provider.Translate(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("translate page %d: %w", page.PageNumber, err)
	}

	jsonStr, err := apiclient.ExtractJSON(rawResponse)
	if err != nil {
		return nil, fmt.Errorf("extract JSON from translation response for page %d: %w", page.PageNumber, err)
	}

	var resp translationResponse
	if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
		return nil, fmt.Errorf("unmarshal translation response for page %d: %w", page.PageNumber, err)
	}

	translated := &model.TranslatedPage{
		SolvedPage:           *page,
		TranslationModel:     modelName,
		TranslationTimestamp: time.Now().UTC().Format(time.RFC3339),
		TranslatedHeader:     resp.TranslatedHeader,
		TranslatedEntries:    resp.TranslatedEntries,
		TranslatedFootnotes:  convertTranslatedFootnotes(resp.TranslatedFootnotes),
		TranslationWarnings:  resp.Warnings,
	}

	return translated, nil
}

func convertTranslatedFootnotes(resps []translatedFootnoteResp) []model.TranslatedFootnote {
	footnotes := make([]model.TranslatedFootnote, 0, len(resps))
	for _, r := range resps {
		footnotes = append(footnotes, model.TranslatedFootnote{
			EntryNumbers:    r.EntryNumbers,
			TranslatedText:  r.TranslatedText,
			SourcesExpanded: r.SourcesExpanded,
		})
	}
	return footnotes
}

// PageSummary creates a brief summary of a translated page for context injection.
func PageSummary(page *model.TranslatedPage) string {
	if page == nil {
		return ""
	}

	summary := fmt.Sprintf("Page %d", page.PageNumber)
	if page.TranslatedHeader != nil && page.TranslatedHeader.Text != "" {
		summary += fmt.Sprintf(" — %s", page.TranslatedHeader.Text)
	}
	if len(page.TranslatedEntries) > 0 {
		first := page.TranslatedEntries[0].Number
		last := page.TranslatedEntries[len(page.TranslatedEntries)-1].Number
		if first > 0 && last > 0 {
			summary += fmt.Sprintf(". Entries %d-%d", first, last)
		}
	}
	return summary
}
