package pipeline

import (
	"testing"

	"github.com/mmdemirbas/mutercim/internal/model"
)

func TestLayoutRegionsToModelRegions(t *testing.T) {
	tests := []struct {
		name   string
		input  []model.LayoutRegion
		wantN  int
	}{
		{"nil input", nil, 0},
		{"empty input", []model.LayoutRegion{}, 0},
		{
			"single region",
			[]model.LayoutRegion{
				{ID: "r1", BBox: model.BBox{10, 20, 100, 50}, Type: "entry", RawClass: "Text", Confidence: 0.95},
			},
			1,
		},
		{
			"multiple regions",
			[]model.LayoutRegion{
				{ID: "r1", BBox: model.BBox{0, 0, 100, 50}, Type: "header"},
				{ID: "r2", BBox: model.BBox{0, 50, 100, 200}, Type: "entry"},
				{ID: "sep1", BBox: model.BBox{0, 250, 100, 5}, Type: "separator"},
			},
			3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LayoutRegionsToModelRegions(tt.input)
			if len(got) != tt.wantN {
				t.Fatalf("len = %d, want %d", len(got), tt.wantN)
			}
			for i, r := range got {
				lr := tt.input[i]
				if r.ID != lr.ID {
					t.Errorf("region[%d].ID = %q, want %q", i, r.ID, lr.ID)
				}
				if r.BBox != lr.BBox {
					t.Errorf("region[%d].BBox = %v, want %v", i, r.BBox, lr.BBox)
				}
				if r.Type != lr.Type {
					t.Errorf("region[%d].Type = %q, want %q", i, r.Type, lr.Type)
				}
				if r.RawClass != lr.RawClass {
					t.Errorf("region[%d].RawClass = %q, want %q", i, r.RawClass, lr.RawClass)
				}
				if r.Confidence != lr.Confidence {
					t.Errorf("region[%d].Confidence = %v, want %v", i, r.Confidence, lr.Confidence)
				}
			}
		})
	}
}
