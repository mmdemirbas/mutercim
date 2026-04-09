# PLAN

Prioritized by impact: wrong output first, then reliability, then everything else.

## P0 — Fix now: silent wrong output or data loss

All items completed.

## P1 — Fix soon: reliability and LLM quality

All items completed.

## P2 — Harden: security, error handling, observability

### Security

| ID    | Issue                             | Location                                | Notes                                                                                                                                                                                                                  |
|-------|-----------------------------------|-----------------------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| P2-4  | File/dir permissions (gosec G301/G302/G306) — remaining locations | `pipeline/cut.go:99`, `reader/debug.go:92` | `cut.go` uses `0750` for `os.MkdirAll`; `debug.go` uses `0750` for `os.MkdirAll`. Both should use explicit octal `0o750`. Low risk — no user-controlled paths, but gosec flags it. `workspace/init.go` already fixed. |
| P2-6  | Log injection via taint — G706    | `cli/root.go:90`                        | `slog.Info("... started", "args", strings.Join(os.Args[1:], " "))` logs raw CLI args. An attacker controlling args could inject fake log lines (newlines, ANSI escapes). Already has `//nolint:gosec` comment. Real risk is low since log file is user-private, but sanitizing newlines/control chars would be cleaner. |

### Error handling

| ID    | Issue                             | Location                                     | Notes                                                                                                                                                                                                                                                |
|-------|-----------------------------------|----------------------------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| P2-11 | FailoverChain TOCTOU on `nextName` | `provider/failover.go:188-203`              | After checking `ne.exhaustedUntil` to find the next provider, the mutex is released. By the time the failover callback fires, that provider may have been exhausted by another goroutine. Today this is safe because failover is single-threaded per request, but adding concurrency (P5-6) would expose a race. Fix: hold the lock while computing `nextName`, or accept the race as cosmetic (callback is for display only). |
| P2-12 | Port TOCTOU in `freePort`          | `ocr/qari.go:398-408`                       | `freePort()` binds to `:0`, reads the port, closes the listener, then Docker binds to that port. Another process could grab the port in between. Fix: retry the entire start sequence (find port + docker run) on bind failure, up to 3 attempts. Low probability in practice — only matters if the machine is under heavy port churn. |
| P2-14 | Unchecked `resp.Body.Close` in OCR client | `ocr/qari.go:162,233,306`              | `resp.Body.Close()` errors are already handled via `defer func() { if err := resp.Body.Close(); err != nil { slog.Warn(...) } }()` in all three locations (`IsReady`, `RecognizeRegions`, `RecognizeFullPage`). This was fixed during P2-23 standardization. Can be closed as done. |
| P2-15 | Unchecked errors in display package | `display/line.go`, `render.go`, `status.go`, `tty.go` | All write errors in the display package are either handled via `warnWrite()` (which calls `slog.Warn`) or are `strings.Builder` writes (which never return errors). Verified during P2 implementation. Can be closed as done. |
| P2-16 | Unchecked `os.Remove` / `f.Close` | `pipeline/atomic.go:9`, `reader/debug.go:101,104` | `atomic.go:9`: `defer func() { _ = os.Remove(tmpPath) }()` — the `_ =` is intentional; remove is best-effort cleanup. `debug.go:100`: `_ = f.Close()` before returning error — intentional, primary error is the encode failure. `debug.go:103`: `f.Close()` error IS checked and returned. All three cases are handled correctly. Can be closed as done. |

### Observability

| ID    | Issue                             | Location                          | Notes                                                                                                                                                                                                   |
|-------|-----------------------------------|-----------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| P2-22 | Translate success log missing metrics | `pipeline/translate.go:314`    | The `page translated` log line has `"page"`, `"model"`, `"completed"` but no region count, character count, or elapsed time. Adding `"regions", len(translated.Regions)` and a start-time measurement would help identify slow/problematic pages in logs. Low priority — the display layer already shows per-page progress. |

## P3 — Strengthen: testing and CI/CD

### Testing gaps

