package model

// RegionPage is the layout-aware read phase output for a single page.
// It replaces the content-centric ReadPage with a layout-centric schema
// where each piece of text is a Region with bounding box coordinates.
type RegionPage struct {
	Version       string   `json:"version"`
	PageNumber    int      `json:"page_number"`
	PageSize      PageSize `json:"page_size"`
	ReadModel     string   `json:"read_model"`
	LayoutTool    string   `json:"layout_tool"`
	ReadTimestamp string   `json:"read_timestamp"`
	Regions       []Region `json:"regions"`
	ReadingOrder  []string `json:"reading_order"`
	RawText       string   `json:"raw_text"`
	Warnings      []string `json:"warnings"`
}

// PageSize holds the pixel dimensions of a page image.
type PageSize struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// Region represents a detected text region on a page with its spatial
// coordinates, content, semantic type, and styling information.
type Region struct {
	ID           string       `json:"id"`
	BBox         BBox         `json:"bbox"`
	Text         string       `json:"text"`
	Type         string       `json:"type"`
	Style        *RegionStyle `json:"style,omitempty"`
	LayoutSource string       `json:"layout_source,omitempty"`
	TextSource   string       `json:"text_source,omitempty"`
	Column       *int         `json:"column,omitempty"`
	Confidence   float64      `json:"confidence,omitempty"` // 0.0-1.0, from layout tool. 0 if not available.
	RawClass     string       `json:"raw_class,omitempty"`  // original class name from layout tool before type mapping
	Zone         string       `json:"zone,omitempty"`       // zone classification from post-processing (header, entry, footnote, etc.)
}

// BBox represents a bounding box as [x, y, width, height] in pixels.
type BBox [4]int

// RegionStyle holds visual styling information for a text region.
type RegionStyle struct {
	FontSize  int    `json:"font_size,omitempty"`
	Bold      bool   `json:"bold,omitempty"`
	Direction string `json:"direction,omitempty"`
	Alignment string `json:"alignment,omitempty"`
}

// Region type constants.
const (
	RegionTypeHeader       = "header"
	RegionTypeEntry        = "entry"
	RegionTypeFootnote     = "footnote"
	RegionTypeSeparator    = "separator"
	RegionTypePageNumber   = "page_number"
	RegionTypeColumnHeader = "column_header"
	RegionTypeTable        = "table"
	RegionTypeImage        = "image"
	RegionTypeMarginNote   = "margin_note"
	RegionTypeOther        = "other"
)

// Layout source constants.
const (
	LayoutSourceSurya     = "surya"
	LayoutSourceDocLayout = "doclayout-yolo"
	LayoutSourceAI        = "ai"
)

// LayoutPage is the output of the layout detection phase for a single page.
// It contains only spatial information (bbox, type, confidence) — no text or style.
type LayoutPage struct {
	Version        string             `json:"version"`
	PageNumber     int                `json:"page_number"`
	PageSize       PageSize           `json:"page_size"`
	Tool           string             `json:"tool"`
	ToolParams     map[string]any     `json:"tool_params,omitempty"`
	Regions        []LayoutRegion     `json:"regions"`
	ReadingOrder   []string           `json:"reading_order,omitempty"`
	SeparatorY     *int               `json:"separator_y,omitempty"`
	PostProcessing *LayoutPostProcess `json:"post_processing,omitempty"`
}

// LayoutPostProcess holds diagnostic info about the post-processing step.
type LayoutPostProcess struct {
	SeparatorFound     bool   `json:"separator_found"`
	SeparatorMethod    string `json:"separator_method,omitempty"`
	ColumnsDetected    int    `json:"columns_detected"`
	RegionsBeforeSplit int    `json:"regions_before_split"`
	RegionsAfterSplit  int    `json:"regions_after_split"`
}

// LayoutRegion is a region detected by the layout tool without text content.
type LayoutRegion struct {
	ID         string  `json:"id"`
	BBox       BBox    `json:"bbox"`
	Type       string  `json:"type"`
	RawClass   string  `json:"raw_class,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
	Zone       string  `json:"zone,omitempty"`
}

// SolvedRegionPage extends RegionPage with solver metadata.
// The solver does NOT modify region text, bbox, or type — it only
// adds metadata that helps the translate phase.
type SolvedRegionPage struct {
	RegionPage
	GlossaryContext     []string `json:"glossary_context,omitempty"`
	PreviousPageSummary string   `json:"previous_page_summary,omitempty"`
	ValidationWarnings  []string `json:"validation_warnings,omitempty"`
}

// TranslatedRegion holds both original and translated text for a region.
type TranslatedRegion struct {
	ID             string       `json:"id"`
	BBox           BBox         `json:"bbox"`
	OriginalText   string       `json:"original_text"`
	TranslatedText string       `json:"translated_text"`
	Type           string       `json:"type"`
	Style          *RegionStyle `json:"style,omitempty"`
}

// TranslatedRegionPage is the output of the translate phase.
type TranslatedRegionPage struct {
	Version            string             `json:"version"`
	PageNumber         int                `json:"page_number"`
	SourceLang         string             `json:"source_lang"`
	TargetLang         string             `json:"target_lang"`
	TranslateModel     string             `json:"translate_model"`
	TranslateTimestamp string             `json:"translate_timestamp"`
	Regions            []TranslatedRegion `json:"regions"`
	ReadingOrder       []string           `json:"reading_order"`
	Warnings           []string           `json:"warnings,omitempty"`
}
