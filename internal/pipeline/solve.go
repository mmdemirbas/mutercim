package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/mmdemirbas/mutercim/internal/display"
	"github.com/mmdemirbas/mutercim/internal/knowledge"
	"github.com/mmdemirbas/mutercim/internal/model"
	"github.com/mmdemirbas/mutercim/internal/rebuild"
	"github.com/mmdemirbas/mutercim/internal/solver"
	"github.com/mmdemirbas/mutercim/internal/workspace"
)

// SolveOptions configures the solver pipeline.
type SolveOptions struct {
	Workspace      *workspace.Workspace
	Knowledge      *knowledge.Knowledge
	KnowledgePaths []string // resolved paths for mtime rebuild checks
	SourceLang     string   // primary source language for glossary matching (default: "ar")
	Pages          []int    // specific pages to process; nil means all available
	Force          bool     // force re-processing of already completed pages
	Logger         *slog.Logger
	Display        display.Display
}

// Solve runs the Phase 2 solver pipeline for all inputs.
func Solve(ctx context.Context, opts SolveOptions) (PhaseResult, error) {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	ws := opts.Workspace

	// Discover inputs from read directory
	inputs, err := discoverSubdirs(ws.ReadDir())
	if err != nil {
		return PhaseResult{}, fmt.Errorf("discover read inputs: %w", err)
	}
	if len(inputs) == 0 {
		return PhaseResult{}, fmt.Errorf("no read pages found in %s (run read first)", ws.ReadDir())
	}

	slvr := solver.NewSolver(opts.Knowledge, opts.SourceLang, logger)

	var total PhaseResult
	for _, stem := range inputs {
		logger.Info("solving input", "input", stem)
		result, err := solveOneInput(ctx, opts, slvr, stem)
		total.Completed += result.Completed
		total.Failed += result.Failed
		total.Skipped += result.Skipped
		if err != nil {
			logger.Error("solve failed", "input", stem, "error", err)
		}
	}

	return total, nil
}

//nolint:cyclop,gocognit,funlen // per-input solve with knowledge resolution and cross-page logic
func solveOneInput(ctx context.Context, opts SolveOptions, slvr *solver.Solver, stem string) (PhaseResult, error) {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	ws := opts.Workspace
	readDir := filepath.Join(ws.ReadDir(), stem)
	solvedDir := filepath.Join(ws.SolveDir(), stem)

	allPages, err := listPageFiles(readDir)
	if err != nil {
		return PhaseResult{}, fmt.Errorf("list read pages: %w", err)
	}
	if len(allPages) == 0 {
		return PhaseResult{}, fmt.Errorf("no read pages in %s", readDir)
	}

	maxPage := maxPageNumber(allPages)
	pages := allPages
	if len(opts.Pages) > 0 {
		pages = filterPages(pages, opts.Pages)
	}

	if err := os.MkdirAll(solvedDir, 0750); err != nil {
		return PhaseResult{}, fmt.Errorf("create solve dir: %w", err)
	}

	// Start progress display
	if opts.Display != nil {
		opts.Display.StartPhase(display.PhaseSolve, stem, len(pages), "")
	}

	completed := 0
	failed := 0
	skipped := 0
	var previous *model.RegionPage

	for i, pf := range pages {
		if ctx.Err() != nil {
			logger.Info("context cancelled, stopping solve phase",
				"input", stem, "processed", completed+failed, "remaining", len(pages)-i)
			break
		}

		outputPath := filepath.Join(solvedDir, pageFilename(pf.pageNum, maxPage))
		rebuildInputs := make([]string, 0, 2+len(opts.KnowledgePaths)+1)
		rebuildInputs = append(rebuildInputs, pf.path, ws.ConfigPath())
		rebuildInputs = append(rebuildInputs, opts.KnowledgePaths...)
		rebuildInputs = append(rebuildInputs, ws.MemoryDir())
		if !opts.Force && !rebuild.NeedsRebuild(outputPath, rebuildInputs...) {
			logger.Debug("skipping page (up-to-date)", "input", stem, "page", pf.pageNum)
			skipped++
			// Load skipped page so it can serve as previous for the next iteration
			if loaded, loadErr := loadRegionPage(pf.path); loadErr == nil {
				previous = loaded
			}
			continue
		}

		current, err := loadRegionPage(pf.path)
		if err != nil {
			logger.Error("failed to load read page", "page", pf.pageNum, "error", err)
			failed++
			continue
		}

		// Build previous page summary
		prevSummary := ""
		if previous != nil {
			prevSummary = solver.PageSummary(previous)
		}

		solved := slvr.SolvePage(current, previous, prevSummary)

		if err := saveSolvedRegionPage(solvedDir, pf.pageNum, maxPage, solved); err != nil {
			logger.Error("failed to save solved page", "input", stem, "page", pf.pageNum, "error", err)
			failed++
			if opts.Display != nil {
				opts.Display.Update(display.PageResult{
					Phase: display.PhaseSolve, Input: stem, PageNum: pf.pageNum,
					Total: len(pages), Completed: completed, Failed: failed, Err: err,
				})
			}
			continue
		}

		completed++
		previous = current

		if opts.Display != nil {
			opts.Display.Update(display.PageResult{
				Phase: display.PhaseSolve, Input: stem, PageNum: pf.pageNum,
				Total: len(pages), Completed: completed, Failed: failed,
				Entries:   countRegionType(solved.Regions, model.RegionTypeEntry),
				Footnotes: countRegionType(solved.Regions, model.RegionTypeFootnote),
			})
		}
	}

	if opts.Display != nil {
		opts.Display.FinishPhase(display.PhaseSolve, stem, "")
	}
	logger.Info("solve complete", "input", stem, "completed", completed, "failed", failed, "skipped", skipped)

	// Write report
	report := map[string]any{
		"pages_completed": completed,
		"pages_failed":    failed,
		"pages_skipped":   skipped,
	}
	if data, err := json.MarshalIndent(report, "", "  "); err == nil {
		if err := atomicWriteFile(filepath.Join(solvedDir, "report.json"), data); err != nil {
			logger.Warn("failed to write solve report", "error", err)
		}
	}

	return PhaseResult{Completed: completed, Failed: failed, Skipped: skipped}, nil
}

func loadRegionPage(path string) (*model.RegionPage, error) {
	data, err := os.ReadFile(path) //nolint:gosec // G304: path is internal workspace path, not user HTTP input
	if err != nil {
		return nil, err
	}
	var page model.RegionPage
	if err := json.Unmarshal(data, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

func saveSolvedRegionPage(dir string, pageNum, maxPageNum int, page *model.SolvedRegionPage) error {
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

// countRegionType counts regions of a specific type.
func countRegionType(regions []model.Region, regionType string) int {
	count := 0
	for _, r := range regions {
		if r.Type == regionType {
			count++
		}
	}
	return count
}
