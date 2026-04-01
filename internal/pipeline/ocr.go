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
	"github.com/mmdemirbas/mutercim/internal/ocr"
	"github.com/mmdemirbas/mutercim/internal/rebuild"
	"github.com/mmdemirbas/mutercim/internal/workspace"
)

// OCROptions configures the OCR pipeline phase.
type OCROptions struct {
	Workspace *workspace.Workspace
	Config    *config.Config
	Tool      ocr.Tool
	Pages     []int // CLI override pages; nil means use per-input or global config
	Force     bool  // force re-processing of already completed pages
	Logger    *slog.Logger
	Display   display.Display
}

// OCR runs the OCR phase: extracts text from page images using the configured OCR tool.
// If layout data exists, it OCRs individual regions; otherwise it OCRs the full page.
func OCR(ctx context.Context, opts OCROptions) (PhaseResult, error) {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	if opts.Tool == nil || opts.Tool.Name() == "" {
		logger.Info("ocr tool disabled, skipping ocr phase")
		return PhaseResult{}, nil
	}

	// Start OCR tool
	if err := opts.Tool.Start(ctx); err != nil {
		return PhaseResult{}, fmt.Errorf("start ocr tool: %w", err)
	}
	// Do NOT stop the tool here — it stays running for potential re-runs.
	// Container is stopped at pipeline exit.

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
		logger.Info("running ocr", "input", stem, "tool", opts.Tool.Name())

		pages := opts.Pages
		if len(pages) == 0 {
			if pp, ok := inputPages[stem]; ok {
				pages = pp
			}
		}

		result, err := ocrOneInput(ctx, opts, stem, pages)
		total.Completed += result.Completed
		total.Failed += result.Failed
		total.Skipped += result.Skipped
		if err != nil {
			logger.Error("ocr failed", "input", stem, "error", err)
		}
	}

	return total, nil
}

