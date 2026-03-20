package model

import "testing"

func TestRegionPageToReadPage_Nil(t *testing.T) {
	result := RegionPageToReadPage(nil)
	if result != nil {
		t.Errorf("expected nil, got %+v", result)
	}
}

func TestRegionPageToReadPage_EmptyRegions(t *testing.T) {
	rp := &RegionPage{
		Version:      "2.0",
		PageNumber:   42,
		ReadModel:    "gemini/gemini-2.5-flash-lite",
		Regions:      []Region{},
		ReadingOrder: []string{},
		Warnings:     []string{"test warning"},
	}

	page := RegionPageToReadPage(rp)

	if page.Version != "1.0" {
		t.Errorf("Version = %q, want %q", page.Version, "1.0")
	}
	if page.PageNumber != 42 {
		t.Errorf("PageNumber = %d, want 42", page.PageNumber)
	}
	if page.ReadModel != "gemini/gemini-2.5-flash-lite" {
		t.Errorf("ReadModel = %q", page.ReadModel)
	}
	if page.Header != nil {
		t.Errorf("Header = %+v, want nil", page.Header)
	}
	if len(page.Entries) != 0 {
		t.Errorf("len(Entries) = %d, want 0", len(page.Entries))
	}
	if len(page.Footnotes) != 0 {
		t.Errorf("len(Footnotes) = %d, want 0", len(page.Footnotes))
	}
	if len(page.ReadWarnings) != 1 || page.ReadWarnings[0] != "test warning" {
		t.Errorf("ReadWarnings = %v, want [\"test warning\"]", page.ReadWarnings)
	}
}

func TestRegionPageToReadPage_FullPage(t *testing.T) {
	col1 := 1
	col2 := 2
	rp := &RegionPage{
		Version:       "2.0",
		PageNumber:    161,
		PageSize:      PageSize{Width: 1500, Height: 2200},
		ReadModel:     "gemini/gemini-2.5-flash-lite",
		LayoutTool:    "surya",
		ReadTimestamp: "2026-03-20T18:45:45Z",
		Regions: []Region{
			{
				ID:   "r1",
				BBox: BBox{400, 50, 700, 60},
				Text: "حرف الألف مع الذال",
				Type: RegionTypeHeader,
				Style: &RegionStyle{
					FontSize:  18,
					Bold:      true,
					Direction: "rtl",
					Alignment: "center",
				},
			},
			{
				ID:     "r2",
				BBox:   BBox{800, 150, 600, 600},
				Text:   "١٠٦٠) اذْهَبِي ، فَأَطْعِمِي هَذَا عِيَالَكِ",
				Type:   RegionTypeEntry,
				Column: &col1,
			},
			{
				ID:     "r3",
				BBox:   BBox{100, 150, 600, 600},
				Text:   "١٠٦٥) اذْهَبِي ، فَقَدْ بَايَعْتُكِ",
				Type:   RegionTypeEntry,
				Column: &col2,
			},
			{
				ID:   "sep1",
				BBox: BBox{100, 800, 1300, 10},
				Text: "",
				Type: RegionTypeSeparator,
			},
			{
				ID:   "r10",
				BBox: BBox{100, 830, 1300, 200},
				Text: "(م ، طب) قالها ﷺ للبدوية المشركة...",
				Type: RegionTypeFootnote,
			},
			{
				ID:   "pn1",
				BBox: BBox{700, 2150, 100, 30},
				Text: "١٦١",
				Type: RegionTypePageNumber,
			},
		},
		ReadingOrder: []string{"r1", "r2", "r3", "sep1", "r10", "pn1"},
	}

	page := RegionPageToReadPage(rp)

	if page.Version != "1.0" {
		t.Errorf("Version = %q, want %q", page.Version, "1.0")
	}
	if page.PageNumber != 161 {
		t.Errorf("PageNumber = %d, want 161", page.PageNumber)
	}
	if page.ReadTimestamp != "2026-03-20T18:45:45Z" {
		t.Errorf("ReadTimestamp = %q", page.ReadTimestamp)
	}

	// Header
	if page.Header == nil {
		t.Fatal("Header is nil")
	}
	if page.Header.Text != "حرف الألف مع الذال" {
		t.Errorf("Header.Text = %q", page.Header.Text)
	}
	if page.Header.Type != "chapter_title" {
		t.Errorf("Header.Type = %q, want %q (font_size >= 18)", page.Header.Type, "chapter_title")
	}

	// Entries (in reading order: r2, r3)
	if len(page.Entries) != 2 {
		t.Fatalf("len(Entries) = %d, want 2", len(page.Entries))
	}
	if page.Entries[0].ArabicText != "١٠٦٠) اذْهَبِي ، فَأَطْعِمِي هَذَا عِيَالَكِ" {
		t.Errorf("Entries[0].ArabicText = %q", page.Entries[0].ArabicText)
	}
	if page.Entries[1].ArabicText != "١٠٦٥) اذْهَبِي ، فَقَدْ بَايَعْتُكِ" {
		t.Errorf("Entries[1].ArabicText = %q", page.Entries[1].ArabicText)
	}

	// Footnotes
	if len(page.Footnotes) != 1 {
		t.Fatalf("len(Footnotes) = %d, want 1", len(page.Footnotes))
	}
	if page.Footnotes[0].ArabicText != "(م ، طب) قالها ﷺ للبدوية المشركة..." {
		t.Errorf("Footnotes[0].ArabicText = %q", page.Footnotes[0].ArabicText)
	}

	// Page footer
	if page.PageFooter != "١٦١" {
		t.Errorf("PageFooter = %q, want %q", page.PageFooter, "١٦١")
	}
}

