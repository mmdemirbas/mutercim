package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/mmdemirbas/mutercim/internal/config"
	"github.com/mmdemirbas/mutercim/internal/display"
	"github.com/mmdemirbas/mutercim/internal/docker"
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
				if expanded, err := model.ExpandPages(ranges); err == nil {
					pages = expanded
				}
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

	ranges := contiguousRanges(pages)
	logger.Info("converting PDF to images", "input", inputPath, "dpi", opts.Config.DPI, "ranges", len(ranges))
	if opts.Display != nil {
		opts.Display.StartPhase(display.PhasePages, stem, 0, "")
		opts.Display.SetStatus(display.StatusLine{
			Text:      fmt.Sprintf("converting %s to images (dpi %d)", filepath.Base(inputPath), opts.Config.DPI),
			StartedAt: time.Now(),
		})
	}
	dockerDir := docker.FindDockerDir("poppler")
	for _, r := range ranges {
		if err := input.ConvertPDFToImages(ctx, inputPath, imagesDir, opts.Config.DPI, r[0], r[1], dockerDir); err != nil {
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
	}

	if opts.Display != nil {
		opts.Display.SetStatus(display.StatusLine{}) // clear conversion status
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

// contiguousRanges groups a sorted list of page numbers into contiguous [first, last] pairs.
// E.g., [1, 2, 3, 10, 11, 500] -> [[1,3], [10,11], [500,500]].
// If pages is nil or empty, returns a single range [0,0] meaning "all pages".
func contiguousRanges(pages []int) [][2]int {
	if len(pages) == 0 {
		return [][2]int{{0, 0}}
	}

	sorted := make([]int, len(pages))
	copy(sorted, pages)
	sort.Ints(sorted)

	var ranges [][2]int
	start := sorted[0]
	end := sorted[0]

	for i := 1; i < len(sorted); i++ {
		if sorted[i] == end+1 {
			end = sorted[i]
		} else {
			ranges = append(ranges, [2]int{start, end})
			start = sorted[i]
			end = sorted[i]
		}
	}
	ranges = append(ranges, [2]int{start, end})
	return ranges
}
