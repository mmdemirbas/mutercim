package pipeline

import (
	"context"
	"testing"

	"github.com/mmdemirbas/mutercim/internal/knowledge"
	"github.com/mmdemirbas/mutercim/internal/model"
)

func TestTranslatePipeline_ContextCancelled(t *testing.T) {
	stem := "testbook"
	pages := map[int]*model.SolvedRegionPage{
		1: makeSolvedRegionPage(1),
		2: makeSolvedRegionPage(2),
	}
	ws, cfg := setupTranslateWorkspace(t, stem, pages)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := Translate(ctx, TranslateOptions{
		Workspace: ws,
		Config:    cfg,
		Provider:  &mockProvider{response: regionTranslateResponseJSON()},
		Knowledge: &knowledge.Knowledge{},
	})
	if err != nil {
		t.Fatalf("Translate() error: %v", err)
	}

	if result.Completed != 0 {
		t.Errorf("expected 0 completed with cancelled context, got %d", result.Completed)
	}
}

func TestTranslatePipeline_NoTargetLangs(t *testing.T) {
	stem := "testbook"
	pages := map[int]*model.SolvedRegionPage{
		1: makeSolvedRegionPage(1),
	}
	ws, cfg := setupTranslateWorkspace(t, stem, pages)
	cfg.Book.TargetLangs = nil

	_, err := Translate(context.Background(), TranslateOptions{
		Workspace: ws,
		Config:    cfg,
		Provider:  &mockProvider{response: regionTranslateResponseJSON()},
		Knowledge: &knowledge.Knowledge{},
	})
	if err == nil {
		t.Fatal("expected error when no target languages configured")
	}
}

func TestTranslatePipeline_ProviderFails(t *testing.T) {
	stem := "testbook"
	pages := map[int]*model.SolvedRegionPage{
		1: makeSolvedRegionPage(1),
	}
	ws, cfg := setupTranslateWorkspace(t, stem, pages)

	result, err := Translate(context.Background(), TranslateOptions{
		Workspace: ws,
		Config:    cfg,
		Provider:  &failingProvider{},
		Knowledge: &knowledge.Knowledge{},
	})
	if err != nil {
		t.Fatalf("Translate() should not return top-level error: %v", err)
	}

	if result.Completed != 0 {
		t.Errorf("expected 0 completed, got %d", result.Completed)
	}
	if result.Failed != 1 {
		t.Errorf("expected 1 failed, got %d", result.Failed)
	}
}

func TestTranslatePipeline_WithGlossaryContext(t *testing.T) {
	stem := "testbook"
	page := &model.SolvedRegionPage{
		RegionPage: model.RegionPage{
			Version:    "2.0",
			PageNumber: 1,
			Regions: []model.Region{
				{ID: "r1", Text: "حديث", Type: model.RegionTypeEntry},
			},
			ReadingOrder: []string{"r1"},
		},
		GlossaryContext: []string{"حديث"}, // canonical source form
	}
	ws, cfg := setupTranslateWorkspace(t, stem, map[int]*model.SolvedRegionPage{1: page})

	k := &knowledge.Knowledge{
		Entries: []knowledge.Entry{
			{Forms: map[string][]string{"ar": {"حديث"}, "tr": {"hadîs-i şerîf"}}},
		},
	}

	result, err := Translate(context.Background(), TranslateOptions{
		Workspace: ws,
		Config:    cfg,
		Provider:  &mockProvider{response: regionTranslateResponseJSON()},
		Knowledge: k,
	})
	if err != nil {
		t.Fatalf("Translate() error: %v", err)
	}
	if result.Completed != 1 {
		t.Errorf("expected 1 completed, got %d", result.Completed)
	}
}
