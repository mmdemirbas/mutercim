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
	"github.com/mmdemirbas/mutercim/internal/input"
	"github.com/mmdemirbas/mutercim/internal/layout"
	"github.com/mmdemirbas/mutercim/internal/model"
	"github.com/mmdemirbas/mutercim/internal/provider"
	"github.com/mmdemirbas/mutercim/internal/reader"
	"github.com/mmdemirbas/mutercim/internal/rebuild"
	"github.com/mmdemirbas/mutercim/internal/workspace"
)

// ReadOptions configures the read pipeline.
type ReadOptions struct {
	Workspace  *workspace.Workspace
	Config     *config.Config
	Provider   provider.Provider
	LayoutTool layout.Tool // optional layout detection tool; nil means AI-only
	Pages      []int       // CLI override pages; nil means use per-input or global config
	Force      bool        // force re-processing of already completed pages
	Logger     *slog.Logger
	Display    display.Display
}

// Read runs the read (OCR) pipeline for all configured inputs.
// Images must already exist in pages/ (run 'mutercim pages' first).
func Read(ctx context.Context, opts ReadOptions) (PhaseResult, error) {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	// Discover input stems from images directory
	stems, err := discoverSubdirs(opts.Workspace.PagesDir())
	if err != nil {
		return PhaseResult{}, fmt.Errorf("discover images: %w", err)
	}
	if len(stems) == 0 {
		return PhaseResult{}, fmt.Errorf("no page images found in %s — run 'mutercim pages' first", opts.Workspace.PagesDir())
	}

	// Build per-input page lookup from config
	inputPages := buildInputPageMap(opts.Config)

	var total PhaseResult
	for _, stem := range stems {
		logger.Info("processing input", "input", stem)

		// Determine effective pages: CLI override > per-input config > global config > all
		pages := opts.Pages
		if len(pages) == 0 {
			if pp, ok := inputPages[stem]; ok {
				pages = pp
			}
		}

		result, err := readOneInput(ctx, opts, stem, pages)
		total.Completed += result.Completed
		total.Failed += result.Failed
		total.Skipped += result.Skipped
		if err != nil {
			logger.Error("input failed", "input", stem, "error", err)
		}
	}

	return total, nil
}

// buildInputPageMap maps input stems to their configured page lists.
func buildInputPageMap(cfg *config.Config) map[string][]int {
	m := make(map[string][]int)
	for _, inp := range cfg.Inputs {
		if inp.Pages != "" {
			stem := fileStem(inp.Path)
			if ranges, err := model.ParsePageRanges(inp.Pages); err == nil {
				if pages, err := model.ExpandPages(ranges); err == nil {
					m[stem] = pages
				}
			}
		}
	}
	return m
}

