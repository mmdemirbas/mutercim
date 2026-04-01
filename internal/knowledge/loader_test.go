package knowledge

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEmptyPaths(t *testing.T) {
	k, err := Load(nil, "")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(k.Entries) != 0 {
		t.Errorf("expected 0 entries with nil paths, got %d", len(k.Entries))
	}
}

func TestParseSingleStringNormalizesToSlice(t *testing.T) {
	yaml := `entries:
  - ar: "فقه"
    tr: "fıkıh"
`
	k := &Knowledge{}
	if err := mergeRawEntries(k, []byte(yaml)); err != nil {
		t.Fatalf("mergeRawEntries() error: %v", err)
	}
	if len(k.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(k.Entries))
	}
	arForms := k.Entries[0].Forms["ar"]
	if len(arForms) != 1 || arForms[0] != "فقه" {
		t.Errorf("ar forms = %v, want [فقه]", arForms)
	}
	trForms := k.Entries[0].Forms["tr"]
	if len(trForms) != 1 || trForms[0] != "fıkıh" {
		t.Errorf("tr forms = %v, want [fıkıh]", trForms)
	}
}

func TestParseListValueStaysAsList(t *testing.T) {
	yaml := `entries:
  - ar: ["صلى الله عليه وسلم", "ﷺ", "صلعم"]
    tr: ["sallallâhu aleyhi ve sellem", "s.a.v."]
`
	k := &Knowledge{}
	if err := mergeRawEntries(k, []byte(yaml)); err != nil {
		t.Fatalf("mergeRawEntries() error: %v", err)
	}
	arForms := k.Entries[0].Forms["ar"]
	if len(arForms) != 3 {
		t.Fatalf("ar forms = %v, want 3 items", arForms)
	}
	if arForms[0] != "صلى الله عليه وسلم" || arForms[1] != "ﷺ" || arForms[2] != "صلعم" {
		t.Errorf("ar forms = %v", arForms)
	}
	trForms := k.Entries[0].Forms["tr"]
	if len(trForms) != 2 {
		t.Fatalf("tr forms = %v, want 2 items", trForms)
	}
}

func TestParseMixedStringAndList(t *testing.T) {
	yaml := `entries:
  - ar: "أبو هريرة"
    tr: "Ebû Hüreyre"
  - ar: ["صلى الله عليه وسلم", "ﷺ"]
    tr: "sallallâhu aleyhi ve sellem"
`
	k := &Knowledge{}
	if err := mergeRawEntries(k, []byte(yaml)); err != nil {
		t.Fatalf("mergeRawEntries() error: %v", err)
	}
	if len(k.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(k.Entries))
	}
	// First entry: string values normalized to single-element slices
	if len(k.Entries[0].Forms["ar"]) != 1 {
		t.Errorf("entry 0 ar forms = %v, want 1 item", k.Entries[0].Forms["ar"])
	}
	// Second entry: list value preserved
	if len(k.Entries[1].Forms["ar"]) != 2 {
		t.Errorf("entry 1 ar forms = %v, want 2 items", k.Entries[1].Forms["ar"])
	}
}

func TestParseEntryWithNote(t *testing.T) {
	yaml := `entries:
  - ar: "فقه"
    tr: "fıkıh"
    note: "Islamic jurisprudence"
`
	k := &Knowledge{}
	if err := mergeRawEntries(k, []byte(yaml)); err != nil {
		t.Fatalf("mergeRawEntries() error: %v", err)
	}
	if k.Entries[0].Note != "Islamic jurisprudence" {
		t.Errorf("note = %q, want %q", k.Entries[0].Note, "Islamic jurisprudence")
	}
}

func TestParseEntryWithoutNote(t *testing.T) {
	yaml := `entries:
  - ar: "فقه"
    tr: "fıkıh"
`
	k := &Knowledge{}
	if err := mergeRawEntries(k, []byte(yaml)); err != nil {
		t.Fatalf("mergeRawEntries() error: %v", err)
	}
	if k.Entries[0].Note != "" {
		t.Errorf("note = %q, want empty", k.Entries[0].Note)
	}
}

func TestParseMinimalEntry(t *testing.T) {
	yaml := `entries:
  - ar: "فقه"
    tr: "fıkıh"
`
	k := &Knowledge{}
	if err := mergeRawEntries(k, []byte(yaml)); err != nil {
		t.Fatalf("mergeRawEntries() error: %v", err)
	}
	if len(k.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(k.Entries))
	}
	e := k.Entries[0]
	if len(e.Forms) != 2 {
		t.Errorf("expected 2 language keys, got %d", len(e.Forms))
	}
	if _, ok := e.Forms["ar"]; !ok {
		t.Error("missing ar key")
	}
	if _, ok := e.Forms["tr"]; !ok {
		t.Error("missing tr key")
	}
}

