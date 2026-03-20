package workspace

import (
	"regexp"
	"strings"
)

// osProhibited matches characters prohibited in filenames on major OSes:
// / \ : * ? " < > | and null byte.
var osProhibited = regexp.MustCompile(`[/\\:*?"<>|\x00]`)

// multiDash collapses runs of dashes into a single dash.
var multiDash = regexp.MustCompile(`-{2,}`)

// SanitizeTitle converts a book title into a safe filename stem.
// Rules:
//   - Replace OS-prohibited chars with -
//   - Trim leading/trailing spaces and dots
//   - Collapse multiple - into single -
//   - If result is empty, fall back to "book"
//   - Unicode is preserved (Arabic, Turkish, Chinese titles stay as-is)
func SanitizeTitle(title string) string {
	s := osProhibited.ReplaceAllString(title, "-")
	s = multiDash.ReplaceAllString(s, "-")
	s = strings.Trim(s, " .")
	s = strings.Trim(s, "-")
	if s == "" {
		return "book"
	}
	return s
}
