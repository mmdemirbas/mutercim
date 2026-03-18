package config

import (
	"fmt"

	"github.com/muhammed/mutercim/internal/model"
)

// SectionLookup provides efficient page-to-section mapping.
type SectionLookup struct {
	sections []sectionEntry
}

type sectionEntry struct {
	section model.Section
	ranges  []model.PageRange
}

// NewSectionLookup builds a lookup from a slice of sections.
func NewSectionLookup(sections []model.Section) (*SectionLookup, error) {
	entries := make([]sectionEntry, 0, len(sections))
	for _, s := range sections {
		ranges, err := model.ParsePageRanges(s.Pages)
		if err != nil {
			return nil, fmt.Errorf("section %q: %w", s.Name, err)
		}
		entries = append(entries, sectionEntry{section: s, ranges: ranges})
	}
	return &SectionLookup{sections: entries}, nil
}

// ForPage returns the section that contains the given page number.
// If no section matches, returns a default "auto" section.
func (sl *SectionLookup) ForPage(page int) model.Section {
	for _, e := range sl.sections {
		if model.PageInRanges(page, e.ranges) {
			return e.section
		}
	}
	return model.Section{
		Name:      "auto",
		Type:      model.SectionAuto,
		Translate: true,
	}
}

// AllPages returns a sorted, deduplicated list of all page numbers covered by sections.
func (sl *SectionLookup) AllPages() []int {
	seen := make(map[int]bool)
	var pages []int
	for _, e := range sl.sections {
		for _, r := range e.ranges {
			for p := r.First; p <= r.Last; p++ {
				if !seen[p] {
					seen[p] = true
					pages = append(pages, p)
				}
			}
		}
	}
	return pages
}

// ShouldSkip returns true if the given page is in a "skip" section.
func (sl *SectionLookup) ShouldSkip(page int) bool {
	s := sl.ForPage(page)
	return s.Type == model.SectionSkip
}

// ShouldTranslate returns true if the given page should be translated.
func (sl *SectionLookup) ShouldTranslate(page int) bool {
	s := sl.ForPage(page)
	return s.Translate
}
