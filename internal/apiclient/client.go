package apiclient

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ClientConfig configures a new API client.
type ClientConfig struct {
	Timeout           time.Duration // HTTP request timeout (default: 120s for vision calls)
	MaxRetries        int           // Max retry attempts (default: 3)
	BaseBackoff       time.Duration // Initial backoff duration (default: 2s)
	RequestsPerMinute int           // Rate limit (default: 14 for Gemini free)
}

// DefaultClientConfig returns a ClientConfig with sensible defaults.
func DefaultClientConfig() ClientConfig {
	return ClientConfig{
		Timeout:           120 * time.Second,
		MaxRetries:        3,
		BaseBackoff:       2 * time.Second,
		RequestsPerMinute: 14,
	}
}

// Client is a shared HTTP client with retry, rate limiting, and response parsing.
type Client struct {
	httpClient  *http.Client
	rateLimiter *RateLimiter
	maxRetries  int
	baseBackoff time.Duration
	logger      *slog.Logger
}

// NewClient creates a Client with the given configuration.
func NewClient(cfg ClientConfig, logger *slog.Logger) *Client {
	if logger == nil {
		logger = slog.Default()
	}
	return &Client{
		httpClient:  &http.Client{Timeout: cfg.Timeout},
		rateLimiter: NewRateLimiter(cfg.RequestsPerMinute),
		maxRetries:  cfg.MaxRetries,
		baseBackoff: cfg.BaseBackoff,
		logger:      logger,
	}
}

// Close releases resources held by the client.
func (c *Client) Close() {
	c.rateLimiter.Close()
}

// Request represents an AI API request.
type Request struct {
	Method  string            // HTTP method (always POST for AI APIs)
	URL     string            // Full endpoint URL
	Headers map[string]string // Auth headers, content-type, API version headers
	Body    any               // Will be JSON-marshaled
}

// HTTPError represents a non-2xx HTTP response.
type HTTPError struct {
	StatusCode int
	Status     string
	Body       []byte
	RetryAfter time.Duration
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Status)
}

// Do executes the request with rate limiting and retry logic.
// Returns the raw response body bytes.
// Retries on: 429 (rate limit), 500, 502, 503, 529 (overloaded).
// Does NOT retry on: 400 (bad request), 401 (auth), 403 (forbidden), 404.
func (c *Client) Do(ctx context.Context, req Request) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			backoff := c.calculateBackoff(attempt, lastErr)
			c.logger.Info("retrying request",
				"attempt", attempt,
				"backoff_seconds", backoff.Seconds(),
				"url", RedactURL(req.URL),
			)
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		if err := c.rateLimiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("rate limiter wait: %w", err)
		}

		body, err := c.doOnce(ctx, req)
		if err == nil {
			return body, nil
		}

		var httpErr *HTTPError
		if errors.As(err, &httpErr) {
			if !isRetryable(httpErr.StatusCode) {
				return nil, err
			}
			c.logger.Warn("retryable error",
				"status", httpErr.StatusCode,
				"body", truncate(string(httpErr.Body), 200),
			)
			lastErr = err
			continue
		}

		// Non-HTTP errors (network errors, etc.) are retryable
		lastErr = err
	}
	return nil, fmt.Errorf("max retries (%d) exceeded: %w", c.maxRetries, lastErr)
}

func (c *Client) doOnce(ctx context.Context, req Request) ([]byte, error) {
	var bodyReader io.Reader
	if req.Body != nil {
		bodyBytes, err := json.Marshal(req.Body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	httpReq, err := http.NewRequestWithContext(ctx, req.Method, req.URL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		httpErr := &HTTPError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Body:       respBody,
		}
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			if seconds, parseErr := strconv.Atoi(ra); parseErr == nil {
				httpErr.RetryAfter = time.Duration(seconds) * time.Second
			}
		}
		return nil, httpErr
	}

	return respBody, nil
}

// maxRetryAfter caps the Retry-After header value to prevent arbitrarily long waits.
const maxRetryAfter = 30 * time.Second

func (c *Client) calculateBackoff(attempt int, lastErr error) time.Duration {
	var httpErr *HTTPError
	if errors.As(lastErr, &httpErr) && httpErr.RetryAfter > 0 {
		// Respect Retry-After header but cap at maxRetryAfter
		if httpErr.RetryAfter > maxRetryAfter {
			return maxRetryAfter
		}
		return httpErr.RetryAfter
	}
	// Exponential backoff: base * 2^(attempt-1) with jitter (0.5x to 1.5x)
	backoff := c.baseBackoff * (1 << (attempt - 1))
	jitter := 0.5 + rand.Float64()
	return time.Duration(float64(backoff) * jitter)
}

// RedactURL strips sensitive query parameters (API keys, tokens, secrets) from URLs before logging.
func RedactURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	q := u.Query()
	for key := range q {
		lower := strings.ToLower(key)
		if strings.Contains(lower, "key") || strings.Contains(lower, "token") || strings.Contains(lower, "secret") {
			q.Set(key, "REDACTED")
		}
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func isRetryable(statusCode int) bool {
	switch statusCode {
	case 429, 500, 502, 503, 529:
		return true
	default:
		return false
	}
}

// DoJSON executes the request and unmarshals the response into the given type.
func DoJSON[T any](c *Client, ctx context.Context, req Request) (T, error) {
	var zero T
	body, err := c.Do(ctx, req)
	if err != nil {
		return zero, err
	}
	var result T
	if err := json.Unmarshal(body, &result); err != nil {
		return zero, fmt.Errorf("unmarshal response: %w", err)
	}
	return result, nil
}

// EncodeImageBase64 reads an image file and returns its base64 encoding and MIME type.
func EncodeImageBase64(imagePath string) (data string, mimeType string, err error) {
	raw, err := os.ReadFile(imagePath)
	if err != nil {
		return "", "", fmt.Errorf("read image %s: %w", imagePath, err)
	}

	ext := strings.ToLower(filepath.Ext(imagePath))
	mimeType = mime.TypeByExtension(ext)
	if mimeType == "" {
		switch ext {
		case ".png":
			mimeType = "image/png"
		case ".jpg", ".jpeg":
			mimeType = "image/jpeg"
		case ".gif":
			mimeType = "image/gif"
		case ".webp":
			mimeType = "image/webp"
		default:
			mimeType = "application/octet-stream"
		}
	}

	data = base64.StdEncoding.EncodeToString(raw)
	return data, mimeType, nil
}
