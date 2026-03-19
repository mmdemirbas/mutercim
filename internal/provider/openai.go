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

// OpenAIProvider implements Provider for OpenAI.
type OpenAIProvider struct {
	client  *apiclient.Client
	apiKey  string
	model   string
	baseURL string
}

const openaiDefaultBaseURL = "https://api.openai.com"

// NewOpenAIProvider creates a new OpenAI provider.
func NewOpenAIProvider(client *apiclient.Client, apiKey, model string) *OpenAIProvider {
	return &OpenAIProvider{
		client:  client,
		apiKey:  apiKey,
		model:   model,
		baseURL: openaiDefaultBaseURL,
	}
}

// Name returns "openai".
func (o *OpenAIProvider) Name() string { return "openai" }

// SupportsVision returns true because OpenAI vision models support image inputs.
func (o *OpenAIProvider) SupportsVision() bool { return true }

// ExtractFromImage sends an image to OpenAI and returns the text response.
func (o *OpenAIProvider) ExtractFromImage(ctx context.Context, image []byte, systemPrompt, userPrompt string) (string, error) {
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
		return "", fmt.Errorf("openai extract: %w", err)
	}
	return o.extractText(resp)
}

// Translate sends text to OpenAI and returns the text response.
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
		return "", fmt.Errorf("openai translate: %w", err)
	}
	return o.extractText(resp)
}

func (o *OpenAIProvider) extractText(resp openaiResponse) (string, error) {
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("openai: no choices in response")
	}
	return resp.Choices[0].Message.Content, nil
}
