# Contributing

## Setup

```bash
git clone https://github.com/mmdemirbas/mutercim.git
cd mutercim
go build ./...
```

Requirements: Go 1.23+, Docker (for integration tests and all external tools).

## Development workflow

This project uses [Task](https://taskfile.dev) for automation:

```bash
task build    # build for current platform
task vet      # static analysis
task test     # tests + race detector
task all      # build + vet + test + install
```

After every change:

```bash
go build ./...
go vet ./...
go test ./...
```

All three must pass before submitting.

## Project conventions

- **CLAUDE.md** — agent instructions, coding rules, build discipline
- **docs/DECISIONS.md** — spec overrides (source of truth over docs/SPEC.md)
- **docs/GO-CONVENTIONS.md** — Go patterns (error handling, testing, entry points)

Key rules:

- Every new function with logic must have tests
- Table-driven tests, `httptest.NewServer` for HTTP, `t.TempDir()` for file I/O
- `log/slog` for logging, `net/http` for HTTP, minimal dependencies
- No `init()` functions, no global mutable state, no `panic`/`os.Exit` outside `main.go`
- All external tools run in Docker containers (no host-installed pdftoppm, pandoc, etc.)
- If you make a design choice that differs from SPEC.md, add an entry to docs/DECISIONS.md

## Config schema

If you change `internal/config/config.go`, regenerate the JSON schema:

```bash
task schema
```

## Docker images

All external tools run in Docker containers. Images are auto-built on first use,
or you can pre-build them:

```bash
task docker-all
```

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
