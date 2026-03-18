## Phase 1 — Foundation

- **What**: Changed module path from `github.com/muhammed/mutercim` to `github.com/mmdemirbas/mutercim`
- **Why**: Original SPEC.md had incorrect GitHub username. Fixed across go.mod and all Go source files during Phase 2.

## Phase 2 — apiclient + providers

- **What**: Added `baseURL` field to `GeminiProvider` struct (not in SPEC)
- **Why**: SPEC hardcodes the Gemini API URL in the provider, but CLAUDE.md requires tests to use `httptest.NewServer` instead of real API calls. The `baseURL` field defaults to the production URL and is only overridden in tests.

## Phase 3 — Extraction Pipeline

- **What**: Preflight check (`CheckPdftoppm`) placed in `internal/input/pdf.go` instead of `internal/workspace/preflight.go`
- **Why**: SPEC shows preflight in workspace package, but the check is specific to PDF input handling and co-locates better with the pdftoppm conversion code. The CLI extract command calls it directly.

- **What**: `resolveAPIKey` and `createProvider` helper functions in `internal/cli/extract.go` instead of using `provider/registry.go`
- **Why**: Phase 3 only implements Gemini. A simple switch in the CLI is sufficient. The registry pattern from Phase 2 is available for later phases when all providers are wired up.
