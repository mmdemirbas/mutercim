Read SPEC.md first. Follow its architecture, naming, and data structures exactly.
Do not deviate from the spec's package structure or interface definitions.

Implement internal/cli/make.go (chains all phases as `make` command) and validate.go.
Add remaining providers (claude.go, openai.go, ollama.go, surya.go) — 
they follow the exact same pattern as gemini.go.

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

**Providers** (`internal/provider/`):
- `claude.go` — Anthropic Claude provider (vision + text, x-api-key auth, anthropic-version header)
- `openai.go` — OpenAI provider (vision + text, Bearer auth, chat completions API)
- `ollama.go` — Ollama local provider (vision + text, /api/generate, OLLAMA_HOST env var)
- `claude_test.go`, `openai_test.go`, `ollama_test.go` — httptest-based tests for each

**CLI** (`internal/cli/`):
- `make.go` — `mutercim make` chains all phases: read → solve → translate → write, with full preflight checks upfront
- `validate.go` — `mutercim validate` reads page JSONs, validates numbering sequences and structural consistency, reports warnings (read-only, no API calls)

### Files Modified

- `internal/cli/read.go` — `createProvider()` now supports gemini, claude, openai, ollama
- `internal/cli/root.go` — registered `make` and `validate` subcommands

### Provider Summary

| Provider | Auth | Vision | Endpoint |
|----------|------|--------|----------|
| gemini | GEMINI_API_KEY | yes | generativelanguage.googleapis.com |
| claude | ANTHROPIC_API_KEY | yes | api.anthropic.com/v1/messages |
| openai | OPENAI_API_KEY | yes | api.openai.com/v1/chat/completions |
| ollama | none (OLLAMA_HOST) | yes | localhost:11434/api/generate |
