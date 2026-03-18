package model

// ExtractedPage is the Phase 1 output for a single page.
type ExtractedPage struct {
	Version             string     `json:"version"`
	PageNumber          int        `json:"page_number"`
	SectionType         string     `json:"section_type"`
	ExtractionModel     string     `json:"extraction_model"`
	ExtractionTimestamp string     `json:"extraction_timestamp"`
	Header              *Header    `json:"header"`
	Entries             []Entry    `json:"entries"`
	Footnotes           []Footnote `json:"footnotes"`
	PageFooter          string     `json:"page_footer"`
	RawText             string     `json:"raw_text"`
	ExtractionWarnings  []string   `json:"extraction_warnings"`
}

// EnrichedPage extends ExtractedPage with Phase 2 enrichment data.
type EnrichedPage struct {
	ExtractedPage

	SourcesResolved    []SourceResolved    `json:"sources_resolved"`
	UnresolvedSources  []string            `json:"unresolved_sources"`
	ContinuationInfo   *ContinuationInfo   `json:"continuation_info"`
	Validation         *Validation         `json:"validation"`
	TranslationContext *TranslationContext `json:"translation_context"`
}

// TranslatedPage extends EnrichedPage with Phase 3 translation data.
type TranslatedPage struct {
	EnrichedPage

	TranslationModel     string               `json:"translation_model"`
	TranslationTimestamp string               `json:"translation_timestamp"`
	TranslatedHeader     *TranslatedHeader    `json:"translated_header"`
	TranslatedEntries    []TranslatedEntry    `json:"translated_entries"`
	TranslatedFootnotes  []TranslatedFootnote `json:"translated_footnotes"`
	TranslationWarnings  []string             `json:"translation_warnings"`
}
