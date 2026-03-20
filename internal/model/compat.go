package model

import "sort"

// RegionPageToReadPage converts a layout-aware RegionPage into the legacy
// ReadPage format expected by solve and translate phases.
// Regions are processed in reading_order sequence. Region types are mapped:
//   - header   → ReadPage.Header
//   - entry    → ReadPage.Entries
//   - footnote → ReadPage.Footnotes
//   - page_number → ReadPage.PageFooter
//   - separator, other, etc. → ignored
func RegionPageToReadPage(rp *RegionPage) *ReadPage {
	if rp == nil {
		return nil
	}

	page := &ReadPage{
		Version:       "1.0",
		PageNumber:    rp.PageNumber,
		ReadModel:     rp.ReadModel,
		ReadTimestamp: rp.ReadTimestamp,
		RawText:       rp.RawText,
		ReadWarnings:  rp.Warnings,
	}

	// Build an order map for efficient lookup
	orderMap := make(map[string]int, len(rp.ReadingOrder))
	for i, id := range rp.ReadingOrder {
		orderMap[id] = i
	}

	// Sort regions by reading order
	ordered := make([]Region, len(rp.Regions))
	copy(ordered, rp.Regions)
	sort.Slice(ordered, func(i, j int) bool {
		oi, okI := orderMap[ordered[i].ID]
		oj, okJ := orderMap[ordered[j].ID]
		if !okI {
			oi = len(rp.ReadingOrder) // unordered regions go last
		}
		if !okJ {
			oj = len(rp.ReadingOrder)
		}
		return oi < oj
	})

	for _, r := range ordered {
		switch r.Type {
		case RegionTypeHeader:
			if page.Header == nil {
				page.Header = &Header{
					Text: r.Text,
					Type: guessHeaderType(r),
				}
			}
		case RegionTypeEntry:
			page.Entries = append(page.Entries, regionToEntry(r))
		case RegionTypeFootnote:
			page.Footnotes = append(page.Footnotes, regionToFootnote(r))
		case RegionTypePageNumber:
			if page.PageFooter == "" {
				page.PageFooter = r.Text
			}
		}
	}

	return page
}

// guessHeaderType infers a header type from the region.
// Regions don't carry explicit header type info, so we default to
// section_title which is the most common case.
func guessHeaderType(r Region) string {
	if r.Style != nil && r.Style.FontSize >= 18 {
		return "chapter_title"
	}
	return "section_title"
}

// regionToEntry converts a Region with type "entry" into a legacy Entry.
// Since regions carry raw text, entry number and continuation info are
// not available and left at zero/false defaults. The AI enrichment step
// in the read phase should populate these before conversion, or the
// solve phase handles them.
func regionToEntry(r Region) Entry {
	return Entry{
		Type:       "other",
		ArabicText: r.Text,
	}
}

// regionToFootnote converts a Region with type "footnote" into a legacy Footnote.
func regionToFootnote(r Region) Footnote {
	return Footnote{
		ArabicText: r.Text,
	}
}
