package reader

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"

	"golang.org/x/image/font"
	"golang.org/x/image/font/inconsolata"
	"golang.org/x/image/math/fixed"

	"github.com/mmdemirbas/mutercim/internal/model"
)

// regionTypeColors maps region types to display colors.
var regionTypeColors = map[string]color.RGBA{ //nolint:gochecknoglobals
	model.RegionTypeHeader:       {R: 30, G: 100, B: 255, A: 255},  // blue
	model.RegionTypeEntry:        {R: 30, G: 200, B: 30, A: 255},   // green
	model.RegionTypeFootnote:     {R: 255, G: 150, B: 30, A: 255},  // orange
	model.RegionTypeSeparator:    {R: 230, G: 30, B: 30, A: 255},   // red
	model.RegionTypePageNumber:   {R: 140, G: 140, B: 140, A: 255}, // gray
	model.RegionTypeTable:        {R: 160, G: 40, B: 220, A: 255},  // purple
	model.RegionTypeImage:        {R: 0, G: 200, B: 220, A: 255},   // cyan
	model.RegionTypeColumnHeader: {R: 230, G: 200, B: 20, A: 255},  // yellow
	model.RegionTypeMarginNote:   {R: 240, G: 120, B: 180, A: 255}, // pink
}

// regionColor returns a display color for the given region type.
func regionColor(regionType string) color.RGBA {
	if c, ok := regionTypeColors[regionType]; ok {
		return c
	}
	return color.RGBA{R: 220, G: 220, B: 220, A: 255} // default: light gray
}

// GenerateDebugOverlay draws layout detection bounding boxes on a page image
// and saves the result as a PNG. readingOrder maps region IDs to their position
// in the reading order (1-based). If readingOrder is nil, no order numbers are drawn.
//nolint:cyclop,gocognit // bbox clamping with per-region color, label, and border drawing
func GenerateDebugOverlay(pageImg image.Image, regions []model.Region, readingOrder map[string]int, outputPath string) error {
	bounds := pageImg.Bounds()
	overlay := image.NewRGBA(bounds)
	draw.Draw(overlay, bounds, pageImg, bounds.Min, draw.Src)

	for _, region := range regions {
		x, y, w, h := region.BBox[0], region.BBox[1], region.BBox[2], region.BBox[3]

		// Clamp bbox to image bounds
		if x < bounds.Min.X {
			w -= bounds.Min.X - x
			x = bounds.Min.X
		}
		if y < bounds.Min.Y {
			h -= bounds.Min.Y - y
			y = bounds.Min.Y
		}
		if x+w > bounds.Max.X {
			w = bounds.Max.X - x
		}
		if y+h > bounds.Max.Y {
			h = bounds.Max.Y - y
		}
		if w <= 0 || h <= 0 {
			continue
		}

		rect := image.Rect(x, y, x+w, y+h)
		c := regionColor(region.Type)

		// Fill with semi-transparent color (~20% opacity)
		fill := color.RGBA{R: c.R, G: c.G, B: c.B, A: 50}
		draw.Draw(overlay, rect, image.NewUniform(fill), image.Point{}, draw.Over)

		// Draw 2px solid border
		drawRectBorder(overlay, rect, c, 2)

		// Draw label with raw class mapping and confidence
		label := formatDebugLabel(region, readingOrder)
		drawLabel(overlay, x, y, label, c)
	}

	// Write PNG atomically (tmp + rename)
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create debug dir: %w", err)
	}

	tmpPath := outputPath + ".tmp"
	f, err := os.Create(tmpPath) //nolint:gosec // G304: outputPath is internal workspace path, not user HTTP input
	if err != nil {
		return fmt.Errorf("create debug image: %w", err)
	}
	defer func() { _ = os.Remove(tmpPath) }() // clean up on failure

	if err := png.Encode(f, overlay); err != nil {
		_ = f.Close()
		return fmt.Errorf("encode debug image: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close debug image: %w", err)
	}

	if err := os.Rename(tmpPath, outputPath); err != nil {
		return fmt.Errorf("rename debug image: %w", err)
	}

	return nil
}

// drawRectBorder draws a solid border around a rectangle.
func drawRectBorder(img *image.RGBA, rect image.Rectangle, c color.RGBA, thickness int) {
	solid := image.NewUniform(c)
	// Top edge
	draw.Draw(img, image.Rect(rect.Min.X, rect.Min.Y, rect.Max.X, rect.Min.Y+thickness), solid, image.Point{}, draw.Over)
	// Bottom edge
	draw.Draw(img, image.Rect(rect.Min.X, rect.Max.Y-thickness, rect.Max.X, rect.Max.Y), solid, image.Point{}, draw.Over)
	// Left edge
	draw.Draw(img, image.Rect(rect.Min.X, rect.Min.Y, rect.Min.X+thickness, rect.Max.Y), solid, image.Point{}, draw.Over)
	// Right edge
	draw.Draw(img, image.Rect(rect.Max.X-thickness, rect.Min.Y, rect.Max.X, rect.Max.Y), solid, image.Point{}, draw.Over)
}

// formatDebugLabel builds the label text for a region in the debug overlay.
// Shows [raw→mapped conf] id format when raw class info is available.
func formatDebugLabel(region model.Region, readingOrder map[string]int) string {
	var label string
	switch {
	case region.RawClass != "" && region.RawClass != region.Type:
		label = fmt.Sprintf("[%s\u2192%s", region.RawClass, region.Type)
		if region.Confidence > 0 {
			label += fmt.Sprintf(" %.2f", region.Confidence)
		}
		label += "] " + region.ID
	case region.Confidence > 0:
		label = fmt.Sprintf("[%s %.2f] %s", region.Type, region.Confidence, region.ID)
	default:
		label = fmt.Sprintf("%s %s", region.Type, region.ID)
	}

	if orderNum, ok := readingOrder[region.ID]; ok {
		label = fmt.Sprintf("[%d] %s", orderNum, label)
	}
	return label
}

// drawLabel draws text with a filled background at the given position.
func drawLabel(img *image.RGBA, x, y int, text string, bgColor color.RGBA) {
	face := inconsolata.Regular8x16

	// Measure text width
	textWidth := len(text) * 8 // inconsolata Regular8x16 is 8px wide per glyph
	textHeight := 16
	padding := 2

	// Position label just inside the top-left of the bbox
	labelX := x + 2
	labelY := y + 2

	// Draw background rectangle for readability
	bgRect := image.Rect(labelX, labelY, labelX+textWidth+2*padding, labelY+textHeight+2*padding)
	draw.Draw(img, bgRect, image.NewUniform(bgColor), image.Point{}, draw.Over)

	// Draw white text
	d := &font.Drawer{
		Dst:  img,
		Src:  image.White,
		Face: face,
		Dot:  fixed.P(labelX+padding, labelY+padding+textHeight-2),
	}
	d.DrawString(text)
}
