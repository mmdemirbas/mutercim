package translation

import (
	"fmt"
	"strings"
)

const translationSystemPrompt = `You are an expert translator of classical Arabic Islamic scholarly texts into Turkish.

TRANSLATION PRINCIPLES:
1. Translate for MEANING, not word-by-word. The Turkish reader should understand the intended message naturally.
2. Use established Turkish Islamic scholarly terminology (see glossary below).
3. Translate Arabic idioms into their Turkish equivalents or explain them naturally — never produce a literal translation that would be cryptic.
4. Preserve the scholarly register and dignity of the text.

HONORIFIC RULES:
%s

PERSON NAME MAPPINGS:
%s

SOURCE ABBREVIATIONS:
%s

TERMINOLOGY GLOSSARY:
%s

CONTEXT FROM PREVIOUS PAGES:
%s

INSTRUCTIONS:
For each entry in the input JSON, produce a Turkish translation.
For footnotes, translate the explanatory text and expand source abbreviations to their full Turkish names.

%s

Return a JSON object with this exact schema:
{
  "translated_header": { "text": "<Turkish header>" } | null,
  "translated_entries": [
    {
      "number": <int>,
      "turkish_text": "<Turkish translation>",
      "translator_notes": "<any notes about difficult passages>"
    }
  ],
  "translated_footnotes": [
    {
      "entry_numbers": [<int>],
      "turkish_text": "<Turkish footnote translation>",
      "sources_expanded": ["<full Turkish source name>"]
    }
  ],
  "warnings": ["<any translation difficulties>"]
}

Respond with ONLY the JSON object. No markdown formatting, no explanations.`

// SectionHint returns section-specific translation guidance.
func SectionHint(sectionType string) string {
	switch sectionType {
	case "scholarly_entries":
		return "This page contains numbered scholarly entries (hadith/athar). Translate each entry preserving its scholarly tone. Expand source abbreviations in footnotes to full Turkish names."
	case "prose":
		return "This page is continuous prose (introduction or commentary). Translate naturally as flowing Turkish text, preserving paragraph structure."
	case "toc":
		return "This page is a table of contents. Translate chapter/section titles while preserving page numbers."
	case "index":
		return "This page is an alphabetical index. Translate terms while preserving their reference structure."
	default:
		return ""
	}
}

// BuildSystemPrompt constructs the full translation system prompt with knowledge injected.
func BuildSystemPrompt(honorifics, people, sources, terminology, context, sectionType string, expandSources bool) string {
	expandInstr := "When translating footnotes, expand all source abbreviation codes to their full Turkish names."
	if !expandSources {
		expandInstr = "Keep source abbreviation codes as-is in footnotes."
	}

	hint := SectionHint(sectionType)
	if hint != "" {
		expandInstr = hint + "\n\n" + expandInstr
	}

	return fmt.Sprintf(translationSystemPrompt,
		honorifics, people, sources, terminology, context, expandInstr)
}

// BuildUserPrompt constructs the user prompt with the solved page JSON.
func BuildUserPrompt(inputJSON string) string {
	return fmt.Sprintf("Translate this page:\n\n%s", inputJSON)
}

// BuildContextSection builds the context section from previous pages.
func BuildContextSection(summaries []string) string {
	if len(summaries) == 0 {
		return "(No previous context available)"
	}
	return strings.Join(summaries, "\n")
}
