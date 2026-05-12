package provider

import "context"

// Provider abstracts AI model interaction for both vision (reading) and text (translation).
type Provider interface {
	// Name returns the provider identifier (e.g., "gemini", "claude", "ollama").
	Name() string

	// ReadFromImage sends an image to a vision model with a system prompt
	// and returns the model's text response (expected to be JSON).
	ReadFromImage(ctx context.Context, image []byte, systemPrompt string, userPrompt string) (string, error)

	// Translate sends text to a language model with a system prompt
	// and returns the model's text response (expected to be JSON).
	Translate(ctx context.Context, systemPrompt string, userPrompt string) (string, error)

	// SupportsVision returns true if this provider can handle image inputs.
	SupportsVision() bool
}

// Class identifies whether a provider sends requests to a third-party
// hosted API (Cloud) or runs entirely under the user's control (Local).
// The distinction drives the local-first allow_cloud gate in the
// failover-chain builder.
type Class string

// Provider class constants. Unknown names map to ClassUnknown; the
// failover-chain builder treats unknown as cloud-equivalent under the
// default-deny posture.
const (
	ClassCloud   Class = "cloud"
	ClassLocal   Class = "local"
	ClassUnknown Class = "unknown"
)

// providerClasses maps provider name to class. Single source of truth.
// Adding a new local provider (llama.cpp / mlx-lm in Phase 3) requires
// an entry here so the allow_cloud gate lets it run under default-deny.
var providerClasses = map[string]Class{
	// Cloud — third-party hosted APIs requiring an API key.
	"gemini":     ClassCloud,
	"claude":     ClassCloud,
	"openai":     ClassCloud,
	"groq":       ClassCloud,
	"mistral":    ClassCloud,
	"openrouter": ClassCloud,
	"xai":        ClassCloud,
	// Local — user-controlled inference.
	"ollama": ClassLocal,
}

// ClassFor returns the class for the given provider name. Unknown names
// map to ClassUnknown; callers must decide policy for them.
func ClassFor(name string) Class {
	if c, ok := providerClasses[name]; ok {
		return c
	}
	return ClassUnknown
}
