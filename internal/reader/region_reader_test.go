package reader

import (
	"context"
	"fmt"
	"testing"

	"github.com/mmdemirbas/mutercim/internal/model"
)

// mockLayoutTool implements layout.Tool for testing.
type mockLayoutTool struct {
	name      string
	available bool
	regions   []model.Region
	err       error
}

func (m *mockLayoutTool) Name() string                     { return m.name }
func (m *mockLayoutTool) Available(_ context.Context) bool { return m.available }
func (m *mockLayoutTool) DetectRegions(_ context.Context, _ string) ([]model.Region, error) {
	return m.regions, m.err
}

func TestReadRegionPage_AIOnly(t *testing.T) {
	response := `{
		"regions": [
			{
				"id": "r1",
				"bbox": [400, 50, 700, 60],
				"text": "باب الألف",
				"type": "header",
				"style": {"font_size": 18, "bold": true, "direction": "rtl", "alignment": "center"}
			},
			{
				"id": "r2",
				"bbox": [800, 150, 600, 400],
				"text": "حديث أول",
				"type": "entry",
				"column": 1
			}
		],
		"reading_order": ["r1", "r2"],
		"warnings": []
	}`

	mock := &mockProvider{response: response}
	r := NewReader(mock, nil)

	page, err := r.ReadRegionPage(context.Background(), []byte("image"), "", 42, "test-model", nil)
	if err != nil {
		t.Fatalf("ReadRegionPage: %v", err)
	}

	if page.Version != "2.0" {
		t.Errorf("Version = %q, want %q", page.Version, "2.0")
	}
	if page.PageNumber != 42 {
		t.Errorf("PageNumber = %d, want 42", page.PageNumber)
	}
	if page.ReadModel != "test-model" {
		t.Errorf("ReadModel = %q", page.ReadModel)
	}
	if page.LayoutTool != "" {
		t.Errorf("LayoutTool = %q, want empty (ai-only)", page.LayoutTool)
	}
	if len(page.Regions) != 2 {
		t.Fatalf("len(Regions) = %d, want 2", len(page.Regions))
	}

	r1 := page.Regions[0]
	if r1.ID != "r1" {
		t.Errorf("Regions[0].ID = %q", r1.ID)
	}
	if r1.Type != model.RegionTypeHeader {
		t.Errorf("Regions[0].Type = %q", r1.Type)
	}
	if r1.LayoutSource != model.LayoutSourceAI {
		t.Errorf("Regions[0].LayoutSource = %q, want %q", r1.LayoutSource, model.LayoutSourceAI)
	}
	if r1.TextSource != "test-model" {
		t.Errorf("Regions[0].TextSource = %q, want %q", r1.TextSource, "test-model")
	}

	r2 := page.Regions[1]
	if r2.Column == nil || *r2.Column != 1 {
		t.Errorf("Regions[1].Column = %v, want 1", r2.Column)
	}

	if len(page.ReadingOrder) != 2 || page.ReadingOrder[0] != "r1" || page.ReadingOrder[1] != "r2" {
		t.Errorf("ReadingOrder = %v, want [r1, r2]", page.ReadingOrder)
	}
}

func TestReadRegionPage_WithLayoutTool(t *testing.T) {
	layoutRegions := []model.Region{
		{
			ID:           "r1",
			BBox:         model.BBox{400, 50, 700, 60},
			Text:         "باب الالف", // preliminary OCR
			LayoutSource: model.LayoutSourceSurya,
		},
		{
			ID:           "r2",
			BBox:         model.BBox{800, 150, 600, 400},
			Text:         "حديث اول",
			LayoutSource: model.LayoutSourceSurya,
		},
	}

	// AI response with refined text
	aiResponse := `{
		"regions": [
			{
				"id": "r1",
				"bbox": [400, 50, 700, 60],
				"text": "بَابُ الأَلِف",
				"type": "header",
				"style": {"font_size": 18, "bold": true, "direction": "rtl", "alignment": "center"}
			},
			{
				"id": "r2",
				"bbox": [800, 150, 600, 400],
				"text": "حَدِيثٌ أَوَّلُ",
				"type": "entry",
				"column": 1
			},
			{
				"id": "r3",
				"bbox": [100, 800, 1300, 200],
				"text": "حاشية",
				"type": "footnote"
			}
		],
		"reading_order": ["r1", "r2", "r3"],
		"warnings": []
	}`

	mock := &mockProvider{response: aiResponse}
	r := NewReader(mock, nil)

	lt := &mockLayoutTool{
		name:      "surya",
		available: true,
		regions:   layoutRegions,
	}

	page, err := r.ReadRegionPage(context.Background(), []byte("image"), "/tmp/page.png", 10, "test-model", lt)
	if err != nil {
		t.Fatalf("ReadRegionPage: %v", err)
	}

	if page.LayoutTool != "surya" {
		t.Errorf("LayoutTool = %q, want %q", page.LayoutTool, "surya")
	}
	if len(page.Regions) != 3 {
		t.Fatalf("len(Regions) = %d, want 3", len(page.Regions))
	}

	// r1 and r2 were from layout tool
	if page.Regions[0].LayoutSource != "surya" {
		t.Errorf("Regions[0].LayoutSource = %q, want %q", page.Regions[0].LayoutSource, "surya")
	}
	if page.Regions[0].TextSource != "test-model" {
		t.Errorf("Regions[0].TextSource = %q, want %q", page.Regions[0].TextSource, "test-model")
	}

	// r3 was added by AI (not in layout regions)
	if page.Regions[2].LayoutSource != model.LayoutSourceAI {
		t.Errorf("Regions[2].LayoutSource = %q, want %q (AI-added region)", page.Regions[2].LayoutSource, model.LayoutSourceAI)
	}
}

