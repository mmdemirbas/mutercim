package renderer

import (
	"strings"
	"testing"

	"github.com/mmdemirbas/mutercim/internal/model"
)

// placeholderBBox is a dummy bounding box for tests.
var placeholderBBox = model.BBox{0, 0, 100, 50}

// --- MarkdownRenderer.RenderPage ---

func TestMarkdownRenderPage(t *testing.T) {
	r := &MarkdownRenderer{}

	page := &model.TranslatedRegionPage{
		PageNumber: 1,
		Regions: []model.TranslatedRegion{
			{ID: "r1", BBox: placeholderBBox, Type: model.RegionTypeHeader, TranslatedText: "Bab Basligi", OriginalText: "باب"},
			{ID: "r2", BBox: placeholderBBox, Type: model.RegionTypeEntry, TranslatedText: "Bu bir hadistir.", OriginalText: "حديث"},
			{ID: "r3", BBox: placeholderBBox, Type: model.RegionTypeEntry, TranslatedText: "Ikinci hadis.", OriginalText: "حديث ثاني"},
			{ID: "r4", BBox: placeholderBBox, Type: model.RegionTypeSeparator},
			{ID: "r5", BBox: placeholderBBox, Type: model.RegionTypeFootnote, TranslatedText: "Sahih-i Buhari'de rivayet edilmistir.", OriginalText: "رواه البخاري"},
		},
		ReadingOrder: []string{"r1", "r2", "r3", "r4", "r5"},
	}

	result := r.RenderPage(page)

	for _, want := range []string{
		"# Bab Basligi",
		"Bu bir hadistir.",
		"Ikinci hadis.",
		"---",
		"> Sahih-i Buhari'de rivayet edilmistir.",
	} {
		if !strings.Contains(result, want) {
			t.Errorf("expected result to contain %q, got:\n%s", want, result)
		}
	}
}

func TestMarkdownRenderPage_RegionTypes(t *testing.T) {
	tests := []struct {
		name     string
		page     *model.TranslatedRegionPage
		contains []string
		excludes []string
	}{
		{
			name: "header renders as heading",
			page: &model.TranslatedRegionPage{
				PageNumber: 1,
				Regions: []model.TranslatedRegion{
					{ID: "h1", BBox: placeholderBBox, Type: model.RegionTypeHeader, TranslatedText: "Chapter One"},
				},
				ReadingOrder: []string{"h1"},
			},
			contains: []string{"# Chapter One\n"},
		},
		{
			name: "entry renders as paragraph",
			page: &model.TranslatedRegionPage{
				PageNumber: 1,
				Regions: []model.TranslatedRegion{
					{ID: "e1", BBox: placeholderBBox, Type: model.RegionTypeEntry, TranslatedText: "Some text here."},
				},
				ReadingOrder: []string{"e1"},
			},
			contains: []string{"Some text here.\n"},
			excludes: []string{"# ", "> ", "---"},
		},
		{
			name: "footnote renders as blockquote",
			page: &model.TranslatedRegionPage{
				PageNumber: 1,
				Regions: []model.TranslatedRegion{
					{ID: "f1", BBox: placeholderBBox, Type: model.RegionTypeFootnote, TranslatedText: "A footnote."},
				},
				ReadingOrder: []string{"f1"},
			},
			contains: []string{"> A footnote.\n"},
		},
		{
			name: "separator renders as horizontal rule",
			page: &model.TranslatedRegionPage{
				PageNumber: 1,
				Regions: []model.TranslatedRegion{
					{ID: "s1", BBox: placeholderBBox, Type: model.RegionTypeSeparator},
				},
				ReadingOrder: []string{"s1"},
			},
			contains: []string{"---\n"},
		},
		{
			name: "page_number renders as HTML comment",
			page: &model.TranslatedRegionPage{
				PageNumber: 1,
				Regions: []model.TranslatedRegion{
					{ID: "pn1", BBox: placeholderBBox, Type: model.RegionTypePageNumber, OriginalText: "42"},
				},
				ReadingOrder: []string{"pn1"},
			},
			contains: []string{"<!-- page 42 -->"},
		},
	}

	r := &MarkdownRenderer{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.RenderPage(tt.page)
			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("expected result to contain %q, got:\n%s", want, result)
				}
			}
			for _, exclude := range tt.excludes {
				if strings.Contains(result, exclude) {
					t.Errorf("expected result NOT to contain %q, got:\n%s", exclude, result)
				}
			}
		})
	}
}

