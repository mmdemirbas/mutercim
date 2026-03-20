package knowledge

// Entry represents a single glossary entry with language-specific forms.
// Each entry maps ISO 639-1 language codes to one or more forms.
// The first form in each list is the canonical/preferred form; the rest are variants.
type Entry struct {
	Forms map[string][]string // language code → forms (first is canonical)
	Note  string              // optional guidance for the AI
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
func (k *Knowledge) LookupByForm(lang, form string) (Entry, bool) {
	for _, e := range k.Entries {
		forms, ok := e.Forms[lang]
		if !ok {
			continue
		}
		for _, f := range forms {
			if f == form {
				return e, true
			}
		}
	}
	return Entry{}, false
}
