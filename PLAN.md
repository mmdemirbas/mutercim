# PLAN

Prioritized by impact: wrong output first, then reliability, then everything else.

## P0–P1

All items completed.

## Remaining — deferred items

Items below are deferred. Each has low impact, high refactor cost, or requires
external infrastructure (Docker images, pip freeze). See rationale in each note.

### Code quality (P4)

| ID    | Issue                             | Notes                                                                                      |
|-------|-----------------------------------|--------------------------------------------------------------------------------------------|
| P4-3  | Global mutable CLI flags          | Idiomatic Cobra pattern, no concurrency issue, large refactor with regression risk.         |
| P4-5  | `interface{}` in knowledge loader | Medium effort, must preserve backward compat with existing YAML glossary files.             |
| P4-6  | Type-assertion on `*FailoverChain` | Medium refactor across provider, pipeline, and CLI packages. Consider when adding P5-6.   |
| P4-9  | readContext/translateContext duplication | Large refactor. Maintenance cost is real but risk outweighs benefit.                  |
| P4-10 | layout.go imports reader          | Move `GenerateDebugOverlay` to shared `internal/imaging` package. Small refactor.           |
| P4-11 | OnFailover field sync             | Already reads under lock after P2-11 fix. Only matters if P5-6 concurrency is added.       |
| P4-13 | Migrate yaml.v3 import path       | Mechanical change, no behavioral benefit. Do during a dependency cleanup pass.              |
| P4-17 | buildInputPageMap once per run    | Trivial optimization, saves 2 redundant computations. Low priority.                         |
| P4-18 | Stream OCR multipart body         | Moderate complexity, matters only for >50MB images. Deferred.                               |

### Testing (P3)

| ID    | Issue                             | Notes                                                                                      |
|-------|-----------------------------------|--------------------------------------------------------------------------------------------|
| P3-4  | time.Sleep in rate limiter tests  | Test is slow (61s) but correct. Clock injection adds production complexity for test-only benefit. |
| P3-9  | OCR package coverage              | Requires httptest mock server for Qari API. P3-2 already added loadOCRPage tests.           |
| P3-10 | Pipeline package coverage         | Integration tests need Docker. Pure functions already tested.                                |
| P3-12 | Unchecked errors in test code     | Low risk — test setup failures produce less helpful errors but tests still fail.             |

### CI/CD (P3)

| ID    | Issue                             | Notes                                                                                      |
|-------|-----------------------------------|--------------------------------------------------------------------------------------------|
| P3-17 | Python pip packages not pinned    | Requires running Docker images to capture pip freeze. Do during release prep.               |

### Complexity (P4)

Functions with high gocognit scores. All are orchestration functions whose complexity is inherent to their role. Already decomposed into helper methods where practical. Further refactoring would hide the linear execution flow.

| ID     | Function                           | Score | Status |
|--------|-------------------------------------|-------|--------|
| P4-21  | readOneInput                        | 103   | Decomposed into readContext methods |
| P4-22  | translateOneInput                   | 65    | Decomposed into translateContext methods |
| P4-23  | newAllCmd                           | 65    | Sequential orchestration of 7 phases |
| P4-24  | ocrOneInput                         | 60    | Per-page loop with dual dispatch |
| P4-25  | runPrerequisites                    | 52    | Phase-by-phase dispatch |
| P4-26  | newCleanCmd                         | 50    | Target matrix with confirmation |
| P4-27  | layoutOneInput                      | 46    | Per-page with rebuild + debug |
| P4-28  | writeOneInput                       | 41    | Multi-format output |
| P4-29  | newTranslateCmd                     | 35    | Standard Cobra pattern |
| P4-30  | newReadCmd                          | 35    | Standard Cobra pattern |
| P4-31  | NewestMtime                         | 32    | Inherently recursive |
| P4-32  | solveOneInput                       | 30    | At threshold |

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

73 items completed across P0–P4.

### P0 — Wrong results (12/12)

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

### P1 — Reliability and LLM quality (10/10)

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

### P2 — Security, error handling, observability, CLI UX (27/32)

