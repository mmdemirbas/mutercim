package knowledge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildGlossary(t *testing.T) {
	k := &Knowledge{
		Entries: []Entry{
			{
				Forms: map[string][]string{
					"ar": {"صلى الله عليه وسلم", "ﷺ", "صلعم"},
					"tr": {"sallallâhu aleyhi ve sellem", "s.a.v."},
				},
				Note: "Salawat",
			},
			{
				Forms: map[string][]string{
					"ar": {"أبو هريرة"},
					"tr": {"Ebû Hüreyre"},
				},
			},
			{
				Forms: map[string][]string{
					"ar": {"حديث"},
					"tr": {"hadîs-i şerîf"},
				},
			},
		},
	}

	glossary := k.BuildGlossary("ar", "tr")

	for _, want := range []string{
		"sallallâhu aleyhi ve sellem",
		"Ebû Hüreyre",
		"hadîs-i şerîf",
		"also: ﷺ, صلعم",
		"also: s.a.v.",
		"Salawat",
	} {
		if !strings.Contains(glossary, want) {
			t.Errorf("glossary should contain %q, got:\n%s", want, glossary)
		}
	}
}

func TestBuildGlossaryEmpty(t *testing.T) {
	k := &Knowledge{}
	glossary := k.BuildGlossary("ar", "tr")
	if glossary != "" {
		t.Errorf("expected empty glossary, got %q", glossary)
	}
}

func TestBuildGlossaryNoMatchingPair(t *testing.T) {
	k := &Knowledge{
		Entries: []Entry{
			{Forms: map[string][]string{"ar": {"فقه"}, "tr": {"fıkıh"}}},
		},
	}
	glossary := k.BuildGlossary("ar", "en")
	if glossary != "" {
		t.Errorf("expected empty glossary for ar→en (no en forms), got %q", glossary)
	}
}

func TestFormatGlossaryLine_Canonical(t *testing.T) {
	e := Entry{
		Forms: map[string][]string{
			"ar": {"فقه"},
			"tr": {"fıkıh"},
		},
	}
	got := FormatGlossaryLine(e, "ar", "tr")
	want := "فقه → fıkıh"
	if got != want {
		t.Errorf("FormatGlossaryLine = %q, want %q", got, want)
	}
}

func TestFormatGlossaryLine_Variants(t *testing.T) {
	e := Entry{
		Forms: map[string][]string{
			"ar": {"صلى الله عليه وسلم", "ﷺ", "صلعم"},
			"tr": {"sallallâhu aleyhi ve sellem", "s.a.v."},
		},
	}
	got := FormatGlossaryLine(e, "ar", "tr")
	want := "صلى الله عليه وسلم (also: ﷺ, صلعم) → sallallâhu aleyhi ve sellem (also: s.a.v.)"
	if got != want {
		t.Errorf("FormatGlossaryLine = %q, want %q", got, want)
	}
}

func TestFormatGlossaryLine_WithNote(t *testing.T) {
	e := Entry{
		Forms: map[string][]string{
			"ar": {"أبو هريرة"},
			"tr": {"Ebû Hüreyre"},
		},
		Note: "Prominent companion",
	}
	got := FormatGlossaryLine(e, "ar", "tr")
	want := "أبو هريرة → Ebû Hüreyre — Prominent companion"
	if got != want {
		t.Errorf("FormatGlossaryLine = %q, want %q", got, want)
	}
}

func TestFormatGlossaryLine_VariantsAndNote(t *testing.T) {
	e := Entry{
		Forms: map[string][]string{
			"ar": {"صلى الله عليه وسلم", "ﷺ", "صلعم"},
			"tr": {"sallallâhu aleyhi ve sellem", "s.a.v."},
		},
		Note: "Salawat. Must appear after every mention of the Prophet.",
	}
	got := FormatGlossaryLine(e, "ar", "tr")
	if !strings.Contains(got, "also: ﷺ, صلعم") {
		t.Errorf("should contain source variants, got %q", got)
	}
	if !strings.Contains(got, "also: s.a.v.") {
		t.Errorf("should contain target variants, got %q", got)
	}
	if !strings.Contains(got, "— Salawat") {
		t.Errorf("should contain note after dash, got %q", got)
	}
}

func TestFormatGlossaryLine_MissingLanguage(t *testing.T) {
	e := Entry{
		Forms: map[string][]string{
			"ar": {"فقه"},
			"tr": {"fıkıh"},
		},
	}
	got := FormatGlossaryLine(e, "ar", "en") // "en" not present
	if got != "" {
		t.Errorf("expected empty for missing language, got %q", got)
	}
}

func TestLoadAndBuildGlossary(t *testing.T) {
	dir := t.TempDir()
	yaml := `entries:
  - ar: "فقه"
    tr: "fıkıh"
    en: "fiqh"
  - ar: "حديث"
    tr: "hadîs-i şerîf"
    en: "hadith"
`
	os.WriteFile(filepath.Join(dir, "glossary.yaml"), []byte(yaml), 0644)

	k, err := Load([]string{dir}, "")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(k.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(k.Entries))
	}

	glossary := k.BuildGlossary("ar", "tr")
	if glossary == "" {
		t.Error("expected non-empty ar→tr glossary")
	}
}