func TestReadRegionPage_LayoutToolUnavailable_FallsBackToAIOnly(t *testing.T) {
	response := `{
		"regions": [
			{"id": "r1", "bbox": [0,0,100,100], "text": "text", "type": "entry"}
		],
		"reading_order": ["r1"],
		"warnings": []
	}`

	mock := &mockProvider{response: response}
	r := NewReader(mock, nil)

	lt := &mockLayoutTool{
		name:      "surya",
		available: false, // not available
	}

	page, err := r.ReadRegionPage(context.Background(), []byte("image"), "", 1, "test-model", lt)
	if err != nil {
		t.Fatalf("ReadRegionPage: %v", err)
	}

	// Should fall back to AI-only (no layout tool name)
	if page.LayoutTool != "" {
		t.Errorf("LayoutTool = %q, want empty (fallback to ai-only)", page.LayoutTool)
	}
}

func TestReadRegionPage_LayoutToolError_FallsBackToAIOnly(t *testing.T) {
	response := `{
		"regions": [
			{"id": "r1", "bbox": [0,0,100,100], "text": "text", "type": "entry"}
		],
		"reading_order": ["r1"],
		"warnings": []
	}`

	mock := &mockProvider{response: response}
	r := NewReader(mock, nil)

	lt := &mockLayoutTool{
		name:      "surya",
		available: true,
		err:       fmt.Errorf("docker container crashed"),
	}

	page, err := r.ReadRegionPage(context.Background(), []byte("image"), "/tmp/page.png", 1, "test-model", lt)
	if err != nil {
		t.Fatalf("ReadRegionPage: %v", err)
	}

	// Should fall back to AI-only due to error
	if page.LayoutTool != "" {
		t.Errorf("LayoutTool = %q, want empty (error fallback)", page.LayoutTool)
	}
}

func TestReadRegionPage_ProviderError(t *testing.T) {
	mock := &mockProvider{err: fmt.Errorf("provider error")}
	r := NewReader(mock, nil)

	_, err := r.ReadRegionPage(context.Background(), []byte("image"), "", 1, "test-model", nil)
	if err == nil {
		t.Fatal("expected error when provider fails")
	}
}

func TestReadRegionPage_BadJSON_PreservesRawText(t *testing.T) {
	mock := &mockProvider{response: "Not JSON at all"}
	r := NewReader(mock, nil)

	page, err := r.ReadRegionPage(context.Background(), []byte("image"), "", 5, "test-model", nil)
	if err != nil {
		t.Fatalf("should not error on bad JSON: %v", err)
	}
	if page.RawText != "Not JSON at all" {
		t.Errorf("RawText = %q, want original response", page.RawText)
	}
	if len(page.Warnings) == 0 {
		t.Error("expected warnings on JSON failure")
	}
	if page.Version != "2.0" {
		t.Errorf("Version = %q, want %q", page.Version, "2.0")
	}
}

func TestReadRegionPage_InvalidJSONStructure(t *testing.T) {
	mock := &mockProvider{response: `{"regions": "not_an_array"}`}
	r := NewReader(mock, nil)

	page, err := r.ReadRegionPage(context.Background(), []byte("image"), "", 1, "test-model", nil)
	if err != nil {
		t.Fatalf("should not error: %v", err)
	}
	if page.RawText == "" {
		t.Error("expected raw text preserved on unmarshal failure")
	}
	if len(page.Warnings) == 0 {
		t.Error("expected warnings on unmarshal failure")
	}
}

func TestReadRegionPage_CodeBlockResponse(t *testing.T) {
	response := "```json\n{\"regions\": [{\"id\": \"r1\", \"bbox\": [0,0,100,100], \"text\": \"test\", \"type\": \"entry\"}], \"reading_order\": [\"r1\"], \"warnings\": []}\n```"

	mock := &mockProvider{response: response}
	r := NewReader(mock, nil)

	page, err := r.ReadRegionPage(context.Background(), []byte("image"), "", 1, "test-model", nil)
	if err != nil {
		t.Fatalf("ReadRegionPage: %v", err)
	}
	if len(page.Regions) != 1 {
		t.Errorf("len(Regions) = %d, want 1", len(page.Regions))
	}
}

func TestReadRegionPage_NilLayoutTool(t *testing.T) {
	response := `{"regions": [], "reading_order": [], "warnings": []}`
	mock := &mockProvider{response: response}
	r := NewReader(mock, nil)

	page, err := r.ReadRegionPage(context.Background(), []byte("image"), "", 1, "test-model", nil)
	if err != nil {
		t.Fatalf("ReadRegionPage: %v", err)
	}
	if page.LayoutTool != "" {
		t.Errorf("LayoutTool = %q, want empty", page.LayoutTool)
	}
}

func TestStrategyName(t *testing.T) {
	tests := []struct {
		layout string
		want   string
	}{
		{"surya", "local+ai"},
		{"", "ai-only"},
	}
	for _, tt := range tests {
		got := strategyName(tt.layout)
		if got != tt.want {
			t.Errorf("strategyName(%q) = %q, want %q", tt.layout, got, tt.want)
		}
	}
}

func TestIsLayoutRegion(t *testing.T) {
	regions := []model.Region{
		{ID: "r1"},
		{ID: "r2"},
	}
	if !isLayoutRegion("r1", regions) {
		t.Error("r1 should be a layout region")
	}
	if !isLayoutRegion("r2", regions) {
		t.Error("r2 should be a layout region")
	}
	if isLayoutRegion("r3", regions) {
		t.Error("r3 should not be a layout region")
	}
	if isLayoutRegion("r1", nil) {
		t.Error("r1 should not match nil regions")
	}
}
