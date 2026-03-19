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
			{Number: 10, TurkishText: "Test text"},
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
			TranslatedEntries: []model.TranslatedEntry{{Number: 1, TurkishText: "test"}},
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
