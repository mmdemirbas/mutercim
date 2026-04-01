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

// CutOptions configures the page cutting pipeline.
type CutOptions struct {
	Workspace *workspace.Workspace
	Config    *config.Config
	Pages     []int // CLI override pages; nil means use per-input or global config
	Force     bool  // force re-conversion of already existing page images
	Logger    *slog.Logger
	Display   display.Display
}

// Cut runs the page cutting phase: converts PDF inputs to per-page images.
// For inputs that are already image directories, this is a no-op.
//nolint:cyclop,gocognit // pipeline phase with multi-input orchestration
func Cut(ctx context.Context, opts CutOptions) error {
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

		if err := cutOneInput(ctx, opts, resolved, stem, pages); err != nil {
			logger.Error("page cutting failed", "input", inp.Path, "error", err)
			failures++
		}
	}

	if failures == len(inputs) {
		return fmt.Errorf("all %d inputs failed page cutting", failures)
	}
	return nil
}

//nolint:cyclop,gocognit // per-input cut logic with multiple format paths
func cutOneInput(ctx context.Context, opts CutOptions, inputPath, stem string, pages []int) error {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	imagesDir := filepath.Join(opts.Workspace.CutDir(), stem)

	if !config.IsPDF(inputPath) {
		logger.Info("images already available, skipping cut", "input", stem)
		return nil
	}

	// Skip if cut directory is up-to-date (images newer than PDF)
	if !opts.Force && dirHasEntries(imagesDir) && !rebuild.NeedsRebuild(imagesDir, inputPath) {
		logger.Debug("skipping cut (up-to-date)", "input", stem)
		images, _ := input.ListImages(imagesDir)
		if opts.Display != nil {
			opts.Display.StartPhase(display.PhaseCut, stem, len(images), "")
			opts.Display.Update(display.PageResult{
				Phase: display.PhaseCut, Input: stem,
				Total: len(images), Completed: len(images),
			})
			opts.Display.FinishPhase(display.PhaseCut, stem, "")
		}
		return nil
	}

	if err := os.MkdirAll(imagesDir, 0750); err != nil {
		return fmt.Errorf("create images dir: %w", err)
	}

	ranges := contiguousRanges(pages)
	dpi := opts.Config.Cut.DPI
	logger.Info("converting PDF to images", "input", inputPath, "dpi", dpi, "ranges", len(ranges))
	if opts.Display != nil {
		opts.Display.StartPhase(display.PhaseCut, stem, 0, "")
		opts.Display.SetStatus(display.StatusLine{
			Text:      fmt.Sprintf("converting %s to images (dpi %d)", filepath.Base(inputPath), dpi),
			StartedAt: time.Now(),
		})
	}
	dockerDir := docker.FindDockerDir("poppler")
	for _, r := range ranges {
		if err := input.ConvertPDFToImages(ctx, inputPath, imagesDir, dpi, r[0], r[1], dockerDir); err != nil {
			if opts.Display != nil {
				opts.Display.StartPhase(display.PhaseCut, stem, 1, "")
				opts.Display.Update(display.PageResult{
					Phase: display.PhaseCut, Input: stem,
					Total: 1, Failed: 1, Err: err,
				})
				opts.Display.FinishPhase(display.PhaseCut, stem, "")
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
		logger.Info("page cutting complete", "input", stem, "images", len(images))
	}
	imageCount := len(images)

	if opts.Display != nil {
		opts.Display.StartPhase(display.PhaseCut, stem, imageCount, "")
		opts.Display.Update(display.PageResult{
			Phase: display.PhaseCut, Input: stem,
			Total: imageCount, Completed: imageCount,
		})
		opts.Display.FinishPhase(display.PhaseCut, stem, "")
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
