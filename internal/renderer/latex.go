package renderer

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/mmdemirbas/mutercim/internal/model"
)

// LaTeXRenderer renders translated pages as LaTeX.
type LaTeXRenderer struct{}

// Extension returns ".tex".
func (r *LaTeXRenderer) Extension() string { return ".tex" }

// RenderPage renders a single translated page as LaTeX.
func (r *LaTeXRenderer) RenderPage(page *model.TranslatedPage) string {
	var b strings.Builder

	fmt.Fprintf(&b, "%% Page %d\n", page.PageNumber)

	// Header
	if page.TranslatedHeader != nil && page.TranslatedHeader.Text != "" {
		fmt.Fprintf(&b, "\\section*{%s}\n\n", latexEscape(page.TranslatedHeader.Text))
	}

	// Entries
	for _, e := range page.TranslatedEntries {
		if e.Number > 0 {
			fmt.Fprintf(&b, "\\textbf{%d.} %s\n\n", e.Number, latexEscape(e.TranslatedText))
		} else {
			fmt.Fprintf(&b, "%s\n\n", latexEscape(e.TranslatedText))
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

	b.WriteString(latexPreamble)

	for _, page := range pages {
		b.WriteString(r.RenderPage(page))
	}

	b.WriteString("\n\\end{document}\n")
	return b.String()
}

const latexPreamble = `\documentclass[12pt,a4paper]{article}
\usepackage{polyglossia}
\setmainlanguage{turkish}
\setotherlanguage{arabic}
\usepackage{fontspec}
\setmainfont{Amiri}
\usepackage{geometry}
\geometry{margin=2.5cm}
\usepackage{fancyhdr}
\usepackage{hyperref}

\begin{document}

`

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
		return fmt.Errorf("docker not found in PATH (required for LaTeX→PDF compilation, or use --skip-pdf)")
	}
	return nil
}