| ID    | Issue                             | Location                          | Notes                                                                                                                                                                                                   |
|-------|-----------------------------------|-----------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| P3-3  | Zero coverage: `cmd/mutercim`, `cmd/gen-schema` | `cmd/`                  | `cmd/mutercim/main.go` calls `cli.Execute()` which returns an error. Test: call `Execute()` with an invalid flag and verify it returns a non-nil error. `cmd/gen-schema/main.go` generates JSON schema — test: call it and verify output is valid JSON. Both are thin wrappers; coverage value is marginal. |
| P3-4  | `time.Sleep` in rate limiter tests | `apiclient/ratelimit_test.go:41` | The rate limiter test uses `time.Sleep(61 * time.Second)` to wait for the refill window. Fix: inject a clock interface (`func() time.Time`) into `RateLimiter` so tests can advance time without sleeping. This is a test-quality issue, not a production bug. The test is slow (61s) but correct. |
| P3-9  | Improve coverage: ocr package — 48.4% | `ocr/`                         | `QariTool.Start/Stop/RecognizeRegions/RecognizeFullPage` require a running Docker container, making them hard to unit test without mocking. Untested paths: container lifecycle, multipart request construction, health polling. Fix: extract an HTTP client interface or use `httptest.NewServer` to mock the Qari API endpoints. |
| P3-10 | Improve coverage: pipeline package — 50.6% | `pipeline/`                 | Major untested functions: `Cut` (requires Docker/pdftoppm), `Layout`/`OCR` (require Docker containers), `Write` PDF path (requires xelatex Docker image). Integration tests would need Docker. Unit tests can cover: `contiguousRanges`, `buildTranslateContext`, `buildOCRRegions`, `fileStem`, `filterPages`. Some already tested. |
| P3-12 | Unchecked errors in test code      | various `*_test.go`              | Spread across ~15 test files: `os.MkdirAll`, `os.WriteFile`, `json.Marshal` return values discarded with `_ =` or not checked. These are test-setup operations that should use `t.Fatal` on failure. Low runtime risk — if setup fails, the test will fail anyway with a less helpful error message. |

### CI/CD

| ID    | Issue                             | Location                                     | Notes                                                                                                                                                                                                   |
|-------|-----------------------------------|----------------------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| P3-17 | Python pip packages not pinned in Dockerfiles | `docker/doclayout-yolo/`, `docker/surya/`, `docker/qari-ocr/` | All three Dockerfiles use `pip install` without version pins (e.g. `pip install doclayout-yolo`). Rebuilds may pull incompatible versions. Fix: add `==version` pins or use a `requirements.txt` with `pip freeze` output. Requires checking current working versions. |
| P3-18 | No coverage threshold in CI        | `.github/workflows/ci.yml`                   | CI runs `go test` but does not enforce a minimum coverage percentage. Fix: add `go test -coverprofile=coverage.out ./...` and a threshold check (e.g. `go tool cover -func=coverage.out \| grep total \| awk '{print $3}'`). Current coverage is ~60% overall. Setting threshold too high blocks merges; 50% is a safe starting point. |
| P3-20 | Actions pinned by version tag, not SHA | `.github/workflows/*.yml`                | `actions/checkout@v4`, `golangci/golangci-lint-action@v6` etc. use mutable tags. A compromised tag could inject malicious code. Fix: replace tags with full commit SHAs (e.g. `actions/checkout@b4ffde65f46...`). Low probability attack, but recommended by GitHub security hardening guides. |
| P3-21 | No `govulncheck` in CI            | `.github/workflows/ci.yml`                   | CI does not scan for known Go vulnerabilities. Fix: add a step `go install golang.org/x/vuln/cmd/govulncheck@latest && govulncheck ./...`. Fast (~2s), catches CVEs in dependencies. Should be a non-blocking warning initially since vuln fixes may require dep updates. |

## P4 — Clean up: code quality, performance, docs

### Code quality

