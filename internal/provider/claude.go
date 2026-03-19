package provider

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/mmdemirbas/mutercim/internal/apiclient"
)

// Claude API request/response types.

type claudeRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	System    string          `json:"system"`
	Messages  []claudeMessage `json:"messages"`
}

type claudeMessage struct {
	Role    string          `json:"role"`
	Content []claudeContent `json:"content"`
}

type claudeContent struct {
	Type   string             `json:"type"`
	Text   string             `json:"text,omitempty"`
	Source *claudeImageSource `json:"source,omitempty"`
}

type claudeImageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

type claudeResponse struct {
	Content []claudeResponseContent `json:"content"`
}

type claudeResponseContent struct {
	Text string `json:"text"`
}

// ClaudeProvider implements Provider for Anthropic Claude.
type ClaudeProvider struct {
	client  *apiclient.Client
	apiKey  string
	model   string
	baseURL string
}

const claudeDefaultBaseURL = "https://api.anthropic.com"

// NewClaudeProvider creates a new Claude provider.
func NewClaudeProvider(client *apiclient.Client, apiKey, model string) *ClaudeProvider {
	return &ClaudeProvider{
		client:  client,
		apiKey:  apiKey,
		model:   model,
		baseURL: claudeDefaultBaseURL,
	}
}

// Name returns "claude".
func (c *ClaudeProvider) Name() string { return "claude" }

// SupportsVision returns true because Claude supports image inputs.
func (c *ClaudeProvider) SupportsVision() bool { return true }

// ExtractFromImage sends an image to Claude and returns the text response.
func (c *ClaudeProvider) ExtractFromImage(ctx context.Context, image []byte, systemPrompt, userPrompt string) (string, error) {
	body := claudeRequest{
		Model:     c.model,
		MaxTokens: 4096,
		System:    systemPrompt,
		Messages: []claudeMessage{{
			Role: "user",
			Content: []claudeContent{
				{Type: "image", Source: &claudeImageSource{Type: "base64", MediaType: "image/png", Data: base64.StdEncoding.EncodeToString(image)}},
				{Type: "text", Text: userPrompt},
			},
		}},
	}

	resp, err := apiclient.DoJSON[claudeResponse](c.client, ctx, apiclient.Request{
		Method: "POST",
		URL:    c.baseURL + "/v1/messages",
		Headers: map[string]string{
			"Content-Type":      "application/json",
			"x-api-key":         c.apiKey,
			"anthropic-version": "2023-06-01",
		},
		Body: body,
	})
	if err != nil {
		return "", fmt.Errorf("claude extract: %w", err)
	}
	return c.extractText(resp)
}

// Translate sends text to Claude and returns the text response.
func (c *ClaudeProvider) Translate(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	body := claudeRequest{
		Model:     c.model,
		MaxTokens: 4096,
		System:    systemPrompt,
		Messages: []claudeMessage{{
			Role: "user",
			Content: []claudeContent{
				{Type: "text", Text: userPrompt},
			},
		}},
	}

	resp, err := apiclient.DoJSON[claudeResponse](c.client, ctx, apiclient.Request{
		Method: "POST",
		URL:    c.baseURL + "/v1/messages",
		Headers: map[string]string{
			"Content-Type":      "application/json",
			"x-api-key":         c.apiKey,
			"anthropic-version": "2023-06-01",
		},
		Body: body,
	})
	if err != nil {
		return "", fmt.Errorf("claude translate: %w", err)
	}
	return c.extractText(resp)
}

func (c *ClaudeProvider) extractText(resp claudeResponse) (string, error) {
	if len(resp.Content) == 0 {
		return "", fmt.Errorf("claude: no content in response")
	}
	return resp.Content[0].Text, nil
}
