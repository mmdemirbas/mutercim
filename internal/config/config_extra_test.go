package config

import "testing"

func TestResolvePath(t *testing.T) {
	cfg := &Config{}

	tests := []struct {
		base, rel string
		want      string
	}{
		{"/workspace", "./input", "/workspace/input"},
		{"/workspace", "/absolute/path", "/absolute/path"},
		{"/workspace", "relative", "/workspace/relative"},
	}
	for _, tt := range tests {
		got := cfg.ResolvePath(tt.base, tt.rel)
		if got != tt.want {
			t.Errorf("ResolvePath(%q, %q) = %q, want %q", tt.base, tt.rel, got, tt.want)
		}
	}
}

func TestValidatePages(t *testing.T) {
	cfg := &Config{Pages: "1-5,10"}
	if err := cfg.Validate(); err != nil {
		t.Errorf("valid pages should not error: %v", err)
	}

	cfg = &Config{Pages: "abc"}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for invalid pages")
	}
}
