Read SPEC.md first. Follow its architecture, naming, and data structures exactly.
Do not deviate from the spec's package structure or interface definitions.

Read SPEC.md sections: "Phase 3: TRANSLATE", "Translation System Prompt".

Implement:
1. internal/translation/ — translator.go, prompts.go
2. internal/pipeline/translate.go
3. internal/cli/translate.go

The result should: take solved pages, translate via Gemini with knowledge-injected
prompts, save translated JSON, write per-page incremental output to output/turkish/pages/.

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

**Translation package** (`internal/translation/`):
- `prompts.go` — Translation system prompt template with placeholders for knowledge sections, section-aware hints, `BuildSystemPrompt()`, `BuildUserPrompt()`, `BuildContextSection()`
- `translator.go` — `Translator` struct: builds knowledge-injected prompts, calls provider, parses response into `TranslatedPage`, `PageSummary()` for sliding context window
- `translator_test.go` — Full translation with mock provider, context passing, page summary generation
- `prompts_test.go` — System prompt construction, section hints, context section building

**Pipeline** (`internal/pipeline/`):
- `translate.go` — Phase 3 orchestrator: discovers inputs from solved dir, sliding context window, per-page incremental markdown output to `output/turkish/pages/<stem>/`, progress tracking

**CLI** (`internal/cli/`):
- `translate.go` — `mutercim translate` subcommand with `--translate-provider`, `--translate-model`, `--context-window` flags

### Key Features

- **Knowledge-injected prompts**: Honorifics, companions, sources, terminology all injected into the system prompt
- **Sliding context window**: Previous N translated pages summarized and passed as context (configurable via `translate.context_window` or `--context-window`)
- **Section-aware translation**: Different prompt hints for scholarly_entries, prose, toc, index
- **Incremental output**: Per-page markdown files written immediately after each page translates
- **Section translate flag**: Pages with `translate: false` in sections config are skipped
