package cli

import "testing"

func TestApiKeyEnvVar(t *testing.T) {
	tests := []struct {
		provider string
		want     string
	}{
		{"gemini", "GEMINI_API_KEY"},
		{"claude", "ANTHROPIC_API_KEY"},
		{"openai", "OPENAI_API_KEY"},
		{"groq", "GROQ_API_KEY"},
		{"mistral", "MISTRAL_API_KEY"},
		{"openrouter", "OPENROUTER_API_KEY"},
		{"xai", "XAI_API_KEY"},
		{"ollama", ""},
		{"surya", ""},
		{"custom", "CUSTOM_API_KEY"},
	}
	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			got := apiKeyEnvVar(tt.provider)
			if got != tt.want {
				t.Errorf("apiKeyEnvVar(%q) = %q, want %q", tt.provider, got, tt.want)
			}
		})
	}
}