| ID    | Issue                             | Location                                     | Notes                                                                                                                                                                                                   |
|-------|-----------------------------------|----------------------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| P4-3  | Global mutable CLI flags           | `cli/root.go:19-26`, `cli/init.go:13-17`     | Six package-level `var` declarations (`cfgFile`, `logLevel`, `pages`, `outputDir`, `auto`, `force`) plus two in `init.go` (`initWorkspaceDir`, `initTemplateName`). Cobra binds flags to these globals. Fix: pass them through the command closure chain. Large refactor touching every command constructor. Risk: accidental regressions. The current approach works correctly, is idiomatic Cobra, and has no concurrency issues since CLI is single-process. |
| P4-5  | `interface{}` in knowledge loader  | `knowledge/loader.go:101,119,142,146`        | `rawFile` uses `map[string]interface{}` for YAML unmarshaling. `parseEntry` and `normalizeValue` type-assert the intermediate values. Fix: define typed structs matching the YAML schema and unmarshal into those directly. Reduces type assertions. Medium effort — must preserve backward compat with existing glossary YAML files. |
| P4-6  | Type-assertion on `*FailoverChain` | `pipeline/read.go:191`, `translate.go:191`   | `buildActiveModelFunc` and `setupDisplayCallbacks` cast `opts.Provider` to `*provider.FailoverChain` to access `ActiveModel()`, `LastUsedModel()`, `SetRetryCallback()`, and `OnFailover`. Fix: extract a `ModelTracker` interface with these methods, add a no-op implementation for single providers. Medium refactor — touches provider, pipeline, and CLI packages. |
| P4-9  | `readContext`/`translateContext` structural duplication | `pipeline/read.go:171`, `translate.go:158` | Both contexts have identical fields: `opts`, `stem`, `totalPages`, `maxPageNum`, `logger`, `activeModel`, `statusPageNum`, `completed`, `failed`, `skipped` and similar methods (`buildActiveModelFunc`, `setupDisplayCallbacks`, `processAllPages`). Fix: extract a shared `phaseContext` base struct. Large refactor — the two contexts differ in page processing logic and display callbacks. Benefit is primarily maintenance: changes to progress tracking logic must be made in both places today. |
| P4-10 | `layout.go` imports `reader`       | `pipeline/layout.go:352`                     | `generateLayoutDebugImage` calls `reader.GenerateDebugOverlay`. This creates a dependency from the layout pipeline to the reader package, which conceptually sits later in the pipeline. Fix: move `GenerateDebugOverlay` to a shared `debug` or `imaging` package. Small refactor. |
| P4-11 | `OnFailover` field unsynchronized  | `provider/failover.go:25`                    | `OnFailover func(string, string)` is a public field set by `setupDisplayCallbacks` and read inside `tryProviders`. Today safe because both happen on the same goroutine. If P5-6 (parallel processing) is implemented, this becomes a data race. Fix: protect with a mutex, or set once during construction and make immutable. |
| P4-13 | Migrate `gopkg.in/yaml.v3`        | `go.mod`                                      | `gopkg.in/yaml.v3` is the legacy import path. The canonical path is now `go.yaml.in/yaml/v3`. API is identical — only the import path changes. Fix: `go get go.yaml.in/yaml/v3` then find-replace all imports and `go mod tidy`. Mechanical but must touch every file that imports YAML (config, knowledge, cli, workspace, cmd/gen-schema). |

### Performance (non-critical)

| ID    | Issue                             | Location                                     | Notes                                                                                                                                                                                                   |
|-------|-----------------------------------|----------------------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| P4-16 | Resolve knowledge paths once before page loop | `pipeline/translate.go:244`, `read.go:321` | `cfg.ResolveKnowledgePaths(ws.Root)` is called inside the per-page rebuild check loop. It walks the filesystem each time. Fix: compute once before the loop and pass the result. Saves N filesystem walks per input (N = page count). Trivial fix. |
| P4-17 | Compute `buildInputPageMap` once per pipeline run | `pipeline/layout.go:70`, `ocr.go:64`, `read.go:51` | Each phase's top-level function calls `buildInputPageMap(opts.Config)` independently. The result is the same across phases. Fix: compute once in the CLI layer and pass through options. Saves 2 redundant computations per run. Trivial. |
| P4-18 | Stream OCR multipart body via `io.Pipe` | `ocr/qari.go:189-216`                     | `RecognizeRegions` reads the entire image into memory, writes it into a `bytes.Buffer` as multipart, then sends the buffer. Peak memory = 2x image size. Fix: use `io.Pipe` to stream the multipart body directly to the HTTP request, avoiding the intermediate buffer. Moderate complexity — must handle errors across goroutines. Matters only for very large page images (>50MB). |
| P4-20 | Cache `mergeKey` in knowledge entries | `knowledge/loader.go:180-194`              | `mergeEntry` computes `mergeKey(entry)` for every existing entry to check for duplicates. `mergeKey` allocates: sorts language keys, concatenates strings. Fix: compute the merge key once when an entry is created and store it in the `Entry` struct. Saves N*M allocations during knowledge loading (N = existing entries, M = new entries). Matters only for very large glossaries (>1000 entries). |

### Complexity (gocognit > 30)

These are functions with high cyclomatic/cognitive complexity. They work correctly but are harder to maintain and review. Refactoring is optional — each function's complexity is inherent to its orchestration role (multi-format dispatch, multi-phase prereqs, per-page processing with rebuild checks, display callbacks, and error handling).

