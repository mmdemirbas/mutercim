package model

import (
	"encoding/json"
	"testing"
)

func TestRegionPage_JSONRoundTrip(t *testing.T) {
	col1 := 1
	col2 := 2
	original := RegionPage{
		Version:    "2.0",
		PageNumber: 161,
		PageSize:   PageSize{Width: 1500, Height: 2200},
		ReadModel:  "gemini/gemini-2.5-flash-lite",
		LayoutTool: "surya",
		Regions: []Region{
			{
				ID:           "r1",
				BBox:         BBox{400, 50, 700, 60},
				Text:         "حرف الألف مع الذال",
				Type:         RegionTypeHeader,
				Style:        &RegionStyle{FontSize: 18, Bold: true, Direction: "rtl", Alignment: "center"},
				LayoutSource: LayoutSourceSurya,
				TextSource:   "gemini",
			},
			{
				ID:           "r2",
				BBox:         BBox{800, 150, 600, 600},
				Text:         "١٠٦٠) اذْهَبِي ، فَأَطْعِمِي هَذَا عِيَالَكِ",
				Type:         RegionTypeEntry,
				Style:        &RegionStyle{FontSize: 14, Direction: "rtl"},
				LayoutSource: LayoutSourceSurya,
				TextSource:   "gemini",
				Column:       &col1,
			},
			{
				ID:           "r3",
				BBox:         BBox{100, 150, 600, 600},
				Text:         "١٠٦٥) اذْهَبِي ، فَقَدْ بَايَعْتُكِ",
				Type:         RegionTypeEntry,
				Style:        &RegionStyle{FontSize: 14, Direction: "rtl"},
				LayoutSource: LayoutSourceSurya,
				TextSource:   "gemini",
				Column:       &col2,
			},
			{
				ID:           "sep1",
				BBox:         BBox{100, 800, 1300, 10},
				Text:         "",
				Type:         RegionTypeSeparator,
				LayoutSource: LayoutSourceSurya,
			},
			{
				ID:           "r10",
				BBox:         BBox{100, 830, 1300, 200},
				Text:         "(م ، طب) قالها ﷺ للبدوية المشركة...",
				Type:         RegionTypeFootnote,
				Style:        &RegionStyle{FontSize: 11, Direction: "rtl"},
				LayoutSource: LayoutSourceSurya,
				TextSource:   "gemini",
			},
		},
		ReadingOrder: []string{"r1", "r2", "r3", "sep1", "r10"},
		Warnings:     []string{},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded RegionPage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.Version != original.Version {
		t.Errorf("Version = %q, want %q", decoded.Version, original.Version)
	}
	if decoded.PageNumber != original.PageNumber {
		t.Errorf("PageNumber = %d, want %d", decoded.PageNumber, original.PageNumber)
	}
	if decoded.PageSize != original.PageSize {
		t.Errorf("PageSize = %+v, want %+v", decoded.PageSize, original.PageSize)
	}
	if decoded.ReadModel != original.ReadModel {
		t.Errorf("ReadModel = %q, want %q", decoded.ReadModel, original.ReadModel)
	}
	if decoded.LayoutTool != original.LayoutTool {
		t.Errorf("LayoutTool = %q, want %q", decoded.LayoutTool, original.LayoutTool)
	}
	if len(decoded.Regions) != len(original.Regions) {
		t.Fatalf("len(Regions) = %d, want %d", len(decoded.Regions), len(original.Regions))
	}
	if len(decoded.ReadingOrder) != len(original.ReadingOrder) {
		t.Fatalf("len(ReadingOrder) = %d, want %d", len(decoded.ReadingOrder), len(original.ReadingOrder))
	}
	for i, id := range decoded.ReadingOrder {
		if id != original.ReadingOrder[i] {
			t.Errorf("ReadingOrder[%d] = %q, want %q", i, id, original.ReadingOrder[i])
		}
	}
}

func TestRegion_JSONOmitsOptionalFields(t *testing.T) {
	r := Region{
		ID:           "sep1",
		BBox:         BBox{100, 800, 1300, 10},
		Text:         "",
		Type:         RegionTypeSeparator,
		LayoutSource: LayoutSourceSurya,
	}

	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal to map: %v", err)
	}

	if _, ok := raw["style"]; ok {
		t.Error("style should be omitted when nil")
	}
	if _, ok := raw["text_source"]; ok {
		t.Error("text_source should be omitted when empty")
	}
	if _, ok := raw["column"]; ok {
		t.Error("column should be omitted when nil")
	}
}

