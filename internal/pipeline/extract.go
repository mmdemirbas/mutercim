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
	"github.com/mmdemirbas/mutercim/internal/extraction"
	"github.com/mmdemirbas/mutercim/internal/input"
	"github.com/mmdemirbas/mutercim/internal/model"
	"github.com/mmdemirbas/mutercim/internal/progress"
	"github.com/mmdemirbas/mutercim/internal/provider"
	"github.com/mmdemirbas/mutercim/internal/workspace"
)

// ExtractOptions configures the extraction pipeline.
type ExtractOptions struct {
	Workspace *workspace.Workspace
	Config    *config.Config
	Provider  provider.Provider
	Tracker   *progress.Tracker
	Pages     []int // specific pages to process; nil means all available
	Logger    *slog.Logger
}

// Extract runs the Phase 1 extraction pipeline for all configured inputs.
func Extract(ctx context.Context, opts ExtractOptions) error {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	inputs := opts.Config.Inputs
	if len(inputs) == 0 {
		return fmt.Errorf("no inputs configured")
	}

	for _, inputPath := range inputs {
		resolved := opts.Config.ResolvePath(opts.Workspace.Root, inputPath)
		stem := fileStem(inputPath)
		logger.Info("processing input", "input", inputPath, "stem", stem)

		if err := extractOneInput(ctx, opts, resolved, stem); err != nil {
			logger.Error("input failed", "input", inputPath, "error", err)
			// Continue to next input — don't abort the whole run
		}
	}

	return nil
}

func extractOneInput(ctx context.Context, opts ExtractOptions, inputPath, stem string) error {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	ws := opts.Workspace
	cfg := opts.Config

	// Per-input subdirectories
	imagesDir := filepath.Join(ws.ImagesDir(), stem)
	extractedDir := filepath.Join(ws.ExtractedDir(), stem)

	// Convert PDF to images if needed
	if config.IsPDF(inputPath) {
		if err := os.MkdirAll(imagesDir, 0755); err != nil {
			return fmt.Errorf("create images dir: %w", err)
		}
		// Only convert requested pages, not the entire PDF
		firstPage, lastPage := 0, 0
		if len(opts.Pages) > 0 {
			firstPage = opts.Pages[0]
			lastPage = opts.Pages[len(opts.Pages)-1]
		}
		logger.Info("converting PDF to images", "input", inputPath, "dpi", cfg.DPI, "first", firstPage, "last", lastPage)
		if err := input.ConvertPDFToImages(ctx, inputPath, imagesDir, cfg.DPI, firstPage, lastPage); err != nil {
			return fmt.Errorf("convert PDF %s: %w", inputPath, err)
		}
	} else {
		// Non-PDF: use the path directly as image directory
		imagesDir = inputPath
	}

	// List available images
	images, err := input.ListImages(imagesDir)
	if err != nil {
		return fmt.Errorf("list images in %s: %w", imagesDir, err)
	}
	if len(images) == 0 {
		return fmt.Errorf("no images found in %s", imagesDir)
	}

	logger.Info("found images", "count", len(images), "input", stem)

	// Build page→image map
	imageMap := make(map[int]string)
	for _, img := range images {
		imageMap[img.PageNumber] = img.Path
	}

	// Determine pages to process
	pagesToProcess := opts.Pages
	if len(pagesToProcess) == 0 {
		for _, img := range images {
			pagesToProcess = append(pagesToProcess, img.PageNumber)
		}
	}

	// Build section lookup
	lookup, err := config.NewSectionLookup(cfg.Sections)
	if err != nil {
		logger.Warn("section lookup error, using auto for all pages", "error", err)
	}

	// Create extractor
	extractor := extraction.NewExtractor(opts.Provider, logger)

	// Ensure output directory exists
	if err := os.MkdirAll(extractedDir, 0755); err != nil {
		return fmt.Errorf("create extracted dir: %w", err)
	}

	// Use compound phase name for per-input progress tracking
	phaseName := progress.PhaseName("extract:" + stem)

	// Process pages
	completed := 0
	failed := 0
	skipped := 0
	for _, pageNum := range pagesToProcess {
		// Skip already completed pages
		state := opts.Tracker.State()
		if phase := state.Phases[phaseName]; phase != nil {
			if containsInt(phase.Completed, pageNum) {
				logger.Debug("skipping already completed page", "input", stem, "page", pageNum)
				skipped++
				continue
			}
		}

		// Skip pages not in the image set
		imgPath, ok := imageMap[pageNum]
		if !ok {
			logger.Warn("no image found for page", "input", stem, "page", pageNum)
			continue
		}

		// Determine section type
		sectionType := string(model.SectionAuto)
		if lookup != nil {
			sec := lookup.ForPage(pageNum)
			if sec.Type == model.SectionSkip {
				logger.Debug("skipping page (section: skip)", "input", stem, "page", pageNum)
				skipped++
				continue
			}
			sectionType = string(sec.Type)
		}

		// Load image
		imageData, err := input.LoadImage(imgPath)
		if err != nil {
			logger.Error("failed to load image", "input", stem, "page", pageNum, "error", err)
			opts.Tracker.MarkFailed(phaseName, pageNum)
			failed++
			continue
		}

		// Extract
		page, err := extractor.ExtractPage(ctx, imageData, pageNum, sectionType, cfg.Extract.Model)
		if err != nil {
			logger.Error("extraction failed", "input", stem, "page", pageNum, "error", err)
			opts.Tracker.MarkFailed(phaseName, pageNum)
			failed++
			continue
		}

		// Save result atomically
		if err := saveExtractedPage(extractedDir, pageNum, page); err != nil {
			logger.Error("failed to save extracted page", "input", stem, "page", pageNum, "error", err)
			opts.Tracker.MarkFailed(phaseName, pageNum)
			failed++
			continue
		}

		opts.Tracker.MarkCompleted(phaseName, pageNum)
		if err := opts.Tracker.Save(); err != nil {
			logger.Error("failed to save progress", "error", err)
		}
		completed++
		logger.Info("page extracted", "input", stem, "page", pageNum, "completed", completed)
	}

	logger.Info("input extraction complete", "input", stem, "completed", completed, "failed", failed, "skipped", skipped)
	return nil
}

// fileStem returns the filename without extension.
// e.g. "./input/Anfas1.pdf" → "Anfas1"
func fileStem(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext)
}

func saveExtractedPage(dir string, pageNum int, page *model.ExtractedPage) error {
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
