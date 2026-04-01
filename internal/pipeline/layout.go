package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mmdemirbas/mutercim/internal/config"
	"github.com/mmdemirbas/mutercim/internal/display"
	"github.com/mmdemirbas/mutercim/internal/input"
	"github.com/mmdemirbas/mutercim/internal/layout"
	"github.com/mmdemirbas/mutercim/internal/model"
	"github.com/mmdemirbas/mutercim/internal/reader"
	"github.com/mmdemirbas/mutercim/internal/rebuild"
	"github.com/mmdemirbas/mutercim/internal/workspace"
)

// LayoutOptions configures the layout detection pipeline.
type LayoutOptions struct {
	Workspace *workspace.Workspace
	Config    *config.Config
	Pages     []int // CLI override pages; nil means use per-input or global config
	Force     bool  // force re-processing of already completed pages
	Logger    *slog.Logger
	Display   display.Display
}

// Layout runs the layout detection phase: detects document regions on page images.
//
//nolint:cyclop // pipeline orchestration complexity is inherent
func Layout(ctx context.Context, opts LayoutOptions) (PhaseResult, error) {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	cfg := opts.Config
	toolName := cfg.Layout.Tool
	if toolName == "" {
		logger.Info("layout tool disabled, skipping layout phase")
		return PhaseResult{}, nil
	}

	tool := layout.NewTool(toolName)
	if tool == nil {
		return PhaseResult{}, fmt.Errorf("unknown layout tool: %q", toolName)
	}
	if !tool.Available(ctx) {
		return PhaseResult{}, fmt.Errorf("layout tool %q is not available (Docker not running or image not built)", toolName)
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
	inputPages := buildInputPageMap(cfg)

	// Build layout params, auto-populating languages from input config
	var total PhaseResult
	for _, stem := range stems {
		logger.Info("detecting layout", "input", stem, "tool", toolName)

		pages := opts.Pages
		if len(pages) == 0 {
			if pp, ok := inputPages[stem]; ok {
				pages = pp
			}
		}

		result, err := layoutOneInput(ctx, opts, tool, stem, pages)
		total.Completed += result.Completed
		total.Failed += result.Failed
		total.Skipped += result.Skipped
		if err != nil {
			logger.Error("layout failed", "input", stem, "error", err)
		}
	}

	return total, nil
}

// layoutReport holds summary stats written to layout/<stem>/report.json.
type layoutReport struct {
	Tool              string         `json:"tool"`
	ToolParams        map[string]any `json:"tool_params,omitempty"`
	PagesProcessed    int            `json:"pages_processed"`
	PagesFailed       int            `json:"pages_failed"`
	AvgMs             int            `json:"avg_ms"`
	ClassDistribution map[string]int `json:"class_distribution"`
}

func layoutOneInput(ctx context.Context, opts LayoutOptions, tool layout.Tool, stem string, pages []int) (PhaseResult, error) {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	ws := opts.Workspace
	cfg := opts.Config
	imagesDir := filepath.Join(ws.CutDir(), stem)
	layoutDir := filepath.Join(ws.LayoutDir(), stem)

	// List available images
	images, err := input.ListImages(imagesDir)
	if err != nil {
		return PhaseResult{}, fmt.Errorf("list images in %s: %w", imagesDir, err)
	}
	if len(images) == 0 {
		return PhaseResult{}, fmt.Errorf("no images found in %s", imagesDir)
	}

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

	if err := os.MkdirAll(layoutDir, 0750); err != nil {
		return PhaseResult{}, fmt.Errorf("create layout dir: %w", err)
	}

	// Build params with auto-populated languages
	params := makeLayoutParams(cfg, tool.Name(), stem)

	// Start progress display
	if opts.Display != nil {
		opts.Display.StartPhase(display.PhaseLayout, stem, len(pagesToProcess), "")
	}

	completed := 0
	failed := 0
	skipped := 0
	var totalMs int64
	classCounts := make(map[string]int)

	for _, pageNum := range pagesToProcess {
		if ctx.Err() != nil {
			break
		}

		imgPath, ok := imageMap[pageNum]
		if !ok {
			continue
		}

		// Skip if output is up-to-date
		outputPath := filepath.Join(layoutDir, pageFilename(pageNum, len(pagesToProcess)))
		if !opts.Force && !rebuild.NeedsRebuild(outputPath, imgPath, ws.ConfigPath()) {
			skipped++
			continue
		}

		// Run layout detection
		if opts.Display != nil {
			opts.Display.SetStatus(display.StatusLine{
				Text:      fmt.Sprintf("page %d: %s ...", pageNum, tool.Name()),
				StartedAt: time.Now(),
			})
		}

		start := time.Now()
		detectResult, err := tool.DetectRegions(ctx, imgPath, params)
		ms := time.Since(start).Milliseconds()
		totalMs += ms

		if opts.Display != nil {
			opts.Display.SetStatus(display.StatusLine{})
		}

		if err != nil {
			logger.Error("layout detection failed", "page", pageNum, "tool", tool.Name(), "error", err)
			failed++
			if opts.Display != nil {
				opts.Display.Update(display.PageResult{
					Phase: display.PhaseLayout, Input: stem, PageNum: pageNum,
					Total: len(pagesToProcess), Completed: completed, Failed: failed, Err: err,
				})
			}
			continue
		}

		regions := detectResult.Regions

		// Count classes for report
		for _, r := range regions {
			if r.RawClass != "" {
				classCounts[r.RawClass]++
			} else {
				classCounts[r.Type]++
			}
		}

		// Get page image dimensions for page_size
		pageSize := getImageSize(imgPath)

		// Build layout regions
		layoutRegions := make([]model.LayoutRegion, len(regions))
		for i, r := range regions {
			layoutRegions[i] = model.LayoutRegion{
				ID:         r.ID,
				BBox:       r.BBox,
				Type:       r.Type,
				RawClass:   r.RawClass,
				Confidence: r.Confidence,
				Zone:       r.Zone,
			}
		}

		layoutPage := &model.LayoutPage{
			Version:        "1.0",
			PageNumber:     pageNum,
			PageSize:       pageSize,
			Tool:           tool.Name(),
			ToolParams:     params,
			Regions:        layoutRegions,
			ReadingOrder:   detectResult.ReadingOrder,
			SeparatorY:     detectResult.SeparatorY,
			PostProcessing: detectResult.PostProcessing,
		}

		// Save layout JSON atomically
		data, err := json.MarshalIndent(layoutPage, "", "  ")
		if err != nil {
			logger.Error("failed to marshal layout page", "page", pageNum, "error", err)
			failed++
			continue
		}
		if err := atomicWriteFile(outputPath, data); err != nil {
			logger.Error("failed to save layout page", "page", pageNum, "error", err)
			failed++
			continue
		}

		// Generate debug overlay if enabled
		if cfg.Layout.Debug && len(regions) > 0 {
			debugDir := filepath.Join(layoutDir, "debug")
			generateLayoutDebugImage(imgPath, regions, pageNum, debugDir, logger)
		}

		logger.Info("page layout complete", "page", pageNum, "tool", tool.Name(), "tool_ms", ms, "regions", len(regions))
		completed++

		if opts.Display != nil {
			opts.Display.Update(display.PageResult{
				Phase: display.PhaseLayout, Input: stem, PageNum: pageNum,
				Total: len(pagesToProcess), Completed: completed, Failed: failed,
			})
		}
	}

	if opts.Display != nil {
		opts.Display.SetStatus(display.StatusLine{})
		opts.Display.FinishPhase(display.PhaseLayout, stem, "")
	}

	// Write report
	avgMs := 0
	if completed > 0 {
		avgMs = int(totalMs / int64(completed))
	}
	report := layoutReport{
		Tool:              tool.Name(),
		ToolParams:        params,
		PagesProcessed:    completed,
		PagesFailed:       failed,
		AvgMs:             avgMs,
		ClassDistribution: classCounts,
	}
	if err := saveLayoutReport(layoutDir, report); err != nil {
		logger.Warn("failed to write layout report", "error", err)
	}

	logger.Info("layout phase complete", "input", stem, "completed", completed, "failed", failed, "skipped", skipped, "avg_ms", avgMs)
	return PhaseResult{Completed: completed, Failed: failed, Skipped: skipped}, nil
}

// getImageSize reads image dimensions without fully decoding the image.
func getImageSize(path string) model.PageSize {
	f, err := os.Open(path) //nolint:gosec // G304: path is internal workspace image path, not user HTTP input
	if err != nil {
		return model.PageSize{}
	}
	defer func() { _ = f.Close() }()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return model.PageSize{}
	}
	return model.PageSize{Width: cfg.Width, Height: cfg.Height}
}