func TestMarkdownRenderPage_NoHeader(t *testing.T) {
	r := &MarkdownRenderer{}

	page := &model.TranslatedRegionPage{
		PageNumber: 1,
		Regions: []model.TranslatedRegion{
			{ID: "e1", BBox: placeholderBBox, Type: model.RegionTypeEntry, TranslatedText: "Metin"},
		},
		ReadingOrder: []string{"e1"},
	}

	result := r.RenderPage(page)

	if strings.Contains(result, "# ") {
		t.Error("expected no header line when no header region exists")
	}
	if !strings.Contains(result, "Metin") {
		t.Error("expected entry text")
	}
}

func TestMarkdownRenderPage_EmptyHeaderText(t *testing.T) {
	r := &MarkdownRenderer{}

	page := &model.TranslatedRegionPage{
		PageNumber: 1,
		Regions: []model.TranslatedRegion{
			{ID: "h1", BBox: placeholderBBox, Type: model.RegionTypeHeader, TranslatedText: ""},
		},
		ReadingOrder: []string{"h1"},
	}

	result := r.RenderPage(page)

	if strings.Contains(result, "# ") {
		t.Error("expected no header line when header TranslatedText is empty")
	}
}

func TestMarkdownRenderPage_EmptyPage(t *testing.T) {
	r := &MarkdownRenderer{}

	page := &model.TranslatedRegionPage{
		PageNumber:   1,
		ReadingOrder: nil,
	}

	result := r.RenderPage(page)

	if result != "" {
		t.Errorf("expected empty result for page with no reading order, got %q", result)
	}
}

func TestMarkdownRenderPage_EmptyRegions(t *testing.T) {
	r := &MarkdownRenderer{}

	page := &model.TranslatedRegionPage{
		PageNumber:   1,
		Regions:      nil,
		ReadingOrder: []string{"missing1", "missing2"},
	}

	result := r.RenderPage(page)

	if result != "" {
		t.Errorf("expected empty result when reading order references missing regions, got %q", result)
	}
}

func TestMarkdownRenderPage_ReadingOrderDeterminesOrder(t *testing.T) {
	r := &MarkdownRenderer{}

	page := &model.TranslatedRegionPage{
		PageNumber: 1,
		Regions: []model.TranslatedRegion{
			{ID: "e1", BBox: placeholderBBox, Type: model.RegionTypeEntry, TranslatedText: "First"},
			{ID: "e2", BBox: placeholderBBox, Type: model.RegionTypeEntry, TranslatedText: "Second"},
			{ID: "h1", BBox: placeholderBBox, Type: model.RegionTypeHeader, TranslatedText: "Title"},
		},
		// Reading order puts header first despite it being last in Regions slice.
		ReadingOrder: []string{"h1", "e1", "e2"},
	}

	result := r.RenderPage(page)

	titleIdx := strings.Index(result, "# Title")
	firstIdx := strings.Index(result, "First")
	secondIdx := strings.Index(result, "Second")

	if titleIdx == -1 || firstIdx == -1 || secondIdx == -1 {
		t.Fatalf("expected all three regions in result, got:\n%s", result)
	}
	if titleIdx >= firstIdx || firstIdx >= secondIdx {
		t.Errorf("expected Title < First < Second in output order, got indices %d, %d, %d", titleIdx, firstIdx, secondIdx)
	}
}

func TestMarkdownRenderPage_MixedRegionTypes(t *testing.T) {
	r := &MarkdownRenderer{}

	page := &model.TranslatedRegionPage{
		PageNumber: 1,
		Regions: []model.TranslatedRegion{
			{ID: "h1", BBox: placeholderBBox, Type: model.RegionTypeHeader, TranslatedText: "Heading"},
			{ID: "e1", BBox: placeholderBBox, Type: model.RegionTypeEntry, TranslatedText: "Entry one."},
			{ID: "e2", BBox: placeholderBBox, Type: model.RegionTypeEntry, TranslatedText: "Entry two."},
			{ID: "s1", BBox: placeholderBBox, Type: model.RegionTypeSeparator},
			{ID: "f1", BBox: placeholderBBox, Type: model.RegionTypeFootnote, TranslatedText: "Footnote text."},
			{ID: "pn", BBox: placeholderBBox, Type: model.RegionTypePageNumber, OriginalText: "7"},
		},
		ReadingOrder: []string{"h1", "e1", "e2", "s1", "f1", "pn"},
	}

	result := r.RenderPage(page)

	expected := "# Heading\n\nEntry one.\n\nEntry two.\n\n---\n\n> Footnote text.\n\n<!-- page 7 -->\n\n"
	if result != expected {
		t.Errorf("unexpected output.\nwant:\n%s\ngot:\n%s", expected, result)
	}
}

