package reader

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/mmdemirbas/mutercim/internal/apiclient"
	"github.com/mmdemirbas/mutercim/internal/model"
	"github.com/mmdemirbas/mutercim/internal/provider"
)

// Reader reads structured data from page images using an AI provider.
type Reader struct {
	provider provider.Provider
	logger   *slog.Logger
}

// ReadResult bundles the read page.
type ReadResult struct {
	Page *model.RegionPage
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
// If layoutRegions is non-empty, it uses the local+AI strategy (with-layout prompt);
// otherwise it uses the AI-only prompt.
// The layoutToolName parameter records which tool produced the layout regions (empty means ai-only).
//nolint:funlen // AI region parsing with OCR merge logic
func (r *Reader) ReadRegionPage(ctx context.Context, image []byte, pageNum int, modelName string, layoutRegions []model.Region, layoutToolName string) (*ReadResult, error) {
	var sysPrompt string
	var userPrompt string

	if len(layoutRegions) > 0 {
		sysPrompt = regionSystemPromptWithLayout
		userPrompt = BuildRegionUserPromptWithLayout(layoutRegions)
	} else {
		sysPrompt = regionSystemPromptAIOnly
		userPrompt = BuildRegionUserPrompt()
		layoutToolName = "" // ensure empty for ai-only
	}

	r.logger.Info("reading page regions", "page", pageNum, "strategy", strategyName(layoutToolName))

	rawResponse, err := r.provider.ReadFromImage(ctx, image, sysPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("read page %d regions: %w", pageNum, err)
	}

	resp, fallback := r.parseRegionResponse(rawResponse, pageNum, modelName, layoutToolName, "")
	if fallback != nil {
		return &ReadResult{Page: fallback}, nil
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

		if layoutToolName != "" {
			// In local+AI strategy, mark sources appropriately
			region.TextSource = modelName
			if isLayoutRegion(rr.ID, layoutRegions) {
				region.LayoutSource = layoutToolName
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
		LayoutTool:    layoutToolName,
		ReadTimestamp: time.Now().UTC().Format(time.RFC3339),
		Regions:       regions,
		ReadingOrder:  resp.ReadingOrder,
		Warnings:      resp.Warnings,
	}

	return &ReadResult{
		Page: page,
	}, nil
}

// OCRRegionData holds OCR text for a region, used when the OCR phase has already extracted text.
type OCRRegionData struct {
	ID   string
	Text string
	BBox model.BBox
	Type string // from layout
}

// ReadRegionPageWithOCR processes a page using pre-extracted OCR text (no vision needed).
// This is the text-only path: the LLM receives OCR text (with optional layout info) and
// refines region types, reading order, and style — but does NOT need to see the image.
//
// Case 1 (layout + ocr): regions have OCR text + layout bboxes/types.
// Case 3 (no layout + ocr): fullText is the unstructured OCR text.
//nolint:funlen // AI region parsing with full OCR merge and region matching
func (r *Reader) ReadRegionPageWithOCR(ctx context.Context, pageNum int, modelName string,
	ocrRegions []OCRRegionData, fullText string,
	layoutToolName, ocrToolName string) (*ReadResult, error) {

	var sysPrompt string
	var userPrompt string

	switch {
	case len(ocrRegions) > 0:
		// Case 1: layout + ocr — best case, text-only LLM
		sysPrompt = regionSystemPromptWithLayoutAndOCR
		userPrompt = BuildRegionUserPromptWithLayoutAndOCR(ocrRegions)
	case fullText != "":
		// Case 3: no layout + ocr — text segmentation
		sysPrompt = regionSystemPromptWithOCROnly
		userPrompt = BuildRegionUserPromptWithOCROnly(fullText)
	default:
		return nil, fmt.Errorf("no OCR data available for page %d", pageNum)
	}

	r.logger.Info("reading page regions (text-only)",
		"page", pageNum, "ocr_source", ocrToolName, "layout_source", layoutToolName)

	// Use Translate (text-only) endpoint since OCR provides text — no vision needed
	rawResponse, err := r.provider.Translate(ctx, sysPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("read page %d regions (text-only): %w", pageNum, err)
	}

	resp, fallback := r.parseRegionResponse(rawResponse, pageNum, modelName, layoutToolName, ocrToolName)
	if fallback != nil {
		return &ReadResult{Page: fallback}, nil
	}

	regions := make([]model.Region, len(resp.Regions))
	for i, rr := range resp.Regions {
		region := model.Region{
			ID:         rr.ID,
			BBox:       rr.BBox,
			Text:       rr.Text,
			Type:       rr.Type,
			Style:      rr.Style,
			Column:     rr.Column,
			TextSource: ocrToolName,
		}

		if layoutToolName != "" {
			region.LayoutSource = layoutToolName
		} else {
			region.LayoutSource = model.LayoutSourceAI
		}

		regions[i] = region
	}

	page := &model.RegionPage{
		Version:       "2.0",
		PageNumber:    pageNum,
		ReadModel:     modelName,
		LayoutTool:    layoutToolName,
		OCRSource:     ocrToolName,
		ReadTimestamp: time.Now().UTC().Format(time.RFC3339),
		Regions:       regions,
		ReadingOrder:  resp.ReadingOrder,
		Warnings:      resp.Warnings,
	}

	return &ReadResult{Page: page}, nil
}

// strategyName returns a human-readable strategy name for logging.
func strategyName(layoutTool string) string {
	if layoutTool != "" {
		return "local+ai"
	}
	return "ai-only"
}

// parseRegionResponse extracts and unmarshals the region JSON from a raw AI response.
// On parse failure, returns a fallback RegionPage with the raw text and a warning.
func (r *Reader) parseRegionResponse(rawResponse string, pageNum int, modelName, layoutToolName, ocrSource string) (*regionResponse, *model.RegionPage) {
	jsonStr, err := apiclient.ExtractJSON(rawResponse)
	if err != nil {
		r.logger.Warn("AI response JSON extraction failed",
			"page", pageNum, "error", err, "response_preview", truncateResponse(rawResponse, 500))
		return nil, &model.RegionPage{
			Version:       "2.0",
			PageNumber:    pageNum,
			ReadModel:     modelName,
			LayoutTool:    layoutToolName,
			OCRSource:     ocrSource,
			ReadTimestamp: time.Now().UTC().Format(time.RFC3339),
			RawText:       rawResponse,
			Warnings:      []string{fmt.Sprintf("JSON extraction failed: %v", err)},
		}
	}

	var resp regionResponse
	if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
		r.logger.Warn("AI response JSON unmarshal failed",
			"page", pageNum, "error", err, "response_preview", truncateResponse(rawResponse, 500))
		return nil, &model.RegionPage{
			Version:       "2.0",
			PageNumber:    pageNum,
			ReadModel:     modelName,
			LayoutTool:    layoutToolName,
			OCRSource:     ocrSource,
			ReadTimestamp: time.Now().UTC().Format(time.RFC3339),
			RawText:       rawResponse,
			Warnings:      []string{fmt.Sprintf("JSON unmarshal failed: %v", err)},
		}
	}

	return &resp, nil
}

// truncateResponse truncates a response string for logging.
func truncateResponse(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
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
