package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"

	"github.com/mmdemirbas/mutercim/internal/config"
	"github.com/mmdemirbas/mutercim/internal/model"
	"github.com/mmdemirbas/mutercim/internal/progress"
	"github.com/mmdemirbas/mutercim/internal/renderer"
	"github.com/mmdemirbas/mutercim/internal/workspace"
)

// WriteOptions configures the compilation pipeline.
type WriteOptions struct {
	Workspace *workspace.Workspace
	Config    *config.Config
	Tracker   *progress.Tracker
	Pages     []int
	Logger    *slog.Logger
}

// Write runs the Phase 4 compilation pipeline for all inputs and target languages.
func Write(ctx context.Context, opts WriteOptions) error {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	ws := opts.Workspace
	cfg := opts.Config

	// Write source language markdown (once, not per target)
	sourceInputs, err := discoverSubdirs(filepath.Join(ws.TranslatedDir(), cfg.Book.TargetLangs[0]))
	if err != nil || len(sourceInputs) == 0 {
		// Fall back to old layout (no per-lang subdir)
		sourceInputs, err = discoverSubdirs(ws.TranslatedDir())
	}
	if err != nil {
		return fmt.Errorf("discover translated inputs: %w", err)
	}

	// Render per target language
	for _, targetLang := range cfg.Book.TargetLangs {
		langDir := filepath.Join(ws.TranslatedDir(), targetLang)
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
	translatedDir := filepath.Join(ws.TranslatedDir(), targetLang, stem)

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

	var pages []*model.TranslatedPage
	for _, pf := range pageFiles {
		page, err := loadTranslatedPage(pf.path)
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

	phaseName := progress.PhaseName("write:" + targetLang + ":" + stem)

	// Render each requested format
	for _, format := range cfg.Write.Formats {
		switch format {
		case "md":
			if err := compileMarkdown(ws, cfg, stem, targetLang, pages, logger); err != nil {
				logger.Error("markdown compilation failed", "input", stem, "error", err)
			}
		case "latex":
			if err := compileLatex(ctx, ws, cfg, stem, targetLang, pages, logger); err != nil {
				logger.Error("latex compilation failed", "input", stem, "error", err)
			}
		case "docx":
			if err := compileDocx(ctx, ws, cfg, stem, targetLang, logger); err != nil {
				logger.Error("docx compilation failed", "input", stem, "error", err)
			}
		default:
			logger.Warn("unknown format", "format", format)
		}
	}

	// Mark all pages as compiled
	for _, page := range pages {
		opts.Tracker.MarkCompleted(phaseName, page.PageNumber)
	}
	if err := opts.Tracker.Save(); err != nil {
		logger.Error("failed to save progress", "error", err)
	}

	logger.Info("write complete", "input", stem, "pages", len(pages), "formats", cfg.Write.Formats)
	return nil
}

func compileMarkdown(ws *workspace.Workspace, cfg *config.Config, stem, targetLang string, pages []*model.TranslatedPage, logger *slog.Logger) error {
	targetDir := filepath.Join(ws.OutputDir(), targetLang)
	sourceDir := filepath.Join(ws.OutputDir(), cfg.Book.PrimarySourceLang())

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("create target output dir: %w", err)
	}
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		return fmt.Errorf("create source output dir: %w", err)
	}

	// Target language book
	mdRenderer := &renderer.MarkdownRenderer{}
	targetBook := mdRenderer.RenderBook(pages)
	targetPath := filepath.Join(targetDir, stem+".md")
	if err := atomicWrite(targetPath, []byte(targetBook)); err != nil {
		return fmt.Errorf("write target markdown: %w", err)
	}
	logger.Info("wrote target markdown", "path", targetPath)

	// Source language book
	arRenderer := &renderer.ArabicMarkdownRenderer{}
	sourceBook := arRenderer.RenderBook(pages)
	sourcePath := filepath.Join(sourceDir, stem+".md")
	if err := atomicWrite(sourcePath, []byte(sourceBook)); err != nil {
		return fmt.Errorf("write source markdown: %w", err)
	}
	logger.Info("wrote source markdown", "path", sourcePath)

	return nil
}

func compileLatex(ctx context.Context, ws *workspace.Workspace, cfg *config.Config, stem, targetLang string, pages []*model.TranslatedPage, logger *slog.Logger) error {
	latexDir := filepath.Join(ws.OutputDir(), targetLang, "latex")
	if err := os.MkdirAll(latexDir, 0755); err != nil {
		return fmt.Errorf("create latex output dir: %w", err)
	}

	texRenderer := &renderer.LaTeXRenderer{}
	texContent := texRenderer.RenderBook(pages)
	texPath := filepath.Join(latexDir, "book.tex")
	if err := atomicWrite(texPath, []byte(texContent)); err != nil {
		return fmt.Errorf("write latex: %w", err)
	}
	logger.Info("wrote latex", "path", texPath)

	// Compile to PDF unless skipped
	if !cfg.Write.SkipPDF {
		logger.Info("compiling PDF via Docker", "image", cfg.Write.LaTeXDockerImage)
		if err := renderer.CompilePDF(ctx, latexDir, cfg.Write.LaTeXDockerImage); err != nil {
			return fmt.Errorf("compile PDF: %w", err)
		}
		logger.Info("wrote PDF", "path", filepath.Join(latexDir, "book.pdf"))
	}

	return nil
}

func compileDocx(ctx context.Context, ws *workspace.Workspace, cfg *config.Config, stem, targetLang string, logger *slog.Logger) error {
	mdPath := filepath.Join(ws.OutputDir(), targetLang, stem+".md")
	docxPath := filepath.Join(ws.OutputDir(), targetLang, stem+".docx")

	if _, err := os.Stat(mdPath); err != nil {
		return fmt.Errorf("markdown file not found at %s (compile md format first): %w", mdPath, err)
	}

	if err := renderer.ConvertMarkdownToDocx(ctx, mdPath, docxPath); err != nil {
		return fmt.Errorf("convert to docx: %w", err)
	}
	logger.Info("wrote docx", "path", docxPath)
	return nil
}

func loadTranslatedPage(path string) (*model.TranslatedPage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var page model.TranslatedPage
	if err := json.Unmarshal(data, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

func atomicWrite(path string, data []byte) error {
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