| ID    | Function                                              | Score | Notes                                                                                                                |
|-------|-------------------------------------------------------|-------|----------------------------------------------------------------------------------------------------------------------|
| P4-21 | `pipeline/read.go` readOneInput                       | 103   | Already decomposed into `readContext` methods (`processAllPages`, `processReadPage`, `loadPagePrereqs`, `dispatchRead`, `recordFailure`, `recordSuccess`). Remaining complexity is in the orchestration glue. Further reduction requires extracting the progress/display wiring. |
| P4-22 | `pipeline/translate.go` translateOneInput              | 65    | Same pattern as read — decomposed into `translateContext` methods. Residual complexity from context window management and display callbacks. |
| P4-23 | `cli/make.go` newAllCmd                                | 65    | Sequential orchestration of 7 phases with per-phase error handling, display setup, and provider chain lifecycle. Cannot be decomposed without hiding the linear execution flow. |
| P4-24 | `pipeline/ocr.go` ocrOneInput                          | 60    | Per-page loop with region-level vs page-level OCR dispatch, layout integration, and progress display. Could extract the OCR dispatch logic into a helper. |
| P4-25 | `cli/auto.go` runPrerequisites                         | 52    | Phase-by-phase dispatch with per-phase provider/knowledge setup. Each block is simple but there are 6 of them. Could extract per-phase runners, but the linear flow is clearer as-is. |
| P4-26 | `cli/clean.go` newCleanCmd                             | 50    | Cobra command with 7 clean targets and interactive confirmation. Complexity from the target matrix. |
| P4-27 | `pipeline/layout.go` layoutOneInput                    | 46    | Per-page layout detection with rebuild checks, debug overlay generation, and progress display. |
| P4-28 | `pipeline/write.go` writeOneInput                      | 41    | Multi-format output (md, latex, pdf, docx) with per-format error handling and partial success semantics. |
| P4-29 | `cli/translate.go` newTranslateCmd                     | 35    | Cobra command with flag wiring, provider chain setup, and auto-prerequisites. Standard pattern. |
| P4-30 | `cli/read.go` newReadCmd                               | 35    | Same pattern as translate command. |
| P4-31 | `rebuild/rebuild.go` NewestMtime                       | 32    | Recursive mtime computation across files and directories. Inherently recursive. |
| P4-32 | `pipeline/solve.go` solveOneInput                      | 30    | Per-page solve with rebuild checks and cross-page context threading. At the threshold — refactoring optional. |
| P4-33 | `reader/region_reader.go` ReadRegionPage               | 88 ln | Two strategies (ai-only vs local+ai) with JSON parse fallback and region source attribution. Could extract JSON parsing into a helper, but the two paths share little code. |
| P4-34 | `reader/region_reader.go` ReadRegionPageWithOCR        | 90 ln | Three cases (layout+ocr, ocr-only, no data) with identical JSON parse/fallback blocks. Deduplicating the parse block with a shared helper would reduce to ~60 lines. |

## P5 — Features

| ID    | Feature                            | Details                                                                                    |
|-------|------------------------------------|--------------------------------------------------------------------------------------------|
| P5-1  | Three-layer prompt customization   | built-in `adab.md` + per-workspace `knowledge/prompt.md` + inline `extra_prompt` in config |
| P5-2  | Decouple source expansion from write phase | replace with optional `source_expansion` step                                        |
| P5-3  | Move `knowledge` to translate step | currently global; scope to translate config block                                          |
| P5-4  | Optimize token usage               | shorter JSON keys in phase output schema; shorter system prompts                           |
| P5-5  | Tashkeel fixing/completion         | optional post-process step for Arabic diacritics                                           |
| P5-6  | Parallel processing                | concurrent page processing within a phase; concurrent read across inputs                   |
| P5-7  | Side-by-side bilingual LaTeX output | ar+tr on same page in write phase                                                         |
| P5-8  | System-wide config                 | `~/.config/mutercim/` for API keys and default models                                      |
| P5-9  | `mutercim init --from-url`         | download PDF directly before scaffolding workspace                                         |
| P5-10 | Consider `unoffice` for docx generation | evaluate as replacement or fallback                                                   |
| P5-11 | Workspace-level lock               | prevent concurrent processes from corrupting workspace state                               |

## P6 — Long-term / exploratory

