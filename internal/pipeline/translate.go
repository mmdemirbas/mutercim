package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/mmdemirbas/mutercim/internal/config"
	"github.com/mmdemirbas/mutercim/internal/knowledge"
	"github.com/mmdemirbas/mutercim/internal/model"
	"github.com/mmdemirbas/mutercim/internal/progress"
	"github.com/mmdemirbas/mutercim/internal/provider"
	"github.com/mmdemirbas/mutercim/internal/translation"
	"github.com/mmdemirbas/mutercim/internal/workspace"
)

// TranslateOptions configures the translation pipeline.
type TranslateOptions struct {
	Workspace     *workspace.Workspace
	Config        *config.Config
	Provider      provider.Provider
	Knowledge     *knowledge.Knowledge
	Tracker       *progress.Tracker
	Pages         []int
	ContextWindow int // number of previous pages for context (default: 2)
	Logger        *slog.Logger
}

// Translate runs the Phase 3 translation pipeline for all inputs.
func Translate(ctx context.Context, opts TranslateOptions) error {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	ws := opts.Workspace

	// Discover inputs from solved directory
	inputs, err := discoverSubdirs(ws.SolvedDir())
	if err != nil {
		return fmt.Errorf("discover solved inputs: %w", err)
	}
	if len(inputs) == 0 {
		return fmt.Errorf("no solved pages found in %s (run solve first)", ws.SolvedDir())
	}

	contextWindow := opts.ContextWindow
	if contextWindow <= 0 {
		contextWindow = opts.Config.Translate.ContextWindow
	}
	if contextWindow <= 0 {
		contextWindow = 2
	}

	translator := translation.NewTranslator(
		opts.Provider,
		opts.Knowledge,
		opts.Config.Write.ExpandSources,
		logger,
	)

	for _, stem := range inputs {
		logger.Info("translating input", "input", stem)
		if err := translateOneInput(ctx, opts, translator, stem, contextWindow); err != nil {
			logger.Error("translation failed", "input", stem, "error", err)
		}
	}

	return nil
}

func translateOneInput(ctx context.Context, opts TranslateOptions, translator *translation.Translator, stem string, contextWindow int) error {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	ws := opts.Workspace
	cfg := opts.Config
	solvedDir := filepath.Join(ws.SolvedDir(), stem)
	translatedDir := filepath.Join(ws.TranslatedDir(), stem)
	outputPagesDir := filepath.Join(ws.OutputDir(), "turkish", "pages", stem)

	// Build section lookup for translate checks
	lookup, _ := config.NewSectionLookup(cfg.Sections)

	// List solved pages
	pages, err := listPageFiles(solvedDir)
	if err != nil {
		return fmt.Errorf("list solved pages: %w", err)
	}
	if len(pages) == 0 {
		return fmt.Errorf("no solved pages in %s", solvedDir)
	}

	if len(opts.Pages) > 0 {
		pages = filterPages(pages, opts.Pages)
	}

	if err := os.MkdirAll(translatedDir, 0755); err != nil {
		return fmt.Errorf("create translated dir: %w", err)
	}
	if err := os.MkdirAll(outputPagesDir, 0755); err != nil {
		return fmt.Errorf("create output pages dir: %w", err)
	}

	phaseName := progress.PhaseName("translate:" + stem)

	// Load all solved pages for context window
	solvedPages := make(map[int]*model.SolvedPage)
	for _, pf := range pages {
		page, err := loadSolvedPage(pf.path)
		if err != nil {
			logger.Error("failed to load solved page", "page", pf.pageNum, "error", err)
			continue
		}
		solvedPages[pf.pageNum] = page
	}

	// Track translated pages for context window
	var recentTranslated []*model.TranslatedPage

	completed := 0
	failed := 0
	skipped := 0

	for _, pf := range pages {
		// Skip already completed — but only if the output file actually exists
		outputPath := filepath.Join(translatedDir, fmt.Sprintf("page_%03d.json", pf.pageNum))
		state := opts.Tracker.State()
		if phase := state.Phases[phaseName]; phase != nil {
			if containsInt(phase.Completed, pf.pageNum) {
				if fileExists(outputPath) {
					skipped++
					continue
				}
				logger.Warn("progress says completed but output missing, re-processing", "input", stem, "page", pf.pageNum)
			}
		}

		// Check section translate flag
		if lookup != nil {
			sec := lookup.ForPage(pf.pageNum)
			if !sec.Translate {
				logger.Debug("skipping page (translate: false)", "input", stem, "page", pf.pageNum)
				skipped++
				continue
			}
		}

		solved, ok := solvedPages[pf.pageNum]
		if !ok {
			continue
		}

		// Build context from recent translated pages
		var contextSummaries []string
		start := len(recentTranslated) - contextWindow
		if start < 0 {
			start = 0
		}
		for _, tp := range recentTranslated[start:] {
			if s := translation.PageSummary(tp); s != "" {
				contextSummaries = append(contextSummaries, s)
			}
		}

		// Also add solver context if available
		if solved.TranslationContext != nil && solved.TranslationContext.PreviousPageSummary != "" {
			contextSummaries = append(contextSummaries, solved.TranslationContext.PreviousPageSummary)
		}

		// Translate
		translated, err := translator.TranslatePage(ctx, solved, contextSummaries, cfg.Translate.Model)
		if err != nil {
			logger.Error("translation failed", "input", stem, "page", pf.pageNum, "error", err)
			opts.Tracker.MarkFailed(phaseName, pf.pageNum)
			failed++
			continue
		}

		// Save translated JSON atomically
		if err := saveTranslatedPage(translatedDir, pf.pageNum, translated); err != nil {
			logger.Error("failed to save translated page", "input", stem, "page", pf.pageNum, "error", err)
			opts.Tracker.MarkFailed(phaseName, pf.pageNum)
			failed++
			continue
		}

		// Write incremental per-page output
		if err := writePageOutput(outputPagesDir, pf.pageNum, translated); err != nil {
			logger.Warn("failed to write page output", "page", pf.pageNum, "error", err)
		}

		opts.Tracker.MarkCompleted(phaseName, pf.pageNum)
		if err := opts.Tracker.Save(); err != nil {
			logger.Error("failed to save progress", "error", err)
		}

		recentTranslated = append(recentTranslated, translated)
		completed++
		logger.Info("page translated", "input", stem, "page", pf.pageNum, "completed", completed)
	}

	logger.Info("translation complete", "input", stem, "completed", completed, "failed", failed, "skipped", skipped)
	return nil
}

