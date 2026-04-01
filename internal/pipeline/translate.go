package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/mmdemirbas/mutercim/internal/config"
	"github.com/mmdemirbas/mutercim/internal/display"
	"github.com/mmdemirbas/mutercim/internal/knowledge"
	"github.com/mmdemirbas/mutercim/internal/model"
	"github.com/mmdemirbas/mutercim/internal/provider"
	"github.com/mmdemirbas/mutercim/internal/rebuild"
	"github.com/mmdemirbas/mutercim/internal/translation"
	"github.com/mmdemirbas/mutercim/internal/workspace"
)

// TranslateOptions configures the translation pipeline.
type TranslateOptions struct {
	Workspace     *workspace.Workspace
	Config        *config.Config
	Provider      provider.Provider
	Knowledge     *knowledge.Knowledge
	Pages         []int
	Force         bool // force re-processing of already completed pages
	ContextWindow int  // number of previous pages for context (default: 2)
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

	if len(cfg.Translate.Languages) == 0 {
		return PhaseResult{}, fmt.Errorf("no target languages configured")
	}

	// Discover inputs from solved directory
	inputs, err := discoverSubdirs(ws.SolveDir())
	if err != nil {
		return PhaseResult{}, fmt.Errorf("discover solved inputs: %w", err)
	}
	if len(inputs) == 0 {
		return PhaseResult{}, fmt.Errorf("no solved pages found in %s (run solve first)", ws.SolveDir())
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
	for _, targetLang := range cfg.Translate.Languages {
		logger.Info("translating to language", "target", targetLang)

		translator := translation.NewTranslator(
			opts.Provider,
			opts.Knowledge,
			cfg.Write.ExpandSources,
			cfg.SourceLanguages(),
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

//nolint:cyclop,gocognit,funlen // per-input translate with context window and retry logic
func translateOneInput(ctx context.Context, opts TranslateOptions, translator *translation.Translator, stem, targetLang string, contextWindow int) (PhaseResult, error) {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	ws := opts.Workspace
	cfg := opts.Config
	solvedDir := filepath.Join(ws.SolveDir(), stem)
	translatedDir := filepath.Join(ws.TranslateDir(), targetLang, stem)

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

	if err := os.MkdirAll(translatedDir, 0750); err != nil {
		return PhaseResult{}, fmt.Errorf("create translated dir: %w", err)
	}

	// Load all solved pages for context window
	solvedPages := make(map[int]*model.SolvedRegionPage)
	for _, pf := range pages {
		page, err := loadSolvedRegionPage(pf.path)
		if err != nil {
			logger.Error("failed to load solved page", "page", pf.pageNum, "error", err)
			continue
		}
		solvedPages[pf.pageNum] = page
	}

	// Track translated pages for context window
	var recentTranslated []*model.TranslatedRegionPage

	// Start progress display
	if opts.Display != nil {
		opts.Display.StartPhase(display.PhaseTranslate, stem, len(pages), targetLang)
	}

	// Set up status callbacks for retry/failover display
	var statusPageNum int
	activeModel := func() string {
		if chain, ok := opts.Provider.(*provider.FailoverChain); ok {
			if m := chain.ActiveModel(false); m != "" {
				return m
			}
		}
		if len(cfg.Translate.Models) > 0 {
			return cfg.Translate.Models[0].Provider + "/" + cfg.Translate.Models[0].Model
		}
		return opts.Provider.Name()
	}
	if opts.Display != nil {
		if chain, ok := opts.Provider.(*provider.FailoverChain); ok {
			chain.OnFailover = func(from, to string) {
				opts.Display.SetStatus(display.StatusLine{
					Text:      fmt.Sprintf("translating page %d via %s \u2014 failover from %s", statusPageNum, to, from),
					StartedAt: time.Now(),
				})
			}
			chain.SetRetryCallback(func(attempt, maxRetries, statusCode int, backoff time.Duration) {
				opts.Display.SetStatus(display.StatusLine{
					Text:      fmt.Sprintf("translating page %d \u2014 retry %d/%d (%d)", statusPageNum, attempt, maxRetries, statusCode),
					StartedAt: time.Now(),
					Countdown: backoff,
				})
			})
		}
	}

	completed := 0
	failed := 0
	skipped := 0

	maxFailPct := opts.Config.Translate.Retry.MaxFailPercent
	for i, pf := range pages {
		if ctx.Err() != nil {
			logger.Info("context cancelled, stopping translate phase",
				"input", stem, "lang", targetLang, "processed", completed+failed, "remaining", len(pages)-i)
			break
		}
		// Check error threshold
		result := PhaseResult{Completed: completed, Failed: failed}
		if result.ExceedsErrorThreshold(maxFailPct) {
			logger.Error("aborting: failure rate exceeds threshold",
				"completed", completed, "failed", failed, "threshold", fmt.Sprintf("%d%%", maxFailPct))
			break
		}
		// Skip if output is up-to-date (mtime check)
		outputPath := filepath.Join(translatedDir, pageFilename(pf.pageNum, len(pages)))
		rebuildInputs := append([]string{pf.path, ws.ConfigPath()}, append(opts.Config.ResolveKnowledgePaths(ws.Root), ws.MemoryDir())...)
		if !opts.Force && !rebuild.NeedsRebuild(outputPath, rebuildInputs...) {
			logger.Debug("skipping page (up-to-date)", "input", stem, "page", pf.pageNum)
			skipped++
			continue
		}

		solved, ok := solvedPages[pf.pageNum]
		if !ok {
			failed++
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
		if solved.PreviousPageSummary != "" {
			contextSummaries = append(contextSummaries, solved.PreviousPageSummary)
		}

		// Translate
		statusPageNum = pf.pageNum
		currentModel := activeModel()
		if opts.Display != nil {
			opts.Display.SetStatus(display.StatusLine{
				Text:      fmt.Sprintf("translating page %d via %s", pf.pageNum, currentModel),
				StartedAt: time.Now(),
			})
		}
		translated, err := translator.TranslatePage(ctx, solved, contextSummaries, currentModel)
		if opts.Display != nil {
			opts.Display.SetStatus(display.StatusLine{}) // clear status
		}
		if err != nil {
			logger.Error("translation failed", "input", stem, "page", pf.pageNum, "error", err)
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
		if err := saveTranslatedRegionPage(translatedDir, pf.pageNum, len(pages), translated); err != nil {
			logger.Error("failed to save translated page", "input", stem, "page", pf.pageNum, "error", err)
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

		recentTranslated = append(recentTranslated, translated)
		if len(recentTranslated) > contextWindow {
			recentTranslated = recentTranslated[len(recentTranslated)-contextWindow:]
		}
		completed++
		usedModel := currentModel
		if chain, ok := opts.Provider.(*provider.FailoverChain); ok {
			if m := chain.LastUsedModel(); m != "" {
				usedModel = m
			}
		}
		logger.Info("page translated", "input", stem, "page", pf.pageNum, "model", usedModel, "completed", completed)
		if opts.Display != nil {
			opts.Display.Update(display.PageResult{
				Phase: display.PhaseTranslate, Input: stem, PageNum: pf.pageNum,
				Total: len(pages), Completed: completed, Failed: failed,
				Lang:    targetLang,
				Entries: countTranslatedRegionType(translated.Regions, model.RegionTypeEntry),
			})
		}
	}

	if opts.Display != nil {
		opts.Display.FinishPhase(display.PhaseTranslate, stem, targetLang)
	}
	logger.Info("translation complete", "input", stem, "completed", completed, "failed", failed, "skipped", skipped)

	// Write report
	report := map[string]any{
		"target_lang":     targetLang,
		"pages_completed": completed,
		"pages_failed":    failed,
		"pages_skipped":   skipped,
	}
	if data, err := json.MarshalIndent(report, "", "  "); err == nil {
		if err := atomicWriteFile(filepath.Join(translatedDir, "report.json"), data); err != nil {
			logger.Warn("failed to write translate report", "error", err)
		}
	}

	return PhaseResult{Completed: completed, Failed: failed, Skipped: skipped}, nil
}

func loadSolvedRegionPage(path string) (*model.SolvedRegionPage, error) {
	data, err := os.ReadFile(path) //nolint:gosec // G304: path is internal workspace path, not user HTTP input
	if err != nil {
		return nil, err
	}
	var page model.SolvedRegionPage
	if err := json.Unmarshal(data, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

func saveTranslatedRegionPage(dir string, pageNum, totalPages int, page *model.TranslatedRegionPage) error {
	data, err := json.MarshalIndent(page, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal page %d: %w", pageNum, err)
	}

	finalPath := filepath.Join(dir, pageFilename(pageNum, totalPages))
	if err := atomicWriteFile(finalPath, data); err != nil {
		return fmt.Errorf("write page %d: %w", pageNum, err)
	}
	return nil
}

// countTranslatedRegionType counts translated regions of a specific type.
func countTranslatedRegionType(regions []model.TranslatedRegion, regionType string) int {
	count := 0
	for _, r := range regions {
		if r.Type == regionType {
			count++
		}
	}
	return count
}
