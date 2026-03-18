Read SPEC.md first. Follow its architecture, naming, and data structures exactly.
Do not deviate from the spec's package structure or interface definitions.

Read SPEC.md thoroughly. Implement in this order:

1. go.mod, go.sum (module: github.com/muhammed/mutercim)
2. internal/model/ — all data structures (page.go, book.go, entry.go, section.go)
3. internal/config/ — config loading with section support (config.go, sections.go)
4. internal/workspace/ — workspace discovery, init scaffolding, path resolution
5. internal/progress/ — tracker with atomic writes
6. cmd/mutercim/main.go + internal/cli/root.go + init.go + config_cmd.go + status.go

Do NOT implement pipeline phases, providers, or renderers yet.
The result should: compile, run `mutercim init` to scaffold a workspace, 
run `mutercim config` to print effective config, and run `mutercim status` 
to show empty progress. Write tests for config loading and section page-range parsing.

After Phase 1: Run it. Verify mutercim init creates the right directory structure, config loads and merges correctly, section page-range parsing works. Fix anything broken before moving on.

## Completion Checklist

Before declaring this phase complete, execute these commands and verify they pass:

1. `go build ./...`
2. `go vet ./...`
3. `go test ./...`
4. List all files you created/modified and verify each exists in SPEC.md's project structure
5. If any file or pattern deviates from SPEC.md, append to DEVIATIONS.md
6. Show me the output of all three commands above
