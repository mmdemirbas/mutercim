package layout

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/mmdemirbas/mutercim/internal/model"
)

func TestDocLayoutTool_Name(t *testing.T) {
	tool := NewDocLayoutTool("")
	if got := tool.Name(); got != "doclayout-yolo" {
		t.Errorf("Name = %q, want %q", got, "doclayout-yolo")
	}
}

func TestDocLayoutTool_DefaultImage(t *testing.T) {
	tool := NewDocLayoutTool("")
	if tool.DockerImage != DefaultDocLayoutImage {
		t.Errorf("DockerImage = %q, want %q", tool.DockerImage, DefaultDocLayoutImage)
	}
}

func TestDocLayoutTool_CustomImage(t *testing.T) {
	tool := NewDocLayoutTool("my-doclayout:v2")
	if tool.DockerImage != "my-doclayout:v2" {
		t.Errorf("DockerImage = %q, want %q", tool.DockerImage, "my-doclayout:v2")
	}
}

func TestDocLayoutTool_Available_DockerNotRunning(t *testing.T) {
	cmd := &mockCommander{
		returns: []mockReturn{
			{output: nil, err: fmt.Errorf("Cannot connect to the Docker daemon")},
		},
	}
	tool := newDocLayoutToolWithCommander("", cmd)

	if tool.Available(context.Background()) {
		t.Error("Available = true, want false (docker not running)")
	}
	if len(cmd.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(cmd.calls))
	}
}

func TestDocLayoutTool_Available_EmptyDockerVersion(t *testing.T) {
	cmd := &mockCommander{
		returns: []mockReturn{
			{output: []byte(""), err: nil},
		},
	}
	tool := newDocLayoutToolWithCommander("", cmd)

	if tool.Available(context.Background()) {
		t.Error("Available = true, want false (empty version)")
	}
}

func TestDocLayoutTool_Available_ImageNotPulled(t *testing.T) {
	cmd := &mockCommander{
		returns: []mockReturn{
			{output: []byte("24.0.7\n"), err: nil},
			{output: nil, err: fmt.Errorf("No such image")},
		},
	}
	tool := newDocLayoutToolWithCommander("", cmd)

	if tool.Available(context.Background()) {
		t.Error("Available = true, want false (image not pulled)")
	}
	if len(cmd.calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(cmd.calls))
	}
}

func TestDocLayoutTool_Available_EmptyImageID(t *testing.T) {
	cmd := &mockCommander{
		returns: []mockReturn{
			{output: []byte("24.0.7\n"), err: nil},
			{output: []byte("  \n"), err: nil},
		},
	}
	tool := newDocLayoutToolWithCommander("", cmd)

	if tool.Available(context.Background()) {
		t.Error("Available = true, want false (empty image ID)")
	}
}

func TestDocLayoutTool_Available_Success(t *testing.T) {
	cmd := &mockCommander{
		returns: []mockReturn{
			{output: []byte("24.0.7\n"), err: nil},
			{output: []byte("sha256:abc123\n"), err: nil},
		},
	}
	tool := newDocLayoutToolWithCommander("", cmd)

	if !tool.Available(context.Background()) {
		t.Error("Available = false, want true")
	}
	if len(cmd.calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(cmd.calls))
	}

	// Verify docker image inspect uses correct image name
	inspectArgs := cmd.calls[1].args
	found := false
	for _, arg := range inspectArgs {
		if arg == DefaultDocLayoutImage {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("docker image inspect should use image %q, args = %v", DefaultDocLayoutImage, inspectArgs)
	}
}

