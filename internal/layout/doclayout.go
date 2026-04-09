package layout

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mmdemirbas/mutercim/internal/docker"
	"github.com/mmdemirbas/mutercim/internal/model"
)

// DefaultDocLayoutImage is the Docker image used for DocLayout-YOLO detection.
const DefaultDocLayoutImage = "mutercim/doclayout-yolo:latest"

// rowClusterThreshold is the maximum vertical distance (in pixels) between
// region midpoints to consider them part of the same row.
const rowClusterThreshold = 30

// DocLayout-YOLO tunable parameters (passed to entrypoint.py via CLI flags):
//
//   --conf      Confidence threshold, 0.0-1.0 (default: 0.2). Minimum score
//               to keep a detection. Lower → more detections, possibly noisy.
//   --iou       IoU threshold for NMS, 0.0-1.0 (default: 0.7). Controls how
//               aggressively overlapping boxes are merged. Lower → less merging.
//   --imgsz     Input image size in pixels (default: 1024). The model resizes
//               the input to this size for inference. Larger → more detail but
//               slower.
//   --max-det   Maximum detections per image (default: 300).
//
// These correspond to keys in the layout_tool_params config map:
//   confidence, iou, image_size, max_det

// knownDocLayoutParams lists the parameter names this tool recognizes.
var knownDocLayoutParams = map[string]bool{
	"confidence": true,
	"iou":        true,
	"image_size": true,
	"max_det":    true,
	"direction":  true,
}

// DocLayoutTool uses the DocLayout-YOLO model running in Docker
// to detect document layout regions with bounding boxes and type labels.
// Unlike Surya (which detects text lines), DocLayout-YOLO understands
// document-level structure — columns, headers, footnotes, tables, etc.
type DocLayoutTool struct {
	// DockerImage is the Docker image to use. Defaults to DefaultDocLayoutImage.
	DockerImage string

	// DockerfileDir is the path to docker/doclayout-yolo/ for auto-building.
	// Empty means skip auto-build (used in tests).
	DockerfileDir string

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
		DockerImage:   image,
		DockerfileDir: docker.FindDockerDir("doclayout-yolo"),
		commander:     execCommander{},
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
	Regions        []docLayoutRegion        `json:"regions"`
	ReadingOrder   []string                 `json:"reading_order"`
	SeparatorY     *int                     `json:"separator_y"`
	PostProcessing *model.LayoutPostProcess `json:"post_processing"`
	ImageSize      struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	} `json:"image_size"`
}

type docLayoutRegion struct {
	ID         string  `json:"id"`
	BBox       [4]int  `json:"bbox"` // [x1, y1, x2, y2] corner format
	Type       string  `json:"type"`
	RawType    string  `json:"raw_type"`
	Confidence float64 `json:"confidence"`
	Zone       string  `json:"zone"`
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
//
// params supports these keys (all optional):
//   - confidence (float64): confidence threshold, default 0.2
//   - iou (float64): IoU threshold for NMS, default 0.7
//   - image_size (int): input image size, default 1024
//   - max_det (int): max detections, default 300
//nolint:cyclop,gocognit,funlen // JSON detection with coordinate transformation and filtering
func (d *DocLayoutTool) DetectRegions(ctx context.Context, imagePath string, params map[string]any) (*DetectResult, error) {
	// Auto-build Docker image if needed
	if d.DockerfileDir != "" {
		if err := docker.EnsureImage(ctx, d.DockerImage, d.DockerfileDir); err != nil {
			return nil, fmt.Errorf("ensure doclayout-yolo image: %w", err)
		}
	}

	// Warn on unknown params
	for k := range params {
		if !knownDocLayoutParams[k] {
			slog.Warn("unknown layout_tool_param ignored", "param", k, "tool", "doclayout-yolo")
		}
	}

	dir := filepath.Dir(imagePath)
	base := filepath.Base(imagePath)
	args := []string{
		"run", "--rm",
		"-v", filepath.ToSlash(dir) + ":/input",
		d.DockerImage,
	}

	// Append tool params as CLI flags
	if v, ok := getFloat(params, "confidence"); ok {
		args = append(args, "--conf", fmt.Sprintf("%.4f", v))
	}
	if v, ok := getFloat(params, "iou"); ok {
		args = append(args, "--iou", fmt.Sprintf("%.4f", v))
	}
	if v, ok := getInt(params, "image_size"); ok {
		args = append(args, "--imgsz", fmt.Sprintf("%d", v))
	}
	if v, ok := getInt(params, "max_det"); ok {
		args = append(args, "--max-det", fmt.Sprintf("%d", v))
	}
	if v, ok := getString(params, "direction"); ok {
		args = append(args, "--direction", v)
	}

	args = append(args, "/input/"+base)

	out, err := d.commander.Run(ctx, "docker", args...)
	if err != nil {
		return nil, fmt.Errorf("doclayout-yolo docker run: %w\noutput: %s", err, string(out))
	}

	var result docLayoutOutput
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("doclayout-yolo parse output: %w\nraw: %s", err, string(out))
	}

	// Log class distribution at DEBUG level
	classCounts := make(map[string]int)
	var confMin, confMax, confSum float64
	confMin = 1.0
	for _, dr := range result.Regions {
		classCounts[dr.Type]++
		if dr.Confidence > 0 {
			if dr.Confidence < confMin {
				confMin = dr.Confidence
			}
			if dr.Confidence > confMax {
				confMax = dr.Confidence
			}
			confSum += dr.Confidence
		}
	}
	if len(result.Regions) > 0 {
		var parts []string
		for cls, count := range classCounts {
			parts = append(parts, fmt.Sprintf("%s:%d", cls, count))
		}
		sort.Strings(parts)
		confMean := confSum / float64(len(result.Regions))
		slog.Debug("layout detection complete",
			"detections", len(result.Regions),
			"classes", strings.Join(parts, ","),
			"conf_min", fmt.Sprintf("%.2f", confMin),
			"conf_max", fmt.Sprintf("%.2f", confMax),
			"conf_mean", fmt.Sprintf("%.2f", confMean),
		)
	}

	// Convert regions — post-processing (IDs, types, zones, reading order)
	// is already done by the Python entrypoint
	regions := make([]model.Region, 0, len(result.Regions))
	for _, dr := range result.Regions {
		bbox := ConvertCornerToXYWH(dr.BBox)
		id := dr.ID
		if id == "" {
			id = fmt.Sprintf("r%d", len(regions)+1)
		}
		regions = append(regions, model.Region{
			ID:           id,
			BBox:         bbox,
			Type:         dr.Type,
			LayoutSource: model.LayoutSourceDocLayout,
			Confidence:   dr.Confidence,
			RawClass:     dr.RawType,
		})
	}

	return &DetectResult{
		Regions:        regions,
		ReadingOrder:   result.ReadingOrder,
		SeparatorY:     result.SeparatorY,
		PostProcessing: result.PostProcessing,
	}, nil
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

// getFloat extracts a float64 from a map[string]any, handling int and float types.
func getFloat(m map[string]any, key string) (float64, bool) {
	v, ok := m[key]
	if !ok {
		return 0, false
	}
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	default:
		return 0, false
	}
}

// getInt extracts an int from a map[string]any, handling int and float types.
func getInt(m map[string]any, key string) (int, bool) {
	v, ok := m[key]
	if !ok {
		return 0, false
	}
	switch val := v.(type) {
	case int:
		return val, true
	case int64:
		return int(val), true
	case float64:
		return int(val), true
	case float32:
		return int(val), true
	default:
		return 0, false
	}
}
