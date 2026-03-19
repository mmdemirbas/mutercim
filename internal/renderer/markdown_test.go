package renderer

import (
	"strings"
	"testing"

	"github.com/mmdemirbas/mutercim/internal/model"
)

func intPtr(n int) *int { return &n }

func TestMarkdownRenderPage(t *testing.T) {
	r := &MarkdownRenderer{}

	page := &model.TranslatedPage{
		SolvedPage: model.SolvedPage{
			ReadPage: model.ReadPage{
				PageNumber: 1,
			},
		},
		TranslatedHeader: &model.TranslatedHeader{Text: "Bab Başlığı"},
		TranslatedEntries: []model.TranslatedEntry{
			{Number: 42, TranslatedText: "Bu bir hadîstir.", TranslatorNotes: "Zor bir metin"},
			{Number: 43, TranslatedText: "İkinci hadîs."},
		},
		TranslatedFootnotes: []model.TranslatedFootnote{
			{EntryNumbers: []int{42}, TranslatedText: "Sahîh-i Buhârî'de rivayet edilmiştir."},
		},
	}

	result := r.RenderPage(page)

	for _, want := range []string{
		"# Bab Başlığı",
		"**42.** Bu bir hadîstir.",
		"_[Not: Zor bir metin]_",
		"**43.** İkinci hadîs.",
		"[42] Sahîh-i Buhârî'de rivayet edilmiştir.",
	} {
		if !strings.Contains(result, want) {
			t.Errorf("expected result to contain %q", want)
		}
	}
}

func TestMarkdownRenderBook(t *testing.T) {
	r := &MarkdownRenderer{}

	pages := []*model.TranslatedPage{
		{
			SolvedPage: model.SolvedPage{
				ReadPage: model.ReadPage{PageNumber: 1},
			},
			TranslatedEntries: []model.TranslatedEntry{{Number: 1, TranslatedText: "Birinci"}},
		},
		{
			SolvedPage: model.SolvedPage{
				ReadPage: model.ReadPage{PageNumber: 2},
			},
			TranslatedEntries: []model.TranslatedEntry{{Number: 2, TranslatedText: "İkinci"}},
		},
	}

	result := r.RenderBook(pages)

	if !strings.Contains(result, "Page 1") || !strings.Contains(result, "Page 2") {
		t.Error("expected both page markers")
	}
	if !strings.Contains(result, "---") {
		t.Error("expected page separator")
	}
}

func TestMarkdownRenderPage_NoHeader(t *testing.T) {
	r := &MarkdownRenderer{}

	page := &model.TranslatedPage{
		SolvedPage: model.SolvedPage{
			ReadPage: model.ReadPage{PageNumber: 1},
		},
		TranslatedEntries: []model.TranslatedEntry{
			{Number: 1, TranslatedText: "Metin"},
		},
	}

	result := r.RenderPage(page)

	if strings.Contains(result, "# ") {
		t.Error("expected no header line when TranslatedHeader is nil")
	}
	if !strings.Contains(result, "**1.** Metin") {
		t.Error("expected entry text")
	}
}

func TestMarkdownRenderPage_EmptyHeaderText(t *testing.T) {
	r := &MarkdownRenderer{}

	page := &model.TranslatedPage{
		SolvedPage: model.SolvedPage{
			ReadPage: model.ReadPage{PageNumber: 1},
		},
		TranslatedHeader: &model.TranslatedHeader{Text: ""},
	}

	result := r.RenderPage(page)

	if strings.Contains(result, "# ") {
		t.Error("expected no header line when header text is empty")
	}
}

func TestMarkdownRenderPage_EntryWithoutNumber(t *testing.T) {
	r := &MarkdownRenderer{}

	page := &model.TranslatedPage{
		SolvedPage: model.SolvedPage{
			ReadPage: model.ReadPage{PageNumber: 1},
		},
		TranslatedEntries: []model.TranslatedEntry{
			{Number: 0, TranslatedText: "Numarasız metin"},
		},
	}

	result := r.RenderPage(page)

	if strings.Contains(result, "**0.**") {
		t.Error("expected no bold number for entry with number 0")
	}
	if !strings.Contains(result, "Numarasız metin") {
		t.Error("expected entry text without number prefix")
	}
}

func TestMarkdownRenderPage_FootnoteWithoutEntryNumbers(t *testing.T) {
	r := &MarkdownRenderer{}

	page := &model.TranslatedPage{
		SolvedPage: model.SolvedPage{
			ReadPage: model.ReadPage{PageNumber: 1},
		},
		TranslatedFootnotes: []model.TranslatedFootnote{
			{EntryNumbers: nil, TranslatedText: "Genel dipnot"},
		},
	}

	result := r.RenderPage(page)

	if !strings.Contains(result, "---") {
		t.Error("expected footnote separator")
	}
	if strings.Contains(result, "[0]") {
		t.Error("expected no numbered footnote marker for entry number 0")
	}
	if !strings.Contains(result, "Genel dipnot") {
		t.Error("expected footnote text")
	}
}

func TestMarkdownRenderPage_EmptyPage(t *testing.T) {
	r := &MarkdownRenderer{}

	page := &model.TranslatedPage{
		SolvedPage: model.SolvedPage{
			ReadPage: model.ReadPage{PageNumber: 1},
		},
	}

	result := r.RenderPage(page)

	if result != "" {
		t.Errorf("expected empty result for empty page, got %q", result)
	}
}

