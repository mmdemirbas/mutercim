package enrichment

import (
	"testing"

	"github.com/mmdemirbas/mutercim/internal/model"
)

func TestDetectContinuationFromPrevious(t *testing.T) {
	current := &model.ExtractedPage{
		PageNumber: 10,
		Entries: []model.Entry{
			{IsContinuation: true, ArabicText: "...continued text"},
		},
	}
	previous := &model.ExtractedPage{PageNumber: 9}

	info := DetectContinuation(current, previous)
	if info == nil {
		t.Fatal("expected continuation info, got nil")
	}
	if info.ContinuesFrom == nil || *info.ContinuesFrom != 9 {
		t.Errorf("expected continues_from=9, got %v", info.ContinuesFrom)
	}
}

func TestDetectContinuationToNext(t *testing.T) {
	current := &model.ExtractedPage{
		PageNumber: 10,
		Entries: []model.Entry{
			{ArabicText: "text", ContinuesOnNextPage: true},
		},
	}

	info := DetectContinuation(current, nil)
	if info == nil {
		t.Fatal("expected continuation info, got nil")
	}
	if info.ContinuesOn == nil || *info.ContinuesOn != 11 {
		t.Errorf("expected continues_on=11, got %v", info.ContinuesOn)
	}
}

func TestDetectContinuationNone(t *testing.T) {
	current := &model.ExtractedPage{
		PageNumber: 10,
		Entries: []model.Entry{
			{ArabicText: "normal text"},
		},
	}

	info := DetectContinuation(current, nil)
	if info != nil {
		t.Errorf("expected nil, got %+v", info)
	}
}

func TestDetectContinuationEmpty(t *testing.T) {
	current := &model.ExtractedPage{PageNumber: 1}
	info := DetectContinuation(current, nil)
	if info != nil {
		t.Errorf("expected nil for empty page, got %+v", info)
	}
}
