package reader

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/mmdemirbas/mutercim/internal/model"
)

func TestGenerateDebugOverlay_BasicRegions(t *testing.T) {
	dir := t.TempDir()

	// Create a 200x300 solid gray test image
	pageImg := image.NewRGBA(image.Rect(0, 0, 200, 300))
	gray := color.RGBA{R: 200, G: 200, B: 200, A: 255}
	for y := range 300 {
		for x := range 200 {
			pageImg.Set(x, y, gray)
		}
	}

	regions := []model.Region{
		{ID: "r1", BBox: model.BBox{10, 10, 80, 30}, Type: model.RegionTypeHeader},
		{ID: "r2", BBox: model.BBox{10, 50, 180, 100}, Type: model.RegionTypeEntry},
		{ID: "r3", BBox: model.BBox{10, 260, 180, 30}, Type: model.RegionTypeFootnote},
	}

	readingOrder := map[string]int{
		"r1": 1,
		"r2": 2,
		"r3": 3,
	}

	outputPath := filepath.Join(dir, "debug", "001_layout.png")
	err := GenerateDebugOverlay(pageImg, regions, readingOrder, outputPath)
	if err != nil {
		t.Fatalf("GenerateDebugOverlay() error = %v", err)
	}

	// Verify output file exists
	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("output file not found: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("output file is empty")
	}

	// Verify it's a valid PNG with expected dimensions
	f, err := os.Open(outputPath)
	if err != nil {
		t.Fatalf("cannot open output: %v", err)
	}
	defer func() { _ = f.Close() }()

	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("cannot decode output PNG: %v", err)
	}

	bounds := img.Bounds()
	if bounds.Dx() != 200 || bounds.Dy() != 300 {
		t.Errorf("dimensions = %dx%d, want 200x300", bounds.Dx(), bounds.Dy())
	}

	// Verify that the header region's bbox area is NOT the original gray
	// (it should have a blue tint from the overlay)
	midX, midY := 50, 25 // middle of header region
	overlayColor := img.At(midX, midY)
	r, g, b, _ := overlayColor.RGBA()
	origR, origG, origB, _ := gray.RGBA()
	if r == origR && g == origG && b == origB {
		t.Error("header region area unchanged from original — overlay was not drawn")
	}

	// Verify a pixel outside all regions is still the original gray
	outsideColor := img.At(195, 180) // outside all regions
	oR, oG, oB, _ := outsideColor.RGBA()
	if oR != origR || oG != origG || oB != origB {
		t.Errorf("pixel outside regions changed: got (%d,%d,%d), want (%d,%d,%d)",
			oR>>8, oG>>8, oB>>8, origR>>8, origG>>8, origB>>8)
	}
}

func TestGenerateDebugOverlay_ClampsBboxToImageBounds(t *testing.T) {
	dir := t.TempDir()

	pageImg := image.NewRGBA(image.Rect(0, 0, 100, 100))

	// Region with bbox extending past the image boundary
	regions := []model.Region{
		{ID: "r1", BBox: model.BBox{80, 80, 50, 50}, Type: model.RegionTypeEntry},
	}

	outputPath := filepath.Join(dir, "clamped.png")
	err := GenerateDebugOverlay(pageImg, regions, nil, outputPath)
	if err != nil {
		t.Fatalf("GenerateDebugOverlay() error = %v", err)
	}

	// Just verify it didn't panic and produced a valid file
	f, err := os.Open(outputPath)
	if err != nil {
		t.Fatalf("cannot open output: %v", err)
	}
	defer func() { _ = f.Close() }()

	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("cannot decode output PNG: %v", err)
	}
	if img.Bounds().Dx() != 100 || img.Bounds().Dy() != 100 {
		t.Errorf("dimensions = %dx%d, want 100x100", img.Bounds().Dx(), img.Bounds().Dy())
	}
}

func TestGenerateDebugOverlay_EmptyRegions(t *testing.T) {
	dir := t.TempDir()

	pageImg := image.NewRGBA(image.Rect(0, 0, 50, 50))
	outputPath := filepath.Join(dir, "empty.png")

	err := GenerateDebugOverlay(pageImg, nil, nil, outputPath)
	if err != nil {
		t.Fatalf("GenerateDebugOverlay() with empty regions error = %v", err)
	}

	// Should still produce a valid PNG (just the original image)
	f, err := os.Open(outputPath)
	if err != nil {
		t.Fatalf("cannot open output: %v", err)
	}
	defer func() { _ = f.Close() }()

	if _, err := png.Decode(f); err != nil {
		t.Fatalf("cannot decode output PNG: %v", err)
	}
}

func TestRegionColor_AllTypes(t *testing.T) {
	types := []string{
		model.RegionTypeHeader,
		model.RegionTypeEntry,
		model.RegionTypeFootnote,
		model.RegionTypeSeparator,
		model.RegionTypePageNumber,
		model.RegionTypeTable,
		model.RegionTypeImage,
		model.RegionTypeColumnHeader,
		model.RegionTypeMarginNote,
		model.RegionTypeOther,
		"unknown_type",
	}

	seen := make(map[color.RGBA]string)
	for _, typ := range types {
		c := regionColor(typ)
		if c.A == 0 {
			t.Errorf("regionColor(%q) has zero alpha", typ)
		}
		if prev, ok := seen[c]; ok && typ != "unknown_type" && prev != model.RegionTypeOther {
			t.Errorf("regionColor(%q) = regionColor(%q) — colors should be distinct", typ, prev)
		}
		seen[c] = typ
	}
}
