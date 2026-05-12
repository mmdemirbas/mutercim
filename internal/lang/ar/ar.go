// Package ar provides the Arabic language profile.
//
// Implements the corrective behaviors Arabic Islamic books (and Arabic
// scholarly content in general) need that the language-agnostic core
// cannot derive from layout data alone:
//
//   - Right-to-left two-column reading order. Both the current pipeline
//     and every open VLM tested in Phase 0 emit columns left-to-right;
//     Arabic readers expect right-to-left. The geometry fixup here is
//     the load-bearing patch flagged in notes/2026-05-12-spikes.md.
//   - "rtl" rendering hint for downstream LaTeX/Typst/docx writers.
//   - "qari" OCR override (Qari-OCR v0.3, Arabic-specialized — currently
//     the only language-aware OCR override, but the surface generalizes).
package ar

import (
	"sort"
	"strings"
	"unicode"

	"github.com/mmdemirbas/mutercim/internal/lang"
	"github.com/mmdemirbas/mutercim/internal/model"
)

// Profile is the Arabic language profile. Implements lang.Profile.
type Profile struct{}

// Code returns "ar".
func (Profile) Code() string { return "ar" }

// ScriptDetect reports whether the input text is in Arabic script.
// Two signals count as Arabic:
//
//   - Arabic letters (Unicode.Arabic table) — ≥25% of all letters in
//     the text.
//   - Arabic-Indic digits (U+0660…U+0669, U+06F0…U+06F9) — even a
//     single one is decisive. Sufi/hadith pages routinely have a
//     digits-only "entry number" region whose script is unambiguously
//     Arabic, so the bare-letter heuristic misses them.
//
// Empty / pure-Latin text returns false.
func (Profile) ScriptDetect(text string) bool {
	var arabic, letters int
	for _, r := range text {
		if isArabicIndicDigit(r) {
			return true
		}
		if !unicode.IsLetter(r) {
			continue
		}
		letters++
		if unicode.In(r, unicode.Arabic) {
			arabic++
		}
	}
	if letters == 0 {
		return false
	}
	return arabic*4 >= letters // ≥ 25%
}

// isArabicIndicDigit reports whether r is in either of the Unicode
// Arabic-Indic digit ranges: Arabic-Indic (U+0660–U+0669) used in
// Arabic, or Extended Arabic-Indic (U+06F0–U+06F9) used in Persian
// and Urdu. Both forms appear in the user's Anfas corpus.
func isArabicIndicDigit(r rune) bool {
	return (r >= 0x0660 && r <= 0x0669) || (r >= 0x06F0 && r <= 0x06F9)
}

// OcrOverride returns "qari" — Qari-OCR v0.3 (NAMAA-Space) is the
// Arabic-specialized OCR validated for diacritic-heavy text in
// KITAB-Bench (WER 0.16 / CER 0.06 per its model card).
func (Profile) OcrOverride() string { return "qari" }

// RenderingHints declares dir=rtl so LaTeX/Typst/docx writers can flip
// paragraph direction without hardcoding the source language.
func (Profile) RenderingHints() map[string]string {
	return map[string]string{"dir": "rtl"}
}

// PromptCorpus is empty for now — the Adab corpus migration is a
// follow-up (P5-1 in PLAN.md tracks the three-layer prompt
// customization that subsumes this).
func (Profile) PromptCorpus() string { return "" }

