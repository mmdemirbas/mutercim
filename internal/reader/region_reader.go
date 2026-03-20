package reader

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mmdemirbas/mutercim/internal/apiclient"
	"github.com/mmdemirbas/mutercim/internal/layout"
	"github.com/mmdemirbas/mutercim/internal/model"
)

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
func (r *Reader) ReadRegionPage(ctx context.Context, image []byte, imagePath string, pageNum int, modelName string, layoutTool layout.Tool) (*model.RegionPage, error) {
	var layoutRegions []model.Region
	var layoutName string
	var sysPrompt string
	var userPrompt string

	if layoutTool != nil && layoutTool.Name() != "" && layoutTool.Available(ctx) {
		// Local+AI strategy: run layout tool first
		r.logger.Info("detecting layout regions", "page", pageNum, "tool", layoutTool.Name())

		var err error
		layoutRegions, err = layoutTool.DetectRegions(ctx, imagePath)
		if err != nil {
			r.logger.Warn("layout detection failed, falling back to AI-only", "page", pageNum, "error", err)
			// Fall through to AI-only
		} else {
			layoutName = layoutTool.Name()
		}
	}

	if len(layoutRegions) > 0 {
		sysPrompt = regionSystemPromptWithLayout
		userPrompt = BuildRegionUserPromptWithLayout(layoutRegions)
	} else {
		sysPrompt = regionSystemPromptAIOnly
		userPrompt = BuildRegionUserPrompt()
	}

	r.logger.Info("reading page regions", "page", pageNum, "strategy", strategyName(layoutName))

	rawResponse, err := r.provider.ReadFromImage(ctx, image, sysPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("read page %d regions: %w", pageNum, err)
	}

	jsonStr, err := apiclient.ExtractJSON(rawResponse)
	if err != nil {
		return &model.RegionPage{
			Version:       "2.0",
			PageNumber:    pageNum,
			ReadModel:     modelName,
			LayoutTool:    layoutName,
			ReadTimestamp: time.Now().UTC().Format(time.RFC3339),
			RawText:       rawResponse,
			Warnings:      []string{fmt.Sprintf("JSON extraction failed: %v", err)},
		}, nil
	}

	var resp regionResponse
	if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
		return &model.RegionPage{
			Version:       "2.0",
			PageNumber:    pageNum,
			ReadModel:     modelName,
			LayoutTool:    layoutName,
			ReadTimestamp: time.Now().UTC().Format(time.RFC3339),
			RawText:       rawResponse,
			Warnings:      []string{fmt.Sprintf("JSON unmarshal failed: %v", err)},
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

		if layoutName != "" {
			// In local+AI strategy, mark sources appropriately
			region.TextSource = modelName
			if isLayoutRegion(rr.ID, layoutRegions) {
				region.LayoutSource = layoutName
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
		LayoutTool:    layoutName,
		ReadTimestamp: time.Now().UTC().Format(time.RFC3339),
		Regions:       regions,
		ReadingOrder:  resp.ReadingOrder,
		Warnings:      resp.Warnings,
	}

	return page, nil
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
