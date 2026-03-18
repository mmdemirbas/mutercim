package config

import (
	"testing"

	"github.com/mmdemirbas/mutercim/internal/model"
)

func TestSectionLookup(t *testing.T) {
	sections := []model.Section{
		{Name: "front_matter", Pages: "1-2", Type: model.SectionSkip},
		{Name: "abbreviations", Pages: "6-8", Type: model.SectionReferenceTable},
		{Name: "hadith", Pages: "13-580", Type: model.SectionScholarlyEntries, Translate: true},
	}

	lookup, err := NewSectionLookup(sections)
	if err != nil {
		t.Fatalf("NewSectionLookup() error = %v", err)
	}

	tests := []struct {
		page     int
		wantName string
		wantType model.SectionType
	}{
		{1, "front_matter", model.SectionSkip},
		{2, "front_matter", model.SectionSkip},
		{3, "auto", model.SectionAuto},
		{6, "abbreviations", model.SectionReferenceTable},
		{8, "abbreviations", model.SectionReferenceTable},
		{12, "auto", model.SectionAuto},
		{13, "hadith", model.SectionScholarlyEntries},
		{300, "hadith", model.SectionScholarlyEntries},
		{580, "hadith", model.SectionScholarlyEntries},
		{581, "auto", model.SectionAuto},
	}

	for _, tt := range tests {
		s := lookup.ForPage(tt.page)
		if s.Name != tt.wantName {
			t.Errorf("ForPage(%d).Name = %q, want %q", tt.page, s.Name, tt.wantName)
		}
		if s.Type != tt.wantType {
			t.Errorf("ForPage(%d).Type = %q, want %q", tt.page, s.Type, tt.wantType)
		}
	}
}

func TestSectionLookupShouldSkip(t *testing.T) {
	sections := []model.Section{
		{Name: "front_matter", Pages: "1-2", Type: model.SectionSkip},
		{Name: "content", Pages: "3-100", Type: model.SectionScholarlyEntries, Translate: true},
	}

	lookup, err := NewSectionLookup(sections)
	if err != nil {
		t.Fatalf("NewSectionLookup() error = %v", err)
	}

	if !lookup.ShouldSkip(1) {
		t.Error("ShouldSkip(1) = false, want true")
	}
	if lookup.ShouldSkip(3) {
		t.Error("ShouldSkip(3) = true, want false")
	}
	if lookup.ShouldSkip(200) {
		t.Error("ShouldSkip(200) = true, want false (auto)")
	}
}

func TestSectionLookupShouldTranslate(t *testing.T) {
	sections := []model.Section{
		{Name: "skip", Pages: "1-2", Type: model.SectionSkip},
		{Name: "abbrev", Pages: "6-8", Type: model.SectionReferenceTable, Translate: false},
		{Name: "content", Pages: "10-100", Type: model.SectionScholarlyEntries, Translate: true},
	}

	lookup, err := NewSectionLookup(sections)
	if err != nil {
		t.Fatalf("NewSectionLookup() error = %v", err)
	}

	if lookup.ShouldTranslate(1) {
		t.Error("ShouldTranslate(1) = true, want false (skip section)")
	}
	if lookup.ShouldTranslate(6) {
		t.Error("ShouldTranslate(6) = true, want false (reference_table)")
	}
	if !lookup.ShouldTranslate(50) {
		t.Error("ShouldTranslate(50) = false, want true")
	}
	// Pages not in any section default to auto with translate=true
	if !lookup.ShouldTranslate(200) {
		t.Error("ShouldTranslate(200) = false, want true (auto)")
	}
}

func TestSectionLookupInvalidRanges(t *testing.T) {
	sections := []model.Section{
		{Name: "bad", Pages: "abc", Type: model.SectionProse},
	}
	_, err := NewSectionLookup(sections)
	if err == nil {
		t.Error("NewSectionLookup() with invalid ranges should error")
	}
}

func TestSectionLookupAllPages(t *testing.T) {
	sections := []model.Section{
		{Name: "a", Pages: "1-3", Type: model.SectionProse},
		{Name: "b", Pages: "5,7", Type: model.SectionProse},
	}

	lookup, err := NewSectionLookup(sections)
	if err != nil {
		t.Fatalf("NewSectionLookup() error = %v", err)
	}

	pages := lookup.AllPages()
	want := []int{1, 2, 3, 5, 7}

	if len(pages) != len(want) {
		t.Fatalf("AllPages() len = %d, want %d", len(pages), len(want))
	}
	// Pages might not be sorted; just check they're all present
	pageSet := make(map[int]bool)
	for _, p := range pages {
		pageSet[p] = true
	}
	for _, w := range want {
		if !pageSet[w] {
			t.Errorf("AllPages() missing page %d", w)
		}
	}
}