// ReadingOrderFixup repairs left-to-right column ordering on Arabic
// pages. Pipeline:
//
//  1. Filter to regions whose content is Arabic-script (skip
//     separators, page numbers, embedded Latin captions, etc.).
//  2. Group those regions into rows by bbox y-overlap.
//  3. Within each row, sort RIGHT-TO-LEFT by bbox x-center. This is
//     the actual fix — the canonical reading-order failure flagged in
//     Phase 0 (both DocLayout-YOLO + AI vision and MinerU pipeline
//     emit body entries left-to-right).
//  4. Stitch the rows back into a single reading-order list, preserving
//     the relative position of non-Arabic regions from the input order.
//
// Pure function. Returns a fresh slice; does not mutate input.
//
// The fixup is bbox-only — it does NOT consult Region.Text beyond the
// script-detect filter, so it works whether the upstream Read/Parse
// phase produced clean text or not.
//
//nolint:cyclop,gocognit // single-purpose layout repair with clear stages
func (Profile) ReadingOrderFixup(regions []model.Region, currentOrder []string) []string {
	if len(regions) < 2 || len(currentOrder) < 2 {
		return currentOrder
	}

	// Index regions by ID for O(1) lookup while traversing currentOrder.
	byID := make(map[string]model.Region, len(regions))
	for _, r := range regions {
		byID[r.ID] = r
	}

	// Split the order into runs of Arabic-script regions and runs of
	// non-Arabic regions. Reorder the Arabic runs right-to-left within
	// rows; leave non-Arabic runs alone so headers and page numbers and
	// embedded Latin material keep their relative positions.
	out := make([]string, 0, len(currentOrder))
	var arRun []string
	flush := func() {
		if len(arRun) == 0 {
			return
		}
		out = append(out, reorderRTLRows(arRun, byID)...)
		arRun = arRun[:0]
	}
	prof := Profile{}
	for _, id := range currentOrder {
		r, ok := byID[id]
		if !ok {
			// Unknown ID: keep it in-place (could be a separator with no
			// region row). flush() first so we don't reorder across it.
			flush()
			out = append(out, id)
			continue
		}
		if !isArabicCandidate(r) || !prof.ScriptDetect(r.Text) {
			flush()
			out = append(out, id)
			continue
		}
		arRun = append(arRun, id)
	}
	flush()
	return out
}

// reorderRTLRows takes Arabic-script region IDs and returns them sorted
// into right-to-left rows: group by y-overlap, then within each row
// sort by descending x-center (rightmost first).
func reorderRTLRows(ids []string, byID map[string]model.Region) []string {
	type entry struct {
		id       string
		x1, y1   int
		x2, y2   int
		yCenter  int
		xCenter  int
	}
	entries := make([]entry, 0, len(ids))
	for _, id := range ids {
		r := byID[id]
		// bbox is [x, y, width, height] in this codebase.
		x1, y1, w, h := r.BBox[0], r.BBox[1], r.BBox[2], r.BBox[3]
		entries = append(entries, entry{
			id:      id,
			x1:      x1,
			y1:      y1,
			x2:      x1 + w,
			y2:      y1 + h,
			yCenter: y1 + h/2,
			xCenter: x1 + w/2,
		})
	}
	// Sort top→bottom by y-center as a stable base ordering.
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].yCenter < entries[j].yCenter
	})

	// Group into rows: two entries belong to the same row if either's
	// y-center falls inside the other's y range. This tolerates mild
	// vertical misalignment without merging clearly-separate rows.
	type row struct{ items []entry }
	var rows []row
	for _, e := range entries {
		placed := false
		for i := range rows {
			ref := rows[i].items[0]
			if e.yCenter >= ref.y1 && e.yCenter <= ref.y2 {
				rows[i].items = append(rows[i].items, e)
				placed = true
				break
			}
			if ref.yCenter >= e.y1 && ref.yCenter <= e.y2 {
				rows[i].items = append(rows[i].items, e)
				placed = true
				break
			}
		}
		if !placed {
			rows = append(rows, row{items: []entry{e}})
		}
	}

	// Within each row, sort right-to-left (descending x-center).
	for i := range rows {
		sort.SliceStable(rows[i].items, func(a, b int) bool {
			return rows[i].items[a].xCenter > rows[i].items[b].xCenter
		})
	}
	// Re-sort rows top→bottom (already stable from above but re-affirm
	// in case the row append order diverged).
	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].items[0].yCenter < rows[j].items[0].yCenter
	})

	out := make([]string, 0, len(ids))
	for _, r := range rows {
		for _, e := range r.items {
			out = append(out, e.id)
		}
	}
	return out
}

// isArabicCandidate reports whether the region's type and bbox make it
// a plausible body content candidate for the RTL reading-order fixup.
// Separators / page numbers / images are skipped — they don't carry
// reading-order semantics that benefit from RTL.
func isArabicCandidate(r model.Region) bool {
	switch r.Type {
	case model.RegionTypeSeparator, model.RegionTypePageNumber,
		model.RegionTypeImage, model.RegionTypeMarginNote:
		return false
	}
	if strings.TrimSpace(r.Text) == "" {
		return false
	}
	return true
}

// Register installs the Arabic profile into the lang registry. Called
// explicitly from the binary entrypoint so we honor the project's
// "no init() functions" rule.
func Register() {
	lang.Register(Profile{})
}
