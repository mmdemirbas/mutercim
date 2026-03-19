package enrichment

import (
	"log/slog"

	"github.com/mmdemirbas/mutercim/internal/knowledge"
	"github.com/mmdemirbas/mutercim/internal/model"
)

// Enricher orchestrates the enrichment of extracted pages.
type Enricher struct {
	knowledge *knowledge.Knowledge
	logger    *slog.Logger
}

// NewEnricher creates a new Enricher.
func NewEnricher(k *knowledge.Knowledge, logger *slog.Logger) *Enricher {
	if logger == nil {
		logger = slog.Default()
	}
	return &Enricher{knowledge: k, logger: logger}
}

// EnrichPage performs all enrichment steps on an extracted page.
// The previous page is used for cross-page continuation detection.
func (e *Enricher) EnrichPage(current *model.ExtractedPage, previous *model.ExtractedPage) *model.EnrichedPage {
	enriched := model.EnrichedPage{
		ExtractedPage: *current,
	}

	// 1. Resolve source abbreviations
	enriched.SourcesResolved, enriched.UnresolvedSources = ResolveAbbreviations(current.Footnotes, e.knowledge)

	if len(enriched.UnresolvedSources) > 0 {
		e.logger.Warn("unresolved sources", "page", current.PageNumber, "codes", enriched.UnresolvedSources)
	}

	// 2. Detect cross-page continuations
	enriched.ContinuationInfo = DetectContinuation(current, previous)

	// 3. Validate structure
	enriched.Validation = Validate(current)

	if enriched.Validation.Status != "ok" {
		e.logger.Warn("validation warnings", "page", current.PageNumber, "warnings", enriched.Validation.Warnings)
	}

	// 4. Build translation context
	enriched.TranslationContext = e.buildTranslationContext(current)

	return &enriched
}

func (e *Enricher) buildTranslationContext(page *model.ExtractedPage) *model.TranslationContext {
	ctx := &model.TranslationContext{}

	// Find relevant glossary terms that appear in this page's text
	for _, entry := range page.Entries {
		for _, term := range e.knowledge.Terminology {
			if containsArabic(entry.ArabicText, term.Arabic) {
				ctx.RelevantGlossaryTerms = append(ctx.RelevantGlossaryTerms,
					term.Arabic+" → "+term.Turkish)
			}
		}
		for _, place := range e.knowledge.Places {
			if containsArabic(entry.ArabicText, place.Arabic) {
				ctx.RelevantGlossaryTerms = append(ctx.RelevantGlossaryTerms,
					place.Arabic+" → "+place.Turkish)
			}
		}
	}

	// Deduplicate
	ctx.RelevantGlossaryTerms = dedupStrings(ctx.RelevantGlossaryTerms)

	return ctx
}

func containsArabic(text, term string) bool {
	return len(term) > 0 && len(text) > 0 && contains(text, term)
}

func contains(s, substr string) bool {
	return len(substr) > 0 && len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func dedupStrings(ss []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}