func loadSolvedPage(path string) (*model.SolvedPage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var page model.SolvedPage
	if err := json.Unmarshal(data, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

func saveTranslatedPage(dir string, pageNum int, page *model.TranslatedPage) error {
	data, err := json.MarshalIndent(page, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal page %d: %w", pageNum, err)
	}

	filename := fmt.Sprintf("page_%03d.json", pageNum)
	tmpPath := filepath.Join(dir, filename+".tmp")
	finalPath := filepath.Join(dir, filename)

	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("write page %d tmp: %w", pageNum, err)
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		return fmt.Errorf("rename page %d: %w", pageNum, err)
	}
	return nil
}

// writePageOutput writes a simple per-page markdown file for incremental review.
func writePageOutput(dir string, pageNum int, page *model.TranslatedPage) error {
	var lines []string

	// Header
	if page.TranslatedHeader != nil && page.TranslatedHeader.Text != "" {
		lines = append(lines, fmt.Sprintf("# %s\n", page.TranslatedHeader.Text))
	}

	// Entries
	for _, e := range page.TranslatedEntries {
		if e.Number > 0 {
			lines = append(lines, fmt.Sprintf("**%d.** %s\n", e.Number, e.TurkishText))
		} else {
			lines = append(lines, e.TurkishText+"\n")
		}
		if e.TranslatorNotes != "" {
			lines = append(lines, fmt.Sprintf("_[Not: %s]_\n", e.TranslatorNotes))
		}
	}

	// Footnotes
	if len(page.TranslatedFootnotes) > 0 {
		lines = append(lines, "---\n")
		for _, fn := range page.TranslatedFootnotes {
			if fn.EntryNumber > 0 {
				lines = append(lines, fmt.Sprintf("[%d] %s\n", fn.EntryNumber, fn.TurkishText))
			} else {
				lines = append(lines, fn.TurkishText+"\n")
			}
		}
	}

	content := ""
	for _, l := range lines {
		content += l + "\n"
	}

	filename := fmt.Sprintf("page_%03d.md", pageNum)
	return os.WriteFile(filepath.Join(dir, filename), []byte(content), 0644)
}
