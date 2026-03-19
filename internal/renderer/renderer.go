package renderer

import "github.com/mmdemirbas/mutercim/internal/model"

// Renderer produces output in a specific format from translated pages.
type Renderer interface {
	// RenderPage renders a single translated page and returns the formatted content.
	RenderPage(page *model.TranslatedPage) string

	// RenderBook renders all translated pages into a single document.
	RenderBook(pages []*model.TranslatedPage) string

	// Extension returns the file extension for this format (e.g., ".md", ".tex").
	Extension() string
}
