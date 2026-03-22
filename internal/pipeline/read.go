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
	"github.com/mmdemirbas/mutercim/internal/model"
	"github.com/mmdemirbas/mutercim/internal/provider"
	"github.com/mmdemirbas/mutercim/internal/reader"
	"github.com/mmdemirbas/mutercim/internal/rebuild"
	"github.com/mmdemirbas/mutercim/internal/workspace"
)

// ReadOptions configures the read pipeline.
type ReadOptions struct {
	Workspace *workspace.Workspace
	Config    *config.Config
	Provider  provider.Provider
	Pages     []int // CLI override pages; nil means use per-input or global config
	Force     bool  // force re-processing of already completed pages
	Logger    *slog.Logger
	Display   display.Display
}

// Read runs the read (OCR) pipeline for all configured inputs.
// Images must already exist in cut/ (run 'mutercim cut' first).
func Read(ctx context.Context, opts ReadOptions) (PhaseResult, error) {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	// Discover input stems from images directory
	stems, err := discoverSubdirs(opts.Workspace.CutDir())
	if err != nil {
		return PhaseResult{}, fmt.Errorf("discover images: %w", err)
	}
	if len(stems) == 0 {
		return PhaseResult{}, fmt.Errorf("no page images found in %s — run 'mutercim cut' first", opts.Workspace.CutDir())
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

	imagesDir := filepath.Join(ws.CutDir(), stem)
	readDir := filepath.Join(ws.ReadDir(), stem)
	layoutDir := filepath.Join(ws.LayoutDir(), stem)
	ocrDir := filepath.Join(ws.OcrDir(), stem)

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

	// Process pages
	completed := 0
	failed := 0
	skipped := 0
	maxFailPct := cfg.Read.Retry.MaxFailPercent
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

		// Check for pre-existing layout data
		layoutPath := filepath.Join(layoutDir, pageFilename(pageNum, len(pagesToProcess)))
		var layoutRegions []model.Region
		var layoutToolName string
		hasLayout := false

		if layoutPage, err := loadLayoutPage(layoutPath); err == nil {
			layoutRegions = LayoutRegionsToModelRegions(layoutPage.Regions)
			layoutToolName = layoutPage.Tool
			hasLayout = true
		}

		// Check for pre-existing OCR data
		ocrPath := filepath.Join(ocrDir, pageFilename(pageNum, len(pagesToProcess)))
		var ocrPage *model.OCRPage
		hasOCR := false
		if op, err := loadOCRPage(ocrPath); err == nil {
			ocrPage = op
			hasOCR = true
		}

		// Skip if output is up-to-date (mtime check)
		// When OCR is available, read depends on OCR output instead of page images
		outputPath := filepath.Join(readDir, pageFilename(pageNum, len(pagesToProcess)))
		var rebuildInputs []string
		if hasOCR {
			rebuildInputs = append([]string{ocrPath, ws.ConfigPath()}, cfg.ResolveKnowledgePaths(ws.Root)...)
		} else {
			rebuildInputs = append([]string{imgPath, ws.ConfigPath()}, cfg.ResolveKnowledgePaths(ws.Root)...)
		}
		if hasLayout {
			rebuildInputs = append(rebuildInputs, layoutPath)
		}
		if !opts.Force && !rebuild.NeedsRebuild(outputPath, rebuildInputs...) {
			logger.Debug("skipping page (up-to-date)", "input", stem, "page", pageNum)
			skipped++
			continue
		}

		// Set status before AI call
		statusPageNum = pageNum
		currentModel := activeModel()

		var readResult *reader.ReadResult

		if hasOCR {
			// OCR provides text — use text-only LLM path (no vision needed)
			ocrToolName := ocrPage.Tool

			if opts.Display != nil {
				statusText := fmt.Sprintf("page %d: reading via %s (ocr: %s)", pageNum, currentModel, ocrToolName)
				opts.Display.SetStatus(display.StatusLine{
					Text:      statusText,
					StartedAt: time.Now(),
				})
			}

			logger.Info("reading page", "page", pageNum, "layout", hasLayout, "ocr_source", ocrToolName)

			if hasLayout && len(ocrPage.Regions) > 0 {
				// Case 1: layout + OCR — build OCR region data with layout bboxes/types
				ocrRegions := make([]reader.OCRRegionData, 0, len(ocrPage.Regions))
				// Build a lookup from OCR region results
				for _, ocrReg := range ocrPage.Regions {
					// Find matching layout region for bbox and type
					var bbox model.BBox
					var regionType string
					for _, lr := range layoutRegions {
						if lr.ID == ocrReg.ID {
							bbox = lr.BBox
							regionType = lr.Type
							break
						}
					}
					ocrRegions = append(ocrRegions, reader.OCRRegionData{
						ID:   ocrReg.ID,
						Text: ocrReg.Text,
						BBox: bbox,
						Type: regionType,
					})
				}
				readResult, err = rdr.ReadRegionPageWithOCR(ctx, pageNum, currentModel,
					ocrRegions, "", layoutToolName, ocrToolName)
			} else {
				// Case 3: OCR text without layout — full text segmentation
				readResult, err = rdr.ReadRegionPageWithOCR(ctx, pageNum, currentModel,
					nil, ocrPage.FullText, "", ocrToolName)
			}
		} else {
			// No OCR — use vision LLM path (existing behavior)

			// Load image
			imageData, err2 := input.LoadImage(imgPath)
			if err2 != nil {
				logger.Error("failed to load image", "input", stem, "page", pageNum, "error", err2)
				failed++
				if opts.Display != nil {
					opts.Display.Update(display.PageResult{
						Phase: display.PhaseRead, Input: stem, PageNum: pageNum,
						Total: len(pagesToProcess), Completed: completed, Failed: failed, Err: err2,
					})
				}
				continue
			}

			if opts.Display != nil {
				statusText := fmt.Sprintf("page %d: reading via %s", pageNum, currentModel)
				if hasLayout {
					statusText = fmt.Sprintf("page %d: %s + %s ...", pageNum, layoutToolName, currentModel)
				}
				opts.Display.SetStatus(display.StatusLine{
					Text:      statusText,
					StartedAt: time.Now(),
				})
			}

			logger.Info("reading page", "page", pageNum, "layout", hasLayout)

			readResult, err = rdr.ReadRegionPage(ctx, imageData, pageNum, currentModel, layoutRegions, layoutToolName)
		}

		// err is set by whichever path was taken
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

		// Pages with no regions (e.g. JSON extraction failed) count as failed
		if len(regionPage.Regions) == 0 && len(regionPage.Warnings) > 0 {
			logger.Warn("page read produced no regions", "input", stem, "page", pageNum, "warnings", regionPage.Warnings)
			failed++
			if opts.Display != nil {
				opts.Display.Update(display.PageResult{
					Phase: display.PhaseRead, Input: stem, PageNum: pageNum,
					Total: len(pagesToProcess), Completed: completed, Failed: failed,
					Err: fmt.Errorf("%s", regionPage.Warnings[0]),
				})
			}
			continue
		}

		// Count region types for display
		entryCount, footnoteCount := countRegionTypes(regionPage.Regions)

		completed++

		// Use the model that actually served the request (reflects failover)
		usedModel := currentModel
		if chain, ok := opts.Provider.(*provider.FailoverChain); ok {
			if m := chain.LastUsedModel(); m != "" {
				usedModel = m
			}
		}

		logger.Info("page read complete",
			"input", stem,
			"page", pageNum,
			"layout", hasLayout,
			"model", usedModel,
			"regions", len(regionPage.Regions),
			"warnings", len(regionPage.Warnings),
		)

		if opts.Display != nil {
			opts.Display.Update(display.PageResult{
				Phase: display.PhaseRead, Input: stem, PageNum: pageNum,
				Total: len(pagesToProcess), Completed: completed, Failed: failed,
				Warnings:      len(regionPage.Warnings),
				Entries:       entryCount,
				Footnotes:     footnoteCount,
				LayoutTool:    layoutToolName,
				LayoutRegions: len(layoutRegions),
			})
		}
	}

	if opts.Display != nil {
		opts.Display.FinishPhase(display.PhaseRead, stem, "")
	}
	logger.Info("input read complete", "input", stem, "completed", completed, "failed", failed, "skipped", skipped)

	// Write report
	report := map[string]any{
		"pages_completed": completed,
		"pages_failed":    failed,
		"pages_skipped":   skipped,
	}
	if data, err := json.MarshalIndent(report, "", "  "); err == nil {
		if err := atomicWriteFile(filepath.Join(readDir, "report.json"), data); err != nil {
			logger.Warn("failed to write read report", "error", err)
		}
	}

	return PhaseResult{Completed: completed, Failed: failed, Skipped: skipped}, nil
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
