package translation

import (
	"fmt"
	"strings"

	"github.com/mmdemirbas/mutercim/internal/model"
)

const translationSystemPrompt = `You are an expert translator of classical scholarly texts.

%s

TRANSLATION PRINCIPLES:
1. Translate for MEANING, not word-by-word. The reader should understand the intended message naturally.
2. Use established scholarly terminology for the target language (see glossary below).
3. Translate idioms into their target language equivalents or explain them naturally — never produce a literal translation that would be cryptic.
4. Preserve the scholarly register and dignity of the text.

GLOSSARY:
%s

CONTEXT FROM PREVIOUS PAGES:
%s

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

// BuildSystemPrompt constructs the full translation system prompt with knowledge injected.
func BuildSystemPrompt(glossary, context string, expandSources bool, sourceLangs []string, targetLang string) string {
	langInstr := buildLanguageInstruction(sourceLangs, targetLang)

	expandInstr := fmt.Sprintf("When translating footnotes, expand all source abbreviation codes to their full names in %s.", targetLang)
	if !expandSources {
		expandInstr = "Keep source abbreviation codes as-is in footnotes."
	}

	return fmt.Sprintf(translationSystemPrompt,
		langInstr, glossary, context, expandInstr)
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
func BuildRegionUserPrompt(page *model.SolvedRegionPage, glossaryContext []string, sourceLangs []string, targetLang string) string {
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
	// Sort by order
	for i := 1; i < len(ordered); i++ {
		for j := i; j > 0 && ordered[j].order < ordered[j-1].order; j-- {
			ordered[j], ordered[j-1] = ordered[j-1], ordered[j]
		}
	}

	for _, ir := range ordered {
		r := ir.region
		if r.Type == model.RegionTypeSeparator {
			fmt.Fprintf(&b, "Region %s (separator): [separator line - do not translate]\n", r.ID)
		} else if r.Type == model.RegionTypePageNumber {
			fmt.Fprintf(&b, "Region %s (page_number): %s [do not translate]\n", r.ID, r.Text)
		} else {
			fmt.Fprintf(&b, "Region %s (%s): %s\n", r.ID, r.Type, r.Text)
		}
	}

	if len(glossaryContext) > 0 {
		b.WriteString("\nGLOSSARY:\n")
		for _, g := range glossaryContext {
			fmt.Fprintf(&b, "- %s\n", g)
		}
	}

	if page.PreviousPageSummary != "" {
		fmt.Fprintf(&b, "\nCONTEXT FROM PREVIOUS PAGE:\n%s\n", page.PreviousPageSummary)
	}

	return b.String()
}

// BuildContextSection builds the context section from previous pages.
func BuildContextSection(summaries []string) string {
	if len(summaries) == 0 {
		return "(No previous context available)"
	}
	return strings.Join(summaries, "\n")
}
