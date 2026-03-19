package translation

import (
	"fmt"
	"strings"
)

const translationSystemPrompt = `You are an expert translator of classical scholarly texts.

%s

TRANSLATION PRINCIPLES:
1. Translate for MEANING, not word-by-word. The reader should understand the intended message naturally.
2. Use established scholarly terminology for the target language (see glossary below).
3. Translate idioms into their target language equivalents or explain them naturally — never produce a literal translation that would be cryptic.
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
For each entry in the input JSON, produce a translation into the target language.
For footnotes, translate the explanatory text and expand source abbreviations to their full names in the target language.

%s

Return a JSON object with this exact schema:
{
  "translated_header": { "text": "<translated header>" } | null,
  "translated_entries": [
    {
      "number": <int>,
      "translated_text": "<translation>",
      "translator_notes": "<any notes about difficult passages>"
    }
  ],
  "translated_footnotes": [
    {
      "entry_numbers": [<int>],
      "translated_text": "<translated footnote>",
      "sources_expanded": ["<full source name in target language>"]
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
func BuildSystemPrompt(honorifics, people, sources, terminology, context, sectionType string, expandSources bool, sourceLangs []string, targetLang string) string {
	langInstr := buildLanguageInstruction(sourceLangs, targetLang)

	expandInstr := fmt.Sprintf("When translating footnotes, expand all source abbreviation codes to their full names in %s.", targetLang)
	if !expandSources {
		expandInstr = "Keep source abbreviation codes as-is in footnotes."
	}

	hint := SectionHint(sectionType)
	if hint != "" {
		expandInstr = hint + "\n\n" + expandInstr
	}

	return fmt.Sprintf(translationSystemPrompt,
		langInstr, honorifics, people, sources, terminology, context, expandInstr)
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