func TestMarkdownRenderBook_MultiplePages(t *testing.T) {
	r := &MarkdownRenderer{}

	pages := []*model.TranslatedPage{
		{
			SolvedPage: model.SolvedPage{
				ReadPage: model.ReadPage{PageNumber: 1},
			},
			TranslatedHeader: &model.TranslatedHeader{Text: "Birinci Bab"},
			TranslatedEntries: []model.TranslatedEntry{
				{Number: 1, TranslatedText: "Birinci hadis"},
			},
		},
		{
			SolvedPage: model.SolvedPage{
				ReadPage: model.ReadPage{PageNumber: 2},
			},
			TranslatedEntries: []model.TranslatedEntry{
				{Number: 2, TranslatedText: "İkinci hadis"},
			},
		},
		{
			SolvedPage: model.SolvedPage{
				ReadPage: model.ReadPage{PageNumber: 3},
			},
			TranslatedFootnotes: []model.TranslatedFootnote{
				{EntryNumbers: []int{3}, TranslatedText: "Dipnot"},
			},
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
	// Two page separators between three pages, plus one footnote separator in page 3.
	// Just verify at least 2 page separators exist (the footnote "---" also appears).
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

	pages := []*model.TranslatedPage{
		{
			SolvedPage: model.SolvedPage{
				ReadPage: model.ReadPage{PageNumber: 5},
			},
			TranslatedEntries: []model.TranslatedEntry{{Number: 1, TranslatedText: "Tek sayfa"}},
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

func TestArabicMarkdownRenderPage_NoHeader(t *testing.T) {
	r := &ArabicMarkdownRenderer{}

	page := &model.TranslatedPage{
		SolvedPage: model.SolvedPage{
			ReadPage: model.ReadPage{
				PageNumber: 1,
				Entries: []model.Entry{
					{Number: intPtr(1), ArabicText: "نص"},
				},
			},
		},
	}

	result := r.RenderPage(page)

	if strings.Contains(result, "# ") {
		t.Error("expected no header line when Header is nil")
	}
	if !strings.Contains(result, "نص") {
		t.Error("expected Arabic entry text")
	}
}

func TestArabicMarkdownRenderPage_EntryWithoutNumber(t *testing.T) {
	r := &ArabicMarkdownRenderer{}

	page := &model.TranslatedPage{
		SolvedPage: model.SolvedPage{
			ReadPage: model.ReadPage{
				PageNumber: 1,
				Entries: []model.Entry{
					{Number: nil, ArabicText: "نص بدون رقم"},
				},
			},
		},
	}

	result := r.RenderPage(page)

	if strings.Contains(result, "**") {
		t.Error("expected no bold number for entry with nil number")
	}
	if !strings.Contains(result, "نص بدون رقم") {
		t.Error("expected Arabic text without number")
	}
}

func TestArabicMarkdownRenderPage_WithFootnotes(t *testing.T) {
	r := &ArabicMarkdownRenderer{}

	page := &model.TranslatedPage{
		SolvedPage: model.SolvedPage{
			ReadPage: model.ReadPage{
				PageNumber: 1,
				Footnotes: []model.Footnote{
					{ArabicText: "حاشية أولى"},
					{ArabicText: "حاشية ثانية"},
				},
			},
		},
	}

	result := r.RenderPage(page)

	if !strings.Contains(result, "---") {
		t.Error("expected footnote separator")
	}
	if !strings.Contains(result, "حاشية أولى") {
		t.Error("expected first footnote")
	}
	if !strings.Contains(result, "حاشية ثانية") {
		t.Error("expected second footnote")
	}
}

func TestArabicMarkdownRenderBook_MultiplePages(t *testing.T) {
	r := &ArabicMarkdownRenderer{}

	pages := []*model.TranslatedPage{
		{
			SolvedPage: model.SolvedPage{
				ReadPage: model.ReadPage{
					PageNumber: 10,
					Header:     &model.Header{Text: "باب"},
					Entries:    []model.Entry{{Number: intPtr(1), ArabicText: "أول"}},
				},
			},
		},
		{
			SolvedPage: model.SolvedPage{
				ReadPage: model.ReadPage{
					PageNumber: 11,
					Entries:    []model.Entry{{Number: intPtr(2), ArabicText: "ثاني"}},
				},
			},
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
}

func TestArabicMarkdownRenderBook_EmptySlice(t *testing.T) {
	r := &ArabicMarkdownRenderer{}
	result := r.RenderBook(nil)
	if result != "" {
		t.Errorf("expected empty result for nil pages, got %q", result)
	}
}

func TestArabicMarkdownRenderPage(t *testing.T) {
	r := &ArabicMarkdownRenderer{}

	page := &model.TranslatedPage{
		SolvedPage: model.SolvedPage{
			ReadPage: model.ReadPage{
				PageNumber: 1,
				Header:     &model.Header{Text: "حرف الألف"},
				Entries: []model.Entry{
					{Number: intPtr(1), ArabicText: "أَبْشِرُوا"},
				},
			},
		},
	}

	result := r.RenderPage(page)

	if !strings.Contains(result, "حرف الألف") {
		t.Error("expected Arabic header")
	}
	if !strings.Contains(result, "أَبْشِرُوا") {
		t.Error("expected Arabic text")
	}
}
