package translation

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mmdemirbas/mutercim/internal/model"
)

const translationSystemPrompt = `You are an expert translator of classical scholarly texts.

%s

TRANSLATION PRINCIPLES:
1. Translate for MEANING, not word-by-word. The reader should understand the intended message naturally.
2. Use established scholarly terminology for the target language.
3. Translate idioms into their target language equivalents or explain them naturally — never produce a literal translation that would be cryptic.
4. Preserve the scholarly register and dignity of the text.

INSTRUCTIONS:
You are translating text regions from a page. Each region has an ID and type.
- Translate the text content of each region into the target language.
- Do NOT merge or split regions — one translated_text per region.
- Separators and page numbers: keep original text, do not translate.
- For entry regions containing numbers (like 1060), keep the number.

%s

Return a JSON object with this exact schema:
{
  "regions": [
    {"id": "r1", "translated_text": "<translated text>"},
    {"id": "r2", "translated_text": "<translated text>"}
  ],
  "warnings": ["<any translation difficulties>"]
}

Respond with ONLY the JSON object. No markdown formatting, no explanations.`

// BuildSystemPrompt constructs the full translation system prompt.
// Glossary and context are injected per-page in the user prompt, not here.
func BuildSystemPrompt(expandSources bool, sourceLangs []string, targetLang string) string {
	langInstr := buildLanguageInstruction(sourceLangs, targetLang)

	expandInstr := fmt.Sprintf("When translating footnotes, expand all source abbreviation codes to their full names in %s.", targetLang)
	if !expandSources {
		expandInstr = "Keep source abbreviation codes as-is in footnotes."
	}

	return fmt.Sprintf(translationSystemPrompt, langInstr, expandInstr)
}

// buildLanguageInstruction creates the source/target language description for the prompt.
func buildLanguageInstruction(sourceLangs []string, targetLang string) string {
	if len(sourceLangs) == 0 {
		return fmt.Sprintf("Translate the source text into %s.", targetLang)
	}
	primary := sourceLangs[0]
	if len(sourceLangs) == 1 {
		return fmt.Sprintf("The source text is in %s. Translate everything into %s.", primary, targetLang)
	}
	rest := strings.Join(sourceLangs[1:], ", ")
	return fmt.Sprintf("The source text is primarily %s but may contain %s fragments. Translate everything into %s.", primary, rest, targetLang)
}

// BuildRegionUserPrompt constructs the user prompt with all regions listed.
// glossaryContext contains pre-formatted glossary lines for the target language.
// contextSummaries contains summaries from previous pages for translation continuity.
//nolint:cyclop,gocognit // prompt construction with many conditional blocks
func BuildRegionUserPrompt(page *model.SolvedRegionPage, glossaryContext []string, contextSummaries []string, sourceLangs []string, targetLang string) string {
	var b strings.Builder

	primary := "the source language"
	if len(sourceLangs) > 0 {
		primary = sourceLangs[0]
	}

	fmt.Fprintf(&b, "Translate the following page from %s to %s.\n", primary, targetLang)
	b.WriteString("The page has these text regions in reading order:\n\n")

	// List regions in reading order
	orderMap := make(map[string]int, len(page.ReadingOrder))
	for i, id := range page.ReadingOrder {
		orderMap[id] = i
	}

	// Use reading order to iterate
	type indexedRegion struct {
		region model.Region
		order  int
	}
	var ordered []indexedRegion
	for _, r := range page.Regions {
		o := len(page.ReadingOrder) // default: after all ordered
		if idx, ok := orderMap[r.ID]; ok {
			o = idx
		}
		ordered = append(ordered, indexedRegion{region: r, order: o})
	}
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].order < ordered[j].order
	})

	for _, ir := range ordered {
		r := ir.region
		switch r.Type {
		case model.RegionTypeSeparator:
			fmt.Fprintf(&b, "Region %s (separator): [separator line - do not translate]\n", r.ID)
		case model.RegionTypePageNumber:
			fmt.Fprintf(&b, "Region %s (page_number): %s [do not translate]\n", r.ID, r.Text)
		default:
			fmt.Fprintf(&b, "Region %s (%s):\n\"\"\"\n%s\n\"\"\"\n", r.ID, r.Type, r.Text)
		}
	}

	if len(glossaryContext) > 0 {
		b.WriteString("\nGLOSSARY:\n")
		for _, g := range glossaryContext {
			fmt.Fprintf(&b, "- %s\n", g)
		}
	}

	contextSection := BuildContextSection(contextSummaries)
	if contextSection != "" {
		fmt.Fprintf(&b, "\nCONTEXT FROM PREVIOUS PAGES:\n%s\n", contextSection)
	}
	if page.PreviousPageSummary != "" {
		fmt.Fprintf(&b, "\nPREVIOUS PAGE SUMMARY:\n%s\n", page.PreviousPageSummary)
	}

	return b.String()
}

// BuildContextSection builds the context section from previous pages.
// Returns empty string when no context is available, saving tokens.
func BuildContextSection(summaries []string) string {
	if len(summaries) == 0 {
		return ""
	}
	return strings.Join(summaries, "\n")
}
