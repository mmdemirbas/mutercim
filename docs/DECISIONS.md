# Decisions & Overrides

Anything here overrides SPEC.md. The codebase is the source of truth.

## CLI Command Names
- extract → read
- enrich → solve
- translate → translate
- compile → write
- run → make

## Directory Names
- cache/ → midstate/

## Gemini Model
- Default: gemini-2.5-flash-lite (not gemini-2.0-flash)
- Rate limit: 10 RPM (not 14)

## Shared Progress Renderer
- Live dashboard (TTYDisplay) and `status` command share rendering via `internal/display/render.go`
- Both use `RenderProgressLine()` and `RenderWarnErrorLine()` with `ProgressRow` data
- `PhaseRow` removed; `ProgressRow` used everywhere (includes optional Rate/ETA/Elapsed fields)
- Color/NO_COLOR handling unified via `StatusColors` in render.go

## Live Dashboard Log Tail
- Ring buffer (size 5) in `internal/display/ring.go` stores recent page summaries
- Each `Update()` pushes a one-line log entry; rendered below progress bars as "recent" section
- Log entries show HH:MM:SS timestamps, ⚠ for warnings, ✗ for errors, truncated at ~80 chars

## Backoff Calculation
- Removed 429-specific 60s minimum backoff; all retryable errors use exponential backoff (2s, 4s, 8s)
- Retry-After header is respected but capped at 30s max (maxRetryAfter constant)
- Retry log line includes `backoff_seconds` for debuggability

## Pipeline Halt on Zero Results
- Read, Solve, Translate return `PhaseResult` (Completed/Failed/Skipped counts) alongside error
- `make` command checks `result.Completed == 0` after each phase and halts with clear log message
- Individual CLI commands (read, solve, translate) ignore the result counts

## URL Redaction
- `sanitizeURL` renamed to exported `RedactURL` in apiclient package
- Strips query params containing "key", "token", or "secret" (case-insensitive)

## Shared Header Rendering
- `HeaderData` struct + `RenderHeader()` used by both status command and live dashboard
- `SetHeader(HeaderData)` added to Display interface; TTYDisplay includes header in every render cycle
- Header labels use `%6s:` format for colon alignment (Book/Input/Langs colons at same column)
- Page range shown as "pages X-Y of N" when configured, otherwise "N pages"