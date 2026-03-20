package apiclient

import "testing"

func TestExtractJSON_UnclosedCodeBlock(t *testing.T) {
	// Code block opened but never closed — should fall through to brace strategy
	input := "```json\n{\"key\": \"value\"}"
	got, err := ExtractJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != `{"key": "value"}` {
		t.Errorf("got %q, want %q", got, `{"key": "value"}`)
	}
}

func TestExtractJSON_TwoJSONObjects(t *testing.T) {
	// Two separate JSON objects — brace strategy takes first { to last }
	// which merges them into invalid JSON
	input := `{"a": 1} {"b": 2}`
	_, err := ExtractJSON(input)
	if err == nil {
		t.Error("expected error for two separate JSON objects")
	}
}

func TestExtractJSON_CodeBlockWithTrailingWhitespace(t *testing.T) {
	// Exact marker match: "```json\n" — trailing whitespace before newline won't match
	input := "```json  \n{\"key\": \"value\"}\n```"
	got, err := ExtractJSON(input)
	if err != nil {
		// Falls through to brace strategy
		t.Fatalf("unexpected error: %v", err)
	}
	if got != `{"key": "value"}` {
		t.Errorf("got %q", got)
	}
}

func TestExtractJSON_OnlyOpenBrace(t *testing.T) {
	input := "here is { but no close"
	_, err := ExtractJSON(input)
	if err == nil {
		t.Error("expected error for open brace without close")
	}
}

func TestExtractJSON_OnlyCloseBrace(t *testing.T) {
	input := "here is } but no open"
	_, err := ExtractJSON(input)
	if err == nil {
		t.Error("expected error for close brace without open")
	}
}

func TestExtractJSON_JSONArray(t *testing.T) {
	// JSON array (not object) — direct parse should work
	input := `[{"id": "r1"}]`
	got, err := ExtractJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != input {
		t.Errorf("got %q", got)
	}
}

func TestExtractJSON_CodeBlockWithWindowsLineEndings(t *testing.T) {
	input := "```json\r\n{\"key\": \"value\"}\r\n```"
	got, err := ExtractJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != `{"key": "value"}` {
		t.Errorf("got %q", got)
	}
}

func TestExtractJSON_MultipleInvisibleChars(t *testing.T) {
	// All invisible chars combined
	input := "\uFEFF\u200B\u200E{\"key\u2060\": \"val\u2028ue\"}\u200F"
	got, err := ExtractJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != `{"key": "value"}` {
		t.Errorf("got %q", got)
	}
}

func TestExtractJSON_MarkdownWithExplanation(t *testing.T) {
	input := `Here is the translation:

` + "```json\n" + `{
  "regions": [
    {"id": "r1", "translated_text": "hello"}
  ],
  "warnings": []
}
` + "```" + `

I translated the text above.`

	got, err := ExtractJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == "" {
		t.Error("expected non-empty JSON")
	}
}
