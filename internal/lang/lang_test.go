package lang

import (
	"testing"

	"github.com/mmdemirbas/mutercim/internal/model"
)

func TestGet_UnknownReturnsEmptyProfile(t *testing.T) {
	p := Get("xyz")
	if p.Code() != "xyz" {
		t.Errorf("empty profile should carry the requested code, got %q", p.Code())
	}
	if p.OcrOverride() != "" {
		t.Errorf("empty profile OcrOverride should be empty, got %q", p.OcrOverride())
	}
	if hints := p.RenderingHints(); hints != nil {
		t.Errorf("empty profile RenderingHints should be nil, got %v", hints)
	}
}

func TestGet_EmptyCodeReturnsEmptyProfile(t *testing.T) {
	p := Get("")
	if p.Code() != "" {
		t.Errorf("empty code → empty profile with empty code, got %q", p.Code())
	}
}

func TestGet_IsCaseInsensitive(t *testing.T) {
	stub := stubProfile{code: "fr"}
	Register(stub)

	for _, in := range []string{"fr", "FR", "Fr", "  fr "} {
		got := Get(in)
		if got.Code() != "fr" {
			t.Errorf("Get(%q).Code() = %q, want \"fr\"", in, got.Code())
		}
	}
}

func TestEmptyProfile_ReadingOrderFixup_PassesThrough(t *testing.T) {
	regions := []model.Region{{ID: "a"}, {ID: "b"}}
	order := []string{"a", "b"}
	got := EmptyProfile{}.ReadingOrderFixup(regions, order)
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("empty profile must pass reading order through, got %v", got)
	}
}

func TestRegister_Idempotent(t *testing.T) {
	// Register the same profile twice — second call must not panic.
	stub := stubProfile{code: "test-dup"}
	Register(stub)
	Register(stub) // duplicate; idempotent contract
	if Get("test-dup").Code() != "test-dup" {
		t.Error("after duplicate Register the profile should still be retrievable")
	}
}

// stubProfile is a no-op profile used by registry tests so we don't
// pull in the ar package (would create test-only test dependency).
type stubProfile struct{ code string }

func (s stubProfile) Code() string                                                    { return s.code }
func (stubProfile) ScriptDetect(string) bool                                          { return false }
func (stubProfile) OcrOverride() string                                               { return "" }
func (stubProfile) ReadingOrderFixup(_ []model.Region, order []string) []string       { return order }
func (stubProfile) RenderingHints() map[string]string                                 { return nil }
func (stubProfile) PromptCorpus() string                                              { return "" }
