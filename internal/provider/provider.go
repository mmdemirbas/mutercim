package provider

import "context"

// Provider abstracts AI model interaction for both vision (extraction) and text (translation).
type Provider interface {
	// Name returns the provider identifier (e.g., "gemini", "claude", "ollama").
	Name() string

	// ExtractFromImage sends an image to a vision model with a system prompt
	// and returns the model's text response (expected to be JSON).
	ExtractFromImage(ctx context.Context, image []byte, systemPrompt string, userPrompt string) (string, error)

	// Translate sends text to a language model with a system prompt
	// and returns the model's text response (expected to be JSON).
	Translate(ctx context.Context, systemPrompt string, userPrompt string) (string, error)

	// SupportsVision returns true if this provider can handle image inputs.
	SupportsVision() bool
}
