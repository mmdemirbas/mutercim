package model

// OCRPage is the output of the OCR phase for a single page.
// It stores extracted text per region (if layout data was available)
// or full-page text (if no layout data).
type OCRPage struct {
	Version    string      `json:"version"`
	PageNumber int         `json:"page_number"`
	Tool       string      `json:"tool"`
	Model      string      `json:"model"`
	ElapsedMs  int         `json:"elapsed_ms"`
	Regions    []OCRRegion `json:"regions,omitempty"`
	FullText   string      `json:"full_text,omitempty"`
}

// OCRRegion stores OCR text for a single layout region.
type OCRRegion struct {
	ID        string `json:"id"`
	Text      string `json:"text"`
	ElapsedMs int    `json:"elapsed_ms"`
}

// OCRReport stores summary statistics for the OCR phase.
type OCRReport struct {
	Tool            string `json:"tool"`
	Model           string `json:"model"`
	Quantize        string `json:"quantize"`
	PagesProcessed  int    `json:"pages_processed"`
	PagesFailed     int    `json:"pages_failed"`
	AvgMs           int    `json:"avg_ms"`
	TotalCharacters int    `json:"total_characters"`
}

// OCR source constants.
const (
	OCRSourceQari = "qari"
)