| ID   | Topic                        | Details                                                        |
|------|------------------------------|----------------------------------------------------------------|
| P6-1 | Multi-language docs          | AR/TR/ZH translations of README and user-facing docs           |
| P6-2 | Multi-language app strings   | localize help messages and log output (ar/tr/zh)               |
| P6-3 | Speech-to-text use cases     | transcription of lectures/sohbet, subtitle generation          |
| P6-4 | Video-to-text use cases      | meeting recording understanding                                |
| P6-5 | `image-to-text` generalization | expand OCR pipeline to general image understanding            |
| P6-6 | Evaluate replacing viper     | direct YAML + os.Getenv would eliminate 9 indirect deps        |

## Done

Items completed and committed.

### P0 — Wrong results

| ID    | Issue                                                                          | Commit    |
|-------|--------------------------------------------------------------------------------|-----------|
| P0-1  | `searchString` byte-level slicing — replaced with `strings.Contains`           | `1186a6f` |
| P0-2  | Config changes do not invalidate solve outputs — added `ws.ConfigPath()`       | `1186a6f` |
| P0-3  | Interrupted cut treated as complete — added report.json completion marker      | `9faddd6` |
| P0-4  | Partial read page written before failure — moved 0-region check before save    | `1186a6f` |
| P0-5  | Filename padding varies with batch size — compute from max page number         | `1186a6f` |
| P0-6  | `auto` prerequisite check too weak — check report.json instead of dirHasEntries | `9faddd6` |
| P0-7  | `append` aliasing on knowledge paths — explicit slice construction             | `1186a6f` |
| P0-8  | Glossary lookup misses tashkeel variants — tashkeel-stripped fallback           | `df0cf52` |
| P0-9  | "JSON array" vs "JSON object" contradiction — fixed user prompts               | `1186a6f` |
| P0-10 | `max_tokens: 4096` hardcoded — increased to 8192                               | `1186a6f` |
| P0-11 | Output filename `"book"` hardcoded — use input stem                            | `1186a6f` |
| P0-12 | Docker volume mounts use OS-native paths — added `filepath.ToSlash`            | `9faddd6` |

### P1 — Reliability and LLM quality

| ID    | Issue                                                                   | Commit    |
|-------|-------------------------------------------------------------------------|-----------|
| P1-1  | OCR container not stopped on error exits — use `defer`                  | `fec36ac` |
| P1-2  | OCR container orphaned on context cancellation — call `stopContainer`   | `fec36ac` |
| P1-3  | Client goroutine leak on "no usable providers" — call `cleanup()`       | `5fee16f` |
| P1-4  | Enable JSON mode on OpenAI — set `response_format: json_object`         | `fec36ac` |
| P1-5  | Glossary duplicated in system + user prompt — kept only in user         | `fec36ac` |
| P1-6  | Context duplicated in system + user prompt — kept only in user          | `fec36ac` |
| P1-7  | No delimiters around region text — added triple-quote delimiters        | `fec36ac` |
| P1-8  | Empty context placeholder wastes tokens — return empty string           | `fec36ac` |
| P1-9  | Pre-strip tashkeel on glossary forms once at startup                    | `fec36ac` |
| P1-10 | Build glossary portion of system prompt once per run                    | `fec36ac` |

### P2 — Security, error handling, observability, CLI UX

| ID    | Issue                                                                    | Commit    |
|-------|--------------------------------------------------------------------------|-----------|
| P2-1  | Gemini API key in URL query — moved to `X-Goog-Api-Key` header          | `8c7482f` |
| P2-2  | Validate `direction` parameter against allowlist                         | `8c7482f` |
| P2-3  | Validate `languages` parameter                                           | `8c7482f` |
| P2-4  | File/dir permissions (gosec) — workspace/init.go                         | `6537392` |
| P2-5  | `math/rand` for jitter — already uses `math/rand/v2` with nolint         | `5fee16f` |
| P2-7  | `buildInputPageMap` silently swallows parse errors — log warning          | `8c7482f` |
| P2-8  | `processTranslatePage` records failure with no log — added message        | `8c7482f` |
| P2-9  | `writePhaseReport` swallows marshal error — log warning                   | `8c7482f` |
| P2-10 | `workspace.Init` writes config non-atomically — atomic write              | `6537392` |
| P2-13 | No timeout on docker pull/build — 10-minute timeout                       | `5fee16f` |
| P2-17 | Log file open failure silently discards — warn on stderr                  | `8c7482f` |
| P2-18 | AI response body never logged on parse failure — log preview at Warn      | `6a86b1b` |
| P2-19 | Retry logged at Info, not Warn — changed to Warn                          | `8c7482f` |
| P2-20 | Solve phase context cancellation silent — added log                       | `8c7482f` |
| P2-21 | Layout/OCR logs missing "input" attribute — added to error logs           | `2b7f515` |
| P2-23 | `"err"` vs `"error"` key inconsistency — standardized                     | `6537392` |
| P2-24 | OCR error response body logged without truncation — truncated to 500 chars | `2b7f515` |
| P2-25 | TTY not restored on interrupt — moved Finish() to PersistentPostRunE      | `c4bca2f` |
| P2-26 | Missing API key error invisible — show on stderr                          | `8c7482f` |
| P2-27 | `config` command ignores workspace discovery — fixed                      | `b642238` |
| P2-28 | `status` returns exit 0 when workspace not found — returns error          | `b642238` |
| P2-29 | `report.json` inflates status counts — exclude from count                 | `b642238` |
| P2-30 | `status` references nonexistent `reports/` — fixed                        | `b642238` |
| P2-31 | Page range error doesn't show provided value — added to message           | `ad0eebd` |
| P2-32 | `--auto` runs prerequisites with no indication — print to stderr          | `ad0eebd` |

