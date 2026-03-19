package renderer

import (
	"strings"
	"testing"

	"github.com/mmdemirbas/mutercim/internal/model"
)

func TestLatexRenderPage(t *testing.T) {
	r := &LaTeXRenderer{}

	page := &model.TranslatedPage{
		SolvedPage: model.SolvedPage{
			ReadPage: model.ReadPage{PageNumber: 5},
		},
		TranslatedHeader: &model.TranslatedHeader{Text: "Bab"},
		TranslatedEntries: []model.TranslatedEntry{
			{Number: 10, TranslatedText: "Test text"},
		},
	}

	result := r.RenderPage(page)

	if !strings.Contains(result, `\section*{Bab}`) {
		t.Error("expected section header")
	}
	if !strings.Contains(result, `\textbf{10.}`) {
		t.Error("expected bold entry number")
	}
	if !strings.Contains(result, `\newpage`) {
		t.Error("expected newpage")
	}
}

func TestLatexRenderBook(t *testing.T) {
	r := &LaTeXRenderer{}

	pages := []*model.TranslatedPage{
		{
			SolvedPage: model.SolvedPage{
				ReadPage: model.ReadPage{PageNumber: 1},
			},
			TranslatedEntries: []model.TranslatedEntry{{Number: 1, TranslatedText: "test"}},
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
	r := &LaTeXRenderer{}

	page := &model.TranslatedPage{
		SolvedPage: model.SolvedPage{
			ReadPage: model.ReadPage{PageNumber: 1},
		},
		TranslatedEntries: []model.TranslatedEntry{
			{Number: 1, TranslatedText: "Test"},
		},
	}

	result := r.RenderPage(page)

	if strings.Contains(result, `\section*`) {
		t.Error("expected no section header when TranslatedHeader is nil")
	}
	if !strings.Contains(result, `\textbf{1.}`) {
		t.Error("expected entry text")
	}
}

func TestLatexRenderPage_EmptyHeaderText(t *testing.T) {
	r := &LaTeXRenderer{}

	page := &model.TranslatedPage{
		SolvedPage: model.SolvedPage{
			ReadPage: model.ReadPage{PageNumber: 1},
		},
		TranslatedHeader: &model.TranslatedHeader{Text: ""},
	}

	result := r.RenderPage(page)

	if strings.Contains(result, `\section*`) {
		t.Error("expected no section header when header text is empty")
	}
}

func TestLatexRenderPage_EntryWithoutNumber(t *testing.T) {
	r := &LaTeXRenderer{}

	page := &model.TranslatedPage{
		SolvedPage: model.SolvedPage{
			ReadPage: model.ReadPage{PageNumber: 1},
		},
		TranslatedEntries: []model.TranslatedEntry{
			{Number: 0, TranslatedText: "Unnumbered"},
		},
	}

	result := r.RenderPage(page)

	if strings.Contains(result, `\textbf`) {
		t.Error("expected no textbf for entry with number 0")
	}
	if !strings.Contains(result, "Unnumbered") {
		t.Error("expected entry text")
	}
}

func TestLatexRenderPage_TranslatorNotes(t *testing.T) {
	r := &LaTeXRenderer{}

	page := &model.TranslatedPage{
		SolvedPage: model.SolvedPage{
			ReadPage: model.ReadPage{PageNumber: 1},
		},
		TranslatedEntries: []model.TranslatedEntry{
			{Number: 1, TranslatedText: "Text", TranslatorNotes: "A note with $pecial chars"},
		},
	}

	result := r.RenderPage(page)

	if !strings.Contains(result, `\emph{[Not:`) {
		t.Error("expected emph translator note")
	}
	if !strings.Contains(result, `\$pecial`) {
		t.Error("expected escaped special char in note")
	}
}

func TestLatexRenderPage_Footnotes(t *testing.T) {
	r := &LaTeXRenderer{}

	page := &model.TranslatedPage{
		SolvedPage: model.SolvedPage{
			ReadPage: model.ReadPage{PageNumber: 1},
		},
		TranslatedFootnotes: []model.TranslatedFootnote{
			{EntryNumber: 5, TranslatedText: "Footnote with number"},
			{EntryNumber: 0, TranslatedText: "Footnote without number"},
		},
	}

	result := r.RenderPage(page)

	if !strings.Contains(result, `\begin{small}`) {
		t.Error("expected small block for footnotes")
	}
	if !strings.Contains(result, `\end{small}`) {
		t.Error("expected end small block")
	}
	if !strings.Contains(result, `\hrule`) {
		t.Error("expected hrule separator")
	}
	if !strings.Contains(result, "[5] Footnote with number") {
		t.Error("expected numbered footnote")
	}
	if strings.Contains(result, "[0]") {
		t.Error("expected no numbered marker for footnote with entry number 0")
	}
}

func TestLatexRenderPage_EmptyPage(t *testing.T) {
	r := &LaTeXRenderer{}

	page := &model.TranslatedPage{
		SolvedPage: model.SolvedPage{
			ReadPage: model.ReadPage{PageNumber: 7},
		},
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
	r := &LaTeXRenderer{}

	page := &model.TranslatedPage{
		SolvedPage: model.SolvedPage{
			ReadPage: model.ReadPage{PageNumber: 1},
		},
		TranslatedHeader: &model.TranslatedHeader{Text: "Chapter #1 & $2"},
		TranslatedEntries: []model.TranslatedEntry{
			{Number: 1, TranslatedText: "100% of the _data_ in {braces}"},
		},
	}

	result := r.RenderPage(page)

	if !strings.Contains(result, `\section*{Chapter \#1 \& \$2}`) {
		t.Errorf("expected escaped header, got relevant portion: %s", result)
	}
	if !strings.Contains(result, `100\%`) {
		t.Error("expected escaped percent in entry")
	}
	if !strings.Contains(result, `\_data\_`) {
		t.Error("expected escaped underscores in entry")
	}
}

func TestLatexRenderBook_MultiplePages(t *testing.T) {
	r := &LaTeXRenderer{}

	pages := []*model.TranslatedPage{
		{
			SolvedPage: model.SolvedPage{
				ReadPage: model.ReadPage{PageNumber: 1},
			},
			TranslatedHeader: &model.TranslatedHeader{Text: "First"},
			TranslatedEntries: []model.TranslatedEntry{
				{Number: 1, TranslatedText: "Page one"},
			},
		},
		{
			SolvedPage: model.SolvedPage{
				ReadPage: model.ReadPage{PageNumber: 2},
			},
			TranslatedEntries: []model.TranslatedEntry{
				{Number: 2, TranslatedText: "Page two"},
			},
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
	// Both pages should have \newpage
	if strings.Count(result, `\newpage`) != 2 {
		t.Errorf("expected 2 newpage commands, got %d", strings.Count(result, `\newpage`))
	}
}

func TestLatexRenderBook_EmptySlice(t *testing.T) {
	r := &LaTeXRenderer{}

	result := r.RenderBook(nil)

	if !strings.Contains(result, `\begin{document}`) {
		t.Error("expected document begin even with no pages")
	}
	if !strings.Contains(result, `\end{document}`) {
		t.Error("expected document end even with no pages")
	}
}
