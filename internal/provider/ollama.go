package provider

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"

	"github.com/mmdemirbas/mutercim/internal/apiclient"
)

// Ollama /api/chat request/response types.

type ollamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
}

type ollamaMessage struct {
	Role    string   `json:"role"`
	Content string   `json:"content"`
	Images  []string `json:"images,omitempty"` // base64 encoded, sibling of content
}

type ollamaChatResponse struct {
	Message ollamaMessage `json:"message"`
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

// ReadFromImage sends an image to Ollama via /api/chat and returns the text response.
func (o *OllamaProvider) ReadFromImage(ctx context.Context, image []byte, systemPrompt, userPrompt string) (string, error) {
	body := ollamaChatRequest{
		Model: o.model,
		Messages: []ollamaMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt, Images: []string{base64.StdEncoding.EncodeToString(image)}},
		},
		Stream: false,
	}

	resp, err := apiclient.DoJSON[ollamaChatResponse](o.client, ctx, apiclient.Request{
		Method:  "POST",
		URL:     o.baseURL + "/api/chat",
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    body,
	})
	if err != nil {
		return "", fmt.Errorf("%s read: %w", o.Name(), err)
	}
	return resp.Message.Content, nil
}

// Translate sends text to Ollama via /api/chat and returns the text response.
func (o *OllamaProvider) Translate(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	body := ollamaChatRequest{
		Model: o.model,
		Messages: []ollamaMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Stream: false,
	}

	resp, err := apiclient.DoJSON[ollamaChatResponse](o.client, ctx, apiclient.Request{
		Method:  "POST",
		URL:     o.baseURL + "/api/chat",
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    body,
	})
	if err != nil {
		return "", fmt.Errorf("%s translate: %w", o.Name(), err)
	}
	return resp.Message.Content, nil
}
