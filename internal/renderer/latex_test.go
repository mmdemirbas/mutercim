package renderer

import (
	"strings"
	"testing"

	"github.com/mmdemirbas/mutercim/internal/model"
)

func TestLatexRenderPage(t *testing.T) {
	r := &LaTeXRenderer{Lang: "tr"}

	page := &model.TranslatedRegionPage{
		PageNumber: 5,
		Regions: []model.TranslatedRegion{
			{ID: "h1", BBox: model.BBox{0, 0, 100, 50}, OriginalText: "باب", TranslatedText: "Bab", Type: model.RegionTypeHeader},
			{ID: "e1", BBox: model.BBox{0, 0, 100, 50}, OriginalText: "نص عربي", TranslatedText: "Test text", Type: model.RegionTypeEntry},
		},
		ReadingOrder: []string{"h1", "e1"},
	}

	result := r.RenderPage(page)

	if !strings.Contains(result, `\section*{Bab}`) {
		t.Errorf("expected section header, got:\n%s", result)
	}
	if !strings.Contains(result, "Test text") {
		t.Error("expected entry text")
	}
	if !strings.Contains(result, `\newpage`) {
		t.Error("expected newpage")
	}
}

func TestLatexRenderBook(t *testing.T) {
	r := &LaTeXRenderer{Lang: "tr"}

	pages := []*model.TranslatedRegionPage{
		{
			PageNumber: 1,
			Regions: []model.TranslatedRegion{
				{ID: "e1", BBox: model.BBox{0, 0, 100, 50}, TranslatedText: "test", Type: model.RegionTypeEntry},
			},
			ReadingOrder: []string{"e1"},
		},
	}

	result := r.RenderBook(pages)

	if !strings.Contains(result, `\begin{document}`) {
		t.Error("expected document begin")
	}
	if !strings.Contains(result, `\end{document}`) {
		t.Error("expected document end")
	}
	if !strings.Contains(result, "polyglossia") {
		t.Error("expected polyglossia package")
	}
}

func TestLatexEscape(t *testing.T) {
	tests := []struct {
		input    string
		contains string
	}{
		{"100%", `100\%`},
		{"$5", `\$5`},
		{"a & b", `a \& b`},
		{"a_b", `a\_b`},
	}

	for _, tt := range tests {
		result := latexEscape(tt.input)
		if !strings.Contains(result, tt.contains) {
			t.Errorf("latexEscape(%q) = %q, expected to contain %q", tt.input, result, tt.contains)
		}
	}
}

