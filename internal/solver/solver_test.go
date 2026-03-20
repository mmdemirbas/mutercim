package solver

import (
	"testing"

	"github.com/mmdemirbas/mutercim/internal/knowledge"
	"github.com/mmdemirbas/mutercim/internal/model"
)

func TestSolvePage(t *testing.T) {
	k := &knowledge.Knowledge{
		Entries: []knowledge.Entry{
			{Forms: map[string][]string{"ar": {"حديث"}, "tr": {"hadîs-i şerîf"}}},
		},
	}

	slvr := NewSolver(k, "ar", nil)

	page := &model.RegionPage{
		Version:    "2.0",
		PageNumber: 1,
		Regions: []model.Region{
			{ID: "r1", Text: "باب الألف", Type: model.RegionTypeHeader},
			{ID: "r2", Text: "هذا حديث", Type: model.RegionTypeEntry},
		},
		ReadingOrder: []string{"r1", "r2"},
	}

	solved := slvr.SolvePage(page, nil, "")

	// RegionPage fields should be preserved
	if solved.PageNumber != 1 {
		t.Errorf("PageNumber = %d, want 1", solved.PageNumber)
	}
	if len(solved.Regions) != 2 {
		t.Errorf("len(Regions) = %d, want 2", len(solved.Regions))
	}

	// Glossary should find "حديث" in entry text — stored as canonical source form
	if len(solved.GlossaryContext) == 0 {
		t.Error("expected glossary terms to be found")
	}
	found := false
	for _, g := range solved.GlossaryContext {
		if g == "حديث" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected glossary to contain canonical form 'حديث', got %v", solved.GlossaryContext)
	}

	// No validation warnings for well-formed page
	if len(solved.ValidationWarnings) != 0 {
		t.Errorf("expected no validation warnings, got %v", solved.ValidationWarnings)
	}

	// No previous page summary
	if solved.PreviousPageSummary != "" {
		t.Errorf("PreviousPageSummary = %q, want empty", solved.PreviousPageSummary)
	}
}

func TestSolvePage_WithPreviousPageSummary(t *testing.T) {
	k := &knowledge.Knowledge{}
	slvr := NewSolver(k, "ar", nil)

	page := &model.RegionPage{
		Version:    "2.0",
		PageNumber: 6,
		Regions: []model.Region{
			{ID: "r1", Text: "text", Type: model.RegionTypeEntry},
		},
		ReadingOrder: []string{"r1"},
	}

	solved := slvr.SolvePage(page, nil, "Page 5 — Entries 100-105")

	if solved.PreviousPageSummary != "Page 5 — Entries 100-105" {
		t.Errorf("PreviousPageSummary = %q", solved.PreviousPageSummary)
	}
}

func TestSolvePage_ValidationWarnings_EmptyText(t *testing.T) {
	slvr := NewSolver(&knowledge.Knowledge{}, "ar", nil)

	page := &model.RegionPage{
		Version:    "2.0",
		PageNumber: 1,
		Regions: []model.Region{
			{ID: "r1", Text: "", Type: model.RegionTypeEntry},       // empty text
			{ID: "sep1", Text: "", Type: model.RegionTypeSeparator}, // ok for separator
		},
		ReadingOrder: []string{"r1", "sep1"},
	}

	solved := slvr.SolvePage(page, nil, "")

	if len(solved.ValidationWarnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(solved.ValidationWarnings), solved.ValidationWarnings)
	}
	if !contains(solved.ValidationWarnings[0], "r1") {
		t.Errorf("warning should reference r1, got %q", solved.ValidationWarnings[0])
	}
}

func TestSolvePage_ValidationWarnings_BadReadingOrder(t *testing.T) {
	slvr := NewSolver(&knowledge.Knowledge{}, "ar", nil)

	page := &model.RegionPage{
		Version:    "2.0",
		PageNumber: 1,
		Regions: []model.Region{
			{ID: "r1", Text: "text", Type: model.RegionTypeEntry},
		},
		ReadingOrder: []string{"r1", "r99"}, // r99 doesn't exist
	}

	solved := slvr.SolvePage(page, nil, "")

	if len(solved.ValidationWarnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(solved.ValidationWarnings), solved.ValidationWarnings)
	}
	if !contains(solved.ValidationWarnings[0], "r99") {
		t.Errorf("warning should reference r99, got %q", solved.ValidationWarnings[0])
	}
}

