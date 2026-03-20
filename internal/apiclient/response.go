package apiclient

import (
	"encoding/json"
	"errors"
	"strings"
)

// sanitizeResponse strips invisible Unicode characters that LLMs occasionally emit
// and that break JSON parsing. Specifically: zero-width spaces (U+200B),
// byte-order marks (U+FEFF), and other zero-width characters.
// Note: U+200C (ZWNJ) and U+200D (ZWJ) are preserved because they have
// legitimate uses in Arabic typography.
func sanitizeResponse(response string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '\u200B': // zero-width space
			return -1
		case '\uFEFF': // BOM
			return -1
		case '\u200E', '\u200F': // LTR/RTL marks
			return -1
		case '\u2060': // word joiner
			return -1
		case '\u2028', '\u2029': // line/paragraph separators
			return -1
		}
		return r
	}, response)
}

// ExtractJSON sanitizes the response, then attempts to extract a JSON object.
// Tries in order: direct parse, markdown code block extraction, first { to last }.
// Returns the raw JSON string (not unmarshaled) so the caller can unmarshal
// into their specific type.
func ExtractJSON(response string) (string, error) {
	sanitized := sanitizeResponse(response)

	// Strategy 1: Direct parse
	trimmed := strings.TrimSpace(sanitized)
	if json.Valid([]byte(trimmed)) {
		return trimmed, nil
	}

	// Strategy 2: Markdown code block extraction
	if extracted, ok := extractFromCodeBlock(sanitized); ok {
		if json.Valid([]byte(extracted)) {
			return extracted, nil
		}
	}

	// Strategy 3: First { to last }
	if extracted, ok := extractByBraces(sanitized); ok {
		if json.Valid([]byte(extracted)) {
			return extracted, nil
		}
	}

	return "", errors.New("no valid JSON found in response")
}

func extractFromCodeBlock(s string) (string, bool) {
	markers := []string{"```json\n", "```json\r\n", "```\n", "```\r\n"}
	for _, marker := range markers {
		start := strings.Index(s, marker)
		if start < 0 {
			continue
		}
		content := s[start+len(marker):]
		end := strings.Index(content, "```")
		if end < 0 {
			continue
		}
		return strings.TrimSpace(content[:end]), true
	}
	return "", false
}

func extractByBraces(s string) (string, bool) {
	start := strings.Index(s, "{")
	if start < 0 {
		return "", false
	}
	end := strings.LastIndex(s, "}")
	if end < 0 || end <= start {
		return "", false
	}
	return s[start : end+1], true
}