func TestLatexEscapeAdditional(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"tilde", "a~b", `a\textasciitilde{}b`},
		{"caret", "a^b", `a\textasciicircum{}b`},
		{"backslash", `a\b`, `a\textbackslash{}b`},
		{"hash", "#tag", `\#tag`},
		{"empty", "", ""},
		{"no special", "plain text", "plain text"},
		{"multiple specials", "$100 & 50%", `\$100 \& 50\%`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := latexEscape(tt.input)
			if got != tt.want {
				t.Errorf("latexEscape(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestLatexRenderPage_NoHeader(t *testing.T) {
	r := &LaTeXRenderer{Lang: "tr"}

	page := &model.TranslatedRegionPage{
		PageNumber: 1,
		Regions: []model.TranslatedRegion{
			{ID: "e1", BBox: model.BBox{0, 0, 100, 50}, TranslatedText: "Test", Type: model.RegionTypeEntry},
		},
		ReadingOrder: []string{"e1"},
	}

	result := r.RenderPage(page)

	if strings.Contains(result, `\section*`) {
		t.Error("expected no section header when no header region exists")
	}
	if !strings.Contains(result, "Test") {
		t.Error("expected entry text")
	}
}

func TestLatexRenderPage_EmptyHeaderText(t *testing.T) {
	r := &LaTeXRenderer{Lang: "tr"}

	page := &model.TranslatedRegionPage{
		PageNumber: 1,
		Regions: []model.TranslatedRegion{
			{ID: "h1", BBox: model.BBox{0, 0, 100, 50}, TranslatedText: "", Type: model.RegionTypeHeader},
		},
		ReadingOrder: []string{"h1"},
	}

	result := r.RenderPage(page)

	if strings.Contains(result, `\section*`) {
		t.Error("expected no section header when header translated text is empty")
	}
}

func TestLatexRenderPage_EntryWithoutNumber(t *testing.T) {
	r := &LaTeXRenderer{Lang: "tr"}

	page := &model.TranslatedRegionPage{
		PageNumber: 1,
		Regions: []model.TranslatedRegion{
			{ID: "e1", BBox: model.BBox{0, 0, 100, 50}, TranslatedText: "Unnumbered plain text", Type: model.RegionTypeEntry},
		},
		ReadingOrder: []string{"e1"},
	}

	result := r.RenderPage(page)

	if !strings.Contains(result, "Unnumbered plain text") {
		t.Error("expected entry text")
	}
}

func TestLatexRenderPage_Footnotes(t *testing.T) {
	r := &LaTeXRenderer{Lang: "tr"}

	page := &model.TranslatedRegionPage{
		PageNumber: 1,
		Regions: []model.TranslatedRegion{
			{ID: "f1", BBox: model.BBox{0, 0, 100, 50}, TranslatedText: "Footnote text here", Type: model.RegionTypeFootnote},
		},
		ReadingOrder: []string{"f1"},
	}

	result := r.RenderPage(page)

	if !strings.Contains(result, `\begin{small}`) {
		t.Error("expected small block for footnotes")
	}
	if !strings.Contains(result, `\end{small}`) {
		t.Error("expected end small block")
	}
	if !strings.Contains(result, "Footnote text here") {
		t.Error("expected footnote text")
	}
}

func TestLatexRenderPage_Separator(t *testing.T) {
	r := &LaTeXRenderer{Lang: "tr"}

	page := &model.TranslatedRegionPage{
		PageNumber: 1,
		Regions: []model.TranslatedRegion{
			{ID: "e1", BBox: model.BBox{0, 0, 100, 50}, TranslatedText: "Before separator", Type: model.RegionTypeEntry},
			{ID: "s1", BBox: model.BBox{0, 0, 100, 50}, Type: model.RegionTypeSeparator},
			{ID: "e2", BBox: model.BBox{0, 0, 100, 50}, TranslatedText: "After separator", Type: model.RegionTypeEntry},
		},
		ReadingOrder: []string{"e1", "s1", "e2"},
	}

	result := r.RenderPage(page)

	if !strings.Contains(result, `\hrule\vspace{0.5em}`) {
		t.Error("expected hrule separator")
	}
}

func TestLatexRenderPage_EmptyPage(t *testing.T) {
	r := &LaTeXRenderer{Lang: "tr"}

	page := &model.TranslatedRegionPage{
		PageNumber:   7,
		Regions:      nil,
		ReadingOrder: nil,
	}

	result := r.RenderPage(page)

	if !strings.Contains(result, "% Page 7") {
		t.Error("expected page comment")
	}
	if !strings.Contains(result, `\newpage`) {
		t.Error("expected newpage even for empty page")
	}
}

func TestLatexRenderPage_SpecialCharsInContent(t *testing.T) {
	r := &LaTeXRenderer{Lang: "tr"}

	page := &model.TranslatedRegionPage{
		PageNumber: 1,
		Regions: []model.TranslatedRegion{
			{ID: "h1", BBox: model.BBox{0, 0, 100, 50}, OriginalText: "عنوان", TranslatedText: "Chapter #1 & $2", Type: model.RegionTypeHeader},
			{ID: "e1", BBox: model.BBox{0, 0, 100, 50}, TranslatedText: "100% of the _data_ in {braces}", Type: model.RegionTypeEntry},
		},
		ReadingOrder: []string{"h1", "e1"},
	}

	result := r.RenderPage(page)

	if !strings.Contains(result, `\section*{Chapter \#1 \& \$2}`) {
		t.Errorf("expected escaped header, got:\n%s", result)
	}
	if !strings.Contains(result, `100\%`) {
		t.Error("expected escaped percent in entry")
	}
	if !strings.Contains(result, `\_data\_`) {
		t.Error("expected escaped underscores in entry")
	}
}

func TestLatexRenderBook_MultiplePages(t *testing.T) {
	r := &LaTeXRenderer{Lang: "tr"}

	pages := []*model.TranslatedRegionPage{
		{
			PageNumber: 1,
			Regions: []model.TranslatedRegion{
				{ID: "h1", BBox: model.BBox{0, 0, 100, 50}, OriginalText: "عنوان", TranslatedText: "First", Type: model.RegionTypeHeader},
				{ID: "e1", BBox: model.BBox{0, 0, 100, 50}, TranslatedText: "Page one", Type: model.RegionTypeEntry},
			},
			ReadingOrder: []string{"h1", "e1"},
		},
		{
			PageNumber: 2,
			Regions: []model.TranslatedRegion{
				{ID: "e1", BBox: model.BBox{0, 0, 100, 50}, TranslatedText: "Page two", Type: model.RegionTypeEntry},
			},
			ReadingOrder: []string{"e1"},
		},
	}

	result := r.RenderBook(pages)

	if !strings.Contains(result, `\begin{document}`) {
		t.Error("expected document begin")
	}
	if !strings.Contains(result, `\end{document}`) {
		t.Error("expected document end")
	}
	if !strings.Contains(result, "% Page 1") {
		t.Error("expected page 1 comment")
	}
	if !strings.Contains(result, "% Page 2") {
		t.Error("expected page 2 comment")
	}
	if strings.Count(result, `\newpage`) != 2 {
		t.Errorf("expected 2 newpage commands, got %d", strings.Count(result, `\newpage`))
	}
}

func TestLatexRenderBook_EmptySlice(t *testing.T) {
	r := &LaTeXRenderer{Lang: "tr"}

	result := r.RenderBook(nil)

	if !strings.Contains(result, `\begin{document}`) {
		t.Error("expected document begin even with no pages")
	}
	if !strings.Contains(result, `\end{document}`) {
		t.Error("expected document end even with no pages")
	}
}

// --- Preamble tests ---

func TestLatexPreamble_TurkishMain(t *testing.T) {
	r := &LaTeXRenderer{Lang: "tr"}
	preamble := r.buildPreamble()

	if !strings.Contains(preamble, `\setmainlanguage{turkish}`) {
		t.Error("expected turkish as main language")
	}
	if !strings.Contains(preamble, `\setotherlanguage{arabic}`) {
		t.Error("expected arabic as other language")
	}
	if !strings.Contains(preamble, `\newfontfamily\arabicfont[Script=Arabic`) {
		t.Error("expected arabicfont definition")
	}
}

func TestLatexPreamble_ArabicMain(t *testing.T) {
	r := &LaTeXRenderer{Lang: "ar"}
	preamble := r.buildPreamble()

	if !strings.Contains(preamble, `\setmainlanguage[numerals=maghrib]{arabic}`) {
		t.Error("expected arabic as main language with maghrib numerals")
	}
	if !strings.Contains(preamble, `\setotherlanguage{turkish}`) {
		t.Error("expected turkish as other language")
	}
}

func TestLatexPreamble_EnglishMain(t *testing.T) {
	r := &LaTeXRenderer{Lang: "en"}
	preamble := r.buildPreamble()

	if !strings.Contains(preamble, `\setmainlanguage{english}`) {
		t.Error("expected english as main language")
	}
}

func TestLatexPreamble_EmptyLang(t *testing.T) {
	r := &LaTeXRenderer{Lang: ""}
	preamble := r.buildPreamble()

	if !strings.Contains(preamble, `\setmainlanguage{turkish}`) {
		t.Error("expected turkish as default main language")
	}
}

// --- Arabic text wrapping tests ---

func TestLatexRenderPage_ArabicTextWrapped(t *testing.T) {
	r := &LaTeXRenderer{Lang: "tr"}

	page := &model.TranslatedRegionPage{
		PageNumber: 1,
		Regions: []model.TranslatedRegion{
			{ID: "e1", BBox: model.BBox{0, 0, 100, 50}, OriginalText: "أَبْشِرُوا", TranslatedText: "Müjdelenin!", Type: model.RegionTypeEntry},
		},
		ReadingOrder: []string{"e1"},
	}

	result := r.RenderPage(page)

	if !strings.Contains(result, `\begin{Arabic}`) {
		t.Errorf("expected \\textarabic wrapper for Arabic content, got:\n%s", result)
	}
	if !strings.Contains(result, "Müjdelenin") {
		t.Error("expected Turkish translation text")
	}
}

func TestLatexRenderPage_ArabicTextWrappedHeader(t *testing.T) {
	r := &LaTeXRenderer{Lang: "tr"}

	page := &model.TranslatedRegionPage{
		PageNumber: 1,
		Regions: []model.TranslatedRegion{
			{ID: "h1", BBox: model.BBox{0, 0, 100, 50}, OriginalText: "حرف الألف", TranslatedText: "Elif Harfi", Type: model.RegionTypeHeader},
		},
		ReadingOrder: []string{"h1"},
	}

	result := r.RenderPage(page)

	if !strings.Contains(result, `\section*{Elif Harfi}`) {
		t.Error("expected translated section header")
	}
	if !strings.Contains(result, `\begin{Arabic}`) {
		t.Error("expected \\textarabic wrapper for Arabic original header")
	}
}

func TestLatexRenderPage_ArabicPrimary(t *testing.T) {
	r := &LaTeXRenderer{Lang: "ar"}

	page := &model.TranslatedRegionPage{
		PageNumber: 1,
		Regions: []model.TranslatedRegion{
			{ID: "h1", BBox: model.BBox{0, 0, 100, 50}, OriginalText: "حرف الألف", TranslatedText: "Elif Harfi", Type: model.RegionTypeHeader},
			{ID: "e1", BBox: model.BBox{0, 0, 100, 50}, OriginalText: "نص عربي", TranslatedText: "Arabic output text", Type: model.RegionTypeEntry},
		},
		ReadingOrder: []string{"h1", "e1"},
	}

	result := r.RenderPage(page)

	// For Arabic output, the header should use the original Arabic text
	if !strings.Contains(result, latexEscape("حرف الألف")) {
		t.Error("expected original Arabic header in Arabic-primary output")
	}
	// Should NOT wrap in \textarabic (already Arabic-primary)
	if strings.Contains(result, `\begin{Arabic}`) {
		t.Error("Arabic-primary output should not use \\textarabic wrapper")
	}
}

// --- Mixed content test ---

func TestLatexRenderPage_MixedContent(t *testing.T) {
	r := &LaTeXRenderer{Lang: "tr"}

	page := &model.TranslatedRegionPage{
		PageNumber: 1,
		Regions: []model.TranslatedRegion{
			{ID: "h1", BBox: model.BBox{0, 0, 100, 50}, OriginalText: "حرف الألف", TranslatedText: "Elif Harfi", Type: model.RegionTypeHeader},
			{ID: "e1", BBox: model.BBox{0, 0, 100, 50}, OriginalText: "نص عربي", TranslatedText: "Turkish translation 1", Type: model.RegionTypeEntry},
			{ID: "s1", BBox: model.BBox{0, 0, 100, 50}, Type: model.RegionTypeSeparator},
			{ID: "e2", BBox: model.BBox{0, 0, 100, 50}, OriginalText: "نص آخر", TranslatedText: "Turkish translation 2", Type: model.RegionTypeEntry},
			{ID: "f1", BBox: model.BBox{0, 0, 100, 50}, TranslatedText: "A footnote", Type: model.RegionTypeFootnote},
		},
		ReadingOrder: []string{"h1", "e1", "s1", "e2", "f1"},
	}

	result := r.RenderPage(page)

	// Turkish header
	if !strings.Contains(result, `\section*{Elif Harfi}`) {
		t.Error("expected Turkish section header")
	}
	// Arabic original header wrapped
	if !strings.Contains(result, `\begin{Arabic}`) {
		t.Error("expected \\textarabic for Arabic header")
	}
	// Both Turkish translations
	if !strings.Contains(result, "Turkish translation 1") {
		t.Error("expected first translation")
	}
	if !strings.Contains(result, "Turkish translation 2") {
		t.Error("expected second translation")
	}
	// Arabic originals wrapped (header + 2 entries = at least 3)
	if strings.Count(result, `\begin{Arabic}`) < 3 {
		t.Errorf("expected at least 3 \\textarabic wrappers (header + entries), got %d", strings.Count(result, `\begin{Arabic}`))
	}
	// Separator
	if !strings.Contains(result, `\hrule\vspace{0.5em}`) {
		t.Error("expected separator hrule")
	}
	// Footnote
	if !strings.Contains(result, `\begin{small}`) {
		t.Error("expected small block for footnote")
	}
	if !strings.Contains(result, "A footnote") {
		t.Error("expected footnote text")
	}
}

func TestLatexRenderPage_ReadingOrderSkipsMissingRegions(t *testing.T) {
	r := &LaTeXRenderer{Lang: "tr"}

	page := &model.TranslatedRegionPage{
		PageNumber: 1,
		Regions: []model.TranslatedRegion{
			{ID: "e1", BBox: model.BBox{0, 0, 100, 50}, TranslatedText: "Present", Type: model.RegionTypeEntry},
		},
		ReadingOrder: []string{"missing_id", "e1"},
	}

	result := r.RenderPage(page)

	if !strings.Contains(result, "Present") {
		t.Error("expected present entry text")
	}
	if !strings.Contains(result, `\newpage`) {
		t.Error("expected newpage")
	}
}

func TestLatexRenderPage_HeaderWithoutOriginalText(t *testing.T) {
	r := &LaTeXRenderer{Lang: "tr"}

	page := &model.TranslatedRegionPage{
		PageNumber: 1,
		Regions: []model.TranslatedRegion{
			{ID: "h1", BBox: model.BBox{0, 0, 100, 50}, TranslatedText: "Some Header", Type: model.RegionTypeHeader},
		},
		ReadingOrder: []string{"h1"},
	}

	result := r.RenderPage(page)

	if !strings.Contains(result, `\section*{Some Header}`) {
		t.Error("expected section header")
	}
	// Should not have textarabic when original text is empty
	if strings.Contains(result, `\begin{Arabic}`) {
		t.Error("expected no \\textarabic when original text is empty")
	}
}

func TestLatexRenderPage_EntryWithoutOriginalText(t *testing.T) {
	r := &LaTeXRenderer{Lang: "tr"}

	page := &model.TranslatedRegionPage{
		PageNumber: 1,
		Regions: []model.TranslatedRegion{
			{ID: "e1", BBox: model.BBox{0, 0, 100, 50}, TranslatedText: "Only translation", Type: model.RegionTypeEntry},
		},
		ReadingOrder: []string{"e1"},
	}

	result := r.RenderPage(page)

	if !strings.Contains(result, "Only translation") {
		t.Error("expected translation text")
	}
	if strings.Contains(result, `\begin{Arabic}`) {
		t.Error("expected no \\textarabic when original text is empty")
	}
}
