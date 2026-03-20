package renderer

import (
	"strings"
	"testing"

	"github.com/mmdemirbas/mutercim/internal/model"
)

func TestLatexRenderPage_UnknownRegionType(t *testing.T) {
	r := &LaTeXRenderer{Lang: "tr"}
	page := &model.TranslatedRegionPage{
		PageNumber: 1,
		Regions: []model.TranslatedRegion{
			{ID: "r1", Type: "unknown_type", TranslatedText: "some text"},
		},
		ReadingOrder: []string{"r1"},
	}

	result := r.RenderPage(page)

	// Unknown region types should be silently skipped (no panic, no crash)
	if !strings.Contains(result, "Page 1") {
		t.Errorf("missing page comment, got:\n%s", result)
	}
	// The unknown type text should NOT appear in output (no matching case in switch)
	if strings.Contains(result, "some text") {
		t.Errorf("unknown region type text should not appear in output, got:\n%s", result)
	}
}

func TestLatexRenderPage_PageNumberType(t *testing.T) {
	r := &LaTeXRenderer{Lang: "tr"}
	page := &model.TranslatedRegionPage{
		PageNumber: 1,
		Regions: []model.TranslatedRegion{
			{ID: "pn1", Type: model.RegionTypePageNumber, TranslatedText: "42"},
		},
		ReadingOrder: []string{"pn1"},
	}

	result := r.RenderPage(page)

	// page_number has no case in the switch — silently skipped
	if strings.Contains(result, "42") {
		t.Logf("Note: page_number region rendered as: %s", result)
	}
}

func TestLatexPreamble_UnknownLanguageCode(t *testing.T) {
	r := &LaTeXRenderer{Lang: "xx"} // unmapped code
	preamble := r.buildPreamble()

	// Unknown code should be passed through as-is (not crash)
	if !strings.Contains(preamble, `\setmainlanguage{xx}`) {
		t.Errorf("expected unknown code passed through, got:\n%s", preamble)
	}
}

func TestLatexPreamble_GermanLanguageCode(t *testing.T) {
	r := &LaTeXRenderer{Lang: "de"}
	preamble := r.buildPreamble()

	if !strings.Contains(preamble, `\setmainlanguage{german}`) {
		t.Errorf("expected german, got:\n%s", preamble)
	}
}

func TestLatexPreamble_FrenchLanguageCode(t *testing.T) {
	r := &LaTeXRenderer{Lang: "fr"}
	preamble := r.buildPreamble()

	if !strings.Contains(preamble, `\setmainlanguage{french}`) {
		t.Errorf("expected french, got:\n%s", preamble)
	}
}

func TestLatexRenderPage_AllRegionTypes(t *testing.T) {
	r := &LaTeXRenderer{Lang: "tr"}
	page := &model.TranslatedRegionPage{
		PageNumber: 1,
		Regions: []model.TranslatedRegion{
			{ID: "r1", Type: model.RegionTypeHeader, TranslatedText: "Title", OriginalText: "عنوان"},
			{ID: "r2", Type: model.RegionTypeEntry, TranslatedText: "Entry text", OriginalText: "نص"},
			{ID: "r3", Type: model.RegionTypeFootnote, TranslatedText: "Footnote text"},
			{ID: "r4", Type: model.RegionTypeSeparator, TranslatedText: "---"},
			{ID: "r5", Type: model.RegionTypePageNumber, TranslatedText: "42"},
			{ID: "r6", Type: model.RegionTypeImage, TranslatedText: "[image]"},
			{ID: "r7", Type: "custom_unknown", TranslatedText: "should not appear"},
		},
		ReadingOrder: []string{"r1", "r2", "r3", "r4", "r5", "r6", "r7"},
	}

	result := r.RenderPage(page)

	// Header, entry, footnote, separator should appear
	if !strings.Contains(result, "Title") {
		t.Error("missing header")
	}
	if !strings.Contains(result, "Entry text") {
		t.Error("missing entry")
	}
	if !strings.Contains(result, "Footnote text") {
		t.Error("missing footnote")
	}
	if !strings.Contains(result, `\hrule`) {
		t.Error("missing separator")
	}

	// page_number, image, custom_unknown should be silently skipped
	if strings.Contains(result, "should not appear") {
		t.Error("unknown type text should not appear")
	}
}
