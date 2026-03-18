# Project: mutercim

## Source of Truth

docs/SPEC.md is the authoritative architecture document. Read it before implementing anything. Follow
its package structure, interface definitions, data structures, and naming exactly. Do not invent
alternative patterns.

docs/GO-CONVENTIONS.md contains code-level Go conventions (error handling, testing patterns, entry
point structure). Follow these for implementation style. If SPEC.md and GO-CONVENTIONS.md conflict,
SPEC.md wins.

## Build Discipline

After implementing any phase or making any significant change:

1. Run `go build ./...` — fix all compilation errors before proceeding
2. Run `go vet ./...` — fix all warnings before proceeding
3. Run `go test ./...` — fix all test failures before proceeding

Do NOT proceed to the next task if any of these fail. Fix the issue first.

## Deviation Tracking

If you make a design choice that differs from SPEC.md (different function signature, renamed
package, added/removed a field, changed a data flow), append a short entry to docs/DEVIATIONS.md
explaining what changed and why. Format:

```
## Phase N — <date or description>
- **What**: Changed X from SPEC to Y
- **Why**: <one sentence reason>
```

Create DEVIATIONS.md if it doesn't exist.

## Code Style

- Use `log/slog` for all logging (Go stdlib, no external logging library)
- Use `errors.New` and `fmt.Errorf` with `%w` for error wrapping — no custom error libraries
- Use Go stdlib `net/http` for HTTP — no external HTTP client libraries
- Minimal dependencies: only cobra, viper, yaml.v3 as specified in SPEC.md
- All exported types and functions must have doc comments
- No `init()` functions anywhere
- No global mutable state
- Context propagation: all functions that do I/O take `context.Context` as first parameter

## Testing

- Write table-driven tests for any parsing logic (page ranges, config merging, JSON extraction,
  section lookup)
- Write at least one test per public function in `apiclient/`, `config/`, `knowledge/`, `progress/`
- Tests for provider implementations can use a local HTTP test server (`httptest.NewServer`) — do
  not make real API calls in tests
- Name test files `*_test.go` in the same package (not `_test` package)

## File Writes

- All state files (progress.json, cache JSONs, staged knowledge YAMLs) must use atomic write: write
  to `.tmp` then `os.Rename`
- Per-page output files (markdown, translated JSON) should be written immediately after processing
  each page, not batched

## Error Handling

- Never silently swallow errors. Either return them, log them, or both.
- Pipeline phases (extract, enrich, translate, compile) must not abort on single-page failures. Log
  the error, save partial/raw data, record the failure in progress.json, continue to next page.
- System dependency checks (pdftoppm, docker, pandoc) happen at command startup via preflight, not
  lazily on first use.

## What NOT To Do

- Do not add dependencies not listed in SPEC.md without asking
- Do not create files outside the package structure defined in SPEC.md
- Do not use `interface{}` or `any` for typed data — use the concrete model types from
  `internal/model/`
- Do not use `panic` or `os.Exit` outside of `main.go`
- Do not write Python scripts, shell scripts, or Makefiles that duplicate Go logic
- Do not embed large string literals for prompts inline in function bodies — use `prompts.go` files
  as specified

## After Completion of Each Phase

- Add summary of the changes to the end of the relevant phase file.