func TestRegionPageToReadPage_ReadingOrderRespected(t *testing.T) {
	// Regions defined out of reading order
	rp := &RegionPage{
		Version:    "2.0",
		PageNumber: 1,
		Regions: []Region{
			{ID: "r3", Text: "third", Type: RegionTypeEntry},
			{ID: "r1", Text: "first", Type: RegionTypeEntry},
			{ID: "r2", Text: "second", Type: RegionTypeEntry},
		},
		ReadingOrder: []string{"r1", "r2", "r3"},
	}

	page := RegionPageToReadPage(rp)

	if len(page.Entries) != 3 {
		t.Fatalf("len(Entries) = %d, want 3", len(page.Entries))
	}
	want := []string{"first", "second", "third"}
	for i, e := range page.Entries {
		if e.ArabicText != want[i] {
			t.Errorf("Entries[%d].ArabicText = %q, want %q", i, e.ArabicText, want[i])
		}
	}
}

func TestRegionPageToReadPage_SeparatorsIgnored(t *testing.T) {
	rp := &RegionPage{
		Version:    "2.0",
		PageNumber: 1,
		Regions: []Region{
			{ID: "s1", Text: "---", Type: RegionTypeSeparator},
			{ID: "r1", Text: "entry", Type: RegionTypeEntry},
		},
		ReadingOrder: []string{"s1", "r1"},
	}

	page := RegionPageToReadPage(rp)

	if len(page.Entries) != 1 {
		t.Fatalf("len(Entries) = %d, want 1", len(page.Entries))
	}
	if page.Header != nil {
		t.Error("Header should be nil (no header region)")
	}
}

func TestRegionPageToReadPage_MultipleHeaders_FirstWins(t *testing.T) {
	rp := &RegionPage{
		Version:    "2.0",
		PageNumber: 1,
		Regions: []Region{
			{ID: "h1", Text: "First Header", Type: RegionTypeHeader},
			{ID: "h2", Text: "Second Header", Type: RegionTypeHeader},
		},
		ReadingOrder: []string{"h1", "h2"},
	}

	page := RegionPageToReadPage(rp)

	if page.Header == nil {
		t.Fatal("Header is nil")
	}
	if page.Header.Text != "First Header" {
		t.Errorf("Header.Text = %q, want %q", page.Header.Text, "First Header")
	}
}

func TestRegionPageToReadPage_UnorderedRegionsGoLast(t *testing.T) {
	rp := &RegionPage{
		Version:    "2.0",
		PageNumber: 1,
		Regions: []Region{
			{ID: "r1", Text: "ordered", Type: RegionTypeEntry},
			{ID: "extra", Text: "unordered", Type: RegionTypeEntry},
		},
		ReadingOrder: []string{"r1"}, // "extra" not in reading order
	}

	page := RegionPageToReadPage(rp)

	if len(page.Entries) != 2 {
		t.Fatalf("len(Entries) = %d, want 2", len(page.Entries))
	}
	if page.Entries[0].ArabicText != "ordered" {
		t.Errorf("Entries[0] = %q, want %q", page.Entries[0].ArabicText, "ordered")
	}
	if page.Entries[1].ArabicText != "unordered" {
		t.Errorf("Entries[1] = %q, want %q", page.Entries[1].ArabicText, "unordered")
	}
}

func TestGuessHeaderType(t *testing.T) {
	tests := []struct {
		name string
		r    Region
		want string
	}{
		{
			"large font is chapter_title",
			Region{Style: &RegionStyle{FontSize: 20}},
			"chapter_title",
		},
		{
			"exactly 18 is chapter_title",
			Region{Style: &RegionStyle{FontSize: 18}},
			"chapter_title",
		},
		{
			"small font is section_title",
			Region{Style: &RegionStyle{FontSize: 14}},
			"section_title",
		},
		{
			"no style is section_title",
			Region{},
			"section_title",
		},
		{
			"zero font_size is section_title",
			Region{Style: &RegionStyle{}},
			"section_title",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := guessHeaderType(tt.r)
			if got != tt.want {
				t.Errorf("guessHeaderType() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRegionPageToReadPage_RawTextPreserved(t *testing.T) {
	rp := &RegionPage{
		Version:    "2.0",
		PageNumber: 1,
		RawText:    "raw AI response here",
		Warnings:   []string{"parse warning"},
	}

	page := RegionPageToReadPage(rp)

	if page.RawText != "raw AI response here" {
		t.Errorf("RawText = %q, want %q", page.RawText, "raw AI response here")
	}
	if len(page.ReadWarnings) != 1 || page.ReadWarnings[0] != "parse warning" {
		t.Errorf("ReadWarnings = %v", page.ReadWarnings)
	}
}
