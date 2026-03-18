package model

import (
	"fmt"
	"strconv"
	"strings"
)

// SectionType defines the structural type of a book section.
type SectionType string

const (
	SectionSkip             SectionType = "skip"
	SectionProse            SectionType = "prose"
	SectionScholarlyEntries SectionType = "scholarly_entries"
	SectionReferenceTable   SectionType = "reference_table"
	SectionTOC              SectionType = "toc"
	SectionIndex            SectionType = "index"
	SectionAuto             SectionType = "auto"
)

// ValidSectionTypes lists all recognized section types.
var ValidSectionTypes = []SectionType{
	SectionSkip, SectionProse, SectionScholarlyEntries,
	SectionReferenceTable, SectionTOC, SectionIndex, SectionAuto,
}

// IsValid returns true if the section type is recognized.
func (s SectionType) IsValid() bool {
	_, ok := validSectionTypes[s]
	return ok
}

// validSectionTypes is a set for O(1) lookup.
var validSectionTypes = map[SectionType]bool{
	SectionSkip:             true,
	SectionProse:            true,
	SectionScholarlyEntries: true,
	SectionReferenceTable:   true,
	SectionTOC:              true,
	SectionIndex:            true,
	SectionAuto:             true,
}

// Section represents a named section of the book with a page range and type.
type Section struct {
	Name      string      `yaml:"name" json:"name"`
	Pages     string      `yaml:"pages" json:"pages"`
	Type      SectionType `yaml:"type" json:"type"`
	Translate bool        `yaml:"translate" json:"translate"`
}

// PageRange represents an inclusive range of page numbers.
type PageRange struct {
	First int
	Last  int
}

// Contains returns true if the given page number falls within this range.
func (pr PageRange) Contains(page int) bool {
	return page >= pr.First && page <= pr.Last
}

// ParsePageRanges parses a page range string like "1-50", "1,5,10-20", or "all".
// Returns a slice of PageRange. For "all", returns nil (meaning all pages).
func ParsePageRanges(s string) ([]PageRange, error) {
	s = strings.TrimSpace(s)
	if s == "" || strings.EqualFold(s, "all") {
		return nil, nil
	}

	var ranges []PageRange
	parts := strings.Split(s, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		if idx := strings.Index(part, "-"); idx >= 0 {
			firstStr := strings.TrimSpace(part[:idx])
			lastStr := strings.TrimSpace(part[idx+1:])

			first, err := strconv.Atoi(firstStr)
			if err != nil {
				return nil, fmt.Errorf("invalid page number %q: %w", firstStr, err)
			}
			last, err := strconv.Atoi(lastStr)
			if err != nil {
				return nil, fmt.Errorf("invalid page number %q: %w", lastStr, err)
			}
			if first > last {
				return nil, fmt.Errorf("invalid range: %d > %d", first, last)
			}
			if first < 1 {
				return nil, fmt.Errorf("page numbers must be >= 1, got %d", first)
			}
			ranges = append(ranges, PageRange{First: first, Last: last})
		} else {
			num, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("invalid page number %q: %w", part, err)
			}
			if num < 1 {
				return nil, fmt.Errorf("page numbers must be >= 1, got %d", num)
			}
			ranges = append(ranges, PageRange{First: num, Last: num})
		}
	}
	return ranges, nil
}

// ExpandPages expands page ranges into individual page numbers.
func ExpandPages(ranges []PageRange) []int {
	if ranges == nil {
		return nil
	}
	var pages []int
	for _, r := range ranges {
		for p := r.First; p <= r.Last; p++ {
			pages = append(pages, p)
		}
	}
	return pages
}

// PageInRanges returns true if the given page number is within any of the ranges.
// If ranges is nil (meaning "all"), returns true.
func PageInRanges(page int, ranges []PageRange) bool {
	if ranges == nil {
		return true
	}
	for _, r := range ranges {
		if r.Contains(page) {
			return true
		}
	}
	return false
}
