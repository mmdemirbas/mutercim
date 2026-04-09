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

// regionTranslationResponse matches the JSON returned by the AI for region translation.
type regionTranslationResponse struct {
	Regions  []translatedRegionResp `json:"regions"`
	Warnings []string               `json:"warnings"`
}

type translatedRegionResp struct {
	ID             string `json:"id"`
	TranslatedText string `json:"translated_text"`
}

// Translator translates solved region pages using an AI provider.
type Translator struct {
	provider     provider.Provider
	knowledge    *knowledge.Knowledge
	sourceLangs  []string
	targetLang   string
	logger       *slog.Logger
	systemPrompt string // built once at construction, reused for all pages
}

// NewTranslator creates a new Translator for a specific target language.
// The system prompt is built once here and reused for all pages.
func NewTranslator(p provider.Provider, k *knowledge.Knowledge, expandSources bool, sourceLangs []string, targetLang string, logger *slog.Logger) *Translator {
	if logger == nil {
		logger = slog.Default()
	}
	return &Translator{
		provider:     p,
		knowledge:    k,
		sourceLangs:  sourceLangs,
		targetLang:   targetLang,
		logger:       logger,
		systemPrompt: BuildSystemPrompt(expandSources, sourceLangs, targetLang),
	}
}

// TranslatePage translates a single solved region page.
func (t *Translator) TranslatePage(ctx context.Context, page *model.SolvedRegionPage, contextSummaries []string, modelName string) (*model.TranslatedRegionPage, error) {
	sourceLang := primaryLang(t.sourceLangs)

	// Format page-specific glossary context for the target language
	glossaryContext := t.formatGlossaryContext(page.GlossaryContext, sourceLang)
	userPrompt := BuildRegionUserPrompt(page, glossaryContext, contextSummaries, t.sourceLangs, t.targetLang)

	t.logger.Info("translating page", "page", page.PageNumber)

	rawResponse, err := t.provider.Translate(ctx, t.systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("translate page %d: %w", page.PageNumber, err)
	}

	jsonStr, err := apiclient.ExtractJSON(rawResponse)
	if err != nil {
		t.logger.Warn("translation response JSON extraction failed",
			"page", page.PageNumber, "error", err, "response_len", len(rawResponse))
		return nil, fmt.Errorf("extract JSON from translation response for page %d: %w", page.PageNumber, err)
	}

	var resp regionTranslationResponse
	if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
		t.logger.Warn("translation response JSON unmarshal failed",
			"page", page.PageNumber, "error", err, "response_len", len(rawResponse))
		return nil, fmt.Errorf("unmarshal translation response for page %d: %w", page.PageNumber, err)
	}

	// Build translation map from AI response
	translationMap := make(map[string]string)
	for _, tr := range resp.Regions {
		translationMap[tr.ID] = tr.TranslatedText
	}

	// Build translated regions preserving original structure
	var translated []model.TranslatedRegion
	for _, r := range page.Regions {
		tr := model.TranslatedRegion{
			ID:           r.ID,
			BBox:         r.BBox,
			OriginalText: r.Text,
			Type:         r.Type,
			Style:        r.Style,
		}

		// Apply translation if available
		if text, ok := translationMap[r.ID]; ok {
			tr.TranslatedText = text
		} else if r.Type == model.RegionTypeSeparator || r.Type == model.RegionTypePageNumber || r.Type == model.RegionTypeImage {
			// Non-translatable regions keep original text
			tr.TranslatedText = r.Text
		}

		translated = append(translated, tr)
	}

	result := &model.TranslatedRegionPage{
		Version:            "2.0",
		PageNumber:         page.PageNumber,
		SourceLang:         sourceLang,
		TargetLang:         t.targetLang,
		TranslateModel:     modelName,
		TranslateTimestamp: time.Now().UTC().Format(time.RFC3339),
		Regions:            translated,
		ReadingOrder:       page.ReadingOrder,
		Warnings:           resp.Warnings,
	}

	return result, nil
}

// formatGlossaryContext converts source-language canonical forms into formatted glossary lines
// for the specific target language.
func (t *Translator) formatGlossaryContext(sourceTerms []string, sourceLang string) []string {
	var lines []string
	for _, term := range sourceTerms {
		entry, ok := t.knowledge.LookupByForm(sourceLang, term)
		if !ok {
			lines = append(lines, term) // fallback: show source form only
			continue
		}
		if line := knowledge.FormatGlossaryLine(entry, sourceLang, t.targetLang); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// PageSummary creates a brief summary of a translated region page for context injection.
func PageSummary(page *model.TranslatedRegionPage) string {
	if page == nil || len(page.Regions) == 0 {
		return ""
	}

	// Find header text
	for _, r := range page.Regions {
		if r.Type == model.RegionTypeHeader && r.TranslatedText != "" {
			return r.TranslatedText
		}
	}

	return ""
}

func primaryLang(langs []string) string {
	if len(langs) > 0 {
		return langs[0]
	}
	return ""
}
