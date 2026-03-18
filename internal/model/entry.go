package model

// Entry represents a single entry (hadith, athar, commentary, etc.) on a page.
type Entry struct {
	Number              *int   `json:"number"`
	Type                string `json:"type"`
	ArabicText          string `json:"arabic_text"`
	IsContinuation      bool   `json:"is_continuation"`
	ContinuesOnNextPage bool   `json:"continues_on_next_page"`
}

// Footnote represents a footnote entry on a page.
type Footnote struct {
	EntryNumber          *int                  `json:"entry_number,omitempty"`
	EntryNumbers         []int                 `json:"entry_numbers,omitempty"`
	ArabicText           string                `json:"arabic_text"`
	SourceCodes          []string              `json:"source_codes"`
	AdditionalReferences []AdditionalReference `json:"additional_references,omitempty"`
}

// AdditionalReference is a cross-reference within a footnote.
type AdditionalReference struct {
	EntryNumber int    `json:"entry_number"`
	Text        string `json:"text"`
}

// Header represents a page header.
type Header struct {
	Text string `json:"text"`
	Type string `json:"type"` // section_title, chapter_title, none
}

// SourceResolved represents a resolved source abbreviation.
type SourceResolved struct {
	Code   string `json:"code"`
	NameAr string `json:"name_ar"`
	NameTr string `json:"name_tr"`
	Number string `json:"number,omitempty"`
	Layer  string `json:"layer"` // embedded, workspace, staged
}

// ContinuationInfo represents cross-page continuation linkage.
type ContinuationInfo struct {
	ContinuesFrom *int `json:"continues_from,omitempty"`
	ContinuesOn   *int `json:"continues_on,omitempty"`
}

// Validation represents the validation status of a page.
type Validation struct {
	Status                    string   `json:"status"`
	Warnings                  []string `json:"warnings"`
	HadithNumberSequenceValid bool     `json:"hadith_number_sequence_valid"`
}

// TranslationContext holds context injected during enrichment.
type TranslationContext struct {
	RelevantGlossaryTerms []string `json:"relevant_glossary_terms"`
	PreviousPageSummary   string   `json:"previous_page_summary"`
}

// TranslatedEntry represents a translated entry.
type TranslatedEntry struct {
	Number          int    `json:"number"`
	TurkishText     string `json:"turkish_text"`
	TranslatorNotes string `json:"translator_notes"`
}

// TranslatedFootnote represents a translated footnote.
type TranslatedFootnote struct {
	EntryNumber     int      `json:"entry_number"`
	TurkishText     string   `json:"turkish_text"`
	SourcesExpanded []string `json:"sources_expanded"`
}

// TranslatedHeader represents a translated page header.
type TranslatedHeader struct {
	Text string `json:"text"`
}
