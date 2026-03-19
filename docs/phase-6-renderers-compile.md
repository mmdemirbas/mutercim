Read SPEC.md first. Follow its architecture, naming, and data structures exactly.
Do not deviate from the spec's package structure or interface definitions.

Read SPEC.md sections: "Phase 4: WRITE", LaTeX Docker container.

Implement:
1. internal/renderer/ — renderer.go, markdown.go, latex.go, docx.go
2. internal/pipeline/write.go
3. internal/cli/write.go
4. docker/xelatex/Dockerfile
5. templates in defaults/templates/

The result should: generate Markdown and LaTeX from translated JSON.

## Completion Checklist

Before declaring this phase complete, execute these commands and verify they pass:

1. `go build ./...`
2. `go vet ./...`
3. `go test ./...`
4. List all files you created/modified and verify each exists in SPEC.md's project structure
5. If any file or pattern deviates from SPEC.md, append to DEVIATIONS.md
6. Show me the output of all three commands above

## Summary

### Files Created

**Renderer package** (`internal/renderer/`):
- `renderer.go` — `Renderer` interface: `RenderPage()`, `RenderBook()`, `Extension()`
- `markdown.go` — `MarkdownRenderer` (Turkish) and `ArabicMarkdownRenderer` with section-aware formatting
- `latex.go` — `LaTeXRenderer` with preamble (polyglossia, bidi, Amiri font), `latexEscape()`, `CompilePDF()` via Docker, `CheckDocker()`
- `docx.go` — `ConvertMarkdownToDocx()` via pandoc, `CheckPandoc()`
- `markdown_test.go` — Turkish rendering, Arabic rendering, book aggregation
- `latex_test.go` — LaTeX rendering, book structure, escape function

**Pipeline** (`internal/pipeline/`):
- `write.go` — Phase 4 orchestrator: discovers inputs from translated dir, loads pages, renders per format (md/latex/docx), atomic writes, progress tracking, `CompilePDF()` via Docker

**CLI** (`internal/cli/`):
- `write.go` — `mutercim write` subcommand with `--format`, `--latex-docker-image`, `--skip-pdf` flags, preflight checks for docker/pandoc

**Docker** (`docker/xelatex/`):
- `Dockerfile` — XeLaTeX container with polyglossia, bidi, arabxetex, Amiri font

**Templates** (`defaults/templates/`):
- `book.tex` — Master LaTeX document template
- `page.tex` — Per-page LaTeX template fragment

### Output Structure

- `output/turkish/<stem>.md` — Combined Turkish markdown
- `output/arabic/<stem>.md` — Combined Arabic markdown
- `output/latex/book.tex` — LaTeX document (optionally compiled to PDF)
- `output/<stem>.docx` — DOCX via pandoc (if format includes docx)
