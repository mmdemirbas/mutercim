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
	"github.com/mmdemirbas/mutercim/internal/progress"
	"github.com/mmdemirbas/mutercim/internal/solver"
	"github.com/mmdemirbas/mutercim/internal/workspace"
)

// SolveOptions configures the solver pipeline.
type SolveOptions struct {
	Workspace *workspace.Workspace
	Knowledge *knowledge.Knowledge
	Tracker   *progress.Tracker
	Pages     []int // specific pages to process; nil means all available
	Logger    *slog.Logger
	Display   display.Display
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

	slvr := solver.NewSolver(opts.Knowledge, logger)

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

func solveOneInput(ctx context.Context, opts SolveOptions, slvr *solver.Solver, stem string) (PhaseResult, error) {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	ws := opts.Workspace
	readDir := filepath.Join(ws.ReadDir(), stem)
	solvedDir := filepath.Join(ws.SolvedDir(), stem)
	stagedDir := ws.StagedDir()

	// List read page files
	pages, err := listPageFiles(readDir)
	if err != nil {
		return PhaseResult{}, fmt.Errorf("list read pages: %w", err)
	}
	if len(pages) == 0 {
		return PhaseResult{}, fmt.Errorf("no read pages in %s", readDir)
	}

	// Filter to requested pages
	if len(opts.Pages) > 0 {
		pages = filterPages(pages, opts.Pages)
	}

	if err := os.MkdirAll(solvedDir, 0755); err != nil {
		return PhaseResult{}, fmt.Errorf("create solved dir: %w", err)
	}

	phaseName := progress.PhaseName("solve:" + stem)

	// Load all read pages for cross-page context
	allPages := make(map[int]*model.ReadPage)
	for _, pf := range pages {
		page, err := loadReadPage(pf.path)
		if err != nil {
			logger.Error("failed to load read page", "page", pf.pageNum, "error", err)
			continue
		}
		allPages[pf.pageNum] = page
	}

	// Start progress display
	if opts.Display != nil {
		opts.Display.StartPhase(display.PhaseSolve, stem, len(pages), "")
	}

	completed := 0
	failed := 0
	skipped := 0

	for _, pf := range pages {
		if ctx.Err() != nil {
			break
		}
		// Skip already completed — but only if the output file actually exists
		outputPath := filepath.Join(solvedDir, fmt.Sprintf("page_%03d.json", pf.pageNum))
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

		current, ok := allPages[pf.pageNum]
		if !ok {
			continue
		}

		// Get previous page for continuation detection
		var previous *model.ReadPage
		if prev, ok := allPages[pf.pageNum-1]; ok {
			previous = prev
		}

		// Solve: solve page with knowledge resolution
		solved := slvr.SolvePage(current, previous)

		// Auto-stage from reference_table pages
		if current.SectionType == "reference_table" {
			if err := solver.StageFromReferenceTable(current, stagedDir); err != nil {
				logger.Warn("staging failed", "page", pf.pageNum, "error", err)
			}
		}

		// Save solved page
		if err := saveSolvedPage(solvedDir, pf.pageNum, solved); err != nil {
			logger.Error("failed to save solved page", "page", pf.pageNum, "error", err)
			opts.Tracker.MarkFailed(phaseName, pf.pageNum)
			failed++
			if opts.Display != nil {
				opts.Display.Update(display.PageResult{
					Phase: display.PhaseSolve, Input: stem, PageNum: pf.pageNum,
					Total: len(pages), Completed: completed, Failed: failed, Err: err,
				})
			}
			continue
		}

		opts.Tracker.MarkCompleted(phaseName, pf.pageNum)
		if err := opts.Tracker.Save(); err != nil {
			logger.Error("failed to save progress", "error", err)
		}
		completed++
		if opts.Display != nil {
			opts.Display.Update(display.PageResult{
				Phase: display.PhaseSolve, Input: stem, PageNum: pf.pageNum,
				Total: len(pages), Completed: completed, Failed: failed,
				Entries: len(solved.Entries), Footnotes: len(solved.Footnotes),
			})
		}
	}

	if opts.Display != nil {
		opts.Display.FinishPhase(display.PhaseSolve, stem)
	}
	logger.Info("solve complete", "input", stem, "completed", completed, "failed", failed, "skipped", skipped)
	return PhaseResult{Completed: completed, Failed: failed, Skipped: skipped}, nil
}

func loadReadPage(path string) (*model.ReadPage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var page model.ReadPage
	if err := json.Unmarshal(data, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

func saveSolvedPage(dir string, pageNum int, page *model.SolvedPage) error {
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
