package provider

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/mmdemirbas/mutercim/internal/apiclient"
)

const geminiDefaultBaseURL = "https://generativelanguage.googleapis.com"

// geminiRequest is the Gemini API request body.
type geminiRequest struct {
	Contents          []geminiContent `json:"contents"`
	SystemInstruction *geminiContent  `json:"systemInstruction,omitempty"`
	GenerationConfig  geminiGenConfig `json:"generationConfig"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text       string            `json:"text,omitempty"`
	InlineData *geminiInlineData `json:"inlineData,omitempty"`
}

type geminiInlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

type geminiGenConfig struct {
	Temperature      float64 `json:"temperature"`
	ResponseMIMEType string  `json:"responseMimeType,omitempty"`
}

// geminiResponse is the Gemini API response envelope.
type geminiResponse struct {
	Candidates []geminiCandidate `json:"candidates"`
}

type geminiCandidate struct {
	Content geminiContent `json:"content"`
}

// GeminiProvider implements Provider for Google Gemini.
type GeminiProvider struct {
	client  *apiclient.Client
	apiKey  string
	model   string
	baseURL string
}

// NewGeminiProvider creates a new Gemini provider.
func NewGeminiProvider(client *apiclient.Client, apiKey, model string) *GeminiProvider {
	return &GeminiProvider{
		client:  client,
		apiKey:  apiKey,
		model:   model,
		baseURL: geminiDefaultBaseURL,
	}
}

// Name returns "gemini".
func (g *GeminiProvider) Name() string { return "gemini" }

// SupportsVision returns true because Gemini supports image inputs.
func (g *GeminiProvider) SupportsVision() bool { return true }

// ExtractFromImage sends an image to Gemini and returns the text response.
func (g *GeminiProvider) ExtractFromImage(ctx context.Context, image []byte, systemPrompt, userPrompt string) (string, error) {
	body := geminiRequest{
		Contents: []geminiContent{{
			Parts: []geminiPart{
				{InlineData: &geminiInlineData{MimeType: "image/png", Data: base64.StdEncoding.EncodeToString(image)}},
				{Text: userPrompt},
			},
		}},
		SystemInstruction: &geminiContent{Parts: []geminiPart{{Text: systemPrompt}}},
		GenerationConfig:  geminiGenConfig{Temperature: 0, ResponseMIMEType: "application/json"},
	}

	resp, err := apiclient.DoJSON[geminiResponse](g.client, ctx, apiclient.Request{
		Method:  "POST",
		URL:     g.endpoint(),
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    body,
	})
	if err != nil {
		return "", fmt.Errorf("gemini extract: %w", err)
	}
	return g.extractText(resp)
}

// Translate sends text to Gemini and returns the text response.
func (g *GeminiProvider) Translate(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	body := geminiRequest{
		Contents: []geminiContent{{
			Parts: []geminiPart{
				{Text: userPrompt},
			},
		}},
		SystemInstruction: &geminiContent{Parts: []geminiPart{{Text: systemPrompt}}},
		GenerationConfig:  geminiGenConfig{Temperature: 0, ResponseMIMEType: "application/json"},
	}

	resp, err := apiclient.DoJSON[geminiResponse](g.client, ctx, apiclient.Request{
		Method:  "POST",
		URL:     g.endpoint(),
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    body,
	})
	if err != nil {
		return "", fmt.Errorf("gemini translate: %w", err)
	}
	return g.extractText(resp)
}

func (g *GeminiProvider) endpoint() string {
	return fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s", g.baseURL, g.model, g.apiKey)
}

func (g *GeminiProvider) extractText(resp geminiResponse) (string, error) {
	if len(resp.Candidates) == 0 {
		return "", fmt.Errorf("gemini: no candidates in response")
	}
	if len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("gemini: no parts in response")
	}
	return resp.Candidates[0].Content.Parts[0].Text, nil
}
