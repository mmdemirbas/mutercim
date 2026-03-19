package enrichment

import "github.com/mmdemirbas/mutercim/internal/model"

// DetectContinuation checks if the current page continues from the previous page
// or continues onto the next page. Returns nil if no continuation is detected.
func DetectContinuation(current, previous *model.ExtractedPage) *model.ContinuationInfo {
	if current == nil || len(current.Entries) == 0 {
		return nil
	}

	var info model.ContinuationInfo
	hasContinuation := false

	// Check if first entry is a continuation from previous page
	firstEntry := current.Entries[0]
	if firstEntry.IsContinuation && previous != nil {
		prevPage := previous.PageNumber
		info.ContinuesFrom = &prevPage
		hasContinuation = true
	}

	// Check if last entry continues on next page
	lastEntry := current.Entries[len(current.Entries)-1]
	if lastEntry.ContinuesOnNextPage {
		// The next page number will be set when that page is processed
		nextPage := current.PageNumber + 1
		info.ContinuesOn = &nextPage
		hasContinuation = true
	}

	if !hasContinuation {
		return nil
	}
	return &info
}
