## Phase 1 — Foundation

- **What**: Changed module path from `github.com/muhammed/mutercim` to `github.com/mmdemirbas/mutercim`
- **Why**: Original SPEC.md had incorrect GitHub username. Fixed across go.mod and all Go source files during Phase 2.

## Phase 2 — apiclient + providers

- **What**: Added `baseURL` field to `GeminiProvider` struct (not in SPEC)
- **Why**: SPEC hardcodes the Gemini API URL in the provider, but CLAUDE.md requires tests to use `httptest.NewServer` instead of real API calls. The `baseURL` field defaults to the production URL and is only overridden in tests.
