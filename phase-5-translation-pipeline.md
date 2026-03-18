Read SPEC.md first. Follow its architecture, naming, and data structures exactly.
Do not deviate from the spec's package structure or interface definitions.

Read SPEC.md sections: "Phase 3: TRANSLATE", "Translation System Prompt".

Implement:
1. internal/translation/ — translator.go, prompts.go
2. internal/pipeline/translate.go
3. internal/cli/translate.go

The result should: take enriched pages, translate via Gemini with knowledge-injected 
prompts, save translated JSON, write per-page incremental output to output/turkish/pages/.

## Completion Checklist

Before declaring this phase complete, execute these commands and verify they pass:

1. `go build ./...`
2. `go vet ./...`
3. `go test ./...`
4. List all files you created/modified and verify each exists in SPEC.md's project structure
5. If any file or pattern deviates from SPEC.md, append to DEVIATIONS.md
6. Show me the output of all three commands above
