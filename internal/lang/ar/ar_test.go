package ar

import (
	"strings"
	"testing"

	"github.com/mmdemirbas/mutercim/internal/lang"
	"github.com/mmdemirbas/mutercim/internal/model"
)

func TestProfile_Code(t *testing.T) {
	if got := (Profile{}).Code(); got != "ar" {
		t.Errorf("Code() = %q, want \"ar\"", got)
	}
}

func TestProfile_RenderingHints_DeclaresRTL(t *testing.T) {
	hints := (Profile{}).RenderingHints()
	if hints["dir"] != "rtl" {
		t.Errorf("rendering hints should include dir=rtl, got %v", hints)
	}
}

func TestProfile_OcrOverride_IsQari(t *testing.T) {
	if got := (Profile{}).OcrOverride(); got != "qari" {
		t.Errorf("OcrOverride should be qari, got %q", got)
	}
}

func TestProfile_ScriptDetect(t *testing.T) {
	tests := []struct {
		name string
		text string
		want bool
	}{
		// Real content lifted from example/output/read/Anfas1/200.json — the
		// Phase 0 spike fixture. Validates the detector on actual user data.
		{"arabic-with-tashkeel", "أَصبَحتُ بِصُحْبَةٍ أَبَداً", true},
		{"arabic-with-numerals", "(ت : صح ١٤٦٨)", true},
		{"arabic-mixed-with-latin-citation", "النبي ﷺ (Bukhari 6639)", true},
		{"pure-latin", "Bukhari 6639 sahih", false},
		{"empty", "", false},
		// Arabic-Indic numerals ARE script — page 200's entry-number
		// regions are digits-only and must qualify for the RTL fixup.
		{"arabic-indic-digits-only", "١٤٦٨", true},
		{"mixed-latin-with-arabic-indic-digits", "page 12 ١٤٦٨", true},
		{"latin-digits-only", "123 456 789", false},
		{"mostly-latin-with-one-arabic-letter", "Lorem ipsum dolor sit م", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := (Profile{}).ScriptDetect(tt.text); got != tt.want {
				t.Errorf("ScriptDetect(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}

// TestReadingOrderFixup_TwoColumnFlipsRightToLeft is the load-bearing
// test for Phase 5. Constructed from the actual visual shape of
// example/Anfas1.pdf page 200: top half is a 2-column entry table with
// entry numbers on the right side and text on the left. The current
// pipeline emits them left-to-right; Arabic readers expect
// right-to-left. The fixup must flip the order within each row.
func TestReadingOrderFixup_TwoColumnFlipsRightToLeft(t *testing.T) {
	// Page coords: x grows right, y grows down. Two columns: left
	// column at x≈100, right column at x≈1500. Three rows.
	regions := []model.Region{
		{ID: "L1", BBox: model.BBox{100, 200, 800, 50}, Text: "أَصْبَحْتُ بَعْضًا", Type: model.RegionTypeEntry},
		{ID: "R1", BBox: model.BBox{1500, 200, 800, 50}, Text: "١٥٣٠", Type: model.RegionTypeEntry},
		{ID: "L2", BBox: model.BBox{100, 300, 800, 50}, Text: "أَصْبَحْتُ يَا سَعْدُ", Type: model.RegionTypeEntry},
		{ID: "R2", BBox: model.BBox{1500, 300, 800, 50}, Text: "١٥٣١", Type: model.RegionTypeEntry},
		{ID: "L3", BBox: model.BBox{100, 400, 800, 50}, Text: "أَصْبَحْتُ يَا مُعَاوِيَة", Type: model.RegionTypeEntry},
		{ID: "R3", BBox: model.BBox{1500, 400, 800, 50}, Text: "١٥٣٢", Type: model.RegionTypeEntry},
	}
	// Upstream emits left-to-right within each row → "L1 R1 L2 R2 L3 R3".
	currentOrder := []string{"L1", "R1", "L2", "R2", "L3", "R3"}
	got := (Profile{}).ReadingOrderFixup(regions, currentOrder)
	// Fixup must emit right-to-left within each row → "R1 L1 R2 L2 R3 L3".
	want := []string{"R1", "L1", "R2", "L2", "R3", "L3"}
	if !equalSlices(got, want) {
		t.Errorf("RTL fixup did not reorder rows correctly\n got: %v\nwant: %v", got, want)
	}
}

// TestReadingOrderFixup_SingleColumnUnchanged verifies the fixup does
// not disturb single-column input. The footnote section of an Anfas
// page is single-column; reordering would corrupt it.
func TestReadingOrderFixup_SingleColumnUnchanged(t *testing.T) {
	regions := []model.Region{
		{ID: "F1", BBox: model.BBox{100, 1000, 1500, 80}, Text: "‏(ت : صح ١٤٦٨) أَصبَحتُ بِصُحْبَةٍ", Type: model.RegionTypeFootnote},
		{ID: "F2", BBox: model.BBox{100, 1100, 1500, 80}, Text: "‏(ت : صح ١٤٦٩) أَصْبَحتُ حُكمَ اللهِ", Type: model.RegionTypeFootnote},
		{ID: "F3", BBox: model.BBox{100, 1200, 1500, 80}, Text: "‏(ت : صح ١٤٧٠) أَصْبَحتُ وَأَحسَنتُ", Type: model.RegionTypeFootnote},
	}
	currentOrder := []string{"F1", "F2", "F3"}
	got := (Profile{}).ReadingOrderFixup(regions, currentOrder)
	if !equalSlices(got, currentOrder) {
		t.Errorf("single-column order must not be reordered\n got: %v\nwant: %v", got, currentOrder)
	}
}

// TestReadingOrderFixup_PreservesNonArabicRegionsInPlace verifies that
// header / page-number / separator regions keep their relative
// positions across an Arabic body section.
func TestReadingOrderFixup_PreservesNonArabicRegionsInPlace(t *testing.T) {
	regions := []model.Region{
		{ID: "H", BBox: model.BBox{500, 50, 1000, 60}, Text: "Chapter 12", Type: model.RegionTypeHeader},
		{ID: "L1", BBox: model.BBox{100, 200, 800, 50}, Text: "أَصْبَحْتُ", Type: model.RegionTypeEntry},
		{ID: "R1", BBox: model.BBox{1500, 200, 800, 50}, Text: "١٥٣٠", Type: model.RegionTypeEntry},
		{ID: "SEP", BBox: model.BBox{0, 280, 2400, 5}, Text: "", Type: model.RegionTypeSeparator},
		{ID: "L2", BBox: model.BBox{100, 300, 800, 50}, Text: "أَصْبَحْتُ", Type: model.RegionTypeEntry},
		{ID: "R2", BBox: model.BBox{1500, 300, 800, 50}, Text: "١٥٣١", Type: model.RegionTypeEntry},
		{ID: "P", BBox: model.BBox{1100, 2900, 200, 40}, Text: "205", Type: model.RegionTypePageNumber},
	}
	currentOrder := []string{"H", "L1", "R1", "SEP", "L2", "R2", "P"}
	got := (Profile{}).ReadingOrderFixup(regions, currentOrder)
	// Header / separator / page-number stay in place; only the Arabic
	// entries flip RTL within their row.
	want := []string{"H", "R1", "L1", "SEP", "R2", "L2", "P"}
	if !equalSlices(got, want) {
		t.Errorf("non-Arabic regions should anchor positions\n got: %v\nwant: %v", got, want)
	}
}

func TestReadingOrderFixup_EmptyAndShortInputs(t *testing.T) {
	tests := []struct {
		name    string
		regions []model.Region
		order   []string
	}{
		{"nil-regions", nil, nil},
		{"empty-order", []model.Region{{ID: "x", BBox: model.BBox{0, 0, 10, 10}, Text: "أ"}}, nil},
		{"single-region", []model.Region{{ID: "x", BBox: model.BBox{0, 0, 10, 10}, Text: "أ", Type: model.RegionTypeEntry}}, []string{"x"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := (Profile{}).ReadingOrderFixup(tt.regions, tt.order)
			if !equalSlices(got, tt.order) {
				t.Errorf("trivial input should pass through\n got: %v\nwant: %v", got, tt.order)
			}
		})
	}
}

func TestRegister_InstallsProfile(t *testing.T) {
	Register()
	p := lang.Get("ar")
	if p.Code() != "ar" {
		t.Errorf("after Register, lang.Get(\"ar\") should return ar profile, got code %q", p.Code())
	}
	if p.OcrOverride() != "qari" {
		t.Errorf("Get(\"ar\").OcrOverride() = %q, want \"qari\"", p.OcrOverride())
	}
}

func TestRegister_IsIdempotent(t *testing.T) {
	// Multiple Register calls must not panic.
	for range 3 {
		Register()
	}
	p := lang.Get("ar")
	if p.Code() != "ar" {
		t.Errorf("repeat Register: lang.Get(\"ar\").Code() = %q, want \"ar\"", p.Code())
	}
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ensure strings is used in non-test source for the linter
var _ = strings.TrimSpace
