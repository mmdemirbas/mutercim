package model

import (
	"encoding/json"
	"testing"
)

func TestOCRPage_JSONRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		page OCRPage
	}{
		{
			name: "full page with regions",
			page: OCRPage{
				Version:    "1.0",
				PageNumber: 5,
				Tool:       "qari",
				Model:      "qari-v1",
				ElapsedMs:  1234,
				Regions: []OCRRegion{
					{ID: "r1", Text: "bismillah", ElapsedMs: 100},
					{ID: "r2", Text: "alhamdulillah", ElapsedMs: 200},
				},
			},
		},
		{
			name: "full text only (no regions)",
			page: OCRPage{
				Version:    "1.0",
				PageNumber: 1,
				Tool:       "qari",
				Model:      "qari-v1",
				ElapsedMs:  500,
				FullText:   "full page text content here",
			},
		},
		{
			name: "empty regions and empty full text",
			page: OCRPage{
				Version:    "1.0",
				PageNumber: 3,
				Tool:       "qari",
				Model:      "qari-v1",
				ElapsedMs:  0,
			},
		},
		{
			name: "single region",
			page: OCRPage{
				Version:    "2.0",
				PageNumber: 10,
				Tool:       "qari",
				Model:      "qari-v2",
				ElapsedMs:  42,
				Regions:    []OCRRegion{{ID: "r1", Text: "solo region", ElapsedMs: 42}},
			},
		},
		{
			name: "region with empty text",
			page: OCRPage{
				Version:    "1.0",
				PageNumber: 7,
				Tool:       "qari",
				Model:      "qari-v1",
				ElapsedMs:  10,
				Regions:    []OCRRegion{{ID: "r1", Text: "", ElapsedMs: 0}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.page)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}

			var got OCRPage
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}

			if got.Version != tt.page.Version {
				t.Errorf("Version = %q, want %q", got.Version, tt.page.Version)
			}
			if got.PageNumber != tt.page.PageNumber {
				t.Errorf("PageNumber = %d, want %d", got.PageNumber, tt.page.PageNumber)
			}
			if got.Tool != tt.page.Tool {
				t.Errorf("Tool = %q, want %q", got.Tool, tt.page.Tool)
			}
			if got.Model != tt.page.Model {
				t.Errorf("Model = %q, want %q", got.Model, tt.page.Model)
			}
			if got.ElapsedMs != tt.page.ElapsedMs {
				t.Errorf("ElapsedMs = %d, want %d", got.ElapsedMs, tt.page.ElapsedMs)
			}
			if got.FullText != tt.page.FullText {
				t.Errorf("FullText = %q, want %q", got.FullText, tt.page.FullText)
			}
			if len(got.Regions) != len(tt.page.Regions) {
				t.Fatalf("Regions len = %d, want %d", len(got.Regions), len(tt.page.Regions))
			}
			for i, r := range got.Regions {
				want := tt.page.Regions[i]
				if r.ID != want.ID {
					t.Errorf("Regions[%d].ID = %q, want %q", i, r.ID, want.ID)
				}
				if r.Text != want.Text {
					t.Errorf("Regions[%d].Text = %q, want %q", i, r.Text, want.Text)
				}
				if r.ElapsedMs != want.ElapsedMs {
					t.Errorf("Regions[%d].ElapsedMs = %d, want %d", i, r.ElapsedMs, want.ElapsedMs)
				}
			}
		})
	}
}

func TestOCRRegion_JSONRoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		region OCRRegion
	}{
		{
			name:   "normal region",
			region: OCRRegion{ID: "r1", Text: "sample text", ElapsedMs: 150},
		},
		{
			name:   "empty text",
			region: OCRRegion{ID: "r2", Text: "", ElapsedMs: 0},
		},
		{
			name:   "unicode text",
			region: OCRRegion{ID: "r3", Text: "\u0628\u0633\u0645 \u0627\u0644\u0644\u0647 \u0627\u0644\u0631\u062d\u0645\u0646 \u0627\u0644\u0631\u062d\u064a\u0645", ElapsedMs: 300},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.region)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}

			var got OCRRegion
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}

			if got.ID != tt.region.ID {
				t.Errorf("ID = %q, want %q", got.ID, tt.region.ID)
			}
			if got.Text != tt.region.Text {
				t.Errorf("Text = %q, want %q", got.Text, tt.region.Text)
			}
			if got.ElapsedMs != tt.region.ElapsedMs {
				t.Errorf("ElapsedMs = %d, want %d", got.ElapsedMs, tt.region.ElapsedMs)
			}
		})
	}
}

func TestOCRReport_JSONRoundTrip(t *testing.T) {
	report := OCRReport{
		Tool:            "qari",
		Model:           "qari-v1",
		PagesProcessed:  50,
		PagesFailed:     2,
		AvgMs:           800,
		TotalCharacters: 125000,
	}

	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var got OCRReport
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if got != report {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, report)
	}
}

func TestOCRPage_JSONOmitsEmptyFields(t *testing.T) {
	page := OCRPage{
		Version:    "1.0",
		PageNumber: 1,
		Tool:       "qari",
		Model:      "qari-v1",
		ElapsedMs:  100,
		// Regions and FullText intentionally empty
	}

	data, err := json.Marshal(page)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal raw map error = %v", err)
	}

	if _, ok := raw["regions"]; ok {
		t.Error("expected regions to be omitted when empty")
	}
	if _, ok := raw["full_text"]; ok {
		t.Error("expected full_text to be omitted when empty")
	}
}