func TestBBox_JSON(t *testing.T) {
	tests := []struct {
		name string
		bbox BBox
		want string
	}{
		{"typical", BBox{100, 200, 300, 400}, "[100,200,300,400]"},
		{"zero", BBox{0, 0, 0, 0}, "[0,0,0,0]"},
		{"large", BBox{1500, 2200, 800, 600}, "[1500,2200,800,600]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.bbox)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			if string(data) != tt.want {
				t.Errorf("Marshal = %s, want %s", data, tt.want)
			}

			var decoded BBox
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}
			if decoded != tt.bbox {
				t.Errorf("Unmarshal = %v, want %v", decoded, tt.bbox)
			}
		})
	}
}

func TestRegionPage_EmptyRegions(t *testing.T) {
	page := RegionPage{
		Version:      "2.0",
		PageNumber:   1,
		PageSize:     PageSize{Width: 1500, Height: 2200},
		Regions:      []Region{},
		ReadingOrder: []string{},
		Warnings:     []string{},
	}

	data, err := json.Marshal(page)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded RegionPage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if len(decoded.Regions) != 0 {
		t.Errorf("len(Regions) = %d, want 0", len(decoded.Regions))
	}
	if len(decoded.ReadingOrder) != 0 {
		t.Errorf("len(ReadingOrder) = %d, want 0", len(decoded.ReadingOrder))
	}
}

func TestRegionPage_UnmarshalFromJSON(t *testing.T) {
	// Simulates what we'd get from an AI provider response
	input := `{
		"version": "2.0",
		"page_number": 42,
		"page_size": {"width": 1200, "height": 1800},
		"read_model": "gemini/gemini-2.5-flash-lite",
		"layout_tool": "",
		"read_timestamp": "2026-03-20T18:45:45Z",
		"regions": [
			{
				"id": "r1",
				"bbox": [50, 30, 1100, 80],
				"text": "باب الألف",
				"type": "header",
				"style": {"font_size": 20, "bold": true, "direction": "rtl", "alignment": "center"}
			},
			{
				"id": "r2",
				"bbox": [600, 150, 550, 400],
				"text": "٤٢) حديث...",
				"type": "entry",
				"column": 1
			}
		],
		"reading_order": ["r1", "r2"],
		"raw_text": "",
		"warnings": ["approximate bounding boxes"]
	}`

	var page RegionPage
	if err := json.Unmarshal([]byte(input), &page); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if page.PageNumber != 42 {
		t.Errorf("PageNumber = %d, want 42", page.PageNumber)
	}
	if page.PageSize.Width != 1200 {
		t.Errorf("PageSize.Width = %d, want 1200", page.PageSize.Width)
	}
	if len(page.Regions) != 2 {
		t.Fatalf("len(Regions) = %d, want 2", len(page.Regions))
	}

	r1 := page.Regions[0]
	if r1.ID != "r1" {
		t.Errorf("Regions[0].ID = %q, want %q", r1.ID, "r1")
	}
	if r1.BBox != (BBox{50, 30, 1100, 80}) {
		t.Errorf("Regions[0].BBox = %v, want [50,30,1100,80]", r1.BBox)
	}
	if r1.Type != RegionTypeHeader {
		t.Errorf("Regions[0].Type = %q, want %q", r1.Type, RegionTypeHeader)
	}
	if r1.Style == nil {
		t.Fatal("Regions[0].Style is nil, want non-nil")
	}
	if !r1.Style.Bold {
		t.Error("Regions[0].Style.Bold = false, want true")
	}

	r2 := page.Regions[1]
	if r2.Column == nil {
		t.Fatal("Regions[1].Column is nil, want non-nil")
	}
	if *r2.Column != 1 {
		t.Errorf("Regions[1].Column = %d, want 1", *r2.Column)
	}
	if r2.Style != nil {
		t.Errorf("Regions[1].Style = %+v, want nil (omitted in input)", r2.Style)
	}

	if len(page.Warnings) != 1 || page.Warnings[0] != "approximate bounding boxes" {
		t.Errorf("Warnings = %v, want [\"approximate bounding boxes\"]", page.Warnings)
	}
}

func TestRegionTypeConstants(t *testing.T) {
	// Verify constants match expected strings used in JSON schema
	types := map[string]string{
		"header":        RegionTypeHeader,
		"entry":         RegionTypeEntry,
		"footnote":      RegionTypeFootnote,
		"separator":     RegionTypeSeparator,
		"page_number":   RegionTypePageNumber,
		"column_header": RegionTypeColumnHeader,
		"table":         RegionTypeTable,
		"image":         RegionTypeImage,
		"margin_note":   RegionTypeMarginNote,
		"other":         RegionTypeOther,
	}
	for want, got := range types {
		if got != want {
			t.Errorf("RegionType constant = %q, want %q", got, want)
		}
	}
}