func TestLanguageDetection_NoteExcluded(t *testing.T) {
	yaml := `entries:
  - ar: "حديث"
    tr: "hadîs-i şerîf"
    en: "hadith"
    note: "Prophetic tradition"
`
	k := &Knowledge{}
	if err := mergeRawEntries(k, []byte(yaml)); err != nil {
		t.Fatalf("mergeRawEntries() error: %v", err)
	}
	e := k.Entries[0]
	// "note" should NOT be a language key
	if _, ok := e.Forms["note"]; ok {
		t.Error("note should not be treated as a language code")
	}
	// Should have exactly 3 language keys
	if len(e.Forms) != 3 {
		t.Errorf("expected 3 language keys (ar, tr, en), got %d", len(e.Forms))
	}
	if e.Note != "Prophetic tradition" {
		t.Errorf("note = %q, want %q", e.Note, "Prophetic tradition")
	}
}

func TestGlossaryForPair_BothLanguages(t *testing.T) {
	k := &Knowledge{
		Entries: []Entry{
			{Forms: map[string][]string{"ar": {"فقه"}, "tr": {"fıkıh"}}},
			{Forms: map[string][]string{"ar": {"مكة"}, "en": {"Mecca"}}},
			{Forms: map[string][]string{"ar": {"حديث"}, "tr": {"hadîs-i şerîf"}, "en": {"hadith"}}},
		},
	}

	arTr := k.GlossaryForPair("ar", "tr")
	if len(arTr) != 2 {
		t.Errorf("ar→tr: expected 2 entries, got %d", len(arTr))
	}

	arEn := k.GlossaryForPair("ar", "en")
	if len(arEn) != 2 {
		t.Errorf("ar→en: expected 2 entries, got %d", len(arEn))
	}
}

func TestGlossaryForPair_ThreeLanguageEntry(t *testing.T) {
	k := &Knowledge{
		Entries: []Entry{
			{Forms: map[string][]string{"ar": {"حديث"}, "tr": {"hadîs-i şerîf"}, "en": {"hadith"}}},
		},
	}

	arTr := k.GlossaryForPair("ar", "tr")
	if len(arTr) != 1 {
		t.Errorf("ar→tr: expected 1, got %d", len(arTr))
	}
	arEn := k.GlossaryForPair("ar", "en")
	if len(arEn) != 1 {
		t.Errorf("ar→en: expected 1, got %d", len(arEn))
	}
	trEn := k.GlossaryForPair("tr", "en")
	if len(trEn) != 1 {
		t.Errorf("tr→en: expected 1, got %d", len(trEn))
	}
}

func TestLoadMultipleYAMLFiles(t *testing.T) {
	dir := t.TempDir()

	yaml1 := `entries:
  - ar: "فقه"
    tr: "fıkıh"
`
	yaml2 := `entries:
  - ar: "مكة"
    tr: "Mekke"
`
	if err := os.WriteFile(filepath.Join(dir, "terms.yaml"), []byte(yaml1), 0600); err != nil {
		t.Fatalf("write yaml1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "places.yaml"), []byte(yaml2), 0600); err != nil {
		t.Fatalf("write yaml2: %v", err)
	}

	k := &Knowledge{}
	if err := loadFromDir(k, dir); err != nil {
		t.Fatalf("loadFromDir() error: %v", err)
	}

	if len(k.Entries) != 2 {
		t.Fatalf("expected 2 entries from 2 files, got %d", len(k.Entries))
	}
}

func TestLoadFromKnowledgeAndMemoryMerge(t *testing.T) {
	knowledgeDir := t.TempDir()
	memoryDir := t.TempDir()

	wsYaml := `entries:
  - ar: "فقه"
    tr: "fıkıh (workspace)"
`
	memYaml := `entries:
  - ar: "فقه"
    tr: "fıkıh (memory)"
`
	if err := os.WriteFile(filepath.Join(knowledgeDir, "glossary.yaml"), []byte(wsYaml), 0600); err != nil {
		t.Fatalf("write workspace: %v", err)
	}
	if err := os.WriteFile(filepath.Join(memoryDir, "glossary.yaml"), []byte(memYaml), 0600); err != nil {
		t.Fatalf("write memory: %v", err)
	}

	k, err := Load([]string{knowledgeDir}, memoryDir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// Memory should override workspace (and workspace overrides embedded)
	entry, ok := k.LookupByForm("ar", "فقه")
	if !ok {
		t.Fatal("expected to find فقه")
	}
	trForms := entry.Forms["tr"]
	if len(trForms) != 1 || trForms[0] != "fıkıh (memory)" {
		t.Errorf("expected memory override, got %v", trForms)
	}
}

func TestLoadNonexistentPaths(t *testing.T) {
	k, err := Load([]string{"/nonexistent/knowledge"}, "/nonexistent/memory")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(k.Entries) != 0 {
		t.Errorf("expected 0 entries with nonexistent paths, got %d", len(k.Entries))
	}
}

func TestLoadMixedFileAndDir(t *testing.T) {
	dir := t.TempDir()

	// Create a directory with one entry
	subDir := filepath.Join(dir, "subdir")
	if err := os.MkdirAll(subDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "terms.yaml"), []byte(`entries:
  - ar: "فقه"
    tr: "fıkıh"
`), 0600); err != nil {
		t.Fatal(err)
	}

	// Create a standalone file with a different entry
	filePath := filepath.Join(dir, "extra.yaml")
	if err := os.WriteFile(filePath, []byte(`entries:
  - ar: "مكة"
    tr: "Mekke"
`), 0600); err != nil {
		t.Fatal(err)
	}

	k, err := Load([]string{subDir, filePath}, "")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(k.Entries) != 2 {
		t.Errorf("expected 2 entries from dir+file, got %d", len(k.Entries))
	}
}

func TestLoadInvalidSchemaWarnsAndContinues(t *testing.T) {
	dir := t.TempDir()

	// Valid file
	if err := os.WriteFile(filepath.Join(dir, "good.yaml"), []byte(`entries:
  - ar: "فقه"
    tr: "fıkıh"
`), 0600); err != nil {
		t.Fatal(err)
	}

	// Invalid file (not a knowledge schema)
	if err := os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte(`not_entries: true`), 0600); err != nil {
		t.Fatal(err)
	}

	k, err := Load([]string{dir}, "")
	if err != nil {
		t.Fatalf("Load() should not error on invalid schema: %v", err)
	}
	// Only the good file's entry should be loaded
	if len(k.Entries) != 1 {
		t.Errorf("expected 1 entry (bad file skipped), got %d", len(k.Entries))
	}
}

