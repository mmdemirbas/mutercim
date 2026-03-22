package cli

import "testing"

func TestDetectVision(t *testing.T) {
	tests := []struct {
		provider string
		model    string
		want     bool
	}{
		// Native providers always have vision
		{"gemini", "gemini-2.5-flash-lite", true},
		{"claude", "claude-sonnet-4-20250514", true},
		{"openai", "gpt-4o", true},
		{"ollama", "qwen2.5vl:7b", true},

		// OpenAI-compatible: vision patterns in model name
		{"groq", "llama-3.2-90b-vision-preview", true},
		{"openrouter", "qwen/qwen2.5-vl-72b-instruct:free", true},
		{"groq", "meta-llama/llama-4-scout-17b-16e-instruct", true},
		{"mistral", "pixtral-large-latest", true},
		{"openrouter", "google/gemma-3-27b-it:free", true},
		{"xai", "grok-2-vision-latest", true},

		// OpenAI-compatible: no vision patterns
		{"groq", "llama-3.3-70b-versatile", false},
		{"mistral", "mistral-small-latest", false},
		{"openrouter", "meta-llama/llama-3.3-70b-instruct:free", false},
		{"xai", "grok-2-latest", false},

		// Unknown provider defaults to pattern matching
		{"custom", "my-vision-model", true},
		{"custom", "my-text-model", false},
	}

	for _, tt := range tests {
		t.Run(tt.provider+"/"+tt.model, func(t *testing.T) {
			got := detectVision(tt.provider, tt.model)
			if got != tt.want {
				t.Errorf("detectVision(%q, %q) = %v, want %v", tt.provider, tt.model, got, tt.want)
			}
		})
	}
}
