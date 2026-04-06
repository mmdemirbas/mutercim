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

	imagesDir := filepath.Join(ws.CutDir(), stem)
	readDir := filepath.Join(ws.ReadDir(), stem)

	// List and map available images
	images, err := input.ListImages(imagesDir)
	if err != nil {
		return PhaseResult{}, fmt.Errorf("list images in %s: %w", imagesDir, err)
	}
	if len(images) == 0 {
		return PhaseResult{}, fmt.Errorf("no images found in %s", imagesDir)
	}
	logger.Info("found images", "count", len(images), "input", stem)

	imageMap := make(map[int]string)
	for _, img := range images {
		imageMap[img.PageNumber] = img.Path
	}

	pagesToProcess := pages
	if len(pagesToProcess) == 0 {
		for _, img := range images {
			pagesToProcess = append(pagesToProcess, img.PageNumber)
		}
	}

	if err := os.MkdirAll(readDir, 0750); err != nil {
		return PhaseResult{}, fmt.Errorf("create read dir: %w", err)
	}

	rc := &readContext{
		opts:       opts,
		stem:       stem,
		readDir:    readDir,
		layoutDir:  filepath.Join(ws.LayoutDir(), stem),
		ocrDir:     filepath.Join(ws.OcrDir(), stem),
		imageMap:   imageMap,
		totalPages: len(pagesToProcess),
		rdr:        reader.NewReader(opts.Provider, logger),
		logger:     logger,
	}
	rc.activeModel = rc.buildActiveModelFunc()
	rc.setupDisplayCallbacks()

	rc.processAllPages(ctx, pagesToProcess)

	if opts.Display != nil {
		opts.Display.FinishPhase(display.PhaseRead, stem, "")
	}
	logger.Info("input read complete", "input", stem, "completed", rc.completed, "failed", rc.failed, "skipped", rc.skipped)

	writePhaseReport(readDir, rc.completed, rc.failed, rc.skipped, logger)
	return PhaseResult{Completed: rc.completed, Failed: rc.failed, Skipped: rc.skipped}, nil
}

// readContext holds per-input state for the read phase page loop.
type readContext struct {
	opts       ReadOptions
	stem       string
	readDir    string
	layoutDir  string
	ocrDir     string
	imageMap   map[int]string
	totalPages int
	rdr        *reader.Reader
	logger     *slog.Logger

	activeModel   func() string
	statusPageNum int
	completed     int
	failed        int
	skipped       int
}

// buildActiveModelFunc returns a function that resolves the currently active model label.
func (rc *readContext) buildActiveModelFunc() func() string {
	return func() string {
		if chain, ok := rc.opts.Provider.(*provider.FailoverChain); ok {
			if m := chain.ActiveModel(true); m != "" {
				return m
			}
		}
		if len(rc.opts.Config.Read.Models) > 0 {
			m := rc.opts.Config.Read.Models[0]
			return m.Provider + "/" + m.Model
		}
		return rc.opts.Provider.Name()
	}
}

// setupDisplayCallbacks wires failover and retry callbacks to the display.
func (rc *readContext) setupDisplayCallbacks() {
	if rc.opts.Display == nil {
		return
	}
	chain, ok := rc.opts.Provider.(*provider.FailoverChain)
	if !ok {
		return
	}
	chain.OnFailover = func(from, to string) {
		rc.opts.Display.SetStatus(display.StatusLine{
			Text:      fmt.Sprintf("reading page %d via %s \u2014 failover from %s", rc.statusPageNum, to, from),
			StartedAt: time.Now(),
		})
	}
	chain.SetRetryCallback(func(attempt, maxRetries, statusCode int, backoff time.Duration) {
		rc.opts.Display.SetStatus(display.StatusLine{
			Text:      fmt.Sprintf("reading page %d \u2014 retry %d/%d (%d)", rc.statusPageNum, attempt, maxRetries, statusCode),
			StartedAt: time.Now(),
			Countdown: backoff,
		})
	})
}

// processAllPages iterates over pages, checking for cancellation and error thresholds.
func (rc *readContext) processAllPages(ctx context.Context, pages []int) {
	if rc.opts.Display != nil {
		rc.opts.Display.StartPhase(display.PhaseRead, rc.stem, len(pages), "")
	}
	maxFailPct := rc.opts.Config.Read.Retry.MaxFailPercent
	for i, pageNum := range pages {
		if ctx.Err() != nil {
			rc.logger.Info("context cancelled, stopping read phase",
				"input", rc.stem, "processed", rc.completed+rc.failed, "remaining", len(pages)-i)
			return
		}
		result := PhaseResult{Completed: rc.completed, Failed: rc.failed}
		if result.ExceedsErrorThreshold(maxFailPct) {
			rc.logger.Error("aborting: failure rate exceeds threshold",
				"completed", rc.completed, "failed", rc.failed, "threshold", fmt.Sprintf("%d%%", maxFailPct))
			return
		}
		rc.processReadPage(ctx, pageNum)
	}
}

