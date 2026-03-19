package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/mmdemirbas/mutercim/internal/config"
	"github.com/mmdemirbas/mutercim/internal/display"
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
	Display       display.Display
}

// Translate runs the Phase 3 translation pipeline for all inputs and target languages.
func Translate(ctx context.Context, opts TranslateOptions) (PhaseResult, error) {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	ws := opts.Workspace
	cfg := opts.Config

	if len(cfg.Book.TargetLangs) == 0 {
		return PhaseResult{}, fmt.Errorf("no target languages configured")
	}

	// Discover inputs from solved directory
	inputs, err := discoverSubdirs(ws.SolvedDir())
	if err != nil {
		return PhaseResult{}, fmt.Errorf("discover solved inputs: %w", err)
	}
	if len(inputs) == 0 {
		return PhaseResult{}, fmt.Errorf("no solved pages found in %s (run solve first)", ws.SolvedDir())
	}

	contextWindow := opts.ContextWindow
	if contextWindow <= 0 {
		contextWindow = cfg.Translate.ContextWindow
	}
	if contextWindow <= 0 {
		contextWindow = 2
	}

	// Translate for each target language
	var total PhaseResult
	for _, targetLang := range cfg.Book.TargetLangs {
		logger.Info("translating to language", "target", targetLang)

		translator := translation.NewTranslator(
			opts.Provider,
			opts.Knowledge,
			cfg.Write.ExpandSources,
			cfg.Book.SourceLangs,
			targetLang,
			logger,
		)

		for _, stem := range inputs {
			logger.Info("translating input", "input", stem, "target", targetLang)
			result, err := translateOneInput(ctx, opts, translator, stem, targetLang, contextWindow)
			total.Completed += result.Completed
			total.Failed += result.Failed
			total.Skipped += result.Skipped
			if err != nil {
				logger.Error("translation failed", "input", stem, "target", targetLang, "error", err)
			}
		}
	}

	return total, nil
}

func translateOneInput(ctx context.Context, opts TranslateOptions, translator *translation.Translator, stem, targetLang string, contextWindow int) (PhaseResult, error) {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	ws := opts.Workspace
	cfg := opts.Config
	solvedDir := filepath.Join(ws.SolvedDir(), stem)
	translatedDir := filepath.Join(ws.TranslatedDir(), targetLang, stem)
	outputPagesDir := filepath.Join(ws.OutputDir(), targetLang, "pages", stem)

	// Build section lookup for translate checks
	lookup, _ := config.NewSectionLookup(cfg.Sections)

	// List solved pages
	pages, err := listPageFiles(solvedDir)
	if err != nil {
		return PhaseResult{}, fmt.Errorf("list solved pages: %w", err)
	}
	if len(pages) == 0 {
		return PhaseResult{}, fmt.Errorf("no solved pages in %s", solvedDir)
	}

	if len(opts.Pages) > 0 {
		pages = filterPages(pages, opts.Pages)
	}

	if err := os.MkdirAll(translatedDir, 0755); err != nil {
		return PhaseResult{}, fmt.Errorf("create translated dir: %w", err)
	}
	if err := os.MkdirAll(outputPagesDir, 0755); err != nil {
		return PhaseResult{}, fmt.Errorf("create output pages dir: %w", err)
	}

	phaseName := progress.PhaseName("translate:" + targetLang + ":" + stem)

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

	// Start progress display
	if opts.Display != nil {
		opts.Display.StartPhase(display.PhaseTranslate, stem, len(pages), targetLang)
	}

	completed := 0
	failed := 0
	skipped := 0

	for _, pf := range pages {
		if ctx.Err() != nil {
			break
		}
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
			if opts.Display != nil {
				opts.Display.Update(display.PageResult{
					Phase: display.PhaseTranslate, Input: stem, PageNum: pf.pageNum,
					Total: len(pages), Completed: completed, Failed: failed,
					Lang: targetLang, Err: err,
				})
			}
			continue
		}

		// Save translated JSON atomically
		if err := saveTranslatedPage(translatedDir, pf.pageNum, translated); err != nil {
			logger.Error("failed to save translated page", "input", stem, "page", pf.pageNum, "error", err)
			opts.Tracker.MarkFailed(phaseName, pf.pageNum)
			failed++
			if opts.Display != nil {
				opts.Display.Update(display.PageResult{
					Phase: display.PhaseTranslate, Input: stem, PageNum: pf.pageNum,
					Total: len(pages), Completed: completed, Failed: failed,
					Lang: targetLang, Err: err,
				})
			}
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
		if opts.Display != nil {
			opts.Display.Update(display.PageResult{
				Phase: display.PhaseTranslate, Input: stem, PageNum: pf.pageNum,
				Total: len(pages), Completed: completed, Failed: failed,
				Lang: targetLang, Entries: len(translated.TranslatedEntries),
				Footnotes: len(translated.TranslatedFootnotes),
			})
		}
	}

	if opts.Display != nil {
		opts.Display.FinishPhase(display.PhaseTranslate, stem)
	}
	logger.Info("translation complete", "input", stem, "completed", completed, "failed", failed, "skipped", skipped)
	return PhaseResult{Completed: completed, Failed: failed, Skipped: skipped}, nil
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
			lines = append(lines, fmt.Sprintf("**%d.** %s\n", e.Number, e.TranslatedText))
		} else {
			lines = append(lines, e.TranslatedText+"\n")
		}
		if e.TranslatorNotes != "" {
			lines = append(lines, fmt.Sprintf("_[Not: %s]_\n", e.TranslatorNotes))
		}
	}

	// Footnotes
	if len(page.TranslatedFootnotes) > 0 {
		lines = append(lines, "---\n")
		for _, fn := range page.TranslatedFootnotes {
			if len(fn.EntryNumbers) > 0 {
				nums := make([]string, len(fn.EntryNumbers))
				for i, n := range fn.EntryNumbers {
					nums[i] = fmt.Sprintf("%d", n)
				}
				lines = append(lines, fmt.Sprintf("[%s] %s\n", strings.Join(nums, ","), fn.TranslatedText))
			} else {
				lines = append(lines, fn.TranslatedText+"\n")
			}
		}
	}

	content := strings.Join(lines, "\n")

	filename := fmt.Sprintf("page_%03d.md", pageNum)
	tmpPath := filepath.Join(dir, filename+".tmp")
	finalPath := filepath.Join(dir, filename)
	if err := os.WriteFile(tmpPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("write page %d md tmp: %w", pageNum, err)
	}
	return os.Rename(tmpPath, finalPath)
}
