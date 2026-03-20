package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/mmdemirbas/mutercim/internal/config"
	"github.com/mmdemirbas/mutercim/internal/display"
	"github.com/mmdemirbas/mutercim/internal/input"
	"github.com/mmdemirbas/mutercim/internal/model"
	"github.com/mmdemirbas/mutercim/internal/rebuild"
	"github.com/mmdemirbas/mutercim/internal/workspace"
)

// PagesOptions configures the pagination pipeline.
type PagesOptions struct {
	Workspace *workspace.Workspace
	Config    *config.Config
	Pages     []int // CLI override pages; nil means use per-input or global config
	Force     bool  // force re-conversion of already existing page images
	Logger    *slog.Logger
	Display   display.Display
}

// Pages runs the pagination phase: converts PDF inputs to per-page images.
// For inputs that are already image directories, this is a no-op.
func Pages(ctx context.Context, opts PagesOptions) error {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	inputs := opts.Config.Inputs
	if len(inputs) == 0 {
		return fmt.Errorf("no inputs configured")
	}

	var failures int
	for _, inp := range inputs {
		resolved := opts.Config.ResolvePath(opts.Workspace.Root, inp.Path)
		stem := fileStem(inp.Path)

		// Determine effective pages: CLI override > per-input > global config > all
		pages := opts.Pages
		if len(pages) == 0 && inp.Pages != "" {
			if ranges, err := model.ParsePageRanges(inp.Pages); err == nil {
				pages = model.ExpandPages(ranges)
			}
		}

		if err := pagesOneInput(ctx, opts, resolved, stem, pages); err != nil {
			logger.Error("pagination failed", "input", inp.Path, "error", err)
			failures++
		}
	}

	if failures == len(inputs) {
		return fmt.Errorf("all %d inputs failed pagination", failures)
	}
	return nil
}

func pagesOneInput(ctx context.Context, opts PagesOptions, inputPath, stem string, pages []int) error {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	imagesDir := filepath.Join(opts.Workspace.PagesDir(), stem)

	if !config.IsPDF(inputPath) {
		logger.Info("images already available, skipping pagination", "input", stem)
		return nil
	}

	// Skip if pages directory is up-to-date (images newer than PDF)
	if !opts.Force && dirHasEntries(imagesDir) && !rebuild.NeedsRebuild(imagesDir, inputPath) {
		logger.Debug("skipping pagination (up-to-date)", "input", stem)
		images, _ := input.ListImages(imagesDir)
		if opts.Display != nil {
			opts.Display.StartPhase(display.PhasePages, stem, len(images), "")
			opts.Display.Update(display.PageResult{
				Phase: display.PhasePages, Input: stem,
				Total: len(images), Completed: len(images),
			})
			opts.Display.FinishPhase(display.PhasePages, stem, "")
		}
		return nil
	}

	if err := os.MkdirAll(imagesDir, 0755); err != nil {
		return fmt.Errorf("create images dir: %w", err)
	}

	firstPage, lastPage := 0, 0
	if len(pages) > 0 {
		firstPage = pages[0]
		lastPage = pages[len(pages)-1]
	}
	logger.Info("converting PDF to images", "input", inputPath, "dpi", opts.Config.DPI, "first", firstPage, "last", lastPage)
	if err := input.ConvertPDFToImages(ctx, inputPath, imagesDir, opts.Config.DPI, firstPage, lastPage); err != nil {
		if opts.Display != nil {
			opts.Display.StartPhase(display.PhasePages, stem, 1, "")
			opts.Display.Update(display.PageResult{
				Phase: display.PhasePages, Input: stem,
				Total: 1, Failed: 1, Err: err,
			})
			opts.Display.FinishPhase(display.PhasePages, stem, "")
		}
		return fmt.Errorf("convert PDF %s: %w", inputPath, err)
	}

	// Count resulting images
	images, err := input.ListImages(imagesDir)
	if err == nil {
		logger.Info("pagination complete", "input", stem, "images", len(images))
	}
	imageCount := len(images)

	if opts.Display != nil {
		opts.Display.StartPhase(display.PhasePages, stem, imageCount, "")
		opts.Display.Update(display.PageResult{
			Phase: display.PhasePages, Input: stem,
			Total: imageCount, Completed: imageCount,
		})
		opts.Display.FinishPhase(display.PhasePages, stem, "")
	}

	return nil
}
