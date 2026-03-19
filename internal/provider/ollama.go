package provider

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"

	"github.com/mmdemirbas/mutercim/internal/apiclient"
)

// Ollama API request/response types.

type ollamaRequest struct {
	Model  string   `json:"model"`
	Prompt string   `json:"prompt"`
	System string   `json:"system,omitempty"`
	Images []string `json:"images,omitempty"` // base64 encoded
	Stream bool     `json:"stream"`
}

type ollamaResponse struct {
	Response string `json:"response"`
}

// OllamaProvider implements Provider for local Ollama.
type OllamaProvider struct {
	client  *apiclient.Client
	model   string
	baseURL string
}

// NewOllamaProvider creates a new Ollama provider.
// The host defaults to http://localhost:11434, overridable via OLLAMA_HOST.
func NewOllamaProvider(client *apiclient.Client, model string) *OllamaProvider {
	host := os.Getenv("OLLAMA_HOST")
	if host == "" {
		host = "http://localhost:11434"
	}
	return &OllamaProvider{
		client:  client,
		model:   model,
		baseURL: host,
	}
}

// Name returns "ollama".
func (o *OllamaProvider) Name() string { return "ollama" }

// SupportsVision returns true because Ollama vision models support image inputs.
func (o *OllamaProvider) SupportsVision() bool { return true }

// ExtractFromImage sends an image to Ollama and returns the text response.
func (o *OllamaProvider) ExtractFromImage(ctx context.Context, image []byte, systemPrompt, userPrompt string) (string, error) {
	body := ollamaRequest{
		Model:  o.model,
		Prompt: userPrompt,
		System: systemPrompt,
		Images: []string{base64.StdEncoding.EncodeToString(image)},
		Stream: false,
	}

	resp, err := apiclient.DoJSON[ollamaResponse](o.client, ctx, apiclient.Request{
		Method:  "POST",
		URL:     o.baseURL + "/api/generate",
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    body,
	})
	if err != nil {
		return "", fmt.Errorf("ollama extract: %w", err)
	}
	return resp.Response, nil
}

// Translate sends text to Ollama and returns the text response.
func (o *OllamaProvider) Translate(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	body := ollamaRequest{
		Model:  o.model,
		Prompt: userPrompt,
		System: systemPrompt,
		Stream: false,
	}

	resp, err := apiclient.DoJSON[ollamaResponse](o.client, ctx, apiclient.Request{
		Method:  "POST",
		URL:     o.baseURL + "/api/generate",
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    body,
	})
	if err != nil {
		return "", fmt.Errorf("ollama translate: %w", err)
	}
	return resp.Response, nil
}
