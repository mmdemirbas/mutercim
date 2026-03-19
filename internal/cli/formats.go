package cli

import "fmt"

// validFormats is the set of recognized output format names.
var validFormats = map[string]string{
	"md":    "md",
	"latex": "latex",
	"tex":   "latex", // alias
	"pdf":   "pdf",
	"docx":  "docx",
}

// normalizeFormats validates and normalizes a list of format names.
// It resolves aliases (e.g. "tex" → "latex") and rejects unknown formats.
func normalizeFormats(raw []string) ([]string, error) {
	seen := make(map[string]bool)
	var result []string
	for _, f := range raw {
		canonical, ok := validFormats[f]
		if !ok {
			return nil, fmt.Errorf("unknown output format %q (valid: md, latex, tex, pdf, docx)", f)
		}
		if !seen[canonical] {
			seen[canonical] = true
			result = append(result, canonical)
		}
	}
	return result, nil
}