| ID    | Issue                                                                    | Commit    |
|-------|--------------------------------------------------------------------------|-----------|
| P2-1  | Gemini API key in URL query — moved to `X-Goog-Api-Key` header          | `8c7482f` |
| P2-2  | Validate `direction` parameter against allowlist                         | `8c7482f` |
| P2-3  | Validate `languages` parameter                                           | `8c7482f` |
| P2-4  | File/dir permissions (gosec G301/G302/G306) — all locations              | `e4fc5e3` |
| P2-5  | `math/rand` for jitter — already uses `math/rand/v2` with nolint         | `5fee16f` |
| P2-6  | Log injection via taint — sanitize CLI args before logging               | `e4fc5e3` |
| P2-7  | `buildInputPageMap` silently swallows parse errors — log warning          | `8c7482f` |
| P2-8  | `processTranslatePage` records failure with no log — added message        | `8c7482f` |
| P2-9  | `writePhaseReport` swallows marshal error — log warning                   | `8c7482f` |
| P2-10 | `workspace.Init` writes config non-atomically — atomic write              | `6537392` |
| P2-11 | FailoverChain TOCTOU on nextName — consolidated lock                      | `e4fc5e3` |
| P2-12 | Port TOCTOU in freePort — retry loop (3 attempts)                         | `e4fc5e3` |
| P2-13 | No timeout on docker pull/build — 10-minute timeout                       | `5fee16f` |
| P2-14 | Unchecked resp.Body.Close — already handled (verified)                    | —         |
| P2-15 | Unchecked errors in display — already handled (verified)                  | —         |
| P2-16 | Unchecked os.Remove/f.Close — already handled (verified)                  | —         |
| P2-17 | Log file open failure silently discards — warn on stderr                  | `8c7482f` |
| P2-18 | AI response body never logged on parse failure — log preview at Warn      | `6a86b1b` |
| P2-19 | Retry logged at Info, not Warn — changed to Warn                          | `8c7482f` |
| P2-20 | Solve phase context cancellation silent — added log                       | `8c7482f` |
| P2-21 | Layout/OCR logs missing "input" attribute — added to error logs           | `2b7f515` |
| P2-22 | Translate success log missing metrics — added regions and elapsed_ms      | `8d353f4` |
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

### P3 — Testing and CI/CD (18/24)

| ID    | Issue                                                                   | Commit    |
|-------|-------------------------------------------------------------------------|-----------|
| P3-1  | Zero coverage: `pipeline/layout.go` — added LayoutRegionsToModelRegions tests | `3e2d174` |
| P3-2  | Zero coverage: `pipeline/ocr.go` — added loadOCRPage/loadLayoutPage tests     | `5dc380b` |
| P3-3  | Zero coverage: `cmd/` — added Execute() error path test                       | `65b7466` |
| P3-5  | No-assertion tests — added proper assertions                                  | `7055398` |
| P3-6  | Missing tests: `SourceLanguages()` / `SourceLanguagesForStem()`               | `b8167f0` |
| P3-7  | `TestReadPipelineSkipsCompleted` never asserts skip counter — added           | `3e2d174` |
| P3-8  | OCRPage/OCRRegion JSON round-trip tests                                       | `bd3b3e0` |
| P3-11 | Hardcoded `/tmp` in tests — replaced with `t.TempDir()`                       | `7055398` |
| P3-13 | No release workflow — created release.yml                                     | `abd2c85` |
| P3-14 | Docker workflow has no PR trigger — added                                     | `abd2c85` |
| P3-15 | `pandoc/Dockerfile` uses floating `latest` tag — pinned                       | `abd2c85` |
| P3-16 | `xelatex/Dockerfile` uses floating `latest` tag — pinned                      | `abd2c85` |
| P3-18 | No coverage threshold in CI — added 50% threshold                             | `87429b3` |
| P3-19 | `golangci-lint-action` uses `version: latest` — pinned                        | `abd2c85` |
| P3-20 | Actions pinned by version tag — pinned to commit SHAs                         | `87429b3` |
| P3-21 | No govulncheck in CI — added                                                 | `87429b3` |
| P3-22 | `docker-all` task missing `qari-ocr` — added                                 | `abd2c85` |
| P3-23 | `run` task uses `$@` — use `{{.CLI_ARGS}}`                                   | `abd2c85` |
| P3-24 | `dist` task — `-ldflags="-s -w"` already present                             | `abd2c85` |

### P4 — Code quality, performance, docs (16/39)

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
| P4-16 | Resolve knowledge paths once before page loop                     | `074de95` |
| P4-19 | Use `sort.Slice` for region ordering — replaced insertion sort    | `c16bbde` |
| P4-20 | Cache mergeKey in knowledge entries                                | `074de95` |
| P4-33 | ReadRegionPage response parsing dedup                              | `a8f64cc` |
| P4-34 | ReadRegionPageWithOCR response parsing dedup                       | `a8f64cc` |
| P4-35 | DECISIONS.md stale entries — fixed                                | `ceda446` |
| P4-36 | README `--auto` description missing `ocr` phase — added          | `ceda446` |
| P4-37 | README missing `completion` command — added                       | `ceda446` |
| P4-38 | CLAUDE.md deps list missing `x/image` — added                    | `ceda446` |
| P4-39 | GO-CONVENTIONS.md stale pdftoppm example — updated for Docker     | `3e2d174` |
