package solver

import (
	"log/slog"

	"github.com/mmdemirbas/mutercim/internal/knowledge"
	"github.com/mmdemirbas/mutercim/internal/model"
)

// Solver orchestrates the solving of read pages.
type Solver struct {
	knowledge *knowledge.Knowledge
	logger    *slog.Logger
}

// NewSolver creates a new Solver.
func NewSolver(k *knowledge.Knowledge, logger *slog.Logger) *Solver {
	if logger == nil {
		logger = slog.Default()
	}
	return &Solver{knowledge: k, logger: logger}
}

// SolvePage performs all solving steps on a read page.
// The previous page is used for cross-page continuation detection.
func (e *Solver) SolvePage(current *model.ReadPage, previous *model.ReadPage) *model.SolvedPage {
	solved := model.SolvedPage{
		ReadPage: *current,
	}

	// 1. Resolve source abbreviations
	solved.SourcesResolved, solved.UnresolvedSources = ResolveAbbreviations(current.Footnotes, e.knowledge)

	if len(solved.UnresolvedSources) > 0 {
		e.logger.Warn("unresolved sources", "page", current.PageNumber, "codes", solved.UnresolvedSources)
	}

	// 2. Detect cross-page continuations
	solved.ContinuationInfo = DetectContinuation(current, previous)

	// 3. Validate structure
	solved.Validation = Validate(current)

	if solved.Validation.Status != "ok" {
		e.logger.Warn("validation warnings", "page", current.PageNumber, "warnings", solved.Validation.Warnings)
	}

	// 4. Build translation context
	solved.TranslationContext = e.buildTranslationContext(current)

	return &solved
}

func (e *Solver) buildTranslationContext(page *model.ReadPage) *model.TranslationContext {
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
