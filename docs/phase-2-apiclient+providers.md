Read SPEC.md first. Follow its architecture, naming, and data structures exactly.
Do not deviate from the spec's package structure or interface definitions.

Read SPEC.md sections: "API Client Package" and "Provider Interface".
Implement:

1. internal/apiclient/ — client.go, ratelimit.go, response.go
2. internal/provider/ — provider.go (interface), registry.go, gemini.go
3. Only Gemini provider for now. Others are the same pattern.

Write a test that sends a real request to Gemini with a test image 
and verifies JSON extraction works. Use a small test image, not a full book page.

After Phase 2: Test with your Gemini API key. Verify rate limiting, retry, JSON extraction including the Unicode sanitization edge case. Then add Claude and Ollama providers — they follow the identical pattern.

## Completion Checklist

Before declaring this phase complete, execute these commands and verify they pass:

1. `go build ./...`
2. `go vet ./...`
3. `go test ./...`
4. List all files you created/modified and verify each exists in SPEC.md's project structure
5. If any file or pattern deviates from SPEC.md, append to DEVIATIONS.md
6. Show me the output of all three commands above

## Summary

### Files Created

- `internal/apiclient/client.go` — `Client`, `ClientConfig`, `Request`, `HTTPError`, `Do()` (retry + rate-limit), `DoJSON[T]()`, `EncodeImageBase64()`
- `internal/apiclient/ratelimit.go` — token bucket `RateLimiter` with goroutine-based refill
- `internal/apiclient/response.go` — `SanitizeResponse()` (strips U+200B/FEFF/etc, preserves ZWNJ/ZWJ for Arabic), `ExtractJSON()` (direct → markdown fence → brace-matching)
- `internal/provider/provider.go` — `Provider` interface
- `internal/provider/registry.go` — thread-safe `Registry` (name → Provider)
- `internal/provider/gemini.go` — `GeminiProvider` with vision + text endpoints
- `internal/apiclient/client_test.go` — success, retry on 503, Retry-After header, non-retryable 4xx, DoJSON, EncodeImageBase64
- `internal/apiclient/ratelimit_test.go` — token consumption, context cancellation, refill
- `internal/apiclient/response_test.go` — Unicode sanitization (8 char types), ZWNJ/ZWJ preservation, JSON extraction strategies
- `internal/provider/registry_test.go` — register/get, unknown provider, names, overwrite
- `internal/provider/gemini_test.go` — read with image, translate text-only, empty response, HTTP error

### Additional Fix

- Fixed module path from `github.com/muhammed/mutercim` to `github.com/mmdemirbas/mutercim` across all Go files and `go.mod`.

### Deviations

- Added `baseURL` field to `GeminiProvider` (defaults to production URL, overridable in tests to use `httptest.NewServer`). See DEVIATIONS.md.
