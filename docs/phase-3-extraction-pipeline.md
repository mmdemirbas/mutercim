Read SPEC.md first. Follow its architecture, naming, and data structures exactly.
Do not deviate from the spec's package structure or interface definitions.

Read SPEC.md sections: "Phase 1: READ", "Read System Prompt",
"System Dependency Validation".

Implement:
1. internal/input/ — loader.go, pdf.go (with preflight checks)
2. internal/reader/ — reader.go, prompts.go (section-aware)
3. internal/pipeline/read.go — orchestrator
4. internal/cli/read.go — `read` subcommand wiring

The result should: take the sample page image, run read via Gemini,
save structured JSON to midstate/read/page_001.json, and update progress.json.

After Phase 3: Run against your actual sample page. Review the read JSON. This is where you'll tune the read prompt — the code is just plumbing, the prompt is where quality lives.

After Phase 3, you'll have a working read pipeline. Stop and evaluate quality on 5-10 real pages before continuing. If Gemini's reading is poor (garbled tashkeel, missed footnote separation), the translation phases will amplify those errors. Better to tune read prompts early than debug bad translations later.

## Completion Checklist

Before declaring this phase complete, execute these commands and verify they pass:

1. `go build ./...`
2. `go vet ./...`
3. `go test ./...`
4. List all files you created/modified and verify each exists in SPEC.md's project structure
5. If any file or pattern deviates from SPEC.md, append to DEVIATIONS.md
6. Show me the output of all three commands above
7. Run read on the sample page image and print the resulting JSON to stdout
8. Verify the JSON contains: page_number, section_type, entries with hadith numbers, footnotes with source_codes

## Summary

### Files Created

- `internal/input/loader.go` — `ListImages()` (scans directory for page images, parses page numbers from filenames), `LoadImage()`, `PageImage` struct
- `internal/input/pdf.go` — `ConvertPDFToImages()` (shells out to pdftoppm), `CheckPdftoppm()` preflight check
- `internal/reader/prompts.go` — Read system prompt from SPEC, `SectionHint()` for section-aware prompt context, `BuildUserPrompt()`
- `internal/reader/reader.go` — `Reader` struct using `Provider` interface, `ReadPage()` (calls provider, parses JSON, builds `ReadPage`)
- `internal/pipeline/read.go` — Phase 1 orchestrator: PDF conversion, image listing, section lookup, per-page reading with progress tracking, atomic JSON output
- `internal/cli/read.go` — `read` subcommand with flags (`--read-provider`, `--read-model`, `--concurrency`, `--dpi`), API key resolution, provider creation

### Test Files

- `internal/input/loader_test.go` — image listing (various naming patterns, empty/nonexistent dirs, JPEG support), page number parsing, image loading
- `internal/reader/reader_test.go` — full read with Arabic text, null page number fallback, markdown code block response, provider error handling
- `internal/reader/prompts_test.go` — user prompt generation per section type, section hint coverage
- `internal/pipeline/read_test.go` — end-to-end pipeline (mock provider → JSON output + progress update), skip-completed logic, no-images error, atomic save verification

### Deviations

- Preflight check in `input/pdf.go` instead of `workspace/preflight.go` (co-location with pdftoppm code)
- Provider creation via switch in CLI instead of registry (sufficient for single-provider phase)
