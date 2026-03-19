package solver

import (
	"github.com/mmdemirbas/mutercim/internal/knowledge"
	"github.com/mmdemirbas/mutercim/internal/model"
)

// ResolveAbbreviations resolves source abbreviation codes from footnotes
// against the knowledge sources. Returns resolved sources with layer info
// and a list of unresolved codes.
func ResolveAbbreviations(footnotes []model.Footnote, k *knowledge.Knowledge) ([]model.SourceResolved, []string) {
	seen := make(map[string]bool)
	var resolved []model.SourceResolved
	var unresolved []string

	for _, fn := range footnotes {
		for _, code := range fn.SourceCodes {
			if seen[code] {
				continue
			}
			seen[code] = true

			src, ok := k.LookupSource(code)
			if ok {
				resolved = append(resolved, model.SourceResolved{
					Code:   code,
					NameAr: src.NameAr,
					NameTr: src.NameTr,
					Layer:  src.Layer,
				})
			} else {
				unresolved = append(unresolved, code)
			}
		}
	}

	return resolved, unresolved
}
