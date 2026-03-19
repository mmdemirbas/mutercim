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
			{EntryNumber: 42, TranslatedText: "Sahîh-i Buhârî'de rivayet edilmiştir."},
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
