package knowledge

import (
	"fmt"
	"strings"
)

// BuildGlossary creates a formatted glossary string for prompt injection
// containing entries that have both the source and target language.
func (k *Knowledge) BuildGlossary(source, target string) string {
	entries := k.GlossaryForPair(source, target)
	if len(entries) == 0 {
		return ""
	}
	var lines []string
	for _, e := range entries {
		lines = append(lines, "- "+FormatGlossaryLine(e, source, target))
	}
	return strings.Join(lines, "\n")
}

// FormatGlossaryLine formats a single entry for prompt injection.
// Format: "source (also: variant1, variant2) → target (also: variant1) — note"
func FormatGlossaryLine(e Entry, source, target string) string {
	srcForms := e.Forms[source]
	tgtForms := e.Forms[target]
	if len(srcForms) == 0 || len(tgtForms) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString(srcForms[0])
	if len(srcForms) > 1 {
		fmt.Fprintf(&b, " (also: %s)", strings.Join(srcForms[1:], ", "))
	}
	b.WriteString(" → ")
	b.WriteString(tgtForms[0])
	if len(tgtForms) > 1 {
		fmt.Fprintf(&b, " (also: %s)", strings.Join(tgtForms[1:], ", "))
	}
	if e.Note != "" {
		b.WriteString(" — ")
		b.WriteString(e.Note)
	}
	return b.String()
}
