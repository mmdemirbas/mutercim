package solver

import (
	"log/slog"
	"strings"

	"github.com/mmdemirbas/mutercim/internal/knowledge"
	"github.com/mmdemirbas/mutercim/internal/model"
)

// Solver orchestrates the solving of region pages.
type Solver struct {
	knowledge  *knowledge.Knowledge
	sourceLang string
	logger     *slog.Logger
}

// NewSolver creates a new Solver.
func NewSolver(k *knowledge.Knowledge, sourceLang string, logger *slog.Logger) *Solver {
	if logger == nil {
		logger = slog.Default()
	}
	if sourceLang == "" {
		sourceLang = "ar"
	}
	return &Solver{knowledge: k, sourceLang: sourceLang, logger: logger}
}

// SolvePage performs all solving steps on a region page.
// It adds glossary context, validation warnings, and a previous page summary.
// It does NOT modify region text, bbox, or type.
func (s *Solver) SolvePage(current *model.RegionPage, previous *model.RegionPage, previousSummary string) *model.SolvedRegionPage {
	solved := &model.SolvedRegionPage{
		RegionPage: *current,
	}

	// 1. Build glossary context from region text
	solved.GlossaryContext = s.buildGlossaryContext(current)

	if len(solved.GlossaryContext) > 0 {
		s.logger.Debug("glossary matches", "page", current.PageNumber, "count", len(solved.GlossaryContext))
	}

	// 2. Validate region structure
	solved.ValidationWarnings = validateRegions(current)

	if len(solved.ValidationWarnings) > 0 {
		s.logger.Warn("validation warnings", "page", current.PageNumber, "warnings", solved.ValidationWarnings)
	}

	// 3. Set previous page summary for translation context
	if previousSummary != "" {
		solved.PreviousPageSummary = previousSummary
	}

	return solved
}

// PageSummary creates a brief summary of a region page for context injection.
func PageSummary(page *model.RegionPage) string {
	if page == nil || len(page.Regions) == 0 {
		return ""
	}

	summary := ""
	for _, r := range page.Regions {
		if r.Type == model.RegionTypeHeader && r.Text != "" {
			summary = r.Text
			break
		}
	}

	// Count entries
	entryCount := 0
	for _, r := range page.Regions {
		if r.Type == model.RegionTypeEntry {
			entryCount++
		}
	}

	if summary == "" {
		return ""
	}
	if entryCount > 0 {
		return summary + " (" + itoa(entryCount) + " entries)"
	}
	return summary
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// stripTashkeel removes Arabic diacritical marks (tashkeel/harakat) from a string.
// This includes Fathatan (U+064B) through Sukun (U+0652) and Superscript Alef (U+0670).
func stripTashkeel(s string) string {
	return strings.Map(func(r rune) rune {
		if (r >= '\u064B' && r <= '\u0652') || r == '\u0670' {
			return -1
		}
		return r
	}, s)
}

// buildGlossaryContext finds relevant glossary terms that appear in the page's text.
// Returns canonical source-language forms for matched entries.
// Uses tashkeel-stripped matching so vowelized text matches unvowelized glossary entries.
func (s *Solver) buildGlossaryContext(page *model.RegionPage) []string {
	var matched []string

	for _, region := range page.Regions {
		if region.Text == "" {
			continue
		}
		strippedText := stripTashkeel(region.Text)
		for _, entry := range s.knowledge.Entries {
			forms, ok := entry.Forms[s.sourceLang]
			if !ok {
				continue
			}
			for _, form := range forms {
				if containsText(strippedText, stripTashkeel(form)) {
					matched = append(matched, forms[0]) // canonical source form
					break
				}
			}
		}
	}

	return dedupStrings(matched)
}

// validateRegions checks structural consistency of regions.
func validateRegions(page *model.RegionPage) []string {
	var warnings []string

	// Check for empty text in non-separator regions
	for _, r := range page.Regions {
		if r.Type != model.RegionTypeSeparator && r.Type != model.RegionTypeImage && r.Text == "" {
			warnings = append(warnings, "region "+r.ID+" ("+r.Type+") has empty text")
		}
	}

	// Check that reading order references valid region IDs
	regionIDs := make(map[string]bool)
	for _, r := range page.Regions {
		regionIDs[r.ID] = true
	}
	for _, id := range page.ReadingOrder {
		if !regionIDs[id] {
			warnings = append(warnings, "reading_order references unknown region: "+id)
		}
	}

	return warnings
}

func containsText(text, term string) bool {
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