func TestDocLayoutTool_DetectRegions_Success(t *testing.T) {
	// Simulate post-processed output from the Python entrypoint
	dlJSON := docLayoutOutput{
		Regions: []docLayoutRegion{
			{ID: "r1", BBox: [4]int{100, 50, 800, 100}, Type: "header", RawType: "title", Confidence: 0.95, Zone: "header"},
			{ID: "r2", BBox: [4]int{500, 200, 800, 500}, Type: "entry", RawType: "plain text", Confidence: 0.85, Zone: "entry"},
			{ID: "r3", BBox: [4]int{100, 200, 400, 500}, Type: "entry", RawType: "plain text", Confidence: 0.80, Zone: "entry"},
			{ID: "r4", BBox: [4]int{100, 550, 800, 580}, Type: "page_number", RawType: "page-footer", Confidence: 0.70, Zone: "page_number"},
		},
		ReadingOrder: []string{"r1", "r2", "r3", "r4"},
	}
	dlJSON.ImageSize.Width = 1000
	dlJSON.ImageSize.Height = 1500
	out, _ := json.Marshal(dlJSON)

	cmd := &mockCommander{
		returns: []mockReturn{
			{output: out, err: nil},
		},
	}
	tool := newDocLayoutToolWithCommander("", cmd)

	result, err := tool.DetectRegions(context.Background(), "/tmp/pages/156.png", nil)
	if err != nil {
		t.Fatalf("DetectRegions: %v", err)
	}
	if len(result.Regions) != 4 {
		t.Fatalf("len(regions) = %d, want 4", len(result.Regions))
	}

	// Types come from Python post-processing (already refined)
	if result.Regions[0].Type != "header" {
		t.Errorf("regions[0].Type = %q, want %q", result.Regions[0].Type, "header")
	}
	// BBox should be converted from [x1,y1,x2,y2] to [x,y,w,h]
	if result.Regions[0].BBox != (model.BBox{100, 50, 700, 50}) {
		t.Errorf("regions[0].BBox = %v, want [100,50,700,50]", result.Regions[0].BBox)
	}
	if result.Regions[0].LayoutSource != model.LayoutSourceDocLayout {
		t.Errorf("regions[0].LayoutSource = %q, want %q", result.Regions[0].LayoutSource, model.LayoutSourceDocLayout)
	}
	if result.Regions[0].Confidence != 0.95 {
		t.Errorf("regions[0].Confidence = %v, want 0.95", result.Regions[0].Confidence)
	}
	// RawClass comes from raw_type field
	if result.Regions[0].RawClass != "title" {
		t.Errorf("regions[0].RawClass = %q, want %q", result.Regions[0].RawClass, "title")
	}
	if result.Regions[0].Text != "" {
		t.Errorf("regions[0].Text = %q, want empty", result.Regions[0].Text)
	}
	// IDs preserved from Python output
	if result.Regions[0].ID != "r1" {
		t.Errorf("regions[0].ID = %q, want %q", result.Regions[0].ID, "r1")
	}

	// Reading order from Python
	if len(result.ReadingOrder) != 4 {
		t.Errorf("ReadingOrder len = %d, want 4", len(result.ReadingOrder))
	}

	// Verify docker command args
	call := cmd.calls[0]
	if call.name != "docker" {
		t.Errorf("call.name = %q, want %q", call.name, "docker")
	}
	// Check volume mount
	foundMount := false
	for _, arg := range call.args {
		if arg == "/tmp/pages:/input" {
			foundMount = true
			break
		}
	}
	if !foundMount {
		t.Errorf("expected volume mount /tmp/pages:/input, args = %v", call.args)
	}
	// Check container path
	foundPath := false
	for _, arg := range call.args {
		if arg == "/input/156.png" {
			foundPath = true
			break
		}
	}
	if !foundPath {
		t.Errorf("expected container path /input/156.png, args = %v", call.args)
	}
}

func TestDocLayoutTool_DetectRegions_DockerError(t *testing.T) {
	cmd := &mockCommander{
		returns: []mockReturn{
			{output: []byte("error running container"), err: fmt.Errorf("exit status 1")},
		},
	}
	tool := newDocLayoutToolWithCommander("", cmd)

	_, err := tool.DetectRegions(context.Background(), "/tmp/page.png", nil)
	if err == nil {
		t.Fatal("DetectRegions: expected error, got nil")
	}
	if got := err.Error(); !contains(got, "doclayout-yolo docker run") {
		t.Errorf("error = %q, want to contain %q", got, "doclayout-yolo docker run")
	}
}

func TestDocLayoutTool_DetectRegions_InvalidJSON(t *testing.T) {
	cmd := &mockCommander{
		returns: []mockReturn{
			{output: []byte("not json at all"), err: nil},
		},
	}
	tool := newDocLayoutToolWithCommander("", cmd)

	_, err := tool.DetectRegions(context.Background(), "/tmp/page.png", nil)
	if err == nil {
		t.Fatal("DetectRegions: expected error, got nil")
	}
	if got := err.Error(); !contains(got, "doclayout-yolo parse output") {
		t.Errorf("error = %q, want to contain %q", got, "doclayout-yolo parse output")
	}
}

func TestDocLayoutTool_DetectRegions_EmptyRegions(t *testing.T) {
	out, _ := json.Marshal(docLayoutOutput{Regions: []docLayoutRegion{}})
	cmd := &mockCommander{
		returns: []mockReturn{
			{output: out, err: nil},
		},
	}
	tool := newDocLayoutToolWithCommander("", cmd)

	result, err := tool.DetectRegions(context.Background(), "/tmp/page.png", nil)
	if err != nil {
		t.Fatalf("DetectRegions: %v", err)
	}
	if len(result.Regions) != 0 {
		t.Errorf("len(regions) = %d, want 0", len(result.Regions))
	}
}

