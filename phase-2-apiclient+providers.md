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