func ocrOneInput(ctx context.Context, opts OCROptions, stem string, pages []int) (PhaseResult, error) {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	ws := opts.Workspace
	imagesDir := filepath.Join(ws.CutDir(), stem)
	ocrDir := filepath.Join(ws.OcrDir(), stem)
	layoutDir := filepath.Join(ws.LayoutDir(), stem)

	// List available images
	images, err := input.ListImages(imagesDir)
	if err != nil {
		return PhaseResult{}, fmt.Errorf("list images in %s: %w", imagesDir, err)
	}
	if len(images) == 0 {
		return PhaseResult{}, fmt.Errorf("no images found in %s", imagesDir)
	}

	logger.Info("found images for ocr", "count", len(images), "input", stem)

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

	if err := os.MkdirAll(ocrDir, 0750); err != nil {
		return PhaseResult{}, fmt.Errorf("create ocr dir: %w", err)
	}

	// Start progress display
	if opts.Display != nil {
		opts.Display.StartPhase(display.PhaseOCR, stem, len(pagesToProcess), "")
	}

	completed := 0
	failed := 0
	skipped := 0
	var totalMs int64
	var totalChars int

	for _, pageNum := range pagesToProcess {
		if ctx.Err() != nil {
			break
		}

		imgPath, ok := imageMap[pageNum]
		if !ok {
			continue
		}

		// Check for layout data
		layoutPath := filepath.Join(layoutDir, pageFilename(pageNum, len(pagesToProcess)))
		var layoutPage *model.LayoutPage
		if lp, err := loadLayoutPage(layoutPath); err == nil {
			layoutPage = lp
		}

		// Skip if output is up-to-date
		outputPath := filepath.Join(ocrDir, pageFilename(pageNum, len(pagesToProcess)))
		rebuildInputs := []string{imgPath, ws.ConfigPath()}
		if layoutPage != nil {
			rebuildInputs = append(rebuildInputs, layoutPath)
		}
		if !opts.Force && !rebuild.NeedsRebuild(outputPath, rebuildInputs...) {
			logger.Debug("skipping page (up-to-date)", "input", stem, "page", pageNum)
			skipped++
			continue
		}

		// Set status
		if opts.Display != nil {
			regionCount := 0
			if layoutPage != nil {
				regionCount = len(layoutPage.Regions)
			}
			statusText := fmt.Sprintf("page %d: %s", pageNum, opts.Tool.Name())
			if regionCount > 0 {
				statusText = fmt.Sprintf("page %d: %s (%d regions)", pageNum, opts.Tool.Name(), regionCount)
			}
			opts.Display.SetStatus(display.StatusLine{
				Text:      statusText,
				StartedAt: time.Now(),
			})
		}

		// Run OCR
		var ocrResult *ocr.Result
		var ocrErr error

		if layoutPage != nil && len(layoutPage.Regions) > 0 {
			logger.Debug("ocr with layout regions", "page", pageNum, "regions", len(layoutPage.Regions))
			// Convert layout regions to OCR region inputs (using corner bbox format)
			regions := make([]ocr.RegionInput, len(layoutPage.Regions))
			for i, lr := range layoutPage.Regions {
				regions[i] = ocr.RegionInput{
					ID: lr.ID,
					BBox: [4]int{
						lr.BBox[0],              // x1
						lr.BBox[1],              // y1
						lr.BBox[0] + lr.BBox[2], // x2 = x + w
						lr.BBox[1] + lr.BBox[3], // y2 = y + h
					},
				}
			}
			ocrResult, ocrErr = opts.Tool.RecognizeRegions(ctx, imgPath, regions)
		} else {
			logger.Debug("ocr full page (no layout)", "page", pageNum)
			ocrResult, ocrErr = opts.Tool.RecognizeFullPage(ctx, imgPath)
		}

		if opts.Display != nil {
			opts.Display.SetStatus(display.StatusLine{})
		}

		if ocrErr != nil {
			logger.Error("page ocr failed", "page", pageNum, "tool", opts.Tool.Name(), "error", ocrErr)
			failed++
			if opts.Display != nil {
				opts.Display.Update(display.PageResult{
					Phase: display.PhaseOCR, Input: stem, PageNum: pageNum,
					Total: len(pagesToProcess), Completed: completed, Failed: failed, Err: ocrErr,
				})
			}
			continue
		}

		// Build OCR page output
		ocrPage := &model.OCRPage{
			Version:    "1.0",
			PageNumber: pageNum,
			Tool:       opts.Tool.Name(),
			Model:      ocrResult.Model,
			ElapsedMs:  ocrResult.TotalMs,
		}

		pageChars := 0
		if len(ocrResult.Regions) > 0 {
			ocrPage.Regions = make([]model.OCRRegion, len(ocrResult.Regions))
			for i, r := range ocrResult.Regions {
				ocrPage.Regions[i] = model.OCRRegion{
					ID:        r.ID,
					Text:      r.Text,
					ElapsedMs: r.ElapsedMs,
				}
				pageChars += len(r.Text)
			}
		} else {
			ocrPage.FullText = ocrResult.FullText
			pageChars = len(ocrResult.FullText)
		}

		// Save atomically
		data, err := json.MarshalIndent(ocrPage, "", "  ")
		if err != nil {
			logger.Error("failed to marshal ocr page", "page", pageNum, "error", err)
			failed++
			continue
		}
		if err := atomicWriteFile(outputPath, data); err != nil {
			logger.Error("failed to save ocr page", "page", pageNum, "error", err)
			failed++
			continue
		}

		totalMs += int64(ocrResult.TotalMs)
		totalChars += pageChars
		completed++

		regionCount := len(ocrResult.Regions)
		logger.Info("page ocr complete",
			"page", pageNum, "tool", opts.Tool.Name(),
			"regions", regionCount, "chars", pageChars,
			"elapsed_ms", ocrResult.TotalMs,
		)

		if opts.Display != nil {
			opts.Display.Update(display.PageResult{
				Phase: display.PhaseOCR, Input: stem, PageNum: pageNum,
				Total: len(pagesToProcess), Completed: completed, Failed: failed,
			})
		}
	}

	if opts.Display != nil {
		opts.Display.SetStatus(display.StatusLine{})
		opts.Display.FinishPhase(display.PhaseOCR, stem, "")
	}

	// Write report
	avgMs := 0
	if completed > 0 {
		avgMs = int(totalMs / int64(completed))
	}
	report := model.OCRReport{
		Tool:            opts.Tool.Name(),
		PagesProcessed:  completed,
		PagesFailed:     failed,
		AvgMs:           avgMs,
		TotalCharacters: totalChars,
	}
	if data, err := json.MarshalIndent(report, "", "  "); err == nil {
		if err := atomicWriteFile(filepath.Join(ocrDir, "report.json"), data); err != nil {
			logger.Warn("failed to write ocr report", "error", err)
		}
	}

	logger.Info("ocr phase complete",
		"input", stem, "pages", completed, "failed", failed,
		"avg_ms", avgMs, "total_chars", totalChars,
	)

	return PhaseResult{Completed: completed, Failed: failed, Skipped: skipped}, nil
}

// loadOCRPage loads an OCR page JSON file.
func loadOCRPage(path string) (*model.OCRPage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var page model.OCRPage
	if err := json.Unmarshal(data, &page); err != nil {
		return nil, err
	}
	return &page, nil
}
