Read SPEC.md first. Follow its architecture, naming, and data structures exactly.
Do not deviate from the spec's package structure or interface definitions.

Read SPEC.md sections: "Phase 1: EXTRACT", "Extraction System Prompt", 
"System Dependency Validation".

Implement:
1. internal/input/ — loader.go, pdf.go (with preflight checks)
2. internal/extraction/ — extractor.go, prompts.go (section-aware)
3. internal/pipeline/extract.go — orchestrator
4. internal/cli/extract.go — subcommand wiring

The result should: take the sample page image, run extraction via Gemini,
save structured JSON to cache/extracted/page_001.json, and update progress.json.

After Phase 3: Run against your actual sample page. Review the extracted JSON. This is where you'll tune the extraction prompt — the code is just plumbing, the prompt is where quality lives.

After Phase 3, you'll have a working extraction pipeline. Stop and evaluate quality on 5-10 real pages before continuing. If Gemini's extraction is poor (garbled tashkeel, missed footnote separation), the translation phases will amplify those errors. Better to tune extraction prompts early than debug bad translations later.

## Completion Checklist

Before declaring this phase complete, execute these commands and verify they pass:

1. `go build ./...`
2. `go vet ./...`
3. `go test ./...`
4. List all files you created/modified and verify each exists in SPEC.md's project structure
5. If any file or pattern deviates from SPEC.md, append to DEVIATIONS.md
6. Show me the output of all three commands above
7. Run extraction on the sample page image and print the resulting JSON to stdout
8. Verify the JSON contains: page_number, section_type, entries with hadith numbers, footnotes with source_codes
