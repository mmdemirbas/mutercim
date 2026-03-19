package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/mmdemirbas/mutercim/internal/enrichment"
	"github.com/mmdemirbas/mutercim/internal/knowledge"
	"github.com/mmdemirbas/mutercim/internal/model"
	"github.com/mmdemirbas/mutercim/internal/progress"
	"github.com/mmdemirbas/mutercim/internal/workspace"
)

// EnrichOptions configures the enrichment pipeline.
type EnrichOptions struct {
	Workspace *workspace.Workspace
	Knowledge *knowledge.Knowledge
	Tracker   *progress.Tracker
	Pages     []int // specific pages to process; nil means all available
	Logger    *slog.Logger
}

// Enrich runs the Phase 2 enrichment pipeline for all inputs.
func Enrich(ctx context.Context, opts EnrichOptions) error {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	ws := opts.Workspace

	// Discover inputs from extracted directory
	inputs, err := discoverSubdirs(ws.ExtractedDir())
	if err != nil {
		return fmt.Errorf("discover extracted inputs: %w", err)
	}
	if len(inputs) == 0 {
		return fmt.Errorf("no extracted pages found in %s (run extract first)", ws.ExtractedDir())
	}

	enricher := enrichment.NewEnricher(opts.Knowledge, logger)

	for _, stem := range inputs {
		logger.Info("enriching input", "input", stem)
		if err := enrichOneInput(ctx, opts, enricher, stem); err != nil {
			logger.Error("enrichment failed", "input", stem, "error", err)
		}
	}

	return nil
}

func enrichOneInput(ctx context.Context, opts EnrichOptions, enricher *enrichment.Enricher, stem string) error {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	ws := opts.Workspace
	extractedDir := filepath.Join(ws.ExtractedDir(), stem)
	enrichedDir := filepath.Join(ws.EnrichedDir(), stem)
	stagedDir := ws.StagedDir()

	// List extracted page files
	pages, err := listPageFiles(extractedDir)
	if err != nil {
		return fmt.Errorf("list extracted pages: %w", err)
	}
	if len(pages) == 0 {
		return fmt.Errorf("no extracted pages in %s", extractedDir)
	}

	// Filter to requested pages
	if len(opts.Pages) > 0 {
		pages = filterPages(pages, opts.Pages)
	}

	if err := os.MkdirAll(enrichedDir, 0755); err != nil {
		return fmt.Errorf("create enriched dir: %w", err)
	}

	phaseName := progress.PhaseName("enrich:" + stem)

	// Load all extracted pages for cross-page context
	allPages := make(map[int]*model.ExtractedPage)
	for _, pf := range pages {
		page, err := loadExtractedPage(pf.path)
		if err != nil {
			logger.Error("failed to load extracted page", "page", pf.pageNum, "error", err)
			continue
		}
		allPages[pf.pageNum] = page
	}

	completed := 0
	failed := 0
	skipped := 0

	for _, pf := range pages {
		// Skip already completed — but only if the output file actually exists
		outputPath := filepath.Join(enrichedDir, fmt.Sprintf("page_%03d.json", pf.pageNum))
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
		var previous *model.ExtractedPage
		if prev, ok := allPages[pf.pageNum-1]; ok {
			previous = prev
		}

		// Enrich
		enriched := enricher.EnrichPage(current, previous)

		// Auto-stage from reference_table pages
		if current.SectionType == "reference_table" {
			if err := enrichment.StageFromReferenceTable(current, stagedDir); err != nil {
				logger.Warn("staging failed", "page", pf.pageNum, "error", err)
			}
		}

		// Save enriched page
		if err := saveEnrichedPage(enrichedDir, pf.pageNum, enriched); err != nil {
			logger.Error("failed to save enriched page", "page", pf.pageNum, "error", err)
			opts.Tracker.MarkFailed(phaseName, pf.pageNum)
			failed++
			continue
		}

		opts.Tracker.MarkCompleted(phaseName, pf.pageNum)
		if err := opts.Tracker.Save(); err != nil {
			logger.Error("failed to save progress", "error", err)
		}
		completed++
	}

	logger.Info("enrichment complete", "input", stem, "completed", completed, "failed", failed, "skipped", skipped)
	return nil
}

func loadExtractedPage(path string) (*model.ExtractedPage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var page model.ExtractedPage
	if err := json.Unmarshal(data, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

func saveEnrichedPage(dir string, pageNum int, page *model.EnrichedPage) error {
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
