# Decisions & Overrides

Anything here overrides SPEC.md. The codebase is the source of truth.

## CLI Command Names
- extract → read
- enrich → solve
- translate → translate
- compile → write
- run → make

## Workspace Directory Layout (replaces midstate/ and output/)
- Removed midstate/ — phase directories are now top-level: pages/, read/, solve/, translate/
- Removed output/ — renamed to write/
- Log file moved from workspace root to log/mutercim.log
- Staged knowledge moved from midstate/staged/ to memory/
- Removed `midstate_dir` and `output` config fields (directories are now fixed)
- Per-page markdown output removed from translate phase (translations live in translate/)

## Write Output Structure
- write/{lang}/{title}.md, .tex, .pdf, .docx — all deliverables at same level per language
- latex-build/ subdirectory holds compilation artifacts (.aux, .log, .out)
- {title} derived from book.title via SanitizeTitle: replaces OS-prohibited chars, trims, falls back to "book"
- Unicode preserved — Arabic, Turkish, Chinese titles stay as-is

## Clean Command
- `mutercim clean <targets...>` deletes generated directories and resets progress
- Targets: log, memory, pages, read, solve, translate, write, all
- "+" suffix cascades downstream: `clean read+` → read/ solve/ translate/ write/
- Never deletes input/ or knowledge/
- Prints sizes before deleting, resets progress.json entries for cleaned phases

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

## Model Failover Chains
- `ReadConfig` and `TranslateConfig` gain `Models []ModelSpec` field (ordered failover list)
- Legacy `provider`/`model` fields migrated to single-element `Models` list (backward compatible)
- `FailoverChain` in `provider/failover.go` implements `Provider` interface, wraps multiple providers
- On 429/quota: marks model exhausted, fails over to next in chain. 60s rolling recovery window
- Non-vision models skipped for read phase; usable for translate
- Each model gets its own `apiclient.Client` + `RateLimiter` (per-model RPM)

## OpenAI-Compatible Provider
- `openai.go` refactored to generic `NewOpenAICompatProvider(name, baseURL, vision)`
- `OpenAICompatPresets` maps provider names to base URLs: openai, groq, mistral, openrouter, xai
- All use the same `/v1/chat/completions` endpoint with Bearer auth
- `NewOpenAIProvider()` preserved as convenience wrapper

## Per-Provider Rate Limits
- Default RPMs: gemini=10, groq=30, mistral=60, openrouter=200, openai=500, ollama=1000
- Overridable per-model via `rpm` field in `ModelSpec`
- Global `rate_limit` config kept for backward compat but per-model RPM takes precedence

## Validate Merged into Status
- Removed standalone `validate` CLI command; validation now runs as part of `mutercim status`
- Core validation logic (`solver/validator.go`) unchanged — still used by solve phase
- Status collects per-page validation warnings + cross-page entry number gap checks
- Deleted `cli/validate.go` and `pipeline/export.go` (DiscoverSubdirs wrapper, no longer needed)

## Smart Timestamp-Based Rebuild (replaces progress.json)
- Every phase compares output mtime against ALL its input mtimes via `rebuild.NeedsRebuild()`
- If any input is newer than the output → regenerate. Otherwise skip.
- Directory inputs include the directory's own mtime (catches file additions/deletions)
- Missing output → always rebuild. Missing input → error (treated as rebuild needed).
- Per-phase dependencies:
  - pages: input PDF → pages/{input}/ images
  - read: page image + mutercim.yaml + knowledge/ → read/{input}/NNN.json
  - solve: read JSON + knowledge/ + memory/ → solve/{input}/NNN.json
  - translate: solve JSON + mutercim.yaml + knowledge/ + memory/ → translate/{input}/{lang}/NNN.json
  - write: translate dir + mutercim.yaml + knowledge/ → write/{lang}/{title}.md etc.
- `--force` flag bypasses all timestamp checks, reprocesses everything
- progress.json removed entirely — filesystem mtimes are the source of truth

## On-Demand Output Formats (positional args)
- `write` and `make` accept positional format arguments: `mutercim write pdf`, `mutercim make md docx`
- Positional args override config; `--format` flag is kept as an alternative
- `tex` is an alias for `latex`; unknown formats are rejected with a clear error
- Deduplication: `tex latex` resolves to a single `latex`

## Auto-Run Prerequisites (--auto flag)
- `--auto` persistent flag on root command, available to all subcommands
- When set, each phase checks for missing prerequisite output and runs earlier phases automatically
- Finds first missing phase and runs from there through the target phase's prerequisites
- Detection: `hasPhaseOutput` checks if midstate directories have entries
- Phases: pages → read → solve → translate → write (each depends on the previous)

## PDF as Output Format (replaces skip_pdf)
- `pdf` is now a first-class output format (default: `[md, pdf]`)
- `pdf` implies LaTeX generation + PDF compilation via Docker
- `latex` format generates only `.tex` without compiling
- Removed `skip_pdf` config field and `--skip-pdf` CLI flag

## Arabic RTL and Letter Shaping in LaTeX/PDF
- LaTeXRenderer accepts `Lang` field to configure language-aware preamble
- Arabic-primary output (`Lang: "ar"`): `\setmainlanguage[numerals=maghrib]{arabic}`, no `\textarabic{}` wrappers
- Non-Arabic output (Turkish, English): `\setmainlanguage{turkish}`, Arabic content wrapped in `\textarabic{}`
- Arabic font: `\newfontfamily\arabicfont[Script=Arabic,Scale=1.2]{Amiri}` (via polyglossia)
- Mixed content: translated header as `\section*{}`, original Arabic header in `\textarabic{}` below it
- Original Arabic entry text included via `\textarabic{}` after each translated entry
- Docker image includes: polyglossia, bidi, fontspec, amiri, etoolbox

## Live Status Line During API Calls
- `StatusLine` type + `SetStatus(StatusLine)` added to Display interface
- Shows "→ reading page N via provider/model ... Xs" below the active phase bar
- 1-second ticker goroutine in TTYDisplay re-renders elapsed time live
- Countdown mode for backoff waits: shows remaining seconds ticking down
- `FormatStatusLine()` in render.go handles both elapsed and countdown formatting
- `Client.OnRetry` callback in apiclient: called before retry backoff wait
- `FailoverChain.OnFailover` callback + `SetRetryCallback()`: propagates retry callbacks to all clients
- Pipeline wires callbacks via type assertion on `*provider.FailoverChain`
- Status cleared automatically when API call completes (pipeline calls `SetStatus({})`)