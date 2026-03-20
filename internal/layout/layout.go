// Package layout provides layout detection tools for page images.
// Layout tools detect text regions with bounding boxes on page images,
// providing spatial information that supplements AI-based OCR.
package layout

import (
	"context"

	"github.com/mmdemirbas/mutercim/internal/model"
)

// Tool detects text regions with bounding boxes on a page image.
type Tool interface {
	// DetectRegions analyzes a page image and returns detected text regions
	// with bounding boxes and preliminary OCR text.
	DetectRegions(ctx context.Context, imagePath string) ([]model.Region, error)

	// Available reports whether this layout tool is ready to use.
	// For Docker-based tools, this checks if Docker is running and the
	// required image is available.
	Available(ctx context.Context) bool

	// Name returns the tool identifier (e.g. "surya").
	Name() string
}

// NoneTool is a no-op layout tool that returns no regions.
// It is used when no layout tool is configured, falling back to
// AI-only region detection.
type NoneTool struct{}

// DetectRegions returns an empty region list.
func (n NoneTool) DetectRegions(_ context.Context, _ string) ([]model.Region, error) {
	return nil, nil
}

// Available always returns true — the no-op tool is always available.
func (n NoneTool) Available(_ context.Context) bool {
	return true
}

// Name returns an empty string indicating no layout tool.
func (n NoneTool) Name() string {
	return ""
}
