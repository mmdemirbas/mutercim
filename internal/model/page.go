package model

// ReadPage is the Phase 1 output for a single page.
type ReadPage struct {
	Version       string     `json:"version"`
	PageNumber    int        `json:"page_number"`
	ReadModel     string     `json:"read_model"`
	ReadTimestamp string     `json:"read_timestamp"`
	Header        *Header    `json:"header"`
	Entries       []Entry    `json:"entries"`
	Footnotes     []Footnote `json:"footnotes"`
	PageFooter    string     `json:"page_footer"`
	RawText       string     `json:"raw_text"`
	ReadWarnings  []string   `json:"read_warnings"`
}

// SolvedPage extends ReadPage with Phase 2 solver data.
type SolvedPage struct {
	ReadPage

	SourcesResolved    []SourceResolved    `json:"sources_resolved"`
	UnresolvedSources  []string            `json:"unresolved_sources"`
	ContinuationInfo   *ContinuationInfo   `json:"continuation_info"`
	Validation         *Validation         `json:"validation"`
	TranslationContext *TranslationContext `json:"translation_context"`
}

// TranslatedPage extends SolvedPage with Phase 3 translation data.
type TranslatedPage struct {
	SolvedPage

	TranslationModel     string               `json:"translation_model"`
	TranslationTimestamp string               `json:"translation_timestamp"`
	TranslatedHeader     *TranslatedHeader    `json:"translated_header"`
	TranslatedEntries    []TranslatedEntry    `json:"translated_entries"`
	TranslatedFootnotes  []TranslatedFootnote `json:"translated_footnotes"`
	TranslationWarnings  []string             `json:"translation_warnings"`
}
