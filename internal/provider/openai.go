package provider

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/mmdemirbas/mutercim/internal/apiclient"
)

// OpenAI API request/response types.

type openaiRequest struct {
	Model    string          `json:"model"`
	Messages []openaiMessage `json:"messages"`
}

type openaiMessage struct {
	Role    string          `json:"role"`
	Content []openaiContent `json:"content,omitempty"`
}

type openaiContent struct {
	Type     string          `json:"type"`
	Text     string          `json:"text,omitempty"`
	ImageURL *openaiImageURL `json:"image_url,omitempty"`
}

type openaiImageURL struct {
	URL string `json:"url"`
}

type openaiResponse struct {
	Choices []openaiChoice `json:"choices"`
}

type openaiChoice struct {
	Message openaiChoiceMessage `json:"message"`
}

type openaiChoiceMessage struct {
	Content string `json:"content"`
}

// OpenAICompatPresets maps provider names to their default base URLs.
// All these providers use the OpenAI-compatible chat completions API format.
var OpenAICompatPresets = map[string]string{
	"openai":     "https://api.openai.com",
	"groq":       "https://api.groq.com/openai",
	"mistral":    "https://api.mistral.ai",
	"openrouter": "https://openrouter.ai/api",
	"xai":        "https://api.x.ai",
}

// OpenAIProvider implements Provider for OpenAI-compatible APIs.
// Works with OpenAI, Groq, Mistral, OpenRouter, xAI, and any other
// service that implements the /v1/chat/completions endpoint.
type OpenAIProvider struct {
	client         *apiclient.Client
	apiKey         string
	model          string
	baseURL        string
	name           string
	supportsVision bool
}

const openaiDefaultBaseURL = "https://api.openai.com"

// NewOpenAIProvider creates a new OpenAI provider with default settings.
func NewOpenAIProvider(client *apiclient.Client, apiKey, model string) *OpenAIProvider {
	return NewOpenAICompatProvider(client, "openai", apiKey, model, openaiDefaultBaseURL, true)
}

// NewOpenAICompatProvider creates an OpenAI-compatible provider with a custom name and base URL.
func NewOpenAICompatProvider(client *apiclient.Client, name, apiKey, model, baseURL string, vision bool) *OpenAIProvider {
	return &OpenAIProvider{
		client:         client,
		apiKey:         apiKey,
		model:          model,
		baseURL:        baseURL,
		name:           name,
		supportsVision: vision,
	}
}

// Name returns the provider identifier.
func (o *OpenAIProvider) Name() string { return o.name }

// SupportsVision returns whether this provider can handle image inputs.
func (o *OpenAIProvider) SupportsVision() bool { return o.supportsVision }

// ReadFromImage sends an image to an OpenAI-compatible API and returns the text response.
func (o *OpenAIProvider) ReadFromImage(ctx context.Context, image []byte, systemPrompt, userPrompt string) (string, error) {
	dataURI := fmt.Sprintf("data:image/png;base64,%s", base64.StdEncoding.EncodeToString(image))

	body := openaiRequest{
		Model: o.model,
		Messages: []openaiMessage{
			{Role: "system", Content: []openaiContent{{Type: "text", Text: systemPrompt}}},
			{Role: "user", Content: []openaiContent{
				{Type: "image_url", ImageURL: &openaiImageURL{URL: dataURI}},
				{Type: "text", Text: userPrompt},
			}},
		},
	}

	resp, err := apiclient.DoJSON[openaiResponse](o.client, ctx, apiclient.Request{
		Method: "POST",
		URL:    o.baseURL + "/v1/chat/completions",
		Headers: map[string]string{
			"Content-Type":  "application/json",
			"Authorization": "Bearer " + o.apiKey,
		},
		Body: body,
	})
	if err != nil {
		return "", fmt.Errorf("%s read: %w", o.name, err)
	}
	return o.extractText(resp)
}

// Translate sends text to an OpenAI-compatible API and returns the text response.
func (o *OpenAIProvider) Translate(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	body := openaiRequest{
		Model: o.model,
		Messages: []openaiMessage{
			{Role: "system", Content: []openaiContent{{Type: "text", Text: systemPrompt}}},
			{Role: "user", Content: []openaiContent{{Type: "text", Text: userPrompt}}},
		},
	}

	resp, err := apiclient.DoJSON[openaiResponse](o.client, ctx, apiclient.Request{
		Method: "POST",
		URL:    o.baseURL + "/v1/chat/completions",
		Headers: map[string]string{
			"Content-Type":  "application/json",
			"Authorization": "Bearer " + o.apiKey,
		},
		Body: body,
	})
	if err != nil {
		return "", fmt.Errorf("%s translate: %w", o.name, err)
	}
	return o.extractText(resp)
}

func (o *OpenAIProvider) extractText(resp openaiResponse) (string, error) {
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("%s: no choices in response", o.name)
	}
	return resp.Choices[0].Message.Content, nil
}
