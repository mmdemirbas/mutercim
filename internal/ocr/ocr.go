// Package ocr provides OCR tool implementations for text extraction from page images.
// OCR tools are specialized single-purpose engines (not LLM providers) that extract
// text from images, optionally using layout regions to crop and OCR each region independently.
package ocr

import (
	"context"
	"errors"
)

// ErrDisabled is returned when the OCR tool is disabled (no-op tool).
var ErrDisabled = errors.New("ocr is disabled")

// RegionInput is a region to OCR, with its bounding box from the layout phase.
type RegionInput struct {
	ID   string
	BBox [4]int // x1, y1, x2, y2
}

// RegionResult is the OCR result for a single region.
type RegionResult struct {
	ID        string
	Text      string
	ElapsedMs int
}

// Result is the full OCR result for a page.
type Result struct {
	Regions  []RegionResult // populated when layout regions are provided
	FullText string         // populated when no layout regions (full page OCR)
	Model    string
	TotalMs  int
}

// Tool is the interface for OCR implementations.
type Tool interface {
	// Name returns the tool identifier (e.g. "qari").
	Name() string
	// Start starts the OCR tool (e.g. launches a Docker container).
	Start(ctx context.Context) error
	// Stop stops the OCR tool (e.g. stops the Docker container).
	Stop(ctx context.Context) error
	// IsReady reports whether the OCR tool is ready to accept requests.
	IsReady(ctx context.Context) bool
	// RecognizeRegions OCRs cropped regions from a page image.
	RecognizeRegions(ctx context.Context, imagePath string, regions []RegionInput) (*Result, error)
	// RecognizeFullPage OCRs an entire page image when no layout regions are available.
	RecognizeFullPage(ctx context.Context, imagePath string) (*Result, error)
}

// NewTool creates an OCR tool by name.
// Known names: "qari".
// Returns nil for empty or unknown names (OCR disabled).
func NewTool(name string) Tool {
	return NewToolWithBackend(name, "")
}

// NewToolWithBackend creates an OCR tool by name with an explicit backend.
//
// Backends:
//   - "" or "docker" — run the tool inside a Docker container (default, the
//     historically validated path).
//   - "uv" — run the tool as a uv-managed Python subprocess on the host.
//     Requires `uv` installed and the matching python-tools/<tool>/ directory.
//     Adds a model auto-download on first run.
//
// Returns nil for empty or unknown tool names (OCR disabled).
func NewToolWithBackend(name, backend string) Tool {
	switch name {
	case "qari":
		t := NewQariTool("")
		t.Backend = backend
		return t
	default:
		return nil
	}
}
