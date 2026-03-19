package renderer

import (
	"fmt"
	"strings"

	"github.com/mmdemirbas/mutercim/internal/model"
)

// MarkdownRenderer renders translated pages as Markdown.
type MarkdownRenderer struct{}

// Extension returns ".md".
func (r *MarkdownRenderer) Extension() string { return ".md" }

// RenderPage renders a single translated page as Markdown.
func (r *MarkdownRenderer) RenderPage(page *model.TranslatedPage) string {
	var b strings.Builder

	// Header
	if page.TranslatedHeader != nil && page.TranslatedHeader.Text != "" {
		fmt.Fprintf(&b, "# %s\n\n", page.TranslatedHeader.Text)
	}

	// Entries
	for _, e := range page.TranslatedEntries {
		if e.Number > 0 {
			fmt.Fprintf(&b, "**%d.** %s\n\n", e.Number, e.TurkishText)
		} else {
			fmt.Fprintf(&b, "%s\n\n", e.TurkishText)
		}
		if e.TranslatorNotes != "" {
			fmt.Fprintf(&b, "_[Not: %s]_\n\n", e.TranslatorNotes)
		}
	}

	// Footnotes
	if len(page.TranslatedFootnotes) > 0 {
		b.WriteString("---\n\n")
		for _, fn := range page.TranslatedFootnotes {
			if fn.EntryNumber > 0 {
				fmt.Fprintf(&b, "[%d] %s\n\n", fn.EntryNumber, fn.TurkishText)
			} else {
				fmt.Fprintf(&b, "%s\n\n", fn.TurkishText)
			}
		}
	}

	return b.String()
}

// RenderBook renders all translated pages as a single Markdown document.
func (r *MarkdownRenderer) RenderBook(pages []*model.TranslatedPage) string {
	var b strings.Builder

	for i, page := range pages {
		if i > 0 {
			b.WriteString("\n---\n\n")
		}
		fmt.Fprintf(&b, "<!-- Page %d -->\n\n", page.PageNumber)
		b.WriteString(r.RenderPage(page))
	}

	return b.String()
}

// ArabicMarkdownRenderer renders the original Arabic text as Markdown.
type ArabicMarkdownRenderer struct{}

// Extension returns ".md".
func (r *ArabicMarkdownRenderer) Extension() string { return ".md" }

// RenderPage renders the Arabic text of a single page.
func (r *ArabicMarkdownRenderer) RenderPage(page *model.TranslatedPage) string {
	var b strings.Builder

	if page.Header != nil && page.Header.Text != "" {
		fmt.Fprintf(&b, "# %s\n\n", page.Header.Text)
	}

	for _, e := range page.Entries {
		if e.Number != nil {
			fmt.Fprintf(&b, "**%d.** %s\n\n", *e.Number, e.ArabicText)
		} else {
			fmt.Fprintf(&b, "%s\n\n", e.ArabicText)
		}
	}

	if len(page.Footnotes) > 0 {
		b.WriteString("---\n\n")
		for _, fn := range page.Footnotes {
			fmt.Fprintf(&b, "%s\n\n", fn.ArabicText)
		}
	}

	return b.String()
}

// RenderBook renders all pages' Arabic text as a single document.
func (r *ArabicMarkdownRenderer) RenderBook(pages []*model.TranslatedPage) string {
	var b strings.Builder

	for i, page := range pages {
		if i > 0 {
			b.WriteString("\n---\n\n")
		}
		fmt.Fprintf(&b, "<!-- Page %d -->\n\n", page.PageNumber)
		b.WriteString(r.RenderPage(page))
	}

	return b.String()
}
