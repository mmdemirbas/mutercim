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

func translateOneInput(ctx context.Context, opts TranslateOptions, translator *translation.Translator, stem, targetLang string, contextWindow int) (PhaseResult, error) {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	ws := opts.Workspace
	solvedDir := filepath.Join(ws.SolveDir(), stem)
	translatedDir := filepath.Join(ws.TranslateDir(), targetLang, stem)

	allPages, err := listPageFiles(solvedDir)
	if err != nil {
		return PhaseResult{}, fmt.Errorf("list solved pages: %w", err)
	}
	if len(allPages) == 0 {
		return PhaseResult{}, fmt.Errorf("no solved pages in %s", solvedDir)
	}
	maxPage := maxPageNumber(allPages)
	pages := allPages
	if len(opts.Pages) > 0 {
		pages = filterPages(pages, opts.Pages)
	}
	if err := os.MkdirAll(translatedDir, 0750); err != nil {
		return PhaseResult{}, fmt.Errorf("create translated dir: %w", err)
	}

	solvedPages := make(map[int]*model.SolvedRegionPage)
	for _, pf := range pages {
		page, err := loadSolvedRegionPage(pf.path)
		if err != nil {
			logger.Error("failed to load solved page", "page", pf.pageNum, "error", err)
			continue
		}
		solvedPages[pf.pageNum] = page
	}

	tc := &translateContext{
		opts:           opts,
		stem:           stem,
		targetLang:     targetLang,
		translatedDir:  translatedDir,
		contextWindow:  contextWindow,
		translator:     translator,
		solvedPages:    solvedPages,
		totalPages:     len(pages),
		maxPageNum:     maxPage,
		knowledgePaths: opts.Config.ResolveKnowledgePaths(opts.Workspace.Root),
		logger:         logger,
	}
	tc.activeModel = tc.buildActiveModelFunc()
	tc.setupDisplayCallbacks()

	tc.processAllPages(ctx, pages)

	if opts.Display != nil {
		opts.Display.FinishPhase(display.PhaseTranslate, stem, targetLang)
	}
	logger.Info("translation complete", "input", stem, "completed", tc.completed, "failed", tc.failed, "skipped", tc.skipped)

	writePhaseReport(translatedDir, tc.completed, tc.failed, tc.skipped, logger)
	return PhaseResult{Completed: tc.completed, Failed: tc.failed, Skipped: tc.skipped}, nil
}

// translateContext holds per-input state for the translate phase page loop.
type translateContext struct {
	opts          TranslateOptions
	stem          string
	targetLang    string
	translatedDir string
	contextWindow int
	translator    *translation.Translator
	solvedPages   map[int]*model.SolvedRegionPage
	totalPages    int
	maxPageNum     int // max page number across all pages (not just filtered), for consistent filenames
	knowledgePaths []string // resolved once, reused per page
	logger         *slog.Logger

	activeModel      func() string
	statusPageNum    int
	recentTranslated []*model.TranslatedRegionPage
	completed        int
	failed           int
	skipped          int
}

// buildActiveModelFunc returns a function that resolves the currently active model label.
func (tc *translateContext) buildActiveModelFunc() func() string {
	return func() string {
		if chain, ok := tc.opts.Provider.(*provider.FailoverChain); ok {
			if m := chain.ActiveModel(false); m != "" {
				return m
			}
		}
		models := tc.opts.Config.Translate.Models
		if len(models) > 0 {
			return models[0].Provider + "/" + models[0].Model
		}
		return tc.opts.Provider.Name()
	}
}

// setupDisplayCallbacks wires failover and retry callbacks to the display.
func (tc *translateContext) setupDisplayCallbacks() {
	if tc.opts.Display == nil {
		return
	}
	chain, ok := tc.opts.Provider.(*provider.FailoverChain)
	if !ok {
		return
	}
	chain.OnFailover = func(from, to string) {
		tc.opts.Display.SetStatus(display.StatusLine{
			Text:      fmt.Sprintf("translating page %d via %s \u2014 failover from %s", tc.statusPageNum, to, from),
			StartedAt: time.Now(),
		})
	}
	chain.SetRetryCallback(func(attempt, maxRetries, statusCode int, backoff time.Duration) {
		tc.opts.Display.SetStatus(display.StatusLine{
			Text:      fmt.Sprintf("translating page %d \u2014 retry %d/%d (%d)", tc.statusPageNum, attempt, maxRetries, statusCode),
			StartedAt: time.Now(),
			Countdown: backoff,
		})
	})
}

// processAllPages iterates over pages, checking for cancellation and error thresholds.
func (tc *translateContext) processAllPages(ctx context.Context, pages []pageFile) {
	if tc.opts.Display != nil {
		tc.opts.Display.StartPhase(display.PhaseTranslate, tc.stem, len(pages), tc.targetLang)
	}
	maxFailPct := tc.opts.Config.Translate.Retry.MaxFailPercent
	for i, pf := range pages {
		if ctx.Err() != nil {
			tc.logger.Info("context cancelled, stopping translate phase",
				"input", tc.stem, "lang", tc.targetLang, "processed", tc.completed+tc.failed, "remaining", len(pages)-i)
			return
		}
		result := PhaseResult{Completed: tc.completed, Failed: tc.failed}
		if result.ExceedsErrorThreshold(maxFailPct) {
			tc.logger.Error("aborting: failure rate exceeds threshold",
				"completed", tc.completed, "failed", tc.failed, "threshold", fmt.Sprintf("%d%%", maxFailPct))
			return
		}
		tc.processTranslatePage(ctx, pf)
	}
}