// --- MarkdownRenderer.RenderBook ---

func TestMarkdownRenderBook(t *testing.T) {
	r := &MarkdownRenderer{}

	pages := []*model.TranslatedRegionPage{
		{
			PageNumber: 1,
			Regions: []model.TranslatedRegion{
				{ID: "e1", BBox: placeholderBBox, Type: model.RegionTypeEntry, TranslatedText: "Birinci"},
			},
			ReadingOrder: []string{"e1"},
		},
		{
			PageNumber: 2,
			Regions: []model.TranslatedRegion{
				{ID: "e1", BBox: placeholderBBox, Type: model.RegionTypeEntry, TranslatedText: "Ikinci"},
			},
			ReadingOrder: []string{"e1"},
		},
	}

	result := r.RenderBook(pages)

	if !strings.Contains(result, "<!-- Page 1 -->") || !strings.Contains(result, "<!-- Page 2 -->") {
		t.Error("expected both page markers")
	}
	if !strings.Contains(result, "\n---\n") {
		t.Error("expected page separator between pages")
	}
}

func TestMarkdownRenderBook_MultiplePages(t *testing.T) {
	r := &MarkdownRenderer{}

	pages := []*model.TranslatedRegionPage{
		{
			PageNumber: 1,
			Regions: []model.TranslatedRegion{
				{ID: "h1", BBox: placeholderBBox, Type: model.RegionTypeHeader, TranslatedText: "Birinci Bab"},
				{ID: "e1", BBox: placeholderBBox, Type: model.RegionTypeEntry, TranslatedText: "Birinci hadis"},
			},
			ReadingOrder: []string{"h1", "e1"},
		},
		{
			PageNumber: 2,
			Regions: []model.TranslatedRegion{
				{ID: "e1", BBox: placeholderBBox, Type: model.RegionTypeEntry, TranslatedText: "Ikinci hadis"},
			},
			ReadingOrder: []string{"e1"},
		},
		{
			PageNumber: 3,
			Regions: []model.TranslatedRegion{
				{ID: "f1", BBox: placeholderBBox, Type: model.RegionTypeFootnote, TranslatedText: "Dipnot"},
			},
			ReadingOrder: []string{"f1"},
		},
	}

	result := r.RenderBook(pages)

	if !strings.Contains(result, "<!-- Page 1 -->") {
		t.Error("expected Page 1 comment")
	}
	if !strings.Contains(result, "<!-- Page 2 -->") {
		t.Error("expected Page 2 comment")
	}
	if !strings.Contains(result, "<!-- Page 3 -->") {
		t.Error("expected Page 3 comment")
	}
	if !strings.Contains(result, "# Birinci Bab") {
		t.Error("expected header on first page")
	}
	// Two page separators between three pages.
	if strings.Count(result, "\n---\n") < 2 {
		t.Errorf("expected at least 2 page separators, got %d", strings.Count(result, "\n---\n"))
	}
}

func TestMarkdownRenderBook_EmptySlice(t *testing.T) {
	r := &MarkdownRenderer{}
	result := r.RenderBook(nil)
	if result != "" {
		t.Errorf("expected empty result for nil pages, got %q", result)
	}
}

func TestMarkdownRenderBook_SinglePage(t *testing.T) {
	r := &MarkdownRenderer{}

	pages := []*model.TranslatedRegionPage{
		{
			PageNumber: 5,
			Regions: []model.TranslatedRegion{
				{ID: "e1", BBox: placeholderBBox, Type: model.RegionTypeEntry, TranslatedText: "Tek sayfa"},
			},
			ReadingOrder: []string{"e1"},
		},
	}

	result := r.RenderBook(pages)

	if strings.Contains(result, "\n---\n") {
		t.Error("expected no separator for single page")
	}
	if !strings.Contains(result, "<!-- Page 5 -->") {
		t.Error("expected Page 5 comment")
	}
}

func TestMarkdownRenderBook_EmptyPages(t *testing.T) {
	r := &MarkdownRenderer{}

	pages := []*model.TranslatedRegionPage{
		{PageNumber: 1},
		{PageNumber: 2},
	}

	result := r.RenderBook(pages)

	if !strings.Contains(result, "<!-- Page 1 -->") {
		t.Error("expected Page 1 comment even for empty page")
	}
	if !strings.Contains(result, "<!-- Page 2 -->") {
		t.Error("expected Page 2 comment even for empty page")
	}
}

