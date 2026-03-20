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
	Force     bool  // force re-processing of already completed pages
	Logger    *slog.Logger
	Display   display.Display
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
				m[stem] = model.ExpandPages(ranges)
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

	// Build section lookup
	lookup, err := config.NewSectionLookup(cfg.Sections)
	if err != nil {
		logger.Warn("section lookup error, using auto for all pages", "error", err)
	}

	// Create reader
	rdr := reader.NewReader(opts.Provider, logger)

	// Ensure output directory exists
	if err := os.MkdirAll(readDir, 0755); err != nil {
		return PhaseResult{}, fmt.Errorf("create read dir: %w", err)
	}

	// Use compound phase name for per-input progress tracking
	phaseName := progress.PhaseName("read:" + stem)

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
		// Skip already completed pages — but only if the output file actually exists
		if !opts.Force {
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
			if opts.Display != nil {
				opts.Display.Update(display.PageResult{
					Phase: display.PhaseRead, Input: stem, PageNum: pageNum,
					Total: len(pagesToProcess), Completed: completed, Failed: failed, Err: err,
				})
			}
			continue
		}

		// Read page via AI
		statusPageNum = pageNum
		if opts.Display != nil {
			opts.Display.SetStatus(display.StatusLine{
				Text:      fmt.Sprintf("reading page %d via %s", pageNum, readModel),
				StartedAt: time.Now(),
			})
		}
		page, err := rdr.ReadPage(ctx, imageData, pageNum, sectionType, readModel)
		if opts.Display != nil {
			opts.Display.SetStatus(display.StatusLine{}) // clear status
		}
		if err != nil {
			logger.Error("read failed", "input", stem, "page", pageNum, "error", err)
			opts.Tracker.MarkFailed(phaseName, pageNum)
			failed++
			if opts.Display != nil {
				opts.Display.Update(display.PageResult{
					Phase: display.PhaseRead, Input: stem, PageNum: pageNum,
					Total: len(pagesToProcess), Completed: completed, Failed: failed, Err: err,
				})
			}
			continue
		}

		// Save result atomically
		if err := saveReadPage(readDir, pageNum, page); err != nil {
			logger.Error("failed to save read page", "input", stem, "page", pageNum, "error", err)
			opts.Tracker.MarkFailed(phaseName, pageNum)
			failed++
			if opts.Display != nil {
				opts.Display.Update(display.PageResult{
					Phase: display.PhaseRead, Input: stem, PageNum: pageNum,
					Total: len(pagesToProcess), Completed: completed, Failed: failed, Err: err,
				})
			}
			continue
		}

		opts.Tracker.MarkCompleted(phaseName, pageNum)
		if err := opts.Tracker.Save(); err != nil {
			logger.Error("failed to save progress", "error", err)
		}
		completed++
		logger.Info("page read", "input", stem, "page", pageNum, "completed", completed)
		if opts.Display != nil {
			opts.Display.Update(display.PageResult{
				Phase: display.PhaseRead, Input: stem, PageNum: pageNum,
				Total: len(pagesToProcess), Completed: completed, Failed: failed,
				Warnings: len(page.ReadWarnings), Entries: len(page.Entries), Footnotes: len(page.Footnotes),
			})
		}
	}

	if opts.Display != nil {
		opts.Display.FinishPhase(display.PhaseRead, stem)
	}
	logger.Info("input read complete", "input", stem, "completed", completed, "failed", failed, "skipped", skipped)
	return PhaseResult{Completed: completed, Failed: failed, Skipped: skipped}, nil
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
