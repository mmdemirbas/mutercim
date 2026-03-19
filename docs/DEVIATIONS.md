## Phase 1 — Foundation

- **What**: Changed module path from `github.com/muhammed/mutercim` to `github.com/mmdemirbas/mutercim`
- **Why**: Original SPEC.md had incorrect GitHub username. Fixed across go.mod and all Go source files during Phase 2.

## Phase 2 — apiclient + providers

- **What**: Added `baseURL` field to `GeminiProvider` struct (not in SPEC)
- **Why**: SPEC hardcodes the Gemini API URL in the provider, but CLAUDE.md requires tests to use `httptest.NewServer` instead of real API calls. The `baseURL` field defaults to the production URL and is only overridden in tests.

## Phase 3 — Read Pipeline

- **What**: Preflight check (`CheckPdftoppm`) placed in `internal/input/pdf.go` instead of `internal/workspace/preflight.go`
- **Why**: SPEC shows preflight in workspace package, but the check is specific to PDF input handling and co-locates better with the pdftoppm conversion code. The CLI read command calls it directly.

- **What**: `resolveAPIKey` and `createProvider` helper functions in `internal/cli/read.go` instead of using `provider/registry.go`
- **Why**: Phase 3 only implements Gemini. A simple switch in the CLI is sufficient. The registry pattern from Phase 2 is available for later phases when all providers are wired up.

- **What**: Added `Inputs []string` and `Pages string` fields to Config (SPEC has `Input string` only)
- **Why**: Support multiple input PDF files and config-based page ranges so the user can define everything in `mutercim.yaml` without CLI flags. Old `input:` (singular) still works via migration in `applyDefaults`.

- **What**: Read pipeline uses per-input subdirectories (`midstate/images/<stem>/`, `midstate/read/<stem>/`) and compound progress phase names (`"read:<stem>"`)
- **Why**: Multiple inputs would have conflicting page numbers (both PDFs have page 1). Per-input namespacing avoids conflicts in both file output and progress tracking.

## Phase 4 — Knowledge & Solve

- **What**: Embedded default YAML files live only in `internal/knowledge/defaults/` (single source of truth)
- **Why**: `go:embed` can only access files within or below the package directory. Former top-level `defaults/` was a redundant copy; moved to `example/defaults/` as reference only.
