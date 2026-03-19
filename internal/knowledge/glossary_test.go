package knowledge

import (
	"strings"
	"testing"
)

func TestBuildGlossary(t *testing.T) {
	k := &Knowledge{
		Honorifics: []Honorific{
			{Arabic: "صلى الله عليه وسلم", Turkish: "sallallâhu aleyhi ve sellem"},
		},
		Companions: []Companion{
			{Arabic: "أبو هريرة", Turkish: "Ebû Hüreyre"},
		},
		Sources: []Source{
			{Code: "خ", NameAr: "صحيح البخاري", NameTr: "Sahîh-i Buhârî"},
		},
		Terminology: []Term{
			{Arabic: "حديث", Turkish: "hadîs-i şerîf"},
		},
	}

	glossary := k.BuildGlossary()

	for _, want := range []string{
		"sallallâhu aleyhi ve sellem",
		"Ebû Hüreyre",
		"Sahîh-i Buhârî",
		"hadîs-i şerîf",
	} {
		if !strings.Contains(glossary, want) {
			t.Errorf("glossary should contain %q", want)
		}
	}
}

func TestBuildGlossaryEmpty(t *testing.T) {
	k := &Knowledge{}
	glossary := k.BuildGlossary()
	if glossary != "" {
		t.Errorf("expected empty glossary, got %q", glossary)
	}
}

func TestHonorificsSection(t *testing.T) {
	k := &Knowledge{
		Honorifics: []Honorific{
			{Arabic: "رحمه الله", Turkish: "rahimehullâh"},
		},
	}
	s := k.HonorificsSection()
	if !strings.Contains(s, "رحمه الله") || !strings.Contains(s, "rahimehullâh") {
		t.Errorf("unexpected section: %q", s)
	}
}

func TestHonorificsSectionEmpty(t *testing.T) {
	k := &Knowledge{}
	if s := k.HonorificsSection(); s != "" {
		t.Errorf("expected empty, got %q", s)
	}
}

func TestCompanionsSection(t *testing.T) {
	k := &Knowledge{
		Companions: []Companion{{Arabic: "عائشة", Turkish: "Hz. Âişe"}},
	}
	s := k.CompanionsSection()
	if !strings.Contains(s, "عائشة") || !strings.Contains(s, "Hz. Âişe") {
		t.Errorf("unexpected section: %q", s)
	}
}

func TestSourcesSection(t *testing.T) {
	k := &Knowledge{
		Sources: []Source{{Code: "م", NameAr: "صحيح مسلم", NameTr: "Sahîh-i Müslim"}},
	}
	s := k.SourcesSection()
	if !strings.Contains(s, "م") || !strings.Contains(s, "Sahîh-i Müslim") {
		t.Errorf("unexpected section: %q", s)
	}
}

func TestTerminologySection(t *testing.T) {
	k := &Knowledge{
		Terminology: []Term{{Arabic: "فقه", Turkish: "fıkıh"}},
	}
	s := k.TerminologySection()
	if !strings.Contains(s, "فقه") || !strings.Contains(s, "fıkıh") {
		t.Errorf("unexpected section: %q", s)
	}
}
