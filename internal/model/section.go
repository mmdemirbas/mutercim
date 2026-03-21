package model

import (
	"fmt"
	"strconv"
	"strings"
)

// PageRange represents an inclusive range of page numbers.
type PageRange struct {
	First int
	Last  int
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

// MaxExpandedPages is the maximum number of pages that ExpandPages will produce.
// This prevents unbounded memory allocation from huge ranges like "1-10000000".
const MaxExpandedPages = 100000

// ExpandPages expands page ranges into individual page numbers.
// Returns an error if the total exceeds MaxExpandedPages.
func ExpandPages(ranges []PageRange) ([]int, error) {
	if ranges == nil {
		return nil, nil
	}
	// Pre-check total count to avoid unbounded allocation
	total := 0
	for _, r := range ranges {
		total += r.Last - r.First + 1
		if total > MaxExpandedPages {
			return nil, fmt.Errorf("page range expands to %d+ pages (max %d)", total, MaxExpandedPages)
		}
	}
	pages := make([]int, 0, total)
	for _, r := range ranges {
		for p := r.First; p <= r.Last; p++ {
			pages = append(pages, p)
		}
	}
	return pages, nil
}