func TestSolvePage_GlossaryWithPlaces(t *testing.T) {
	k := &knowledge.Knowledge{
		Entries: []knowledge.Entry{
			{Forms: map[string][]string{"ar": {"مكة"}, "tr": {"Mekke"}}},
		},
	}
	slvr := NewSolver(k, "ar", nil)

	page := &model.RegionPage{
		Version:    "2.0",
		PageNumber: 1,
		Regions: []model.Region{
			{ID: "r1", Text: "ذهب إلى مكة", Type: model.RegionTypeEntry},
		},
		ReadingOrder: []string{"r1"},
	}

	solved := slvr.SolvePage(page, nil, "")

	if len(solved.GlossaryContext) != 1 || solved.GlossaryContext[0] != "مكة" {
		t.Errorf("GlossaryContext = %v, want [مكة]", solved.GlossaryContext)
	}
}

func TestSolvePage_GlossaryMatchesVariants(t *testing.T) {
	k := &knowledge.Knowledge{
		Entries: []knowledge.Entry{
			{Forms: map[string][]string{
				"ar": {"صلى الله عليه وسلم", "ﷺ", "صلعم"},
				"tr": {"sallallâhu aleyhi ve sellem"},
			}},
		},
	}
	slvr := NewSolver(k, "ar", nil)

	page := &model.RegionPage{
		Version:    "2.0",
		PageNumber: 1,
		Regions: []model.Region{
			{ID: "r1", Text: "قال النبي ﷺ", Type: model.RegionTypeEntry},
		},
		ReadingOrder: []string{"r1"},
	}

	solved := slvr.SolvePage(page, nil, "")

	// Should match variant form and return canonical form
	if len(solved.GlossaryContext) != 1 || solved.GlossaryContext[0] != "صلى الله عليه وسلم" {
		t.Errorf("GlossaryContext = %v, want canonical form [صلى الله عليه وسلم]", solved.GlossaryContext)
	}
}

func TestPageSummary(t *testing.T) {
	page := &model.RegionPage{
		PageNumber: 42,
		Regions: []model.Region{
			{ID: "r1", Text: "باب الألف", Type: model.RegionTypeHeader},
			{ID: "r2", Text: "entry1", Type: model.RegionTypeEntry},
			{ID: "r3", Text: "entry2", Type: model.RegionTypeEntry},
		},
	}

	s := PageSummary(page)
	if s == "" {
		t.Fatal("expected non-empty summary")
	}
	if !contains(s, "باب الألف") {
		t.Errorf("summary should contain header text, got %q", s)
	}
	if !contains(s, "2 entries") {
		t.Errorf("summary should contain entry count, got %q", s)
	}
}

func TestPageSummary_Nil(t *testing.T) {
	if s := PageSummary(nil); s != "" {
		t.Errorf("expected empty for nil, got %q", s)
	}
}

func TestPageSummary_NoHeader(t *testing.T) {
	page := &model.RegionPage{
		PageNumber: 1,
		Regions: []model.Region{
			{ID: "r1", Text: "entry", Type: model.RegionTypeEntry},
		},
	}
	if s := PageSummary(page); s != "" {
		t.Errorf("expected empty for page without header, got %q", s)
	}
}

func TestValidateRegions_AllValid(t *testing.T) {
	page := &model.RegionPage{
		Regions: []model.Region{
			{ID: "r1", Text: "text", Type: model.RegionTypeEntry},
			{ID: "sep1", Text: "", Type: model.RegionTypeSeparator},
		},
		ReadingOrder: []string{"r1", "sep1"},
	}
	warnings := validateRegions(page)
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}
}

func TestDedupStrings(t *testing.T) {
	input := []string{"a", "b", "a", "c", "b"}
	got := dedupStrings(input)
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
