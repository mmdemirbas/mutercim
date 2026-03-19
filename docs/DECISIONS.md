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