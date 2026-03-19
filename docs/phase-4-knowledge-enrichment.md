Read SPEC.md first. Follow its architecture, naming, and data structures exactly.
Do not deviate from the spec's package structure or interface definitions.

Read SPEC.md sections: "Knowledge Module Format", "Knowledge Layering",
"Staging Area", "Phase 2: SOLVE".

Implement:
1. internal/knowledge/defaults/ directory with embedded YAML files
2. internal/knowledge/ — loader.go, embedded.go, types.go, glossary.go
3. internal/workspace/staging.go
4. internal/solver/ — solver.go, abbreviation.go, continuation.go,
   validator.go, staging.go
5. internal/pipeline/solve.go
6. internal/cli/solve.go (for `solve` subcommand) + knowledge_cmd.go

The result should: load all three knowledge layers, run solve on
read pages, write solved JSON with source resolution and layer tracking,
auto-stage reference_table detections, and support `mutercim knowledge staged/promote`.

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

**Knowledge package** (`internal/knowledge/`):
- `types.go` — `Honorific`, `Source`, `Person`, `Term`, `Place`, `Knowledge` structs with `LookupSource()`
- `embedded.go` — `go:embed defaults` for built-in knowledge YAML files
- `loader.go` — Three-layer loading (embedded → workspace → staged) with merge-by-key logic
- `glossary.go` — `BuildGlossary()` and per-section builders for prompt injection
- `internal/knowledge/defaults/` — Embedded honorifics, people, terminology, places YAML files
- `loader_test.go` — Embedded loading, workspace overrides, staged overrides, layer tracking

**Solver package** (`internal/solver/`):
- `solver.go` — `Solver` orchestrator: abbreviation resolution, continuation detection, validation, translation context building
- `abbreviation.go` — Resolves source codes from footnotes against knowledge, tracks layer provenance
- `continuation.go` — Detects cross-page continuations (`continues_from`/`continues_on`)
- `validator.go` — Validates hadith number sequences, flags empty types/text
- `staging.go` — Auto-stages knowledge from `reference_table` pages to `midstate/staged/`
- Tests for all four modules

**Workspace** (`internal/workspace/`):
- `staging.go` — `ListStagedFiles()`, `PromoteStagedFile()` with atomic copy

**Pipeline** (`internal/pipeline/`):
- `solve.go` — Phase 2 orchestrator: discovers inputs from read dir, loads pages, solves with cross-page context, saves solved JSON, tracks progress

**CLI** (`internal/cli/`):
- `solve.go` — `mutercim solve` subcommand
- `knowledge_cmd.go` — `mutercim knowledge list|staged|promote` subcommand group

### Deviations

- Embedded YAML files live only in `internal/knowledge/defaults/` (go:embed requires files within package directory)
