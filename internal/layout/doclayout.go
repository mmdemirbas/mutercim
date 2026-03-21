package layout

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mmdemirbas/mutercim/internal/model"
)

// DefaultDocLayoutImage is the Docker image used for DocLayout-YOLO detection.
const DefaultDocLayoutImage = "mutercim/doclayout-yolo:latest"

// MinConfidence is the minimum confidence threshold for accepting a region.
const MinConfidence = 0.2

// rowClusterThreshold is the maximum vertical distance (in pixels) between
// region midpoints to consider them part of the same row.
const rowClusterThreshold = 30

// DocLayoutTool uses the DocLayout-YOLO model running in Docker
// to detect document layout regions with bounding boxes and type labels.
// Unlike Surya (which detects text lines), DocLayout-YOLO understands
// document-level structure — columns, headers, footnotes, tables, etc.
type DocLayoutTool struct {
	// DockerImage is the Docker image to use. Defaults to DefaultDocLayoutImage.
	DockerImage string

	// commander abstracts command execution for testing.
	commander Commander
}

// NewDocLayoutTool creates a DocLayoutTool with the given Docker image.
// If image is empty, DefaultDocLayoutImage is used.
func NewDocLayoutTool(image string) *DocLayoutTool {
	if image == "" {
		image = DefaultDocLayoutImage
	}
	return &DocLayoutTool{
		DockerImage: image,
		commander:   execCommander{},
	}
}

// newDocLayoutToolWithCommander creates a DocLayoutTool with a custom commander for testing.
func newDocLayoutToolWithCommander(image string, cmd Commander) *DocLayoutTool {
	if image == "" {
		image = DefaultDocLayoutImage
	}
	return &DocLayoutTool{
		DockerImage: image,
		commander:   cmd,
	}
}

// Name returns "doclayout-yolo".
func (d *DocLayoutTool) Name() string {
	return model.LayoutSourceDocLayout
}

// Available checks if Docker is running and the DocLayout-YOLO image exists.
func (d *DocLayoutTool) Available(ctx context.Context) bool {
	out, err := d.commander.Run(ctx, "docker", "info", "--format", "{{.ServerVersion}}")
	if err != nil {
		slog.Debug("docker not available", "error", err)
		return false
	}
	if strings.TrimSpace(string(out)) == "" {
		slog.Debug("docker info returned empty version")
		return false
	}

	out, err = d.commander.Run(ctx, "docker", "image", "inspect", d.DockerImage, "--format", "{{.ID}}")
	if err != nil {
		slog.Debug("doclayout-yolo image not found", "image", d.DockerImage, "error", err)
		return false
	}
	if strings.TrimSpace(string(out)) == "" {
		slog.Debug("doclayout-yolo image inspect returned empty ID", "image", d.DockerImage)
		return false
	}

	return true
}

// docLayoutOutput is the JSON structure returned by the DocLayout-YOLO container.
type docLayoutOutput struct {
	Regions   []docLayoutRegion `json:"regions"`
	ImageSize struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	} `json:"image_size"`
}

type docLayoutRegion struct {
	BBox       [4]int  `json:"bbox"` // [x1, y1, x2, y2] corner format
	Type       string  `json:"type"`
	Confidence float64 `json:"confidence"`
}

// docLayoutTypeMap maps DocLayout-YOLO type labels to our region types.
var docLayoutTypeMap = map[string]string{
	"title":           model.RegionTypeHeader,
	"plain text":      model.RegionTypeEntry,
	"text":            model.RegionTypeEntry,
	"abandon":         "", // skip: artifacts, noise
	"figure":          model.RegionTypeImage,
	"figure_caption":  "caption",
	"table":           model.RegionTypeTable,
	"table_caption":   "caption",
	"table_footnote":  model.RegionTypeFootnote,
	"isolate_formula": "formula",
	"formula_caption": "caption",
	"page-header":     model.RegionTypeHeader,
	"page-footer":     model.RegionTypePageNumber,
	"footnote":        model.RegionTypeFootnote,
	"list-item":       model.RegionTypeEntry,
	"section-header":  model.RegionTypeHeader,
}

// DetectRegions runs the DocLayout-YOLO Docker container on the given image
// and returns detected regions with bounding boxes and type labels.
// Regions have NO text content — only bbox and type. Text is filled in by
// the vision LLM in the next step.
func (d *DocLayoutTool) DetectRegions(ctx context.Context, imagePath string) ([]model.Region, error) {
	dir := filepath.Dir(imagePath)
	base := filepath.Base(imagePath)
	args := []string{
		"run", "--rm",
		"-v", dir + ":/input",
		d.DockerImage,
		"/input/" + base,
	}

	out, err := d.commander.Run(ctx, "docker", args...)
	if err != nil {
		return nil, fmt.Errorf("doclayout-yolo docker run: %w\noutput: %s", err, string(out))
	}

	var result docLayoutOutput
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("doclayout-yolo parse output: %w\nraw: %s", err, string(out))
	}

	var regions []model.Region
	id := 1
	for _, dr := range result.Regions {
		if dr.Confidence < MinConfidence {
			continue
		}

		regionType := MapDocLayoutType(dr.Type)
		if regionType == "" {
			continue // skip "abandon" etc.
		}

		// Convert from [x1,y1,x2,y2] to [x,y,w,h]
		bbox := ConvertCornerToXYWH(dr.BBox)

		regions = append(regions, model.Region{
			ID:           fmt.Sprintf("r%d", id),
			BBox:         bbox,
			Type:         regionType,
			LayoutSource: model.LayoutSourceDocLayout,
		})
		id++
	}

	// Sort by reading order: top-to-bottom, right-to-left (RTL)
	SortReadingOrderRTL(regions)

	// Reassign IDs after sorting
	for i := range regions {
		regions[i].ID = fmt.Sprintf("r%d", i+1)
	}

	return regions, nil
}

// MapDocLayoutType maps a DocLayout-YOLO type label to our region type.
// Returns empty string for types that should be skipped (e.g. "abandon").
func MapDocLayoutType(dlType string) string {
	if mapped, ok := docLayoutTypeMap[dlType]; ok {
		return mapped
	}
	return model.RegionTypeOther
}

// ConvertCornerToXYWH converts a bounding box from [x1, y1, x2, y2] corner
// format to [x, y, width, height] format.
func ConvertCornerToXYWH(corner [4]int) model.BBox {
	return model.BBox{
		corner[0],
		corner[1],
		corner[2] - corner[0],
		corner[3] - corner[1],
	}
}

// SortReadingOrderRTL sorts regions by reading order for right-to-left text:
// rows are ordered top-to-bottom, and within each row, regions are ordered
// right-to-left (by descending X coordinate).
// Regions with similar Y midpoints (within rowClusterThreshold pixels)
// are considered part of the same row.
func SortReadingOrderRTL(regions []model.Region) {
	sort.SliceStable(regions, func(i, j int) bool {
		midYi := regions[i].BBox[1] + regions[i].BBox[3]/2
		midYj := regions[j].BBox[1] + regions[j].BBox[3]/2

		// If Y midpoints are close enough, they're in the same row
		if abs(midYi-midYj) <= rowClusterThreshold {
			// RTL: higher X (further right) comes first
			return regions[i].BBox[0] > regions[j].BBox[0]
		}
		// Different rows: top comes first
		return midYi < midYj
	})
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
