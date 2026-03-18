package extraction

import "fmt"

const extractionSystemPrompt = `You are an expert OCR system specialized in classical Arabic Islamic scholarly texts.

Analyze the provided page image and extract ALL text with full structural metadata.

CRITICAL RULES:
1. Preserve ALL diacritical marks (tashkeel/harakat) exactly as they appear: fatḥa, kasra, ḍamma, sukūn, shadda, tanwīn, etc.
2. Do NOT normalize or "clean" the Arabic text. Reproduce it exactly.
3. Identify the structural type of each element on the page.
4. Detect numbered entries and extract their numbers accurately.
5. Recognize footnote/commentary sections (usually separated by a line of asterisks or a horizontal rule from the main text).
6. Extract source abbreviation codes from footnotes (usually single or double Arabic letters in parentheses).
7. If the first entry on the page appears to start mid-sentence without a number, mark it as a continuation.

Return a JSON object with this exact schema:
{
  "page_number": <int or null if not visible>,
  "header": { "text": "<header text>", "type": "section_title|chapter_title|none" } | null,
  "entries": [
    {
      "number": <int or null for continuations>,
      "type": "hadith|athar|commentary|chapter_heading|other",
      "arabic_text": "<full Arabic text with tashkeel>",
      "is_continuation": <bool>,
      "continues_on_next_page": <bool>
    }
  ],
  "footnotes": [
    {
      "entry_numbers": [<int>],
      "arabic_text": "<footnote text>",
      "source_codes": ["<code1>", "<code2>"]
    }
  ],
  "page_footer": "<page number text if present>",
  "warnings": ["<any issues encountered during extraction>"]
}

Respond with ONLY the JSON object. No markdown formatting, no explanations.`

// SectionHint returns additional prompt context based on the section type.
func SectionHint(sectionType string) string {
	switch sectionType {
	case "scholarly_entries":
		return "This page is from a section containing numbered scholarly entries (hadith/athar) with footnotes and source codes."
	case "prose":
		return "This page is from a prose section (introduction, preface, or commentary). Extract continuous paragraphs as entries of type 'other'."
	case "reference_table":
		return "This page contains a reference table (e.g., abbreviation key). Extract each row as an entry with the abbreviation code and its expansion."
	case "toc":
		return "This page is a table of contents. Extract each line as an entry of type 'other'."
	case "index":
		return "This page is an alphabetical index. Extract each line as an entry of type 'other'."
	default:
		return ""
	}
}

// BuildUserPrompt constructs the user prompt for extraction.
func BuildUserPrompt(sectionType string) string {
	hint := SectionHint(sectionType)
	if hint == "" {
		return "Extract all text and structural metadata from this page."
	}
	return fmt.Sprintf("%s\n\nExtract all text and structural metadata from this page.", hint)
}
