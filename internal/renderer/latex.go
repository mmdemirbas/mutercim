package renderer

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mmdemirbas/mutercim/internal/docker"
	"github.com/mmdemirbas/mutercim/internal/model"
)

// LaTeXRenderer renders translated region pages as LaTeX with proper RTL support.
type LaTeXRenderer struct {
	// Lang is the output language code (e.g. "ar", "tr", "en").
	// Determines the main language in the preamble and text wrapping.
	Lang string
}

// Extension returns ".tex".
func (r *LaTeXRenderer) Extension() string { return ".tex" }

// isArabicLang returns true if the language code indicates Arabic as the primary language.
func isArabicLang(lang string) bool {
	return lang == "ar"
}

// RenderPage renders a single translated region page as LaTeX.
func (r *LaTeXRenderer) RenderPage(page *model.TranslatedRegionPage) string {
	var b strings.Builder

	fmt.Fprintf(&b, "%% Page %d\n", page.PageNumber)

	for _, id := range page.ReadingOrder {
		region := findTranslatedRegion(page.Regions, id)
		if region == nil {
			continue
		}

		switch region.Type {
		case model.RegionTypeHeader:
			if region.TranslatedText == "" {
				continue
			}
			if isArabicLang(r.Lang) {
				// Arabic output: use original text as header
				fmt.Fprintf(&b, "\\section*{%s}\n\n", latexEscape(region.OriginalText))
			} else {
				// Non-Arabic: translated header + Arabic original
				fmt.Fprintf(&b, "\\section*{%s}\n", latexEscape(region.TranslatedText))
				if region.OriginalText != "" {
					fmt.Fprintf(&b, "\\begin{center}\n\\begin{Arabic}\n%s\n\\end{Arabic}\n\\end{center}\n\n", latexEscape(region.OriginalText))
				} else {
					b.WriteString("\n")
				}
			}
		case model.RegionTypeEntry:
			b.WriteString(latexEscape(region.TranslatedText))
			b.WriteString("\n\n")
			// For non-Arabic output, include original Arabic
			if !isArabicLang(r.Lang) && region.OriginalText != "" {
				fmt.Fprintf(&b, "\\begin{Arabic}\n%s\n\\end{Arabic}\n\n", latexEscape(region.OriginalText))
			}
		case model.RegionTypeFootnote:
			fmt.Fprintf(&b, "\\begin{small}\n%s\n\\end{small}\n\n", latexEscape(region.TranslatedText))
		case model.RegionTypeSeparator:
			b.WriteString("\\hrule\\vspace{0.5em}\n")
		}
	}

	b.WriteString("\\newpage\n")
	return b.String()
}

// RenderBook renders all translated pages as a complete LaTeX document.
func (r *LaTeXRenderer) RenderBook(pages []*model.TranslatedRegionPage) string {
	var b strings.Builder

	b.WriteString(r.buildPreamble())

	for _, page := range pages {
		b.WriteString(r.RenderPage(page))
	}

	b.WriteString("\n\\end{document}\n")
	return b.String()
}

// buildPreamble generates the LaTeX preamble with proper language and font configuration.
func (r *LaTeXRenderer) buildPreamble() string {
	var b strings.Builder

	b.WriteString(`\documentclass[12pt,a4paper]{article}
\usepackage{polyglossia}
`)

	if isArabicLang(r.Lang) {
		// Arabic-primary document
		b.WriteString(`\setmainlanguage[numerals=maghrib]{arabic}
\setotherlanguage{turkish}
\setotherlanguage{english}
`)
	} else {
		// Non-Arabic document (Turkish, English, etc.)
		mainLang := r.Lang
		if mainLang == "" {
			mainLang = "turkish"
		}
		// Map common codes to polyglossia language names
		switch mainLang {
		case "tr":
			mainLang = "turkish"
		case "en":
			mainLang = "english"
		case "de":
			mainLang = "german"
		case "fr":
			mainLang = "french"
		}
		fmt.Fprintf(&b, "\\setmainlanguage{%s}\n", mainLang)
		b.WriteString("\\setotherlanguage{arabic}\n")
	}

	b.WriteString(`\usepackage{fontspec}
\newfontfamily\arabicfont[Script=Arabic,Scale=1.2]{Amiri}
\usepackage{geometry}
\geometry{margin=2.5cm}
\usepackage{fancyhdr}
\usepackage{hyperref}

\begin{document}

`)

	return b.String()
}

// latexEscape escapes special LaTeX characters.
func latexEscape(s string) string {
	replacer := strings.NewReplacer(
		`\`, `\textbackslash{}`,
		`{`, `\{`,
		`}`, `\}`,
		`&`, `\&`,
		`%`, `\%`,
		`$`, `\$`,
		`#`, `\#`,
		`_`, `\_`,
		`~`, `\textasciitilde{}`,
		`^`, `\textasciicircum{}`,
	)
	return replacer.Replace(s)
}

// CompilePDF compiles a LaTeX file to PDF using Docker.
// dockerfileDir is the path to docker/xelatex/ for auto-building the image.
func CompilePDF(ctx context.Context, latexDir, dockerImage, dockerfileDir string) error {
	if err := docker.EnsureImage(ctx, dockerImage, dockerfileDir); err != nil {
		return fmt.Errorf("ensure xelatex image: %w", err)
	}

	absDir, err := filepath.Abs(latexDir)
	if err != nil {
		return fmt.Errorf("resolve latex dir: %w", err)
	}

	output, err := docker.Run(ctx, "run", "--rm",
		"-v", absDir+":/data",
		dockerImage,
		"book.tex",
	)
	if err != nil {
		return fmt.Errorf("PDF compilation failed (docker image: %s):\n  %w\n  %s", dockerImage, err, truncateOutput(string(output), 500))
	}
	return nil
}

func truncateOutput(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[len(runes)-maxLen:]) + "\n  ... (truncated, see book.log for full output)"
}
