package cli

import (
	"os"
	"path/filepath"
	"testing"
)

//nolint:funlen // table-driven test with many .env parsing cases
func TestParseEnvLines(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    map[string]string
	}{
		{
			name:    "simple key=value",
			content: "FOO=bar",
			want:    map[string]string{"FOO": "bar"},
		},
		{
			name:    "export prefix",
			content: "export FOO=bar",
			want:    map[string]string{"FOO": "bar"},
		},
		{
			name:    "double quoted value",
			content: `FOO="bar baz"`,
			want:    map[string]string{"FOO": "bar baz"},
		},
		{
			name:    "single quoted value",
			content: `FOO='bar baz'`,
			want:    map[string]string{"FOO": "bar baz"},
		},
		{
			name:    "comment lines skipped",
			content: "# this is a comment\nFOO=bar",
			want:    map[string]string{"FOO": "bar"},
		},
		{
			name:    "blank lines skipped",
			content: "\n\nFOO=bar\n\n",
			want:    map[string]string{"FOO": "bar"},
		},
		{
			name:    "multiple entries",
			content: "A=1\nB=2\nC=3",
			want:    map[string]string{"A": "1", "B": "2", "C": "3"},
		},
		{
			name:    "value with equals sign",
			content: "FOO=bar=baz",
			want:    map[string]string{"FOO": "bar=baz"},
		},
		{
			name:    "empty value",
			content: "FOO=",
			want:    map[string]string{"FOO": ""},
		},
		{
			name:    "line without equals is skipped",
			content: "NOEQUALS",
			want:    map[string]string{},
		},
		{
			name:    "spaces around key and value",
			content: "  FOO  =  bar  ",
			want:    map[string]string{"FOO": "bar"},
		},
		{
			name:    "export with double quotes",
			content: `export API_KEY="sk-12345"`,
			want:    map[string]string{"API_KEY": "sk-12345"},
		},
		{
			name:    "mixed comments and values",
			content: "# comment\nFOO=bar\n# another comment\nBAZ=qux",
			want:    map[string]string{"FOO": "bar", "BAZ": "qux"},
		},
		{
			name:    "empty content",
			content: "",
			want:    map[string]string{},
		},
		{
			name:    "only comments",
			content: "# a\n# b",
			want:    map[string]string{},
		},
		{
			name:    "later value overrides earlier",
			content: "FOO=first\nFOO=second",
			want:    map[string]string{"FOO": "second"},
		},
		{
			name:    "unmatched quotes are not stripped",
			content: `FOO="bar`,
			want:    map[string]string{"FOO": `"bar`},
		},
		{
			name:    "inline comment stripped",
			content: "FOO=bar # this is a comment",
			want:    map[string]string{"FOO": "bar"},
		},
		{
			name:    "inline comment with hash inside double quotes preserved",
			content: `FOO="bar # not a comment"`,
			want:    map[string]string{"FOO": "bar # not a comment"},
		},
		{
			name:    "inline comment with hash inside single quotes preserved",
			content: `FOO='bar # not a comment'`,
			want:    map[string]string{"FOO": "bar # not a comment"},
		},
		{
			name:    "hash without preceding space is not an inline comment",
			content: "FOO=bar#baz",
			want:    map[string]string{"FOO": "bar#baz"},
		},
		{
			name:    "value with multiple inline comments",
			content: "FOO=bar baz # comment here",
			want:    map[string]string{"FOO": "bar baz"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseEnvLines(tt.content)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d entries, want %d: %v", len(got), len(tt.want), got)
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("key %q: got %q, want %q", k, got[k], v)
				}
			}
		})
	}
}

func TestLoadEnvFile(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte("TEST_MUTERCIM_LOAD=hello\n"), 0600); err != nil {
		t.Fatal(err)
	}

	// Ensure it's not set
	_ = os.Unsetenv("TEST_MUTERCIM_LOAD")

	loadEnvFile(envPath)

	got := os.Getenv("TEST_MUTERCIM_LOAD")
	if got != "hello" {
		t.Errorf("expected 'hello', got %q", got)
	}

	// Clean up
	_ = os.Unsetenv("TEST_MUTERCIM_LOAD")
}

func TestLoadEnvFileDoesNotOverride(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte("TEST_MUTERCIM_NOOVERRIDE=fromfile\n"), 0600); err != nil {
		t.Fatal(err)
	}

	// Set it first
	if err := os.Setenv("TEST_MUTERCIM_NOOVERRIDE", "fromenv"); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Unsetenv("TEST_MUTERCIM_NOOVERRIDE") }()

	loadEnvFile(envPath)

	got := os.Getenv("TEST_MUTERCIM_NOOVERRIDE")
	if got != "fromenv" {
		t.Errorf("expected 'fromenv' (not overridden), got %q", got)
	}
}

func TestLoadEnvFileMissing(t *testing.T) {
	// Should not panic or error
	loadEnvFile("/nonexistent/.env")
}

func TestClientTimeout(t *testing.T) {
	ollama := clientTimeout("ollama")
	if ollama.Minutes() < 5 {
		t.Errorf("ollama timeout should be >= 5min, got %v", ollama)
	}

	gemini := clientTimeout("gemini")
	if gemini.Seconds() != 120 {
		t.Errorf("gemini timeout should be 120s, got %v", gemini)
	}

	claude := clientTimeout("claude")
	if claude.Seconds() != 120 {
		t.Errorf("claude timeout should be 120s, got %v", claude)
	}
}

func TestResolveAPIKey(t *testing.T) {
	// Ollama doesn't need a key
	key, err := resolveAPIKey("ollama")
	if err != nil || key != "" {
		t.Errorf("ollama should return empty key, got %q, err: %v", key, err)
	}

	// Unknown provider
	_, err = resolveAPIKey("unknown")
	if err == nil {
		t.Error("expected error for unknown provider")
	}

	// Missing key
	_ = os.Unsetenv("GEMINI_API_KEY")
	_, err = resolveAPIKey("gemini")
	if err == nil {
		t.Error("expected error for missing GEMINI_API_KEY")
	}

	// Set key
	if err := os.Setenv("GEMINI_API_KEY", "test-key"); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Unsetenv("GEMINI_API_KEY") }()
	key, err = resolveAPIKey("gemini")
	if err != nil || key != "test-key" {
		t.Errorf("expected 'test-key', got %q, err: %v", key, err)
	}
}