func TestDocLayoutTool_DetectRegions_AllRegionsPassedThrough(t *testing.T) {
	// Go no longer filters by confidence — Python handles that.
	// All regions from Docker output are passed through.
	dlJSON := docLayoutOutput{
		Regions: []docLayoutRegion{
			{ID: "r1", BBox: [4]int{100, 50, 800, 100}, Type: "header", RawType: "title", Confidence: 0.95},
			{ID: "r2", BBox: [4]int{100, 200, 800, 500}, Type: "entry", RawType: "text", Confidence: 0.15},
			{ID: "r3", BBox: [4]int{100, 550, 800, 580}, Type: "entry", RawType: "text", Confidence: 0.19},
			{ID: "r4", BBox: [4]int{100, 600, 800, 650}, Type: "footnote", RawType: "footnote", Confidence: 0.20},
		},
	}
	out, _ := json.Marshal(dlJSON)

	cmd := &mockCommander{
		returns: []mockReturn{
			{output: out, err: nil},
		},
	}
	tool := newDocLayoutToolWithCommander("", cmd)

	result, err := tool.DetectRegions(context.Background(), "/tmp/page.png", nil)
	if err != nil {
		t.Fatalf("DetectRegions: %v", err)
	}
	if len(result.Regions) != 4 {
		t.Fatalf("len(regions) = %d, want 4", len(result.Regions))
	}
	if result.Regions[0].Type != "header" {
		t.Errorf("regions[0].Type = %q, want %q", result.Regions[0].Type, "header")
	}
}

func TestDocLayoutTool_DetectRegions_AbandonPassedThrough(t *testing.T) {
	// Go no longer skips "abandon" regions — Python post-processing uses them
	// for separator detection and then refines their type.
	dlJSON := docLayoutOutput{
		Regions: []docLayoutRegion{
			{ID: "r1", BBox: [4]int{100, 50, 800, 100}, Type: "header", RawType: "title", Confidence: 0.95},
			{ID: "r2", BBox: [4]int{50, 200, 100, 300}, Type: "separator", RawType: "abandon", Confidence: 0.90},
		},
	}
	out, _ := json.Marshal(dlJSON)

	cmd := &mockCommander{
		returns: []mockReturn{
			{output: out, err: nil},
		},
	}
	tool := newDocLayoutToolWithCommander("", cmd)

	result, err := tool.DetectRegions(context.Background(), "/tmp/page.png", nil)
	if err != nil {
		t.Fatalf("DetectRegions: %v", err)
	}
	if len(result.Regions) != 2 {
		t.Fatalf("len(regions) = %d, want 2 (all regions passed through)", len(result.Regions))
	}
	if result.Regions[1].Type != "separator" {
		t.Errorf("regions[1].Type = %q, want %q", result.Regions[1].Type, "separator")
	}
	if result.Regions[1].RawClass != "abandon" {
		t.Errorf("regions[1].RawClass = %q, want %q", result.Regions[1].RawClass, "abandon")
	}
}

// TestConvertCornerToXYWH tests bbox format conversion.
func TestConvertCornerToXYWH(t *testing.T) {
	tests := []struct {
		name   string
		corner [4]int
		want   model.BBox
	}{
		{
			name:   "normal box",
			corner: [4]int{100, 200, 500, 400},
			want:   model.BBox{100, 200, 400, 200},
		},
		{
			name:   "zero origin",
			corner: [4]int{0, 0, 100, 50},
			want:   model.BBox{0, 0, 100, 50},
		},
		{
			name:   "same point (zero area)",
			corner: [4]int{50, 50, 50, 50},
			want:   model.BBox{50, 50, 0, 0},
		},
		{
			name:   "large values",
			corner: [4]int{1000, 2000, 3000, 4000},
			want:   model.BBox{1000, 2000, 2000, 2000},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertCornerToXYWH(tt.corner)
			if got != tt.want {
				t.Errorf("ConvertCornerToXYWH(%v) = %v, want %v", tt.corner, got, tt.want)
			}
		})
	}
}

