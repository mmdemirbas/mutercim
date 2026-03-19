package enrichment

import (
	"testing"

	"github.com/mmdemirbas/mutercim/internal/knowledge"
	"github.com/mmdemirbas/mutercim/internal/model"
)

func TestResolveAbbreviations(t *testing.T) {
	k := &knowledge.Knowledge{
		Sources: []knowledge.Source{
			{Code: "خ", NameAr: "صحيح البخاري", NameTr: "Sahîh-i Buhârî", Layer: "embedded"},
			{Code: "م", NameAr: "صحيح مسلم", NameTr: "Sahîh-i Müslim", Layer: "workspace"},
		},
	}

	footnotes := []model.Footnote{
		{SourceCodes: []string{"خ", "م"}},
		{SourceCodes: []string{"خ", "فر"}}, // فر is unknown
	}

	resolved, unresolved := ResolveAbbreviations(footnotes, k)

	if len(resolved) != 2 {
		t.Fatalf("expected 2 resolved, got %d", len(resolved))
	}
	if resolved[0].Code != "خ" {
		t.Errorf("expected code 'خ', got %q", resolved[0].Code)
	}
	if resolved[0].Layer != "embedded" {
		t.Errorf("expected layer 'embedded', got %q", resolved[0].Layer)
	}
	if resolved[1].Code != "م" {
		t.Errorf("expected code 'م', got %q", resolved[1].Code)
	}

	if len(unresolved) != 1 || unresolved[0] != "فر" {
		t.Errorf("expected unresolved ['فر'], got %v", unresolved)
	}
}

func TestResolveAbbreviationsEmpty(t *testing.T) {
	k := &knowledge.Knowledge{}
	resolved, unresolved := ResolveAbbreviations(nil, k)
	if len(resolved) != 0 {
		t.Errorf("expected 0 resolved, got %d", len(resolved))
	}
	if len(unresolved) != 0 {
		t.Errorf("expected 0 unresolved, got %d", len(unresolved))
	}
}
