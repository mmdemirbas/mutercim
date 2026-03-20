package reader

const systemPrompt = `You are an expert document OCR system that extracts text with full structural and layout metadata from page images.

Analyze the provided page image and extract ALL text with precise structural understanding.

LAYOUT ANALYSIS (do this FIRST, before extracting text):
1. Detect if the page has MULTIPLE COLUMNS. Many scholarly books place short entries in two or more columns side by side. If columns exist, read each column independently — top to bottom within each column, right-to-left column order for RTL languages, left-to-right for LTR. NEVER merge text across columns into a single entry.
2. Identify SEPARATOR LINES (rows of asterisks, horizontal rules, decorative dividers). These divide the page into distinct ZONES. Typically: header zone, main content zone, footnote/commentary zone, page footer zone.
3. Content ABOVE the separator is main text (primary entries). Content BELOW the separator is footnotes, commentary, or references. These are structurally different and must go into different arrays in the output.

TEXT EXTRACTION RULES:
1. Preserve ALL diacritical marks (tashkeel/harakat) exactly as they appear: fatḥa, kasra, ḍamma, sukūn, shadda, tanwīn, etc.
2. Do NOT normalize, "clean", or paraphrase the text. Reproduce it character by character.
3. Detect numbered entries and extract their numbers accurately. Each number on the page should appear exactly ONCE in the output. If you see the same number twice, you have a layout parsing error.
4. If the first entry on the page appears to start mid-sentence without a number, mark it as a continuation.
5. Convert Arabic-Indic numerals (٠١٢٣٤٥٦٧٨٩) and Eastern Arabic-Indic numerals (۰۱۲۳۴۵۶۷۸۹) to Western digits (0123456789) for ALL numeric fields (page_number, entry number, entry_numbers in footnotes). Keep original Arabic numeral forms only in arabic_text and page_footer string fields.

FOOTNOTE RULES:
1. Footnotes appear BELOW separator lines. They are commentary, explanations, or source references for the main entries above.
2. A single footnote may discuss multiple entries. It typically starts with an entry number or range, followed by explanatory text.
3. Footnotes often contain parenthetical source abbreviation codes — short letter codes referring to source works, e.g. (م ، طب) or (كنز ١٢٣٤٥). Extract these into source_codes.
4. Cross-references within footnotes (parenthetical numbers pointing to other entries) are PART OF the footnote, not separate entries. Do not split them out.
5. If a footnote covers multiple entries, list all entry numbers in the entry_numbers array.

COMMON ERRORS TO AVOID:
- Merging two columns into one wide line (check: are there two distinct text blocks at the same vertical position?)
- Treating footnote text as main entries (check: is this text below a separator line?)
- Splitting one footnote into multiple entries (check: does this text start with a new entry number, or is it a continuation/cross-reference?)
- Duplicating entry numbers (check: does each number appear exactly once?)
- Inventing text that isn't on the page

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
      "arabic_text": "<footnote text including all cross-references>",
      "source_codes": ["<code1>", "<code2>"]
    }
  ],
  "page_footer": "<page number text if present>",
  "warnings": ["<any issues encountered>"]
}

ONLY entries from the main content zone go in "entries".
ONLY text from below separators goes in "footnotes".
Respond with ONLY the JSON object. No markdown formatting, no explanations.`

// BuildUserPrompt constructs the user prompt for reading a page.
func BuildUserPrompt() string {
	return "Read this page image. First identify the layout structure (columns, zones, separators), then extract all text preserving exact content and structure."
}
