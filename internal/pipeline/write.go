package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mmdemirbas/mutercim/internal/config"
	"github.com/mmdemirbas/mutercim/internal/display"
	"github.com/mmdemirbas/mutercim/internal/docker"
	"github.com/mmdemirbas/mutercim/internal/model"
	"github.com/mmdemirbas/mutercim/internal/rebuild"
	"github.com/mmdemirbas/mutercim/internal/renderer"
	"github.com/mmdemirbas/mutercim/internal/workspace"
)

// WriteOptions configures the compilation pipeline.
type WriteOptions struct {
	Workspace *workspace.Workspace
	Config    *config.Config
	Pages     []int
	Force     bool // force regeneration of all output files
	Logger    *slog.Logger
	Display   display.Display
}

// Write runs the Phase 4 compilation pipeline for all inputs and target languages.
func Write(ctx context.Context, opts WriteOptions) error {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	ws := opts.Workspace
	cfg := opts.Config

	if len(cfg.Translate.Languages) == 0 {
		return fmt.Errorf("no target languages configured")
	}

	// Render per target language
	for _, targetLang := range cfg.Translate.Languages {
		langDir := filepath.Join(ws.TranslateDir(), targetLang)
		inputs, err := discoverSubdirs(langDir)
		if err != nil {
			logger.Warn("no translated output for language", "lang", targetLang, "error", err)
			continue
		}
		if len(inputs) == 0 {
			logger.Warn("no translated pages for language", "lang", targetLang)
			continue
		}

		for _, stem := range inputs {
			logger.Info("compiling input", "input", stem, "target", targetLang)
			if err := writeOneInput(ctx, opts, stem, targetLang); err != nil {
				logger.Error("compilation failed", "input", stem, "target", targetLang, "error", err)
			}
		}
	}

	return nil
}

