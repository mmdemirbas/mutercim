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

	// Layout tool name for display/logging
	layoutToolName := "ai-only"
	if opts.LayoutTool != nil && opts.LayoutTool.Name() != "" {
		layoutToolName = opts.LayoutTool.Name()
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
	var statusPageNum int
	activeModel := func() string {
		if chain, ok := opts.Provider.(*provider.FailoverChain); ok {
			if m := chain.ActiveModel(true); m != "" {
				return m
			}
		}
		if len(cfg.Read.Models) > 0 {
			return cfg.Read.Models[0].Provider + "/" + cfg.Read.Models[0].Model
		}
		return opts.Provider.Name()
	}
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

	// Set up layout status callback on the reader
	rdr.OnLayoutDone = func(tool string, ms int, regions int, layoutErr string) {
		if opts.Display == nil {
			return
		}
		currentModel := activeModel()
		if layoutErr != "" {
			// Layout failed — show failure and fallback
			opts.Display.SetStatus(display.StatusLine{
				Text:      fmt.Sprintf("page %d: %s \u2717 (%s) \u2192 ai-only \u2192 %s", statusPageNum, tool, truncateErr(layoutErr), currentModel),
				StartedAt: time.Now(),
			})
		} else if tool == "ai-only" {
			opts.Display.SetStatus(display.StatusLine{
				Text:      fmt.Sprintf("page %d: ai-only \u2192 %s", statusPageNum, currentModel),
				StartedAt: time.Now(),
			})
		} else {
			opts.Display.SetStatus(display.StatusLine{
				Text:      fmt.Sprintf("page %d: %s %dms \u2192 %s", statusPageNum, tool, ms, currentModel),
				StartedAt: time.Now(),
			})
		}
	}

	// Layout stats for report
	var layoutPagesProcessed, layoutPagesFailed int
	var layoutTotalMs int64

	// Process pages
	completed := 0
	failed := 0
	skipped := 0
	maxFailPct := cfg.Retry.MaxFailPercent
	for _, pageNum := range pagesToProcess {
		if ctx.Err() != nil {
			break
		}
		// Check error threshold
		result := PhaseResult{Completed: completed, Failed: failed}
		if result.ExceedsErrorThreshold(maxFailPct) {
			logger.Error("aborting: failure rate exceeds threshold",
				"completed", completed, "failed", failed, "threshold", fmt.Sprintf("%d%%", maxFailPct))
			break
		}
		// Skip pages not in the image set
		imgPath, ok := imageMap[pageNum]
		if !ok {
			logger.Warn("no image found for page", "input", stem, "page", pageNum)
			continue
		}

		// Skip if output is up-to-date (mtime check)
		outputPath := filepath.Join(readDir, pageFilename(pageNum, len(pagesToProcess)))
		rebuildInputs := append([]string{imgPath, ws.ConfigPath()}, cfg.ResolveKnowledgePaths(ws.Root)...)
		if !opts.Force && !rebuild.NeedsRebuild(outputPath, rebuildInputs...) {
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

		// Set status before layout detection begins
		statusPageNum = pageNum
		currentModel := activeModel()
		if opts.Display != nil {
			opts.Display.SetStatus(display.StatusLine{
				Text:      fmt.Sprintf("page %d: %s ...", pageNum, layoutToolName),
				StartedAt: time.Now(),
			})
		}

		readResult, err := rdr.ReadRegionPage(ctx, imageData, imgPath, pageNum, currentModel, opts.LayoutTool)
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

		regionPage := readResult.Page

		// Track layout stats
		if readResult.LayoutTool != "ai-only" || layoutToolName != "ai-only" {
			layoutPagesProcessed++
			layoutTotalMs += int64(readResult.LayoutMs)
			if readResult.LayoutError != "" {
				layoutPagesFailed++
			}
		}

		// Save region page atomically (new v2.0 format)
		if err := saveRegionPage(readDir, pageNum, len(pagesToProcess), regionPage); err != nil {
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

		// Structured log with layout fields (Change 5)
		logAttrs := []any{
			"input", stem,
			"page", pageNum,
			"layout_tool", readResult.LayoutTool,
			"model", currentModel,
			"regions", len(regionPage.Regions),
			"warnings", len(regionPage.Warnings),
		}
		if readResult.LayoutTool != "ai-only" {
			logAttrs = append(logAttrs, "layout_ms", readResult.LayoutMs, "layout_regions", readResult.LayoutRegions)
		}
		logger.Info("page read complete", logAttrs...)

		if opts.Display != nil {
			opts.Display.Update(display.PageResult{
				Phase: display.PhaseRead, Input: stem, PageNum: pageNum,
				Total: len(pagesToProcess), Completed: completed, Failed: failed,
				Warnings:      len(regionPage.Warnings),
				Entries:       entryCount,
				Footnotes:     footnoteCount,
				LayoutTool:    readResult.LayoutTool,
				LayoutMs:      readResult.LayoutMs,
				LayoutRegions: readResult.LayoutRegions,
				LayoutError:   readResult.LayoutError,
			})
		}
	}

	if opts.Display != nil {
		opts.Display.FinishPhase(display.PhaseRead, stem, "")
	}
	logger.Info("input read complete", "input", stem, "completed", completed, "failed", failed, "skipped", skipped)

	// Write layout report (Change 6)
	if completed > 0 {
		avgMs := 0
		if layoutPagesProcessed > 0 {
			avgMs = int(layoutTotalMs / int64(layoutPagesProcessed))
		}
		report := readReport{
			LayoutTool: layoutToolName,
			LayoutStats: layoutStats{
				PagesProcessed: layoutPagesProcessed,
				PagesFailed:    layoutPagesFailed,
				AvgMs:          avgMs,
			},
		}
		if err := saveReadReport(readDir, report); err != nil {
			logger.Warn("failed to write read report", "error", err)
		}
	}

	return PhaseResult{Completed: completed, Failed: failed, Skipped: skipped}, nil
}

// truncateErr shortens an error string for display in the status line.
func truncateErr(s string) string {
	if len(s) > 30 {
		return s[:27] + "..."
	}
	return s
}

// readReport is written to read/{stem}/report.json after processing.
type readReport struct {
	LayoutTool  string      `json:"layout_tool"`
	LayoutStats layoutStats `json:"layout_stats"`
}

type layoutStats struct {
	PagesProcessed int `json:"pages_processed"`
	PagesFailed    int `json:"pages_failed"`
	AvgMs          int `json:"avg_ms"`
}

func saveReadReport(readDir string, report readReport) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}
	return atomicWriteFile(filepath.Join(readDir, "report.json"), data)
}

func saveRegionPage(dir string, pageNum, totalPages int, page *model.RegionPage) error {
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
