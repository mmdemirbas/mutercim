package apiclient

import "testing"

func Test_sanitizeResponse(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "clean input",
			input:    `{"key": "value"}`,
			expected: `{"key": "value"}`,
		},
		{
			name:     "zero-width space",
			input:    "{\"\u200Bkey\": \"value\"}",
			expected: `{"key": "value"}`,
		},
		{
			name:     "BOM",
			input:    "\uFEFF{\"key\": \"value\"}",
			expected: `{"key": "value"}`,
		},
		{
			name:     "preserves ZWNJ for Arabic",
			input:    "{\"key\": \"val\u200Cue\"}",
			expected: "{\"key\": \"val\u200Cue\"}",
		},
		{
			name:     "preserves ZWJ for Arabic",
			input:    "{\"key\": \"val\u200Due\"}",
			expected: "{\"key\": \"val\u200Due\"}",
		},
		{
			name:     "strips LTR mark",
			input:    "{\"\u200Ekey\": \"value\"}",
			expected: `{"key": "value"}`,
		},
		{
			name:     "strips RTL mark",
			input:    "{\"\u200Fkey\": \"value\"}",
			expected: `{"key": "value"}`,
		},
		{
			name:     "strips word joiner",
			input:    "{\"key\u2060\": \"value\"}",
			expected: `{"key": "value"}`,
		},
		{
			name:     "strips line separator",
			input:    "{\"key\": \"val\u2028ue\"}",
			expected: `{"key": "value"}`,
		},
		{
			name:     "strips paragraph separator",
			input:    "{\"key\": \"val\u2029ue\"}",
			expected: `{"key": "value"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeResponse(tt.input)
			if got != tt.expected {
				t.Errorf("sanitizeResponse() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  string
		expectErr bool
	}{
		{
			name:     "direct JSON",
			input:    `{"key": "value"}`,
			expected: `{"key": "value"}`,
		},
		{
			name:     "direct JSON with whitespace",
			input:    "  {\"key\": \"value\"}  \n",
			expected: `{"key": "value"}`,
		},
		{
			name:     "JSON in markdown json code block",
			input:    "```json\n{\"key\": \"value\"}\n```",
			expected: `{"key": "value"}`,
		},
		{
			name:     "JSON in plain code block",
			input:    "```\n{\"key\": \"value\"}\n```",
			expected: `{"key": "value"}`,
		},
		{
			name:     "JSON with surrounding text",
			input:    "Here is the result:\n{\"key\": \"value\"}\nDone.",
			expected: `{"key": "value"}`,
		},
		{
			name:     "JSON with BOM",
			input:    "\uFEFF{\"key\": \"value\"}",
			expected: `{"key": "value"}`,
		},
		{
			name:      "no JSON",
			input:     "This is just text with no braces.",
			expectErr: true,
		},
		{
			name:      "empty input",
			input:     "",
			expectErr: true,
		},
		{
			name:      "invalid JSON in braces",
			input:     "Here is {invalid json content}",
			expectErr: true,
		},
		{
			name:     "nested JSON objects",
			input:    "Result: {\"page\": {\"entries\": [{\"num\": 1}]}}",
			expected: `{"page": {"entries": [{"num": 1}]}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractJSON(tt.input)
			if tt.expectErr {
				if err == nil {
					t.Errorf("ExtractJSON() expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Errorf("ExtractJSON() unexpected error: %v", err)
				return
			}
			if got != tt.expected {
				t.Errorf("ExtractJSON() = %q, want %q", got, tt.expected)
			}
		})
	}
}