// processReadPage handles a single page: load prereqs, check rebuild, dispatch AI, save result.
//
//nolint:cyclop,gocognit // per-page dispatch with OCR/vision paths and multiple failure modes
func (rc *readContext) processReadPage(ctx context.Context, pageNum int) {
	imgPath, ok := rc.imageMap[pageNum]
	if !ok {
		rc.logger.Warn("no image found for page", "input", rc.stem, "page", pageNum)
		return
	}

	prereqs := rc.loadPagePrereqs(pageNum, imgPath)

	if !rc.opts.Force && !rebuild.NeedsRebuild(prereqs.outputPath, prereqs.rebuildInputs...) {
		rc.logger.Debug("skipping page (up-to-date)", "input", rc.stem, "page", pageNum)
		rc.skipped++
		return
	}

	rc.statusPageNum = pageNum
	currentModel := rc.activeModel()

	readResult, err := rc.dispatchRead(ctx, pageNum, currentModel, imgPath, prereqs)
	if rc.opts.Display != nil {
		rc.opts.Display.SetStatus(display.StatusLine{}) // clear status
	}

	if err != nil {
		rc.recordFailure(pageNum, err)
		return
	}

	regionPage := readResult.Page

	if err := saveRegionPage(rc.readDir, pageNum, rc.totalPages, regionPage); err != nil {
		rc.recordFailure(pageNum, err)
		return
	}

	if len(regionPage.Regions) == 0 && len(regionPage.Warnings) > 0 {
		rc.logger.Warn("page read produced no regions", "input", rc.stem, "page", pageNum, "warnings", regionPage.Warnings)
		rc.recordFailure(pageNum, fmt.Errorf("%s", regionPage.Warnings[0]))
		return
	}

	rc.recordSuccess(pageNum, currentModel, regionPage, prereqs)
}

// pagePrereqs holds pre-loaded layout and OCR data for a single page.
type pagePrereqs struct {
	layoutRegions []model.Region
	layoutTool    string
	hasLayout     bool
	ocrPage       *model.OCRPage
	hasOCR        bool
	outputPath    string
	rebuildInputs []string
}

// loadPagePrereqs loads layout and OCR data and computes rebuild inputs for a page.
func (rc *readContext) loadPagePrereqs(pageNum int, imgPath string) pagePrereqs {
	ws := rc.opts.Workspace
	cfg := rc.opts.Config
	filename := pageFilename(pageNum, rc.totalPages)

	var p pagePrereqs

	layoutPath := filepath.Join(rc.layoutDir, filename)
	if layoutPage, err := loadLayoutPage(layoutPath); err == nil {
		p.layoutRegions = LayoutRegionsToModelRegions(layoutPage.Regions)
		p.layoutTool = layoutPage.Tool
		p.hasLayout = true
	}

	ocrPath := filepath.Join(rc.ocrDir, filename)
	if op, err := loadOCRPage(ocrPath); err == nil {
		p.ocrPage = op
		p.hasOCR = true
	}

	p.outputPath = filepath.Join(rc.readDir, filename)
	if p.hasOCR {
		p.rebuildInputs = append([]string{ocrPath, ws.ConfigPath()}, cfg.ResolveKnowledgePaths(ws.Root)...)
	} else {
		p.rebuildInputs = append([]string{imgPath, ws.ConfigPath()}, cfg.ResolveKnowledgePaths(ws.Root)...)
	}
	if p.hasLayout {
		p.rebuildInputs = append(p.rebuildInputs, layoutPath)
	}
	return p
}

// dispatchRead executes the AI read call via either the OCR-text or vision-image path.
func (rc *readContext) dispatchRead(ctx context.Context, pageNum int, currentModel, imgPath string, p pagePrereqs) (*reader.ReadResult, error) {
	if p.hasOCR {
		ocrToolName := p.ocrPage.Tool
		if rc.opts.Display != nil {
			rc.opts.Display.SetStatus(display.StatusLine{
				Text:      fmt.Sprintf("page %d: reading via %s (ocr: %s)", pageNum, currentModel, ocrToolName),
				StartedAt: time.Now(),
			})
		}
		rc.logger.Info("reading page", "page", pageNum, "layout", p.hasLayout, "ocr_source", ocrToolName)
		return dispatchOCRRead(ctx, rc.rdr, pageNum, currentModel, p.ocrPage, p.hasLayout, p.layoutRegions, p.layoutTool)
	}

	imageData, err := input.LoadImage(imgPath)
	if err != nil {
		return nil, err
	}

	if rc.opts.Display != nil {
		statusText := fmt.Sprintf("page %d: reading via %s", pageNum, currentModel)
		if p.hasLayout {
			statusText = fmt.Sprintf("page %d: %s + %s ...", pageNum, p.layoutTool, currentModel)
		}
		rc.opts.Display.SetStatus(display.StatusLine{
			Text:      statusText,
			StartedAt: time.Now(),
		})
	}

	rc.logger.Info("reading page", "page", pageNum, "layout", p.hasLayout)
	return rc.rdr.ReadRegionPage(ctx, imageData, pageNum, currentModel, p.layoutRegions, p.layoutTool)
}

