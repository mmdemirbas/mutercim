package provider

import "testing"

func TestClassFor(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		want     Class
	}{
		// Cloud providers — third-party hosted APIs.
		{"gemini-cloud", "gemini", ClassCloud},
		{"claude-cloud", "claude", ClassCloud},
		{"openai-cloud", "openai", ClassCloud},
		{"groq-cloud", "groq", ClassCloud},
		{"mistral-cloud", "mistral", ClassCloud},
		{"openrouter-cloud", "openrouter", ClassCloud},
		{"xai-cloud", "xai", ClassCloud},
		// Local providers — user-controlled inference.
		{"ollama-local", "ollama", ClassLocal},
		// Unknown — must NOT be silently classified. Caller decides policy
		// (default-deny treats unknown as cloud-equivalent).
		{"unknown-empty", "", ClassUnknown},
		{"unknown-custom", "custom-provider", ClassUnknown},
		{"unknown-future-llamacpp", "llamacpp", ClassUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassFor(tt.provider)
			if got != tt.want {
				t.Errorf("ClassFor(%q) = %q, want %q", tt.provider, got, tt.want)
			}
		})
	}
}