// --- ArabicMarkdownRenderer.RenderPage ---

func TestArabicMarkdownRenderPage(t *testing.T) {
	r := &ArabicMarkdownRenderer{}

	page := &model.TranslatedRegionPage{
		PageNumber: 1,
		Regions: []model.TranslatedRegion{
			{ID: "h1", BBox: placeholderBBox, Type: model.RegionTypeHeader, OriginalText: "حرف الألف", TranslatedText: "Elif Harfi"},
			{ID: "e1", BBox: placeholderBBox, Type: model.RegionTypeEntry, OriginalText: "أَبْشِرُوا", TranslatedText: "Musjdelenin"},
		},
		ReadingOrder: []string{"h1", "e1"},
	}

	result := r.RenderPage(page)

	if !strings.Contains(result, "# حرف الألف") {
		t.Error("expected Arabic header from OriginalText")
	}
	if !strings.Contains(result, "أَبْشِرُوا") {
		t.Error("expected Arabic entry text from OriginalText")
	}
	if strings.Contains(result, "Elif Harfi") {
		t.Error("expected Arabic renderer NOT to use TranslatedText for header")
	}
	if strings.Contains(result, "Musjdelenin") {
		t.Error("expected Arabic renderer NOT to use TranslatedText for entry")
	}
}

func TestArabicMarkdownRenderPage_AllRegionTypes(t *testing.T) {
	tests := []struct {
		name     string
		page     *model.TranslatedRegionPage
		contains []string
		excludes []string
	}{
		{
			name: "header uses OriginalText",
			page: &model.TranslatedRegionPage{
				PageNumber: 1,
				Regions: []model.TranslatedRegion{
					{ID: "h1", BBox: placeholderBBox, Type: model.RegionTypeHeader, OriginalText: "باب", TranslatedText: "Chapter"},
				},
				ReadingOrder: []string{"h1"},
			},
			contains: []string{"# باب"},
			excludes: []string{"Chapter"},
		},
		{
			name: "entry uses OriginalText",
			page: &model.TranslatedRegionPage{
				PageNumber: 1,
				Regions: []model.TranslatedRegion{
					{ID: "e1", BBox: placeholderBBox, Type: model.RegionTypeEntry, OriginalText: "نص عربي", TranslatedText: "Arabic text"},
				},
				ReadingOrder: []string{"e1"},
			},
			contains: []string{"نص عربي"},
			excludes: []string{"Arabic text", "# ", "> "},
		},
		{
			name: "footnote uses OriginalText as blockquote",
			page: &model.TranslatedRegionPage{
				PageNumber: 1,
				Regions: []model.TranslatedRegion{
					{ID: "f1", BBox: placeholderBBox, Type: model.RegionTypeFootnote, OriginalText: "حاشية", TranslatedText: "Footnote"},
				},
				ReadingOrder: []string{"f1"},
			},
			contains: []string{"> حاشية"},
			excludes: []string{"Footnote"},
		},
		{
			name: "separator renders as horizontal rule",
			page: &model.TranslatedRegionPage{
				PageNumber: 1,
				Regions: []model.TranslatedRegion{
					{ID: "s1", BBox: placeholderBBox, Type: model.RegionTypeSeparator},
				},
				ReadingOrder: []string{"s1"},
			},
			contains: []string{"---\n"},
		},
	}

	r := &ArabicMarkdownRenderer{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.RenderPage(tt.page)
			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("expected result to contain %q, got:\n%s", want, result)
				}
			}
			for _, exclude := range tt.excludes {
				if strings.Contains(result, exclude) {
					t.Errorf("expected result NOT to contain %q, got:\n%s", exclude, result)
				}
			}
		})
	}
}

func TestArabicMarkdownRenderPage_NoHeader(t *testing.T) {
	r := &ArabicMarkdownRenderer{}

	page := &model.TranslatedRegionPage{
		PageNumber: 1,
		Regions: []model.TranslatedRegion{
			{ID: "e1", BBox: placeholderBBox, Type: model.RegionTypeEntry, OriginalText: "نص"},
		},
		ReadingOrder: []string{"e1"},
	}

	result := r.RenderPage(page)

	if strings.Contains(result, "# ") {
		t.Error("expected no header line when no header region exists")
	}
	if !strings.Contains(result, "نص") {
		t.Error("expected Arabic entry text")
	}
}