func TestLookupByForm(t *testing.T) {
	k := &Knowledge{
		Entries: []Entry{
			{Forms: map[string][]string{
				"ar": {"صلى الله عليه وسلم", "ﷺ", "صلعم"},
				"tr": {"sallallâhu aleyhi ve sellem", "s.a.v."},
			}},
		},
	}

	// Lookup by canonical form
	e, ok := k.LookupByForm("ar", "صلى الله عليه وسلم")
	if !ok {
		t.Fatal("expected to find entry by canonical form")
	}
	if e.Forms["tr"][0] != "sallallâhu aleyhi ve sellem" {
		t.Errorf("unexpected tr form: %v", e.Forms["tr"])
	}

	// Lookup by variant form
	e, ok = k.LookupByForm("ar", "ﷺ")
	if !ok {
		t.Fatal("expected to find entry by variant form")
	}

	// Lookup nonexistent
	_, ok = k.LookupByForm("ar", "nonexistent")
	if ok {
		t.Error("expected not to find nonexistent form")
	}
}

func TestMergeEntry_PreservesNote(t *testing.T) {
	// Base entry has a note; override without note should preserve it
	base := Entry{
		Forms: map[string][]string{"ar": {"فقه"}, "tr": {"fıkıh"}},
		Note:  "Islamic jurisprudence",
	}
	override := Entry{
		Forms: map[string][]string{"ar": {"فقه"}, "tr": {"fıkıh (updated)"}},
		// Note is empty
	}

	k := &Knowledge{}
	mergeEntry(k, base)
	mergeEntry(k, override)

	if len(k.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(k.Entries))
	}
	if k.Entries[0].Note != "Islamic jurisprudence" {
		t.Errorf("note = %q, want %q (should be preserved)", k.Entries[0].Note, "Islamic jurisprudence")
	}
	if k.Entries[0].Forms["tr"][0] != "fıkıh (updated)" {
		t.Errorf("tr form = %v, want updated", k.Entries[0].Forms["tr"])
	}
}

func TestMergeEntry_OverridesNote(t *testing.T) {
	// Override with a new note should replace the old one
	base := Entry{
		Forms: map[string][]string{"ar": {"فقه"}, "tr": {"fıkıh"}},
		Note:  "old note",
	}
	override := Entry{
		Forms: map[string][]string{"ar": {"فقه"}, "tr": {"fıkıh"}},
		Note:  "new note",
	}

	k := &Knowledge{}
	mergeEntry(k, base)
	mergeEntry(k, override)

	if len(k.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(k.Entries))
	}
	if k.Entries[0].Note != "new note" {
		t.Errorf("note = %q, want %q", k.Entries[0].Note, "new note")
	}
}

func TestMergeKey(t *testing.T) {
	// Merge key uses alphabetically first language's canonical form
	e1 := Entry{Forms: map[string][]string{"ar": {"فقه"}, "tr": {"fıkıh"}}}
	e2 := Entry{Forms: map[string][]string{"ar": {"فقه"}, "tr": {"fıkıh (updated)"}}}

	k := &Knowledge{}
	mergeEntry(k, e1)
	mergeEntry(k, e2) // should override

	if len(k.Entries) != 1 {
		t.Fatalf("expected 1 entry after merge, got %d", len(k.Entries))
	}
	if k.Entries[0].Forms["tr"][0] != "fıkıh (updated)" {
		t.Errorf("expected override, got %v", k.Entries[0].Forms["tr"])
	}
}
