package renderer

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/mmdemirbas/mutercim/internal/model"
)

// LaTeXRenderer renders translated pages as LaTeX with proper RTL support.
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

// RenderPage renders a single translated page as LaTeX.
// Arabic content (headers, original text) is wrapped in \textarabic{} for proper RTL rendering.
func (r *LaTeXRenderer) RenderPage(page *model.TranslatedPage) string {
	var b strings.Builder

	fmt.Fprintf(&b, "%% Page %d\n", page.PageNumber)

	// Header — use original Arabic header if available, otherwise translated
	if page.TranslatedHeader != nil && page.TranslatedHeader.Text != "" {
		headerText := latexEscape(page.TranslatedHeader.Text)
		if page.Header != nil && page.Header.Text != "" && !isArabicLang(r.Lang) {
			// For non-Arabic output, show Arabic original header in \textarabic
			fmt.Fprintf(&b, "\\section*{%s}\n", headerText)
			fmt.Fprintf(&b, "\\begin{center}\\textarabic{%s}\\end{center}\n\n", latexEscape(page.Header.Text))
		} else if isArabicLang(r.Lang) && page.Header != nil && page.Header.Text != "" {
			// For Arabic output, header is already Arabic
			fmt.Fprintf(&b, "\\section*{%s}\n\n", latexEscape(page.Header.Text))
		} else {
			fmt.Fprintf(&b, "\\section*{%s}\n\n", headerText)
		}
	}

	// Entries
	for _, e := range page.TranslatedEntries {
		if isArabicLang(r.Lang) {
			// Arabic-primary output: entries are Arabic text
			if e.Number > 0 {
				fmt.Fprintf(&b, "\\textbf{%d.} %s\n\n", e.Number, latexEscape(e.TranslatedText))
			} else {
				fmt.Fprintf(&b, "%s\n\n", latexEscape(e.TranslatedText))
			}
		} else {
			// Non-Arabic output: translated text + original Arabic
			if e.Number > 0 {
				fmt.Fprintf(&b, "\\textbf{%d.} %s\n\n", e.Number, latexEscape(e.TranslatedText))
			} else {
				fmt.Fprintf(&b, "%s\n\n", latexEscape(e.TranslatedText))
			}
			// Include original Arabic text if available from the read page
			if idx := e.Number - 1; idx >= 0 && idx < len(page.Entries) && page.Entries[idx].ArabicText != "" {
				fmt.Fprintf(&b, "\\textarabic{%s}\n\n", latexEscape(page.Entries[idx].ArabicText))
			}
		}
		if e.TranslatorNotes != "" {
			fmt.Fprintf(&b, "\\emph{[Not: %s]}\n\n", latexEscape(e.TranslatorNotes))
		}
	}

	// Footnotes
	if len(page.TranslatedFootnotes) > 0 {
		b.WriteString("\\begin{small}\n\\hrule\\vspace{0.5em}\n")
		for _, fn := range page.TranslatedFootnotes {
			if len(fn.EntryNumbers) > 0 {
				fmt.Fprintf(&b, "[%s] %s\n\n", formatEntryNums(fn.EntryNumbers), latexEscape(fn.TranslatedText))
			} else {
				fmt.Fprintf(&b, "%s\n\n", latexEscape(fn.TranslatedText))
			}
		}
		b.WriteString("\\end{small}\n")
	}

	b.WriteString("\\newpage\n")
	return b.String()
}

// RenderBook renders all translated pages as a complete LaTeX document.
func (r *LaTeXRenderer) RenderBook(pages []*model.TranslatedPage) string {
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
func CompilePDF(ctx context.Context, latexDir, dockerImage string) error {
	cmd := exec.CommandContext(ctx, "docker", "run", "--rm",
		"-v", latexDir+":/data",
		dockerImage,
		"book.tex",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("PDF compilation failed (docker image: %s):\n  %w\n  %s", dockerImage, err, truncateOutput(string(output), 500))
	}
	return nil
}

func truncateOutput(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[len(s)-maxLen:] + "\n  ... (truncated, see book.log for full output)"
}

// CheckDocker returns an error if docker is not available.
func CheckDocker() error {
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("docker not found in PATH (required for pdf output format)")
	}
	return nil
}