// processTranslatePage handles a single page: rebuild check, translate, save.
func (tc *translateContext) processTranslatePage(ctx context.Context, pf pageFile) {
	ws := tc.opts.Workspace

	outputPath := filepath.Join(tc.translatedDir, pageFilename(pf.pageNum, tc.maxPageNum))
	rebuildInputs := make([]string, 0, 2+len(tc.knowledgePaths)+1)
	rebuildInputs = append(rebuildInputs, pf.path, ws.ConfigPath())
	rebuildInputs = append(rebuildInputs, tc.knowledgePaths...)
	rebuildInputs = append(rebuildInputs, ws.MemoryDir())
	if !tc.opts.Force && !rebuild.NeedsRebuild(outputPath, rebuildInputs...) {
		tc.logger.Debug("skipping page (up-to-date)", "input", tc.stem, "page", pf.pageNum)
		tc.skipped++
		return
	}

	solved, ok := tc.solvedPages[pf.pageNum]
	if !ok {
		tc.logger.Warn("no solved page found, skipping translation", "input", tc.stem, "page", pf.pageNum)
		tc.failed++
		return
	}

	contextSummaries := buildTranslateContext(tc.recentTranslated, tc.contextWindow, solved)

	tc.statusPageNum = pf.pageNum
	currentModel := tc.activeModel()
	if tc.opts.Display != nil {
		tc.opts.Display.SetStatus(display.StatusLine{
			Text:      fmt.Sprintf("translating page %d via %s", pf.pageNum, currentModel),
			StartedAt: time.Now(),
		})
	}
	translateStart := time.Now()
	translated, err := tc.translator.TranslatePage(ctx, solved, contextSummaries, currentModel)
	if tc.opts.Display != nil {
		tc.opts.Display.SetStatus(display.StatusLine{})
	}

	if err != nil {
		tc.recordFailure(pf, err)
		return
	}

	if err := saveTranslatedRegionPage(tc.translatedDir, pf.pageNum, tc.maxPageNum, translated); err != nil {
		tc.recordFailure(pf, err)
		return
	}

	tc.recordSuccess(pf, currentModel, translated, time.Since(translateStart))
}

// recordFailure increments the failure counter and updates the display.
func (tc *translateContext) recordFailure(pf pageFile, err error) {
	tc.logger.Error("translation failed", "input", tc.stem, "page", pf.pageNum, "error", err)
	tc.failed++
	if tc.opts.Display != nil {
		tc.opts.Display.Update(display.PageResult{
			Phase: display.PhaseTranslate, Input: tc.stem, PageNum: pf.pageNum,
			Total: tc.totalPages, Completed: tc.completed, Failed: tc.failed,
			Lang: tc.targetLang, Err: err,
		})
	}
}

// recordSuccess updates context window, increments completion, logs, and updates display.
func (tc *translateContext) recordSuccess(pf pageFile, currentModel string, translated *model.TranslatedRegionPage, elapsed time.Duration) {
	tc.recentTranslated = append(tc.recentTranslated, translated)
	if len(tc.recentTranslated) > tc.contextWindow {
		tc.recentTranslated = tc.recentTranslated[len(tc.recentTranslated)-tc.contextWindow:]
	}
	tc.completed++

	usedModel := currentModel
	if chain, ok := tc.opts.Provider.(*provider.FailoverChain); ok {
		if m := chain.LastUsedModel(); m != "" {
			usedModel = m
		}
	}
	tc.logger.Info("page translated", "input", tc.stem, "page", pf.pageNum, "model", usedModel,
		"regions", len(translated.Regions), "elapsed_ms", elapsed.Milliseconds(), "completed", tc.completed)

	if tc.opts.Display != nil {
		tc.opts.Display.Update(display.PageResult{
			Phase: display.PhaseTranslate, Input: tc.stem, PageNum: pf.pageNum,
			Total: tc.totalPages, Completed: tc.completed, Failed: tc.failed,
			Lang:    tc.targetLang,
			Entries: countTranslatedRegionType(translated.Regions, model.RegionTypeEntry),
		})
	}
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

func saveTranslatedRegionPage(dir string, pageNum, maxPageNum int, page *model.TranslatedRegionPage) error {
	data, err := json.MarshalIndent(page, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal page %d: %w", pageNum, err)
	}

	finalPath := filepath.Join(dir, pageFilename(pageNum, maxPageNum))
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

// buildTranslateContext assembles the context summaries passed to the translator for a single page.
// It takes the trailing contextWindow entries from recentTranslated and appends any solver
// context recorded on the solved page.
func buildTranslateContext(recentTranslated []*model.TranslatedRegionPage, contextWindow int, solved *model.SolvedRegionPage) []string {
	var summaries []string
	start := len(recentTranslated) - contextWindow
	if start < 0 {
		start = 0
	}
	for _, tp := range recentTranslated[start:] {
		if s := translation.PageSummary(tp); s != "" {
			summaries = append(summaries, s)
		}
	}
	if solved.PreviousPageSummary != "" {
		summaries = append(summaries, solved.PreviousPageSummary)
	}
	return summaries
}
