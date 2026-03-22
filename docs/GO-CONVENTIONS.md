# Go Conventions for This Project

> Extracted from project-wide Go standards. Filtered for relevance to mutercim.
> DECISIONS.md covers architectural decisions. This file covers code-level conventions only.

## Entry Point Pattern

`cmd/mutercim/main.go` uses the testable entry point pattern:

```go
func main() {
os.Exit(run())
}

func run() int {
// All setup, flag parsing, execution here
// Defers execute properly because we're not in main()
// Return 0 for success, 1 for failure
}
```

Only `main()` calls `os.Exit`. No `os.Exit` or `log.Fatal` anywhere else in the codebase.

## Error Handling

- Wrap errors with context: `fmt.Errorf("read page %d: %w", pageNum, err)`
- Error messages include enough to diagnose: what failed, what input, what was expected
- User-visible errors are actionable ("pdftoppm not found in PATH — install with: brew install
  poppler")
- Errors go to stderr, data output goes to stdout. Never mix.
- Never swallow errors silently. Every `err` is checked or explicitly documented as intentional.
- Return errors from library code — never `log.Fatal` or `panic` outside main.go.

## Resource Management

- `defer` for cleanup of file handles, HTTP response bodies, goroutines
- Log errors from `Close()` on write operations:
  ```go
  defer func() {
      if err := f.Close(); err != nil {
          slog.Error("close file", "path", path, "err", err)
      }
  }()
  ```
- Every goroutine has a clear exit path tied to context cancellation or channel close
- `context.AfterFunc` for cleanup. Always `defer cancel()`.

## State & Concurrency

- No global mutable state. Pass dependencies explicitly.
- If shared state is unavoidable: mutex-protected with a `Snapshot()` copy method for reads.
- Cancellation support via `context.Context` as first parameter on all I/O functions.
- Graceful shutdown: `context.WithCancel` + OS signal handling + `sync.WaitGroup`.

## Constants & Naming

- Package-level constants over magic numbers (`maxRetries`, `defaultDPI`, `defaultRPM`)
- Field names in serialized structs are final — typos in JSON/YAML tags live forever. Double-check.
- Comments explain the **why**, not the **what**.

## Testing

- Table-driven tests with descriptive name field (not raw input as name):
  ```go
  tests := []struct {
      name     string
      input    string
      expected int
  }{
      {name: "single page", input: "5", expected: 5},
      {name: "range", input: "1-10", expected: 10},
      {name: "empty", input: "", expected: 0},
  }
  ```
- `t.Helper()` on all test helper functions
- `t.TempDir()` for throwaway test directories
- `t.Setenv()` for environment-dependent tests
- `httptest.NewServer` for HTTP client tests — never real API calls in tests
- No `time.Sleep` in tests — use channels, `sync.WaitGroup`, or `atomic`

### Edge Cases to Always Test

- Empty input, nil, zero values
- Single item, maximum items
- Boundary values, off-by-one
- Malformed input, partial matches
- What should **not** match (negative cases)

## Performance (apply only where profiling shows a bottleneck)

- `strings.Builder` over string concatenation
- No `fmt.Sprintf` in tight loops — pre-compute
- Profile first (`go test -cpuprofile`), then optimize. Intuition about bottlenecks is usually
  wrong.
- Benchmark before AND after: `go test -bench=. -benchmem`

## Common Mistakes

| Mistake                          | Fix                                         |
|----------------------------------|---------------------------------------------|
| Goroutine leak on context cancel | Always `defer cancel()`                     |
| `time.Sleep` in tests            | Channels or `sync.WaitGroup`                |
| `log.Fatal` in library code      | Return errors — only `main()` exits         |
| Ignoring `err` from `Close()`    | Log errors for write operations             |
| `fmt.Sprintf` in hot loops       | Pre-compute or `strings.Builder`            |
| Adding deps stdlib handles       | Logging, HTTP, JSON, testing — stdlib first |
| Comments restating the code      | Comment the **why**                         |
| Optimizing without profiling     | Profile → benchmark → fix the right thing   |

## Build

- `CGO_ENABLED=0` for static binaries
- Version injected via `-ldflags="-s -w -X main.version=$(git describe --tags)"`
- Cross-platform awareness: `filepath.Separator`, line ending handling