package knowledge

import "strings"

// Entry represents a single glossary entry with language-specific forms.
// Each entry maps ISO 639-1 language codes to one or more forms.
// The first form in each list is the canonical/preferred form; the rest are variants.
type Entry struct {
	Forms     map[string][]string // language code → forms (first is canonical)
	Note      string              // optional guidance for the AI
	cachedKey string              // computed once by mergeKey, used for dedup during loading
}

// Knowledge holds all glossary entries from all layers merged together.
type Knowledge struct {
	Entries []Entry
}

// GlossaryForPair returns only entries containing both the source and target language.
func (k *Knowledge) GlossaryForPair(source, target string) []Entry {
	var result []Entry
	for _, e := range k.Entries {
		if _, hasSrc := e.Forms[source]; hasSrc {
			if _, hasTgt := e.Forms[target]; hasTgt {
				result = append(result, e)
			}
		}
	}
	return result
}

// LookupByForm finds an entry by matching a form in the given language.
// Uses tashkeel-stripped comparison as fallback so vowelized forms match
// unvowelized glossary entries and vice versa.
func (k *Knowledge) LookupByForm(lang, form string) (Entry, bool) {
	strippedForm := stripTashkeel(form)
	for _, e := range k.Entries {
		forms, ok := e.Forms[lang]
		if !ok {
			continue
		}
		for _, f := range forms {
			if f == form || stripTashkeel(f) == strippedForm {
				return e, true
			}
		}
	}
	return Entry{}, false
}

// stripTashkeel removes Arabic diacritical marks (tashkeel/harakat) from a string.
// Removes Fathatan (U+064B) through Sukun (U+0652) and Superscript Alef (U+0670).
func stripTashkeel(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if (r >= '\u064B' && r <= '\u0652') || r == '\u0670' {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