func writeOneInput(ctx context.Context, opts WriteOptions, stem, targetLang string) error {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	ws := opts.Workspace
	cfg := opts.Config
	translatedDir := filepath.Join(ws.TranslateDir(), targetLang, stem)

	// Skip if ALL expected outputs are up-to-date
	title := "book"
	langWriteDir := filepath.Join(ws.WriteDir(), targetLang)
	if !opts.Force {
		allUpToDate := true
		for _, format := range cfg.Write.Formats {
			var ext string
			switch format {
			case "md":
				ext = ".md"
			case "latex":
				ext = ".tex"
			case "pdf":
				ext = ".pdf"
			case "docx":
				ext = ".docx"
			default:
				continue
			}
			outputPath := filepath.Join(langWriteDir, title+ext)
			rebuildInputs := append([]string{translatedDir, ws.ConfigPath()}, cfg.ResolveKnowledgePaths(ws.Root)...)
			if rebuild.NeedsRebuild(outputPath, rebuildInputs...) {
				allUpToDate = false
				break
			}
		}
		if allUpToDate {
			logger.Debug("skipping write (up-to-date)", "input", stem, "lang", targetLang)
			return nil
		}
	}

	// Load translated pages
	pageFiles, err := listPageFiles(translatedDir)
	if err != nil {
		return fmt.Errorf("list translated pages: %w", err)
	}
	if len(pageFiles) == 0 {
		return fmt.Errorf("no translated pages in %s", translatedDir)
	}

	if len(opts.Pages) > 0 {
		pageFiles = filterPages(pageFiles, opts.Pages)
	}

	var pages []*model.TranslatedRegionPage
	for _, pf := range pageFiles {
		page, err := loadTranslatedRegionPageForWrite(pf.path)
		if err != nil {
			logger.Error("failed to load translated page", "page", pf.pageNum, "error", err)
			continue
		}
		pages = append(pages, page)
	}

	if len(pages) == 0 {
		return fmt.Errorf("no valid translated pages for %s", stem)
	}

	sort.Slice(pages, func(i, j int) bool {
		return pages[i].PageNumber < pages[j].PageNumber
	})

	// Start progress display
	if opts.Display != nil {
		opts.Display.StartPhase(display.PhaseWrite, stem, len(pages), targetLang)
	}

	// Render each requested format independently — partial success is OK
	var succeeded, failed []string
	for _, format := range cfg.Write.Formats {
		if opts.Display != nil {
			opts.Display.SetStatus(display.StatusLine{
				Text:      fmt.Sprintf("writing %s [%s]", format, targetLang),
				StartedAt: time.Now(),
			})
		}

		var err error
		switch format {
		case "md":
			err = compileMarkdown(ws, cfg, stem, targetLang, pages, logger)
		case "latex":
			err = compileLatex(ctx, ws, cfg, stem, targetLang, pages, false, logger)
		case "pdf":
			if opts.Display != nil {
				opts.Display.SetStatus(display.StatusLine{
					Text:      fmt.Sprintf("compiling PDF via Docker [%s]", targetLang),
					StartedAt: time.Now(),
				})
			}
			err = compileLatex(ctx, ws, cfg, stem, targetLang, pages, true, logger)
		case "docx":
			err = compileDocx(ctx, ws, cfg, stem, targetLang, logger)
		default:
			logger.Warn("unknown format", "format", format)
			continue
		}

		if err != nil {
			logger.Warn("format failed", "format", format, "input", stem, "error", err)
			failed = append(failed, fmt.Sprintf("%s (%v)", format, err))
		} else {
			succeeded = append(succeeded, format)
		}
	}

	if opts.Display != nil {
		opts.Display.SetStatus(display.StatusLine{}) // clear write status
	}

	if opts.Display != nil {
		writeCompleted := len(succeeded)
		writeFailed := len(failed)
		writeTotal := len(succeeded) + len(failed)
		var writeErr error
		if writeFailed > 0 {
			writeErr = fmt.Errorf("%d format(s) failed: %s", writeFailed, strings.Join(failed, "; "))
		}
		opts.Display.Update(display.PageResult{
			Phase: display.PhaseWrite, Input: stem,
			Total: writeTotal, Completed: writeCompleted, Failed: writeFailed,
			Lang: targetLang, Err: writeErr,
		})
		opts.Display.FinishPhase(display.PhaseWrite, stem, targetLang)
	}

	if len(succeeded) > 0 {
		logger.Info("write complete", "input", stem, "formats_succeeded", succeeded, "formats_failed", failed)
	}

	// Write report (langWriteDir already defined above)
	report := map[string]any{
		"target_lang":       targetLang,
		"pages":             len(pages),
		"formats_succeeded": succeeded,
		"formats_failed":    failed,
	}
	if data, err := json.MarshalIndent(report, "", "  "); err == nil {
		if err := atomicWriteFile(filepath.Join(langWriteDir, "report.json"), data); err != nil {
			logger.Warn("failed to write report", "error", err)
		}
	}

	// Only return error if ALL formats failed
	if len(succeeded) == 0 && len(failed) > 0 {
		return fmt.Errorf("all formats failed for %s [%s]: %s", stem, targetLang, strings.Join(failed, "; "))
	}
	return nil
}

func compileMarkdown(ws *workspace.Workspace, cfg *config.Config, stem, targetLang string, pages []*model.TranslatedRegionPage, logger *slog.Logger) error {
	targetDir := filepath.Join(ws.WriteDir(), targetLang)
	sourceDir := filepath.Join(ws.WriteDir(), cfg.PrimarySourceLang())

	if err := os.MkdirAll(targetDir, 0750); err != nil {
		return fmt.Errorf("create target output dir: %w", err)
	}
	if err := os.MkdirAll(sourceDir, 0750); err != nil {
		return fmt.Errorf("create source output dir: %w", err)
	}

	// Target language book
	mdRenderer := &renderer.MarkdownRenderer{}
	targetBook := mdRenderer.RenderBook(pages)
	title := "book"
	targetPath := filepath.Join(targetDir, title+".md")
	if err := atomicWrite(targetPath, []byte(targetBook)); err != nil {
		return fmt.Errorf("write target markdown: %w", err)
	}
	logger.Info("wrote target markdown", "path", targetPath)

	// Source language book
	arRenderer := &renderer.ArabicMarkdownRenderer{}
	sourceBook := arRenderer.RenderBook(pages)
	sourcePath := filepath.Join(sourceDir, title+".md")
	if err := atomicWrite(sourcePath, []byte(sourceBook)); err != nil {
		return fmt.Errorf("write source markdown: %w", err)
	}
	logger.Info("wrote source markdown", "path", sourcePath)

	return nil
}