// TestMapDocLayoutType tests type mapping.
func TestMapDocLayoutType(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"title", model.RegionTypeHeader},
		{"plain text", model.RegionTypeEntry},
		{"text", model.RegionTypeEntry},
		{"abandon", ""},
		{"figure", model.RegionTypeImage},
		{"figure_caption", "caption"},
		{"table", model.RegionTypeTable},
		{"table_caption", "caption"},
		{"table_footnote", model.RegionTypeFootnote},
		{"isolate_formula", "formula"},
		{"formula_caption", "caption"},
		{"page-header", model.RegionTypeHeader},
		{"page-footer", model.RegionTypePageNumber},
		{"footnote", model.RegionTypeFootnote},
		{"list-item", model.RegionTypeEntry},
		{"section-header", model.RegionTypeHeader},
		{"unknown-type", model.RegionTypeOther},
		{"", model.RegionTypeOther},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := MapDocLayoutType(tt.input)
			if got != tt.want {
				t.Errorf("MapDocLayoutType(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestSortReadingOrderRTL_TwoColumnPage tests RTL reading order with two columns.
func TestSortReadingOrderRTL_TwoColumnPage(t *testing.T) {
	regions := []model.Region{
		{ID: "left-col", BBox: model.BBox{100, 200, 300, 300}},  // left column
		{ID: "right-col", BBox: model.BBox{500, 200, 300, 300}}, // right column
		{ID: "header", BBox: model.BBox{100, 50, 700, 50}},      // header at top
		{ID: "footer", BBox: model.BBox{100, 550, 700, 30}},     // footer at bottom
	}

	SortReadingOrderRTL(regions)

	// Expected order: header (top), right-col (same row, higher X), left-col, footer (bottom)
	wantOrder := []string{"header", "right-col", "left-col", "footer"}
	for i, want := range wantOrder {
		if regions[i].ID != want {
			t.Errorf("position %d: got %q, want %q", i, regions[i].ID, want)
		}
	}
}

// TestSortReadingOrderRTL_SingleColumn tests top-to-bottom ordering.
func TestSortReadingOrderRTL_SingleColumn(t *testing.T) {
	regions := []model.Region{
		{ID: "bottom", BBox: model.BBox{100, 500, 600, 50}},
		{ID: "middle", BBox: model.BBox{100, 250, 600, 50}},
		{ID: "top", BBox: model.BBox{100, 50, 600, 50}},
	}

	SortReadingOrderRTL(regions)

	wantOrder := []string{"top", "middle", "bottom"}
	for i, want := range wantOrder {
		if regions[i].ID != want {
			t.Errorf("position %d: got %q, want %q", i, regions[i].ID, want)
		}
	}
}

// TestSortReadingOrderRTL_EmptySlice handles empty input gracefully.
func TestSortReadingOrderRTL_EmptySlice(t *testing.T) {
	var regions []model.Region
	SortReadingOrderRTL(regions) // should not panic
	if len(regions) != 0 {
		t.Errorf("len(regions) = %d, want 0", len(regions))
	}
}

// TestSortReadingOrderRTL_SingleRegion handles single region.
func TestSortReadingOrderRTL_SingleRegion(t *testing.T) {
	regions := []model.Region{
		{ID: "only", BBox: model.BBox{100, 200, 300, 50}},
	}
	SortReadingOrderRTL(regions)
	if regions[0].ID != "only" {
		t.Errorf("ID = %q, want %q", regions[0].ID, "only")
	}
}

func TestDocLayoutTool_DetectRegions_WithParams(t *testing.T) {
	dlJSON := docLayoutOutput{
		Regions: []docLayoutRegion{
			{BBox: [4]int{100, 50, 800, 100}, Type: "title", Confidence: 0.95},
		},
	}
	dlJSON.ImageSize.Width = 1000
	dlJSON.ImageSize.Height = 1500
	out, _ := json.Marshal(dlJSON)

	cmd := &mockCommander{
		returns: []mockReturn{
			{output: out, err: nil},
		},
	}
	tool := newDocLayoutToolWithCommander("", cmd)

	params := map[string]any{
		"confidence": 0.15,
		"iou":        0.3,
	}
	_, err := tool.DetectRegions(context.Background(), "/tmp/page.png", params)
	if err != nil {
		t.Fatalf("DetectRegions: %v", err)
	}

	if len(cmd.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(cmd.calls))
	}

	args := cmd.calls[0].args
	// Check that --conf 0.1500 and --iou 0.3000 appear in the args
	foundConf := false
	foundIou := false
	for i, arg := range args {
		if arg == "--conf" && i+1 < len(args) && args[i+1] == "0.1500" {
			foundConf = true
		}
		if arg == "--iou" && i+1 < len(args) && args[i+1] == "0.3000" {
			foundIou = true
		}
	}
	if !foundConf {
		t.Errorf("expected --conf 0.1500 in args, got %v", args)
	}
	if !foundIou {
		t.Errorf("expected --iou 0.3000 in args, got %v", args)
	}
}

// TestNewTool_Factory tests the layout tool factory function.
func TestNewTool_Factory(t *testing.T) {
	tests := []struct {
		name     string
		wantType string
		wantNil  bool
	}{
		{"doclayout-yolo", "doclayout-yolo", false},
		{"surya", "surya", false},
		{"", "", true},
		{"unknown", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool := NewTool(tt.name)
			if tt.wantNil {
				if tool != nil {
					t.Errorf("NewTool(%q) = %v, want nil", tt.name, tool)
				}
				return
			}
			if tool == nil {
				t.Fatalf("NewTool(%q) = nil, want non-nil", tt.name)
			}
			if got := tool.Name(); got != tt.wantType {
				t.Errorf("NewTool(%q).Name() = %q, want %q", tt.name, got, tt.wantType)
			}
		})
	}
}
