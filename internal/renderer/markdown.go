package renderer

import (
	"fmt"
	"strings"

	"github.com/mmdemirbas/mutercim/internal/model"
)

// MarkdownRenderer renders translated region pages as Markdown.
type MarkdownRenderer struct{}

// Extension returns ".md".
func (r *MarkdownRenderer) Extension() string { return ".md" }

// RenderPage renders a single translated region page as Markdown.
func (r *MarkdownRenderer) RenderPage(page *model.TranslatedRegionPage) string {
	var b strings.Builder

	for _, id := range page.ReadingOrder {
		region := findTranslatedRegion(page.Regions, id)
		if region == nil {
			continue
		}

		switch region.Type {
		case model.RegionTypeHeader:
			if region.TranslatedText != "" {
				fmt.Fprintf(&b, "# %s\n\n", region.TranslatedText)
			}
		case model.RegionTypeEntry:
			fmt.Fprintf(&b, "%s\n\n", region.TranslatedText)
		case model.RegionTypeFootnote:
			fmt.Fprintf(&b, "> %s\n\n", region.TranslatedText)
		case model.RegionTypeSeparator:
			b.WriteString("---\n\n")
		case model.RegionTypePageNumber:
			fmt.Fprintf(&b, "<!-- page %s -->\n\n", region.OriginalText)
		}
	}

	return b.String()
}

// RenderBook renders all translated pages as a single Markdown document.
func (r *MarkdownRenderer) RenderBook(pages []*model.TranslatedRegionPage) string {
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

// ArabicMarkdownRenderer renders the original Arabic text from regions.
type ArabicMarkdownRenderer struct{}

// Extension returns ".md".
func (r *ArabicMarkdownRenderer) Extension() string { return ".md" }

// RenderPage renders the original Arabic text of a single page.
func (r *ArabicMarkdownRenderer) RenderPage(page *model.TranslatedRegionPage) string {
	var b strings.Builder

	for _, id := range page.ReadingOrder {
		region := findTranslatedRegion(page.Regions, id)
		if region == nil {
			continue
		}

		switch region.Type {
		case model.RegionTypeHeader:
			if region.OriginalText != "" {
				fmt.Fprintf(&b, "# %s\n\n", region.OriginalText)
			}
		case model.RegionTypeEntry:
			fmt.Fprintf(&b, "%s\n\n", region.OriginalText)
		case model.RegionTypeFootnote:
			fmt.Fprintf(&b, "> %s\n\n", region.OriginalText)
		case model.RegionTypeSeparator:
			b.WriteString("---\n\n")
		}
	}

	return b.String()
}

// RenderBook renders all pages' original text as a single document.
func (r *ArabicMarkdownRenderer) RenderBook(pages []*model.TranslatedRegionPage) string {
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

// findTranslatedRegion finds a region by ID.
func findTranslatedRegion(regions []model.TranslatedRegion, id string) *model.TranslatedRegion {
	for i := range regions {
		if regions[i].ID == id {
			return &regions[i]
		}
	}
	return nil
}