func readOneInput(ctx context.Context, opts ReadOptions, stem string, pages []int) (PhaseResult, error) {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	ws := opts.Workspace
	cfg := opts.Config

	imagesDir := filepath.Join(ws.PagesDir(), stem)
	readDir := filepath.Join(ws.ReadDir(), stem)

	// List available images
	images, err := input.ListImages(imagesDir)
	if err != nil {
		return PhaseResult{}, fmt.Errorf("list images in %s: %w", imagesDir, err)
	}
	if len(images) == 0 {
		return PhaseResult{}, fmt.Errorf("no images found in %s", imagesDir)
	}

	logger.Info("found images", "count", len(images), "input", stem)

	// Build page->image map
	imageMap := make(map[int]string)
	for _, img := range images {
		imageMap[img.PageNumber] = img.Path
	}

	// Determine pages to process
	pagesToProcess := pages
	if len(pagesToProcess) == 0 {
		for _, img := range images {
			pagesToProcess = append(pagesToProcess, img.PageNumber)
		}
	}

	// Create reader
	rdr := reader.NewReader(opts.Provider, logger)

	// Ensure output directory exists
	if err := os.MkdirAll(readDir, 0755); err != nil {
		return PhaseResult{}, fmt.Errorf("create read dir: %w", err)
	}

	// Start progress display
	if opts.Display != nil {
		opts.Display.StartPhase(display.PhaseRead, stem, len(pagesToProcess), "")
	}

	// Set up status callbacks for retry/failover display
	readModel := ""
	if len(cfg.Read.Models) > 0 {
		readModel = cfg.Read.Models[0].Provider + "/" + cfg.Read.Models[0].Model
	}
	var statusPageNum int
	if opts.Display != nil {
		if chain, ok := opts.Provider.(*provider.FailoverChain); ok {
			chain.OnFailover = func(from, to string) {
				opts.Display.SetStatus(display.StatusLine{
					Text:      fmt.Sprintf("reading page %d via %s \u2014 failover from %s", statusPageNum, to, from),
					StartedAt: time.Now(),
				})
			}
			chain.SetRetryCallback(func(attempt, maxRetries, statusCode int, backoff time.Duration) {
				opts.Display.SetStatus(display.StatusLine{
					Text:      fmt.Sprintf("reading page %d \u2014 retry %d/%d (%d)", statusPageNum, attempt, maxRetries, statusCode),
					StartedAt: time.Now(),
					Countdown: backoff,
				})
			})
		}
	}

	// Process pages
	completed := 0
	failed := 0
	skipped := 0
	for _, pageNum := range pagesToProcess {
		if ctx.Err() != nil {
			break
		}
		// Skip pages not in the image set
		imgPath, ok := imageMap[pageNum]
		if !ok {
			logger.Warn("no image found for page", "input", stem, "page", pageNum)
			continue
		}

		// Skip if output is up-to-date (mtime check)
		outputPath := filepath.Join(readDir, fmt.Sprintf("%03d.json", pageNum))
		if !opts.Force && !rebuild.NeedsRebuild(outputPath, imgPath, ws.ConfigPath(), ws.KnowledgeDir()) {
			logger.Debug("skipping page (up-to-date)", "input", stem, "page", pageNum)
			skipped++
			continue
		}

		// Load image
		imageData, err := input.LoadImage(imgPath)
		if err != nil {
			logger.Error("failed to load image", "input", stem, "page", pageNum, "error", err)
			failed++
			if opts.Display != nil {
				opts.Display.Update(display.PageResult{
					Phase: display.PhaseRead, Input: stem, PageNum: pageNum,
					Total: len(pagesToProcess), Completed: completed, Failed: failed, Err: err,
				})
			}
			continue
		}

		// Read page via AI (region-based)
		statusPageNum = pageNum
		if opts.Display != nil {
			opts.Display.SetStatus(display.StatusLine{
				Text:      fmt.Sprintf("reading page %d via %s", pageNum, readModel),
				StartedAt: time.Now(),
			})
		}
		regionPage, err := rdr.ReadRegionPage(ctx, imageData, imgPath, pageNum, readModel, opts.LayoutTool)
		if opts.Display != nil {
			opts.Display.SetStatus(display.StatusLine{}) // clear status
		}
		if err != nil {
			logger.Error("read failed", "input", stem, "page", pageNum, "error", err)
			failed++
			if opts.Display != nil {
				opts.Display.Update(display.PageResult{
					Phase: display.PhaseRead, Input: stem, PageNum: pageNum,
					Total: len(pagesToProcess), Completed: completed, Failed: failed, Err: err,
				})
			}
			continue
		}

		// Save region page atomically (new v2.0 format)
		if err := saveRegionPage(readDir, pageNum, regionPage); err != nil {
			logger.Error("failed to save read page", "input", stem, "page", pageNum, "error", err)
			failed++
			if opts.Display != nil {
				opts.Display.Update(display.PageResult{
					Phase: display.PhaseRead, Input: stem, PageNum: pageNum,
					Total: len(pagesToProcess), Completed: completed, Failed: failed, Err: err,
				})
			}
			continue
		}

		// Count region types for display
		entryCount, footnoteCount := countRegionTypes(regionPage.Regions)

		completed++
		logger.Info("page read", "input", stem, "page", pageNum, "completed", completed)
		if opts.Display != nil {
			opts.Display.Update(display.PageResult{
				Phase: display.PhaseRead, Input: stem, PageNum: pageNum,
				Total: len(pagesToProcess), Completed: completed, Failed: failed,
				Warnings: len(regionPage.Warnings), Entries: entryCount, Footnotes: footnoteCount,
			})
		}
	}

	if opts.Display != nil {
		opts.Display.FinishPhase(display.PhaseRead, stem, "")
	}
	logger.Info("input read complete", "input", stem, "completed", completed, "failed", failed, "skipped", skipped)
	return PhaseResult{Completed: completed, Failed: failed, Skipped: skipped}, nil
}

func saveRegionPage(dir string, pageNum int, page *model.RegionPage) error {
	data, err := json.MarshalIndent(page, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal page %d: %w", pageNum, err)
	}

	finalPath := filepath.Join(dir, fmt.Sprintf("%03d.json", pageNum))
	if err := atomicWriteFile(finalPath, data); err != nil {
		return fmt.Errorf("write page %d: %w", pageNum, err)
	}
	return nil
}

// countRegionTypes counts entries and footnotes in a list of regions.
func countRegionTypes(regions []model.Region) (entries, footnotes int) {
	for _, r := range regions {
		switch r.Type {
		case model.RegionTypeEntry:
			entries++
		case model.RegionTypeFootnote:
			footnotes++
		}
	}
	return
}
