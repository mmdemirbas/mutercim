package enrichment

import (
	"testing"

	"github.com/mmdemirbas/mutercim/internal/knowledge"
	"github.com/mmdemirbas/mutercim/internal/model"
)

func TestEnrichPage(t *testing.T) {
	k := &knowledge.Knowledge{
		Sources: []knowledge.Source{
			{Code: "خ", NameAr: "صحيح البخاري", NameTr: "Sahîh-i Buhârî", Layer: "embedded"},
		},
		Terminology: []knowledge.Term{
			{Arabic: "حديث", Turkish: "hadîs-i şerîf"},
		},
	}

	enricher := NewEnricher(k, nil)

	page := &model.ExtractedPage{
		PageNumber: 1,
		Entries: []model.Entry{
			{Number: intPtr(1), Type: "hadith", ArabicText: "هذا حديث"},
		},
		Footnotes: []model.Footnote{
			{SourceCodes: []string{"خ"}},
		},
	}

	enriched := enricher.EnrichPage(page, nil)

	if len(enriched.SourcesResolved) != 1 {
		t.Fatalf("expected 1 resolved source, got %d", len(enriched.SourcesResolved))
	}
	if enriched.SourcesResolved[0].Code != "خ" {
		t.Errorf("expected code 'خ', got %q", enriched.SourcesResolved[0].Code)
	}

	if enriched.Validation == nil {
		t.Fatal("expected validation, got nil")
	}
	if enriched.Validation.Status != "ok" {
		t.Errorf("expected status 'ok', got %q", enriched.Validation.Status)
	}

	if enriched.TranslationContext == nil {
		t.Fatal("expected translation context, got nil")
	}
	// "حديث" should be found in "هذا حديث"
	if len(enriched.TranslationContext.RelevantGlossaryTerms) == 0 {
		t.Error("expected glossary terms to be found")
	}
}

func TestEnrichPageWithContinuation(t *testing.T) {
	k := &knowledge.Knowledge{}
	enricher := NewEnricher(k, nil)

	previous := &model.ExtractedPage{PageNumber: 5}
	current := &model.ExtractedPage{
		PageNumber: 6,
		Entries: []model.Entry{
			{IsContinuation: true, Type: "hadith", ArabicText: "continued"},
		},
	}

	enriched := enricher.EnrichPage(current, previous)

	if enriched.ContinuationInfo == nil {
		t.Fatal("expected continuation info")
	}
	if enriched.ContinuationInfo.ContinuesFrom == nil || *enriched.ContinuationInfo.ContinuesFrom != 5 {
		t.Errorf("expected continues_from=5, got %v", enriched.ContinuationInfo.ContinuesFrom)
	}
}
