Read SPEC.md first. Follow its architecture, naming, and data structures exactly.
Do not deviate from the spec's package structure or interface definitions.

Read SPEC.md sections: "Knowledge Module Format", "Knowledge Layering", 
"Staging Area", "Phase 2: ENRICH".

Implement:
1. defaults/ directory with embedded YAML files
2. internal/knowledge/ — loader.go, embedded.go, types.go, glossary.go
3. internal/workspace/staging.go
4. internal/enrichment/ — enricher.go, abbreviation.go, continuation.go, 
   validator.go, staging.go
5. internal/pipeline/enrich.go
6. internal/cli/enrich.go + knowledge_cmd.go

The result should: load all three knowledge layers, run enrichment on 
extracted pages, write enriched JSON with source resolution and layer tracking,
auto-stage reference_table extractions, and support `mutercim knowledge staged/promote`.

## Completion Checklist

Before declaring this phase complete, execute these commands and verify they pass:

1. `go build ./...`
2. `go vet ./...`
3. `go test ./...`
4. List all files you created/modified and verify each exists in SPEC.md's project structure
5. If any file or pattern deviates from SPEC.md, append to DEVIATIONS.md
6. Show me the output of all three commands above
