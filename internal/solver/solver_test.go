package solver

import (
	"testing"

	"github.com/mmdemirbas/mutercim/internal/knowledge"
	"github.com/mmdemirbas/mutercim/internal/model"
)

func TestSolvePage(t *testing.T) {
	k := &knowledge.Knowledge{
		Sources: []knowledge.Source{
			{Code: "خ", NameAr: "صحيح البخاري", NameTr: "Sahîh-i Buhârî", Layer: "embedded"},
		},
		Terminology: []knowledge.Term{
			{Arabic: "حديث", Turkish: "hadîs-i şerîf"},
		},
	}

	slvr := NewSolver(k, nil)

	page := &model.ReadPage{
		PageNumber: 1,
		Entries: []model.Entry{
			{Number: intPtr(1), Type: "hadith", ArabicText: "هذا حديث"},
		},
		Footnotes: []model.Footnote{
			{SourceCodes: []string{"خ"}},
		},
	}

	solved := slvr.SolvePage(page, nil)

	if len(solved.SourcesResolved) != 1 {
		t.Fatalf("expected 1 resolved source, got %d", len(solved.SourcesResolved))
	}
	if solved.SourcesResolved[0].Code != "خ" {
		t.Errorf("expected code 'خ', got %q", solved.SourcesResolved[0].Code)
	}

	if solved.Validation == nil {
		t.Fatal("expected validation, got nil")
	}
	if solved.Validation.Status != "ok" {
		t.Errorf("expected status 'ok', got %q", solved.Validation.Status)
	}

	if solved.TranslationContext == nil {
		t.Fatal("expected translation context, got nil")
	}
	// "حديث" should be found in "هذا حديث"
	if len(solved.TranslationContext.RelevantGlossaryTerms) == 0 {
		t.Error("expected glossary terms to be found")
	}
}

func TestSolvePageWithContinuation(t *testing.T) {
	k := &knowledge.Knowledge{}
	slvr := NewSolver(k, nil)

	previous := &model.ReadPage{PageNumber: 5}
	current := &model.ReadPage{
		PageNumber: 6,
		Entries: []model.Entry{
			{IsContinuation: true, Type: "hadith", ArabicText: "continued"},
		},
	}

	solved := slvr.SolvePage(current, previous)

	if solved.ContinuationInfo == nil {
		t.Fatal("expected continuation info")
	}
	if solved.ContinuationInfo.ContinuesFrom == nil || *solved.ContinuationInfo.ContinuesFrom != 5 {
		t.Errorf("expected continues_from=5, got %v", solved.ContinuationInfo.ContinuesFrom)
	}
}
