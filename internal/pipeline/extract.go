package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

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

// Extract runs the Phase 1 extraction pipeline.
func Extract(ctx context.Context, opts ExtractOptions) error {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	ws := opts.Workspace
	cfg := opts.Config

	// Convert PDF to images if needed
	if cfg.InputIsPDF() {
		inputPath := cfg.ResolvePath(ws.Root, cfg.Input)
		if err := os.MkdirAll(ws.ImagesDir(), 0755); err != nil {
			return fmt.Errorf("create images dir: %w", err)
		}
		logger.Info("converting PDF to images", "input", inputPath, "dpi", cfg.DPI)
		if err := input.ConvertPDFToImages(ctx, inputPath, ws.ImagesDir(), cfg.DPI, 0, 0); err != nil {
			return fmt.Errorf("convert PDF: %w", err)
		}
	}

	// List available images
	images, err := input.ListImages(ws.ImagesDir())
	if err != nil {
		return fmt.Errorf("list images: %w", err)
	}
	if len(images) == 0 {
		return fmt.Errorf("no images found in %s", ws.ImagesDir())
	}

	logger.Info("found images", "count", len(images))

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
	if err := os.MkdirAll(ws.ExtractedDir(), 0755); err != nil {
		return fmt.Errorf("create extracted dir: %w", err)
	}

	// Process pages
	completed := 0
	failed := 0
	skipped := 0
	for _, pageNum := range pagesToProcess {
		// Skip already completed pages
		state := opts.Tracker.State()
		if phase := state.Phases[progress.PhaseExtract]; phase != nil {
			if containsInt(phase.Completed, pageNum) {
				logger.Debug("skipping already completed page", "page", pageNum)
				skipped++
				continue
			}
		}

		// Skip pages not in the image set
		imgPath, ok := imageMap[pageNum]
		if !ok {
			logger.Warn("no image found for page", "page", pageNum)
			continue
		}

		// Determine section type
		sectionType := string(model.SectionAuto)
		if lookup != nil {
			sec := lookup.ForPage(pageNum)
			if sec.Type == model.SectionSkip {
				logger.Debug("skipping page (section: skip)", "page", pageNum)
				skipped++
				continue
			}
			sectionType = string(sec.Type)
		}

		// Load image
		imageData, err := input.LoadImage(imgPath)
		if err != nil {
			logger.Error("failed to load image", "page", pageNum, "error", err)
			opts.Tracker.MarkFailed(progress.PhaseExtract, pageNum)
			failed++
			continue
		}

		// Extract
		page, err := extractor.ExtractPage(ctx, imageData, pageNum, sectionType, cfg.Extract.Model)
		if err != nil {
			logger.Error("extraction failed", "page", pageNum, "error", err)
			opts.Tracker.MarkFailed(progress.PhaseExtract, pageNum)
			failed++
			continue
		}

		// Save result atomically
		if err := saveExtractedPage(ws.ExtractedDir(), pageNum, page); err != nil {
			logger.Error("failed to save extracted page", "page", pageNum, "error", err)
			opts.Tracker.MarkFailed(progress.PhaseExtract, pageNum)
			failed++
			continue
		}

		opts.Tracker.MarkCompleted(progress.PhaseExtract, pageNum)
		if err := opts.Tracker.Save(); err != nil {
			logger.Error("failed to save progress", "error", err)
		}
		completed++
		logger.Info("page extracted", "page", pageNum, "completed", completed)
	}

	logger.Info("extraction complete", "completed", completed, "failed", failed, "skipped", skipped)
	return nil
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

func containsInt(s []int, v int) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