### P3 — Testing and CI/CD

| ID    | Issue                                                                   | Commit    |
|-------|-------------------------------------------------------------------------|-----------|
| P3-1  | Zero coverage: `pipeline/layout.go` — added LayoutRegionsToModelRegions tests | `3e2d174` |
| P3-2  | Zero coverage: `pipeline/ocr.go` — added loadOCRPage/loadLayoutPage tests     | `5dc380b` |
| P3-5  | No-assertion tests — added proper assertions                                  | `7055398` |
| P3-6  | Missing tests: `SourceLanguages()` / `SourceLanguagesForStem()`               | `b8167f0` |
| P3-7  | `TestReadPipelineSkipsCompleted` never asserts skip counter — added           | `3e2d174` |
| P3-8  | OCRPage/OCRRegion JSON round-trip tests                                       | `bd3b3e0` |
| P3-11 | Hardcoded `/tmp` in tests — replaced with `t.TempDir()`                       | `7055398` |
| P3-13 | No release workflow — created release.yml                                     | `abd2c85` |
| P3-14 | Docker workflow has no PR trigger — added                                     | `abd2c85` |
| P3-15 | `pandoc/Dockerfile` uses floating `latest` tag — pinned                       | `abd2c85` |
| P3-16 | `xelatex/Dockerfile` uses floating `latest` tag — pinned                      | `abd2c85` |
| P3-19 | `golangci-lint-action` uses `version: latest` — pinned                        | `abd2c85` |
| P3-22 | `docker-all` task missing `qari-ocr` — added                                 | `abd2c85` |
| P3-23 | `run` task uses `$@` — use `{{.CLI_ARGS}}`                                   | `abd2c85` |
| P3-24 | `dist` task — `-ldflags="-s -w"` already present                             | `abd2c85` |

### P4 — Code quality, performance, docs

| ID    | Issue                                                             | Commit    |
|-------|-------------------------------------------------------------------|-----------|
| P4-1  | `os.Exit(1)` in non-main package — `Execute()` returns error     | `6537392` |
| P4-2  | Missing Logger in pipeline.Solve call — added explicit logger     | `df879da` |
| P4-4  | `RegistryPrefix` is mutable global — made constant               | `ceda446` |
| P4-7  | `countRegionType` duplicated — deduplicated to util.go            | `6537392` |
| P4-8  | `atomicWrite` dead wrapper — removed                              | `6537392` |
| P4-12 | Docker subprocess gosec warnings — nolint with justification      | `ceda446` |
| P4-14 | Hoist `latexEscape` replacer to package level                     | `6537392` |
| P4-15 | Build region ID map once in renderers                             | `bebc22d` |
| P4-19 | Use `sort.Slice` for region ordering — replaced insertion sort    | `c16bbde` |
| P4-35 | DECISIONS.md stale entries — fixed                                | `ceda446` |
| P4-36 | README `--auto` description missing `ocr` phase — added          | `ceda446` |
| P4-37 | README missing `completion` command — added                       | `ceda446` |
| P4-38 | CLAUDE.md deps list missing `x/image` — added                    | `ceda446` |
| P4-39 | GO-CONVENTIONS.md stale pdftoppm example — updated for Docker     | `3e2d174` |