// recordFailure increments the failure counter and updates the display.
func (rc *readContext) recordFailure(pageNum int, err error) {
	rc.logger.Error("read failed", "input", rc.stem, "page", pageNum, "error", err)
	rc.failed++
	if rc.opts.Display != nil {
		rc.opts.Display.Update(display.PageResult{
			Phase: display.PhaseRead, Input: rc.stem, PageNum: pageNum,
			Total: rc.totalPages, Completed: rc.completed, Failed: rc.failed, Err: err,
		})
	}
}

// recordSuccess increments the completion counter, logs the result, and updates the display.
func (rc *readContext) recordSuccess(pageNum int, currentModel string, regionPage *model.RegionPage, p pagePrereqs) {
	entryCount, footnoteCount := countRegionTypes(regionPage.Regions)
	rc.completed++

	usedModel := currentModel
	if chain, ok := rc.opts.Provider.(*provider.FailoverChain); ok {
		if m := chain.LastUsedModel(); m != "" {
			usedModel = m
		}
	}

	rc.logger.Info("page read complete",
		"input", rc.stem, "page", pageNum, "layout", p.hasLayout,
		"model", usedModel, "regions", len(regionPage.Regions), "warnings", len(regionPage.Warnings))

	if rc.opts.Display != nil {
		rc.opts.Display.Update(display.PageResult{
			Phase: display.PhaseRead, Input: rc.stem, PageNum: pageNum,
			Total: rc.totalPages, Completed: rc.completed, Failed: rc.failed,
			Warnings: len(regionPage.Warnings), Entries: entryCount, Footnotes: footnoteCount,
			LayoutTool: p.layoutTool, LayoutRegions: len(p.layoutRegions),
		})
	}
}

// writePhaseReport writes a JSON summary of phase results.
func writePhaseReport(dir string, completed, failed, skipped int, logger *slog.Logger) {
	report := map[string]any{
		"pages_completed": completed,
		"pages_failed":    failed,
		"pages_skipped":   skipped,
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return
	}
	if err := atomicWriteFile(filepath.Join(dir, "report.json"), data); err != nil {
		logger.Warn("failed to write phase report", "error", err)
	}
}

// buildOCRRegions merges OCR region text with layout region metadata (BBox and Type).
// It matches each OCR region by ID against the layout regions to produce typed, positioned entries.
func buildOCRRegions(ocrRegions []model.OCRRegion, layoutRegions []model.Region) []reader.OCRRegionData {
	result := make([]reader.OCRRegionData, 0, len(ocrRegions))
	for _, ocrReg := range ocrRegions {
		var bbox model.BBox
		var regionType string
		for _, lr := range layoutRegions {
			if lr.ID == ocrReg.ID {
				bbox = lr.BBox
				regionType = lr.Type
				break
			}
		}
		result = append(result, reader.OCRRegionData{
			ID:   ocrReg.ID,
			Text: ocrReg.Text,
			BBox: bbox,
			Type: regionType,
		})
	}
	return result
}

// dispatchOCRRead dispatches an OCR-backed page read.
// When hasLayout is true and ocrPage has per-region OCR data, it merges region geometry from
// layoutRegions (Case 1). Otherwise it falls back to full-page text segmentation (Case 3).
func dispatchOCRRead(ctx context.Context, rdr *reader.Reader, pageNum int, currentModel string,
	ocrPage *model.OCRPage, hasLayout bool, layoutRegions []model.Region, layoutToolName string) (*reader.ReadResult, error) {
	ocrToolName := ocrPage.Tool
	if hasLayout && len(ocrPage.Regions) > 0 {
		ocrRegions := buildOCRRegions(ocrPage.Regions, layoutRegions)
		return rdr.ReadRegionPageWithOCR(ctx, pageNum, currentModel, ocrRegions, "", layoutToolName, ocrToolName)
	}
	return rdr.ReadRegionPageWithOCR(ctx, pageNum, currentModel, nil, ocrPage.FullText, "", ocrToolName)
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
