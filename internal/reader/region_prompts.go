package reader

import (
	"fmt"
	"strings"

	"github.com/mmdemirbas/mutercim/internal/model"
)

// regionSystemPromptAIOnly is the system prompt for the AI-only strategy
// where no layout tool is available and the LLM detects all regions.
const regionSystemPromptAIOnly = `You are an expert document layout analyzer and OCR system. Analyze page images and detect ALL text regions with their spatial coordinates, content, and semantic classification.

LAYOUT RULES:
- Detect multi-column layouts. Read each column independently.
- Separator lines (horizontal rules, asterisk rows, decorative dividers) divide the page into zones (main content vs footnotes).
- For RTL text, columns are ordered right-to-left.
- Estimate bounding box coordinates in pixels relative to the page image.

For each region provide:
- id: unique identifier (r1, r2, ...)
- bbox: [x, y, width, height] in pixels (approximate is fine)
- text: full text with ALL diacritical marks (tashkeel/harakat) preserved exactly. Do NOT normalize or paraphrase.
- type: header | entry | footnote | separator | page_number | column_header | table | image | margin_note | other
- style: {"font_size": <int>, "bold": <bool>, "direction": "rtl"|"ltr", "alignment": "center"|"right"|"left"|"justify"}
- column: column number if multi-column layout detected (omit for single-column)

TEXT EXTRACTION RULES:
1. Preserve ALL diacritical marks exactly as they appear: fatḥa, kasra, ḍamma, sukūn, shadda, tanwīn, etc.
2. Do NOT normalize, "clean", or paraphrase the text. Reproduce character by character.
3. Convert Arabic-Indic numerals (٠١٢٣٤٥٦٧٨٩) and Eastern Arabic-Indic numerals (۰۱۲۳۴۵۶۷۸۹) to Western digits (0123456789) for ALL numeric fields (entry numbers in text identification, page_number). Keep original Arabic numeral forms in the text field.
4. Detect numbered entries and ensure each number appears exactly ONCE.

COMMON ERRORS TO AVOID:
- Merging two columns into one wide line
- Treating footnote text as main entries (footnotes are BELOW separator lines)
- Splitting one footnote into multiple regions
- Duplicating entry numbers
- Inventing text not on the page

Return JSON with this exact schema:
{
  "regions": [
    {
      "id": "r1",
      "bbox": [x, y, width, height],
      "text": "<full text>",
      "type": "header|entry|footnote|separator|page_number|other",
      "style": {"font_size": 14, "bold": false, "direction": "rtl", "alignment": "right"},
      "column": 1
    }
  ],
  "reading_order": ["r1", "r2", "r3"],
  "warnings": ["<any issues>"]
}

Respond with ONLY JSON. No markdown formatting, no explanations.`

// regionSystemPromptWithLayout is the system prompt for the local+AI strategy
// where a layout tool has already detected region boundaries.
const regionSystemPromptWithLayout = `You are an expert document OCR system. A layout detection tool has already identified text regions on this page with their bounding boxes. Your job is to:

1. For each detected region, provide the ACCURATE text with ALL diacritical marks preserved exactly
2. Classify each region's type: header | entry | footnote | separator | page_number | column_header | table | image | margin_note | other
3. Estimate style: font_size, bold, direction (rtl/ltr), alignment (center/right/left/justify)
4. If any regions should be split or merged, indicate that
5. If the tool missed any text regions, add them with approximate coordinates

TEXT EXTRACTION RULES:
1. Preserve ALL diacritical marks exactly as they appear: fatḥa, kasra, ḍamma, sukūn, shadda, tanwīn, etc.
2. Do NOT normalize, "clean", or paraphrase the text. Reproduce character by character.
3. Convert Arabic-Indic numerals to Western digits for numeric fields only.

Return JSON with this exact schema:
{
  "regions": [
    {
      "id": "r1",
      "bbox": [x, y, width, height],
      "text": "<accurate text with diacritics>",
      "type": "header|entry|footnote|separator|page_number|other",
      "style": {"font_size": 14, "bold": false, "direction": "rtl", "alignment": "right"},
      "column": 1
    }
  ],
  "reading_order": ["r1", "r2", "r3"],
  "warnings": ["<any issues>"]
}

Respond with ONLY JSON. No markdown formatting, no explanations.`

// BuildRegionUserPrompt returns the user prompt for the AI-only strategy.
func BuildRegionUserPrompt() string {
	return "Analyze this page image. Detect ALL text regions on the page with their bounding boxes, content, type, and style. Return regions in reading order."
}

// BuildRegionUserPromptWithLayout returns the user prompt for the local+AI
// strategy, including the pre-detected regions from the layout tool.
func BuildRegionUserPromptWithLayout(regions []model.Region) string {
	if len(regions) == 0 {
		return BuildRegionUserPrompt()
	}

	var b strings.Builder
	b.WriteString("I have detected the following text regions on this page using a layout analysis tool. The bounding boxes are in pixels [x, y, width, height].\n\nDetected regions:\n")

	for _, r := range regions {
		fmt.Fprintf(&b, "[%s: bbox=[%d, %d, %d, %d]", r.ID, r.BBox[0], r.BBox[1], r.BBox[2], r.BBox[3])
		if r.Text != "" {
			fmt.Fprintf(&b, ", preliminary_text=%q", r.Text)
		}
		b.WriteString("]\n")
	}

	b.WriteString("\nFor each region:\n")
	b.WriteString("1. Provide the ACCURATE text with ALL diacritical marks preserved exactly\n")
	b.WriteString("2. Classify: header | entry | footnote | separator | page_number | other\n")
	b.WriteString("3. Estimate style: font_size, bold, direction (rtl/ltr), alignment\n")
	b.WriteString("4. If any regions should be split or merged, indicate that\n")
	b.WriteString("5. If I missed any text regions, add them with approximate coordinates\n\n")
	b.WriteString("Return JSON array of regions with reading_order.")

	return b.String()
}
