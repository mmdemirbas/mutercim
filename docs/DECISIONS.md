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
- Auto-extracted knowledge lives in memory/ (no staging/promote workflow)
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

## Write Phase Partial Failure Resilience
- Each output format (md, latex, pdf, docx) is attempted independently
- Tool checks (docker for pdf, pandoc for docx) happen just-in-time per format, not at startup
- If one format fails, logged as WARN and remaining formats continue
- Exit error only if ALL requested formats fail; partial success = exit 0
- Preflight checks removed from cli/write.go and cli/make.go (moved into pipeline)
- Summary logged: `wrote: [md latex], failed: [docx (pandoc not found)]`

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

## Remove Sections System
- Removed entire sections config block, section type definitions, section lookup, and section-aware prompt selection
- Every page is now AI-auto-detected with a general read prompt
- Users who want to skip pages use --pages flag
- Removed `ExtractToMemory()` (reference_table auto-extraction) from solve phase
- Removed section translate flag check from translate phase
- No section types, no section hints, no section config anywhere in the codebase

## Remove Knowledge Subcommand
- Removed entire `mutercim knowledge` command (list, staged, diff, promote)
- Removed `ListStagedFiles()` and `PromoteStagedFile()` from workspace package
- Renamed "staged" layer to "memory" throughout (loader, model, tests)
- Renamed `StageFromReferenceTable()` to `ExtractToMemory()` in solver package
- Simplified mental model: knowledge/ is user-edited, memory/ is tool-written (auto-extracted during solve)
- To move knowledge from memory/ to knowledge/, user copies files manually
- `mutercim clean memory` resets what the tool learned

## Region-Based Pipeline (v2.0 schema — all phases)
- All phases now use region-based schema exclusively. Old ReadPage/SolvedPage/TranslatedPage types removed.
- Read phase: outputs RegionPage (regions with bbox, type, style, reading_order)
- Solve phase: outputs SolvedRegionPage (RegionPage + glossary_context, previous_page_summary, validation_warnings)
- Translate phase: outputs TranslatedRegionPage (regions with original_text + translated_text, source/target lang)
- Write phase: renders from TranslatedRegionPage — markdown (# header, entries, > footnotes, --- separators), LaTeX (with RTL support)
- Region types: header, entry, footnote, separator, page_number, column_header, table, image, margin_note, other
- Two read strategies (auto-selected): "local+ai" (Surya Docker + AI) or "ai-only" (AI detects everything)
- Config field `read.layout_tool: "surya"` enables Docker-based layout detection
- Translation prompt lists regions in reading order; AI returns [{id, translated_text}] per region
- Solver validates region structure (empty text, reading order consistency) and injects glossary context
- Removed: ReadPage, SolvedPage, TranslatedPage, Entry, Footnote, Header, compat layer, abbreviation resolver, continuation detector

## Unified Glossary Format
- All knowledge types (honorifics, people, places, terms, sources) replaced with a single `Entry` type using ISO 639-1 language code keys
- Values can be string or []string — normalized to []string on load; first item is canonical, rest are variants
- "note" is the only non-language field (optional guidance for AI)
- Any .yaml file in knowledge/ and memory/ directories is loaded and merged; filenames are irrelevant
- `GlossaryForPair(source, target)` returns entries containing both languages
- Prompt formatting: `source (also: variants) → target (also: variants) — note`
- Solver stores matched canonical source forms in GlossaryContext (language-independent); translator formats per target language
- Translation prompt uses single GLOSSARY section instead of separate HONORIFIC/PEOPLE/SOURCES/TERMINOLOGY sections
- `mutercim init` scaffolds knowledge/glossary.yaml with commented format examples
- Embedded defaults in single defaults/glossary.yaml with ar+tr+en entries where available
- Removed: Honorific, Source, Person, Term, Place types; type-specific YAML schemas; per-type loaders; LookupSource; per-section prompt builders

## Config Validation in Load
- `Load()` now calls `Validate()` before returning — invalid page ranges in config are caught at load time

## ExpandPages Bounds Check
- `ExpandPages()` returns `([]int, error)` and caps at 100,000 pages (`MaxExpandedPages`)
- Prevents unbounded memory allocation from huge ranges like "1-10000000"

## Stability and Hardening Fixes
- Solver memory: `solveOneInput` loads pages on-demand instead of pre-loading all into `allPages` map
- Failover errors: `tryProviders` accumulates all provider errors via `errors.Join` instead of keeping only the last
- Code block extraction: tolerates trailing whitespace after ``` markers (e.g. ```` ```json  \n ````)
- Atomic writes: `atomicWriteFile` removes destination before rename for Windows compatibility
- UTF-8 truncation: `truncateOutput` uses `[]rune` slicing to avoid corrupting multi-byte characters
- HTTP pooling: `NewClient` sets `MaxIdleConnsPerHost=100` to improve connection reuse
- Title length: `SanitizeTitle` truncates to 80 runes to stay within 255-byte filesystem limits
- .env parser: handles inline comments (`KEY=value # comment`) for unquoted values
- Surya Docker: mounts parent directory instead of single file (fixes macOS Docker + AppArmor)
- API key redaction: error response bodies are scrubbed for `sk-*` and `AIza*` patterns before storage