func compileLatex(ctx context.Context, ws *workspace.Workspace, cfg *config.Config, stem, targetLang string, pages []*model.TranslatedRegionPage, compilePDF bool, logger *slog.Logger) error {
	title := "book"
	langDir := filepath.Join(ws.WriteDir(), targetLang)
	buildDir := filepath.Join(langDir, "latex-build", stem)
	if err := os.MkdirAll(buildDir, 0750); err != nil {
		return fmt.Errorf("create latex build dir: %w", err)
	}

	texRenderer := &renderer.LaTeXRenderer{Lang: targetLang}
	texContent := texRenderer.RenderBook(pages)

	// Write .tex to build directory for compilation
	buildTexPath := filepath.Join(buildDir, "book.tex")
	if err := atomicWrite(buildTexPath, []byte(texContent)); err != nil {
		return fmt.Errorf("write latex: %w", err)
	}

	// Copy .tex to language root with title-based name
	finalTexPath := filepath.Join(langDir, title+".tex")
	if err := atomicWrite(finalTexPath, []byte(texContent)); err != nil {
		return fmt.Errorf("write latex to lang dir: %w", err)
	}
	logger.Info("wrote latex", "path", finalTexPath)

	if compilePDF {
		const latexDockerImage = "mutercim/xelatex:latest"
		logger.Info("compiling PDF via Docker", "image", latexDockerImage)
		if err := renderer.CompilePDF(ctx, buildDir, latexDockerImage, docker.FindDockerDir("xelatex")); err != nil {
			return fmt.Errorf("compile PDF: %w", err)
		}

		// Copy PDF from build dir to language root with title-based name
		buildPDFPath := filepath.Join(buildDir, "book.pdf")
		finalPDFPath := filepath.Join(langDir, title+".pdf")
		pdfData, err := os.ReadFile(buildPDFPath)
		if err != nil {
			return fmt.Errorf("read compiled PDF: %w", err)
		}
		if err := atomicWrite(finalPDFPath, pdfData); err != nil {
			return fmt.Errorf("copy PDF to lang dir: %w", err)
		}
		logger.Info("wrote PDF", "path", finalPDFPath)
	}

	return nil
}

func compileDocx(ctx context.Context, ws *workspace.Workspace, cfg *config.Config, stem, targetLang string, logger *slog.Logger) error {
	title := "book"
	mdPath := filepath.Join(ws.WriteDir(), targetLang, title+".md")
	docxPath := filepath.Join(ws.WriteDir(), targetLang, title+".docx")

	if _, err := os.Stat(mdPath); err != nil {
		return fmt.Errorf("markdown file not found at %s (compile md format first): %w", mdPath, err)
	}

	if err := renderer.ConvertMarkdownToDocx(ctx, mdPath, docxPath, docker.FindDockerDir("pandoc")); err != nil {
		return fmt.Errorf("convert to docx: %w", err)
	}
	logger.Info("wrote docx", "path", docxPath)
	return nil
}

func loadTranslatedRegionPageForWrite(path string) (*model.TranslatedRegionPage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var page model.TranslatedRegionPage
	if err := json.Unmarshal(data, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

func atomicWrite(path string, data []byte) error {
	return atomicWriteFile(path, data)
}
