Read SPEC.md first. Follow its architecture, naming, and data structures exactly.
Do not deviate from the spec's package structure or interface definitions.

Read SPEC.md sections: "Phase 4: COMPILE", LaTeX Docker container.

Implement:
1. internal/renderer/ — renderer.go, markdown.go, latex.go, docx.go
2. internal/pipeline/compile.go
3. internal/cli/compile.go
4. docker/xelatex/Dockerfile
5. templates in defaults/templates/

The result should: generate Markdown and LaTeX from translated JSON.

## Completion Checklist

Before declaring this phase complete, execute these commands and verify they pass:

1. `go build ./...`
2. `go vet ./...`
3. `go test ./...`
4. List all files you created/modified and verify each exists in SPEC.md's project structure
5. If any file or pattern deviates from SPEC.md, append to DEVIATIONS.md
6. Show me the output of all three commands above