## Gemini Review Fixes (Triage Batch)
- Pages >999: `listPageFiles` uses `%d.json` (no width limit); `pageFilename()` dynamically pads based on total page count
- LaTeX build collision: `buildDir` includes `stem` subdirectory to isolate multi-input builds
- Timer leak: retry loop uses `time.NewTimer` + explicit `Stop()` instead of `time.After`
- Failed page loads: `failed++` incremented before `continue` in solve/translate load errors
- Context window: `recentTranslated` trimmed to `contextWindow` size after each append (prevents OOM)
- URL redaction: fail-closed — returns `"<malformed-url-redacted>"` on parse error
- Rebuild caching: `NewestMtime` skips non-existent paths instead of forcing rebuild
- Tmp cleanup: `defer os.Remove(tmpPath)` in `atomicWriteFile` prevents orphan files
- TOCTOU race: `WalkDir` callback ignores `os.ErrNotExist` for files deleted mid-walk
- Write skip: checks ALL requested formats (not just .md) before skipping write phase
- Docker abs path: `CompilePDF` converts `latexDir` to absolute path before Docker mount
- Clean.go: uses `filepath.Join` instead of hardcoded `/` for cross-platform path building
- Knowledge merge: field-by-field merge preserves notes from earlier layers when override has no note
- Init input: uses `bufio.Reader` instead of `fmt.Scanln` to capture titles with spaces
- Polyglossia: `\textarabic{...}` replaced with `\begin{Arabic}...\end{Arabic}` to handle paragraph breaks
- Image extensions: case-insensitive check (`strings.ToLower`) accepts `.PNG`, `.JPG`
- Failover clock: `now` captured per-provider inside loop, not once before loop
- Page extraction: `contiguousRanges` groups pages into ranges; pdftoppm called per range (no wasted I/O)
- Log clean: truncates active log file instead of deleting (safe on Windows)
- Tashkeel: `stripTashkeel` removes Arabic diacriticals before glossary matching (vowelized text matches unvowelized terms)
- Page images: pdftoppm output renamed from `page-NNN.png` to `NNN.png` (consistent with all subsequent phases)

## DocLayout-YOLO as Default Layout Tool
- Added DocLayout-YOLO as a layout detection tool alongside Surya
- DocLayout-YOLO detects document-level structure (headers, columns, tables, footnotes) vs Surya's text-line detection
- Default `read.layout_tool` changed from `""` (AI-only) to `"doclayout-yolo"`
- Config accepts three values: `"doclayout-yolo"` (default), `"surya"`, `""` (AI-only)
- DocLayout-YOLO outputs regions with bbox + type (NO text); Surya outputs regions with bbox + text
- Docker image: `mutercim/doclayout-yolo:latest` with pre-downloaded model from HuggingFace
- BBox format conversion: DocLayout-YOLO outputs [x1,y1,x2,y2]; converted to [x,y,w,h] in Go code
- Regions sorted by reading order (RTL: top-to-bottom, right-to-left within rows)
- Confidence threshold: 0.2 (regions below this are filtered out)
- `layout.NewTool(name)` factory replaces inline `if/else` in CLI code
- Fixed `make.go` (all command): layout tool was not being passed to read phase
- User prompt updated to include `type=` field alongside `bbox=` for layout-detected regions
- `"abandon"` type from DocLayout-YOLO is silently skipped (artifacts/noise)