func saveLayoutReport(dir string, report layoutReport) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}
	return atomicWriteFile(filepath.Join(dir, "report.json"), data)
}

// generateLayoutDebugImage loads the page image and renders layout overlay.
func generateLayoutDebugImage(imagePath string, regions []model.Region, pageNum int, debugDir string, logger *slog.Logger) {
	f, err := os.Open(imagePath) //nolint:gosec // G304: imagePath is internal workspace path, not user HTTP input
	if err != nil {
		logger.Warn("debug overlay: cannot open page image", "page", pageNum, "error", err)
		return
	}
	defer func() { _ = f.Close() }()

	pageImg, _, err := image.Decode(f)
	if err != nil {
		logger.Warn("debug overlay: cannot decode page image", "page", pageNum, "error", err)
		return
	}

	// Build reading order map (1-based, from region order)
	readingOrder := make(map[string]int)
	for i, region := range regions {
		readingOrder[region.ID] = i + 1
	}

	outputPath := filepath.Join(debugDir, fmt.Sprintf("%d_layout.png", pageNum))
	if err := reader.GenerateDebugOverlay(pageImg, regions, readingOrder, outputPath); err != nil {
		logger.Warn("debug overlay: failed to generate", "page", pageNum, "error", err)
	}
}

// makeLayoutParams builds the layout tool params map from user config,
// then auto-populates tool-specific defaults (e.g. languages for Surya).
func makeLayoutParams(cfg *config.Config, toolName, stem string) map[string]any {
	params := make(map[string]any)
	for k, v := range cfg.Layout.Params {
		params[k] = v
	}

	// Auto-set languages for Surya from input config if not explicitly provided
	if toolName == "surya" {
		if _, ok := params["languages"]; !ok {
			if langs := cfg.SourceLanguagesForStem(stem); len(langs) > 0 {
				params["languages"] = strings.Join(langs, ",")
			}
		}
	}

	return params
}

// loadLayoutPage loads a layout page JSON file.
func loadLayoutPage(path string) (*model.LayoutPage, error) {
	data, err := os.ReadFile(path) //nolint:gosec // G304: path is internal workspace path, not user HTTP input
	if err != nil {
		return nil, err
	}
	var page model.LayoutPage
	if err := json.Unmarshal(data, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

// LayoutRegionsToModelRegions converts layout regions to model.Region for the read phase.
func LayoutRegionsToModelRegions(layoutRegions []model.LayoutRegion) []model.Region {
	regions := make([]model.Region, len(layoutRegions))
	for i, lr := range layoutRegions {
		regions[i] = model.Region{
			ID:         lr.ID,
			BBox:       lr.BBox,
			Type:       lr.Type,
			RawClass:   lr.RawClass,
			Confidence: lr.Confidence,
		}
	}
	return regions
}
