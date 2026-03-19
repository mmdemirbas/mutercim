package solver

import (
	"testing"

	"github.com/mmdemirbas/mutercim/internal/model"
)

func intPtr(n int) *int { return &n }

func TestValidateSequentialNumbers(t *testing.T) {
	page := &model.ReadPage{
		Entries: []model.Entry{
			{Number: intPtr(1), Type: "hadith", ArabicText: "text1"},
			{Number: intPtr(2), Type: "hadith", ArabicText: "text2"},
			{Number: intPtr(3), Type: "hadith", ArabicText: "text3"},
		},
	}

	v := Validate(page)
	if v.Status != "ok" {
		t.Errorf("expected status 'ok', got %q", v.Status)
	}
	if !v.HadithNumberSequenceValid {
		t.Error("expected valid sequence")
	}
}

func TestValidateNumberGap(t *testing.T) {
	page := &model.ReadPage{
		Entries: []model.Entry{
			{Number: intPtr(1), Type: "hadith", ArabicText: "text1"},
			{Number: intPtr(3), Type: "hadith", ArabicText: "text3"}, // gap: 1→3
		},
	}

	v := Validate(page)
	if v.Status != "warnings" {
		t.Errorf("expected status 'warnings', got %q", v.Status)
	}
	if v.HadithNumberSequenceValid {
		t.Error("expected invalid sequence")
	}
	if len(v.Warnings) == 0 {
		t.Error("expected warnings")
	}
}

func TestValidateEmptyType(t *testing.T) {
	page := &model.ReadPage{
		Entries: []model.Entry{
			{Number: intPtr(1), Type: "", ArabicText: "text"},
		},
	}

	v := Validate(page)
	if v.Status != "warnings" {
		t.Errorf("expected warnings for empty type, got %q", v.Status)
	}
}

func TestValidateEmptyArabicText(t *testing.T) {
	page := &model.ReadPage{
		Entries: []model.Entry{
			{Number: intPtr(1), Type: "hadith", ArabicText: ""},
		},
	}

	v := Validate(page)
	if v.Status != "warnings" {
		t.Errorf("expected warnings for empty text, got %q", v.Status)
	}
}

func TestValidateSkipsContinuations(t *testing.T) {
	page := &model.ReadPage{
		Entries: []model.Entry{
			{IsContinuation: true, Type: "hadith", ArabicText: "continued"},
			{Number: intPtr(5), Type: "hadith", ArabicText: "text"},
		},
	}

	v := Validate(page)
	// Continuations have no number, so sequence check starts from 5
	if v.Status != "ok" {
		t.Errorf("expected 'ok', got %q with warnings: %v", v.Status, v.Warnings)
	}
}