## Docker-Only External Dependencies
- All external tools (pdftoppm, pandoc, DocLayout-YOLO, XeLaTeX, Surya) run in Docker containers
- Docker is the single external runtime dependency. No host packages required besides Go and Docker
- Images are auto-built on first use from Dockerfiles in `docker/` via `internal/docker.EnsureImage()`
- New Docker images: `mutercim/poppler` (poppler-utils for pdftoppm), `mutercim/pandoc` (pandoc for DOCX)
- Removed: `CheckPdftoppm()`, `CheckPandoc()`, `CheckDocker()` — replaced by `docker.CheckAvailable()`
- `docker.CheckAvailable()` verifies Docker is installed and daemon is running (single check at startup)
- `docker.FindDockerDir(tool)` discovers `docker/<tool>/` relative to cwd or executable for auto-build

## Remove Built-in Glossary
- Removed `go:embed` of `internal/knowledge/defaults/glossary.yaml` — no built-in glossary in the binary
- Glossary moved to `config/glossary.yaml` as a reference example file users can copy
- Knowledge loads from two layers only: workspace `knowledge/` + auto-extracted `memory/`
- `mutercim init` scaffolds `knowledge/glossary.yaml` with format examples and a pointer to `config/glossary.yaml`
- Smaller binary, clearer separation: tool knows no domain-specific terms, users provide all knowledge

## Rename knowledge_dir to knowledge, accept multiple paths
- Config field renamed: `knowledge_dir` (string) → `knowledge` ([]string)
- Each path can be a directory (all .yaml/.yml files loaded) or a single YAML file
- Schema mismatches in individual files are logged as warnings and skipped, not fatal errors
- Default: `["./knowledge"]` (backward compatible for directory-only usage)
- All pipeline phases updated to pass resolved knowledge paths for mtime rebuild checks

## Config restructure — phase-aligned schema

- Removed `book` section (title, author, source_langs, target_langs)
- `source_langs` moved to per-input item as `languages`
- `target_langs` moved under `translate` as `languages`
- Added top-level `pages` section with `dpi`
- Output filename fixed to 'book' in write phase, no more `book.title`
- Removed `latex_docker_image` — hardcoded like other Docker images
- retry/rate_limit are now per-phase (read, solve, translate), not global

## Layout as independent phase

- Layout detection extracted from read phase into its own pipeline phase
- Pipeline: pages → layout → read → solve → translate → write
- Layout output: bounding boxes with type, confidence, raw_class — no text
- Read phase works with or without layout data (graceful degradation)
- Config: `layout.tool`, `layout.params`, `layout.debug` — separate from read
- Layout tool tuning params (confidence, iou, image_size, max_det) exposed for experimentation
- Debug overlay images moved from read to layout phase
- `mutercim layout` runs standalone; `mutercim clean layout+` cascades correctly

## Phase rename: pages → cut

The "pages" phase renamed to "cut" to maintain alphabetical ordering with the new "layout" phase.

## OCR as independent phase

OCR extracted into its own pipeline phase between layout and read.
Pipeline: cut → layout → ocr → read → solve → translate → write (C, L, O, R, S, T, W).

OCR tools (like layout tools) are specialized single-purpose engines, not LLM providers.
Interface: internal/ocr/Tool with Start/Stop/RecognizeRegions/RecognizeFullPage.

First implementation: Qari-OCR (NAMAA-Space/Qari-OCR-0.2.2.1-VL-2B-Instruct).
2B parameter Arabic OCR model, 8-bit quantized, runs locally in Docker as persistent HTTP server.
State-of-the-art Arabic diacritics (tashkeel) recognition: 0.061 CER.

When OCR is enabled, the read phase switches to text-only LLM (no vision needed).
When OCR is disabled, the read phase falls back to vision-LLM-does-everything.
Four degradation paths (layout±ocr combinations) all produce the same read output schema.