func TestArabicMarkdownRenderPage_EmptyHeaderOriginalText(t *testing.T) {
	r := &ArabicMarkdownRenderer{}

	page := &model.TranslatedRegionPage{
		PageNumber: 1,
		Regions: []model.TranslatedRegion{
			{ID: "h1", BBox: placeholderBBox, Type: model.RegionTypeHeader, OriginalText: ""},
		},
		ReadingOrder: []string{"h1"},
	}

	result := r.RenderPage(page)

	if strings.Contains(result, "# ") {
		t.Error("expected no header line when OriginalText is empty")
	}
}

func TestArabicMarkdownRenderPage_WithFootnotes(t *testing.T) {
	r := &ArabicMarkdownRenderer{}

	page := &model.TranslatedRegionPage{
		PageNumber: 1,
		Regions: []model.TranslatedRegion{
			{ID: "f1", BBox: placeholderBBox, Type: model.RegionTypeFootnote, OriginalText: "حاشية أولى"},
			{ID: "f2", BBox: placeholderBBox, Type: model.RegionTypeFootnote, OriginalText: "حاشية ثانية"},
		},
		ReadingOrder: []string{"f1", "f2"},
	}

	result := r.RenderPage(page)

	if !strings.Contains(result, "> حاشية أولى") {
		t.Error("expected first footnote as blockquote")
	}
	if !strings.Contains(result, "> حاشية ثانية") {
		t.Error("expected second footnote as blockquote")
	}
}

func TestArabicMarkdownRenderPage_EmptyPage(t *testing.T) {
	r := &ArabicMarkdownRenderer{}

	page := &model.TranslatedRegionPage{
		PageNumber:   1,
		ReadingOrder: nil,
	}

	result := r.RenderPage(page)

	if result != "" {
		t.Errorf("expected empty result for page with no reading order, got %q", result)
	}
}

// --- ArabicMarkdownRenderer.RenderBook ---

func TestArabicMarkdownRenderBook_MultiplePages(t *testing.T) {
	r := &ArabicMarkdownRenderer{}

	pages := []*model.TranslatedRegionPage{
		{
			PageNumber: 10,
			Regions: []model.TranslatedRegion{
				{ID: "h1", BBox: placeholderBBox, Type: model.RegionTypeHeader, OriginalText: "باب"},
				{ID: "e1", BBox: placeholderBBox, Type: model.RegionTypeEntry, OriginalText: "أول"},
			},
			ReadingOrder: []string{"h1", "e1"},
		},
		{
			PageNumber: 11,
			Regions: []model.TranslatedRegion{
				{ID: "e1", BBox: placeholderBBox, Type: model.RegionTypeEntry, OriginalText: "ثاني"},
			},
			ReadingOrder: []string{"e1"},
		},
	}

	result := r.RenderBook(pages)

	if !strings.Contains(result, "<!-- Page 10 -->") {
		t.Error("expected Page 10 comment")
	}
	if !strings.Contains(result, "<!-- Page 11 -->") {
		t.Error("expected Page 11 comment")
	}
	if !strings.Contains(result, "\n---\n") {
		t.Error("expected page separator")
	}
	if !strings.Contains(result, "# باب") {
		t.Error("expected Arabic header")
	}
}

func TestArabicMarkdownRenderBook_EmptySlice(t *testing.T) {
	r := &ArabicMarkdownRenderer{}
	result := r.RenderBook(nil)
	if result != "" {
		t.Errorf("expected empty result for nil pages, got %q", result)
	}
}

func TestArabicMarkdownRenderBook_SinglePage(t *testing.T) {
	r := &ArabicMarkdownRenderer{}

	pages := []*model.TranslatedRegionPage{
		{
			PageNumber: 3,
			Regions: []model.TranslatedRegion{
				{ID: "e1", BBox: placeholderBBox, Type: model.RegionTypeEntry, OriginalText: "وحيد"},
			},
			ReadingOrder: []string{"e1"},
		},
	}

	result := r.RenderBook(pages)

	if strings.Contains(result, "\n---\n") {
		t.Error("expected no separator for single page")
	}
	if !strings.Contains(result, "<!-- Page 3 -->") {
		t.Error("expected Page 3 comment")
	}
	if !strings.Contains(result, "وحيد") {
		t.Error("expected Arabic text in single page")
	}
}
