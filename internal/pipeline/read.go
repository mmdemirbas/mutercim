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
	"github.com/mmdemirbas/mutercim/internal/input"
	"github.com/mmdemirbas/mutercim/internal/model"
	"github.com/mmdemirbas/mutercim/internal/progress"
	"github.com/mmdemirbas/mutercim/internal/provider"
	"github.com/mmdemirbas/mutercim/internal/reader"
	"github.com/mmdemirbas/mutercim/internal/workspace"
)

// ReadOptions configures the read pipeline.
type ReadOptions struct {
	Workspace *workspace.Workspace
	Config    *config.Config
	Provider  provider.Provider
	Tracker   *progress.Tracker
	Pages     []int // CLI override pages; nil means use per-input or global config
	Logger    *slog.Logger
}

// Read runs the Phase 1 read pipeline for all configured inputs.
func Read(ctx context.Context, opts ReadOptions) error {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	inputs := opts.Config.Inputs
	if len(inputs) == 0 {
		return fmt.Errorf("no inputs configured")
	}

	for _, inp := range inputs {
		resolved := opts.Config.ResolvePath(opts.Workspace.Root, inp.Path)
		stem := fileStem(inp.Path)
		logger.Info("processing input", "input", inp.Path, "stem", stem)

		// Determine effective pages: CLI override > per-input > global config > all
		pages := opts.Pages
		if len(pages) == 0 && inp.Pages != "" {
			if ranges, err := model.ParsePageRanges(inp.Pages); err == nil {
				pages = model.ExpandPages(ranges)
			}
		}

		if err := readOneInput(ctx, opts, resolved, stem, pages); err != nil {
			logger.Error("input failed", "input", inp.Path, "error", err)
			// Continue to next input — don't abort the whole run
		}
	}

	return nil
}

func readOneInput(ctx context.Context, opts ReadOptions, inputPath, stem string, pages []int) error {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	ws := opts.Workspace
	cfg := opts.Config

	// Per-input subdirectories
	imagesDir := filepath.Join(ws.ImagesDir(), stem)
	readDir := filepath.Join(ws.ReadDir(), stem)

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

	// Build section lookup
	lookup, err := config.NewSectionLookup(cfg.Sections)
	if err != nil {
		logger.Warn("section lookup error, using auto for all pages", "error", err)
	}

	// Create reader
	rdr := reader.NewReader(opts.Provider, logger)

	// Ensure output directory exists
	if err := os.MkdirAll(readDir, 0755); err != nil {
		return fmt.Errorf("create read dir: %w", err)
	}

	// Use compound phase name for per-input progress tracking
	phaseName := progress.PhaseName("read:" + stem)

	// Process pages
	completed := 0
	failed := 0
	skipped := 0
	for _, pageNum := range pagesToProcess {
		// Skip already completed pages — but only if the output file actually exists
		outputPath := filepath.Join(readDir, fmt.Sprintf("page_%03d.json", pageNum))
		state := opts.Tracker.State()
		if phase := state.Phases[phaseName]; phase != nil {
			if containsInt(phase.Completed, pageNum) {
				if fileExists(outputPath) {
					logger.Debug("skipping already completed page", "input", stem, "page", pageNum)
					skipped++
					continue
				}
				logger.Warn("progress says completed but output missing, re-processing", "input", stem, "page", pageNum)
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

		// Read page via AI
		page, err := rdr.ReadPage(ctx, imageData, pageNum, sectionType, cfg.Read.Model)
		if err != nil {
			logger.Error("read failed", "input", stem, "page", pageNum, "error", err)
			opts.Tracker.MarkFailed(phaseName, pageNum)
			failed++
			continue
		}

		// Save result atomically
		if err := saveReadPage(readDir, pageNum, page); err != nil {
			logger.Error("failed to save read page", "input", stem, "page", pageNum, "error", err)
			opts.Tracker.MarkFailed(phaseName, pageNum)
			failed++
			continue
		}

		opts.Tracker.MarkCompleted(phaseName, pageNum)
		if err := opts.Tracker.Save(); err != nil {
			logger.Error("failed to save progress", "error", err)
		}
		completed++
		logger.Info("page read", "input", stem, "page", pageNum, "completed", completed)
	}

	logger.Info("input read complete", "input", stem, "completed", completed, "failed", failed, "skipped", skipped)
	return nil
}

// fileStem returns the filename without extension.
// e.g. "./input/Anfas1.pdf" -> "Anfas1"
func fileStem(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext)
}

func saveReadPage(dir string, pageNum int, page *model.ReadPage) error {
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
