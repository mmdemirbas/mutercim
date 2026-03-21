package reader

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/mmdemirbas/mutercim/internal/apiclient"
	"github.com/mmdemirbas/mutercim/internal/layout"
	"github.com/mmdemirbas/mutercim/internal/model"
	"github.com/mmdemirbas/mutercim/internal/provider"
)

// Reader reads structured data from page images using an AI provider.
type Reader struct {
	provider provider.Provider
	logger   *slog.Logger
	// OnLayoutDone is called after layout detection completes (before AI call).
	// Parameters: tool name, elapsed ms, region count, error string (empty on success).
	OnLayoutDone func(tool string, ms int, regions int, layoutErr string)
}

// ReadResult bundles the read page with layout detection metrics.
type ReadResult struct {
	Page          *model.RegionPage
	LayoutTool    string // "doclayout-yolo", "surya", or "ai-only"
	LayoutMs      int    // milliseconds spent in layout detection
	LayoutRegions int    // number of regions detected by layout tool
	LayoutError   string // non-empty if layout tool failed (fell back to ai-only)
}

// NewReader creates a new Reader.
func NewReader(p provider.Provider, logger *slog.Logger) *Reader {
	if logger == nil {
		logger = slog.Default()
	}
	return &Reader{provider: p, logger: logger}
}

// regionResponse matches the JSON schema returned by the AI for region-based reads.
type regionResponse struct {
	Regions      []regionResp `json:"regions"`
	ReadingOrder []string     `json:"reading_order"`
	Warnings     []string     `json:"warnings"`
}

type regionResp struct {
	ID     string             `json:"id"`
	BBox   model.BBox         `json:"bbox"`
	Text   string             `json:"text"`
	Type   string             `json:"type"`
	Style  *model.RegionStyle `json:"style,omitempty"`
	Column *int               `json:"column,omitempty"`
}

// ReadRegionPage processes a page image using the layout-aware region strategy.
// If layoutTool is non-nil and available, it uses the local+AI strategy;
// otherwise it falls back to AI-only.
func (r *Reader) ReadRegionPage(ctx context.Context, image []byte, imagePath string, pageNum int, modelName string, layoutTool layout.Tool) (*ReadResult, error) {
	var layoutRegions []model.Region
	var layoutName string
	var layoutMs int
	var layoutErr string
	var sysPrompt string
	var userPrompt string

	if layoutTool != nil && layoutTool.Name() != "" && layoutTool.Available(ctx) {
		// Local+AI strategy: run layout tool first
		r.logger.Info("detecting layout regions", "page", pageNum, "tool", layoutTool.Name())
		toolName := layoutTool.Name()

		start := time.Now()
		var err error
		layoutRegions, err = layoutTool.DetectRegions(ctx, imagePath)
		layoutMs = int(time.Since(start).Milliseconds())

		if err != nil {
			r.logger.Warn("layout tool failed, falling back to ai-only",
				"page", pageNum, "layout_tool", toolName, "err", err)
			layoutErr = err.Error()
			layoutName = toolName // record which tool was attempted
		} else {
			layoutName = toolName
		}

		// Notify callback after layout detection
		if r.OnLayoutDone != nil {
			r.OnLayoutDone(toolName, layoutMs, len(layoutRegions), layoutErr)
		}
	} else {
		// AI-only: no layout tool configured or not available
		if r.OnLayoutDone != nil {
			r.OnLayoutDone("ai-only", 0, 0, "")
		}
	}

	effectiveLayoutName := layoutName
	if layoutErr != "" {
		effectiveLayoutName = "" // fell back to ai-only
	}

	if len(layoutRegions) > 0 {
		sysPrompt = regionSystemPromptWithLayout
		userPrompt = BuildRegionUserPromptWithLayout(layoutRegions)
	} else {
		sysPrompt = regionSystemPromptAIOnly
		userPrompt = BuildRegionUserPrompt()
	}

	r.logger.Info("reading page regions", "page", pageNum, "strategy", strategyName(effectiveLayoutName))

	rawResponse, err := r.provider.ReadFromImage(ctx, image, sysPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("read page %d regions: %w", pageNum, err)
	}

	// Determine the layout tool label for the output
	resultLayoutTool := layoutName
	if resultLayoutTool == "" {
		resultLayoutTool = "ai-only"
	}

	jsonStr, err := apiclient.ExtractJSON(rawResponse)
	if err != nil {
		return &ReadResult{
			Page: &model.RegionPage{
				Version:       "2.0",
				PageNumber:    pageNum,
				ReadModel:     modelName,
				LayoutTool:    effectiveLayoutName,
				ReadTimestamp: time.Now().UTC().Format(time.RFC3339),
				RawText:       rawResponse,
				Warnings:      []string{fmt.Sprintf("JSON extraction failed: %v", err)},
			},
			LayoutTool:    resultLayoutTool,
			LayoutMs:      layoutMs,
			LayoutRegions: len(layoutRegions),
			LayoutError:   layoutErr,
		}, nil
	}

	var resp regionResponse
	if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
		return &ReadResult{
			Page: &model.RegionPage{
				Version:       "2.0",
				PageNumber:    pageNum,
				ReadModel:     modelName,
				LayoutTool:    effectiveLayoutName,
				ReadTimestamp: time.Now().UTC().Format(time.RFC3339),
				RawText:       rawResponse,
				Warnings:      []string{fmt.Sprintf("JSON unmarshal failed: %v", err)},
			},
			LayoutTool:    resultLayoutTool,
			LayoutMs:      layoutMs,
			LayoutRegions: len(layoutRegions),
			LayoutError:   layoutErr,
		}, nil
	}

	regions := make([]model.Region, len(resp.Regions))
	for i, rr := range resp.Regions {
		region := model.Region{
			ID:     rr.ID,
			BBox:   rr.BBox,
			Text:   rr.Text,
			Type:   rr.Type,
			Style:  rr.Style,
			Column: rr.Column,
		}

		if effectiveLayoutName != "" {
			// In local+AI strategy, mark sources appropriately
			region.TextSource = modelName
			if isLayoutRegion(rr.ID, layoutRegions) {
				region.LayoutSource = effectiveLayoutName
			} else {
				// AI added a new region not from layout tool
				region.LayoutSource = model.LayoutSourceAI
			}
		} else {
			// AI-only strategy: AI is the source for everything
			region.LayoutSource = model.LayoutSourceAI
			region.TextSource = modelName
		}

		regions[i] = region
	}

	page := &model.RegionPage{
		Version:       "2.0",
		PageNumber:    pageNum,
		ReadModel:     modelName,
		LayoutTool:    effectiveLayoutName,
		ReadTimestamp: time.Now().UTC().Format(time.RFC3339),
		Regions:       regions,
		ReadingOrder:  resp.ReadingOrder,
		Warnings:      resp.Warnings,
	}

	return &ReadResult{
		Page:          page,
		LayoutTool:    resultLayoutTool,
		LayoutMs:      layoutMs,
		LayoutRegions: len(layoutRegions),
		LayoutError:   layoutErr,
	}, nil
}

// strategyName returns a human-readable strategy name for logging.
func strategyName(layoutTool string) string {
	if layoutTool != "" {
		return "local+ai"
	}
	return "ai-only"
}

// isLayoutRegion checks if a region ID was among the layout-tool-detected regions.
func isLayoutRegion(id string, layoutRegions []model.Region) bool {
	for _, r := range layoutRegions {
		if r.ID == id {
			return true
		}
	}
	return false
}
