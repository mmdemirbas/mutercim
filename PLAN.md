# PLAN

Prioritized by impact: wrong output first, then reliability, then everything else.

## P0 — Fix now: silent wrong output or data loss

### Wrong results

| ID | Issue | Location | Status |
|----|-------|----------|--------|
| P0-8 | Glossary lookup misses tashkeel variants — stores `forms[0]` but `LookupByForm` uses exact match; model receives incomplete glossary entries | `solver/solver.go:130-134` | |

## P1 — Fix soon: reliability and LLM quality

All items completed.

## P2 — Harden: security, error handling, observability

### Security

| ID | Issue | Location | Status |
|----|-------|----------|--------|
| P2-4 | File/dir permissions (gosec G301/G302/G306) — remaining locations | `pipeline/cut.go:99`, `reader/debug.go:92` | |
| P2-6 | Log injection via taint — G706 | `cli/root.go:83` | |

### Error handling

| ID | Issue | Location | Status |
|----|-------|----------|--------|
| P2-11 | FailoverChain TOCTOU on `nextName` | `provider/failover.go:189-228` | |
| P2-12 | Port TOCTOU in `freePort` — add retry on bind failure | `ocr/qari.go:398-408` | |
| P2-14 | Unchecked `resp.Body.Close` in OCR client | `ocr/qari.go:159,222,289` | |
| P2-15 | Unchecked errors in display package | `display/line.go:67,92,103`, `render.go:68`, `status.go:26,40`, `tty.go:205,266` | |
| P2-16 | Unchecked `os.Remove` / `f.Close` | `pipeline/atomic.go:9`, `reader/debug.go:101,104` | |

### Observability

| ID | Issue | Location | Status |
|----|-------|----------|--------|
| P2-18 | AI response body never logged on parse failure — page failures not debuggable from logs | `reader/region_reader.go:77-103`, `translation/translator.go:76-83` | |
| P2-21 | Layout/OCR logs missing `"input"` attribute — multi-input runs cannot identify which input failed | `pipeline/layout.go:247,252,263`, `pipeline/ocr.go:255,260,270` | |
| P2-22 | Translate success log missing metrics — no region count, elapsed time, or character count | `pipeline/translate.go:310` | |
| P2-24 | OCR error response body logged without truncation | `ocr/qari.go:239,312` | |

### CLI UX

| ID | Issue | Location | Status |
|----|-------|----------|--------|
| P2-25 | TTY not restored on interrupt — defers `Finish()` only in `all`; other commands leave terminal broken on Ctrl+C | `cli/make.go:62-63` | |

## P3 — Strengthen: testing and CI/CD

### Testing gaps

| ID | Issue | Location | Status |
|----|-------|----------|--------|
| P3-1 | Zero coverage: `pipeline/layout.go` — `LayoutRegionsToModelRegions` is pure and trivially testable | `pipeline/layout.go` | |
| P3-2 | Zero coverage: `pipeline/ocr.go` — test `loadOCRPage` round-trip and skip behavior | `pipeline/ocr.go` | |
| P3-3 | Zero coverage: `cmd/mutercim`, `cmd/gen-schema` — test `Execute()` error path | `cmd/` | |
| P3-4 | `time.Sleep` in rate limiter tests — inject clock interface | `apiclient/ratelimit_test.go:41` | |
| P3-7 | `TestReadPipelineSkipsCompleted` never asserts skip counter | `pipeline/read_test.go` | |
| P3-9 | Improve coverage: ocr package — 48.4% | `ocr/` | |
| P3-10 | Improve coverage: pipeline package — 50.6% | `pipeline/` | |
| P3-12 | Unchecked errors in test code — spread across 15+ test files | various `*_test.go` | |

### CI/CD

| ID | Issue | Location | Status |
|----|-------|----------|--------|
| P3-17 | Python pip packages not pinned in Dockerfiles — rebuilds produce different images | `docker/doclayout-yolo/`, `docker/surya/`, `docker/qari-ocr/` | |
| P3-18 | No coverage threshold in CI | `.github/workflows/ci.yml` | |
| P3-20 | Actions pinned by version tag, not SHA — supply-chain risk | `.github/workflows/*.yml` | |
| P3-21 | No `govulncheck` in CI | `.github/workflows/ci.yml` | |

## P4 — Clean up: code quality, performance, docs

### Code quality

| ID | Issue | Location | Status |
|----|-------|----------|--------|
| P4-2 | Missing `Logger` in `pipeline.Solve` call — latent bug if call order changes | `cli/solve.go:67-75` | |
| P4-3 | Global mutable CLI flags — capture via closures | `cli/root.go:19-26`, `cli/init.go:13-17` | |
| P4-5 | `interface{}` in knowledge loader — use typed intermediate | `knowledge/loader.go:101,119,142,146` | |
| P4-6 | Type-assertion on `*FailoverChain` in pipeline — extract `ModelTracker` interface | `pipeline/read.go:177`, `translate.go:177` | |
| P4-9 | `readContext`/`translateContext` structural duplication | `pipeline/read.go:155`, `translate.go:155` | |
| P4-10 | `layout.go` imports `reader` (inverted dependency) | `pipeline/layout.go:344` | |
| P4-11 | `OnFailover` field unsynchronized — safe today but race-detectable if concurrency added | `provider/failover.go:25` | |
| P4-13 | Migrate `gopkg.in/yaml.v3` to `go.yaml.in/yaml/v3` — unmaintained path; API identical | `go.mod` | |

### Performance (non-critical)

| ID | Issue | Location | Status |
|----|-------|----------|--------|
| P4-16 | Resolve knowledge paths once before page loop | `pipeline/translate.go:241`, `read.go:317` | |
| P4-17 | Compute `buildInputPageMap` once per pipeline run | `pipeline/layout.go:70`, `ocr.go:64`, `read.go:51` | |
| P4-18 | Stream OCR multipart body via `io.Pipe` — doubles peak memory | `ocr/qari.go:189-216` | |
| P4-19 | Use `sort.Slice` for region ordering | `translation/prompts.go:106-110` | |
| P4-20 | Cache `mergeKey` in knowledge entries | `knowledge/loader.go:180-194` | |

### Complexity (gocognit > 30)

| ID | Function | Score |
|----|----------|-------|
| P4-21 | `pipeline/read.go:93` readOneInput | 103 |
| P4-22 | `pipeline/translate.go:95` translateOneInput | 65 |
| P4-23 | `cli/make.go:22` newAllCmd | 65 |
| P4-24 | `pipeline/ocr.go:87` ocrOneInput | 60 |
| P4-25 | `cli/auto.go:68` runPrerequisites | 52 |
| P4-26 | `cli/clean.go:135` newCleanCmd | 50 |
| P4-27 | `pipeline/layout.go:104` layoutOneInput | 46 |
| P4-28 | `pipeline/write.go:71` writeOneInput | 41 |
| P4-29 | `cli/translate.go:16` newTranslateCmd | 35 |
| P4-30 | `cli/read.go:20` newReadCmd | 35 |
| P4-31 | `rebuild/rebuild.go:36` NewestMtime | 32 |
| P4-32 | `pipeline/solve.go:66` solveOneInput | 30 |
| P4-33 | `reader/region_reader.go:54` ReadRegionPage | 88 lines |
| P4-34 | `reader/region_reader.go:163` ReadRegionPageWithOCR | 90 lines |

### Documentation

| ID | Issue | Location | Status |
|----|-------|----------|--------|
| P4-39 | GO-CONVENTIONS.md stale pdftoppm example — pdftoppm now runs in Docker | `docs/GO-CONVENTIONS.md` | |

## P5 — Features

| ID | Feature | Details |
|----|---------|---------|
| P5-1 | Three-layer prompt customization | built-in `adab.md` + per-workspace `knowledge/prompt.md` + inline `extra_prompt` in config |
| P5-2 | Decouple source expansion from write phase | replace with optional `source_expansion` step |
| P5-3 | Move `knowledge` to translate step | currently global; scope to translate config block |
| P5-4 | Optimize token usage | shorter JSON keys in phase output schema; shorter system prompts |
| P5-5 | Tashkeel fixing/completion | optional post-process step for Arabic diacritics |
| P5-6 | Parallel processing | concurrent page processing within a phase; concurrent read across inputs |
| P5-7 | Side-by-side bilingual LaTeX output | ar+tr on same page in write phase |
| P5-8 | System-wide config | `~/.config/mutercim/` for API keys and default models |
| P5-9 | `mutercim init --from-url` | download PDF directly before scaffolding workspace |
| P5-10 | Consider `unoffice` for docx generation | evaluate as replacement or fallback |
| P5-11 | Workspace-level lock | prevent concurrent processes from corrupting workspace state |

## P6 — Long-term / exploratory

| ID | Topic | Details |
|----|-------|---------|
| P6-1 | Multi-language docs | AR/TR/ZH translations of README and user-facing docs |
| P6-2 | Multi-language app strings | localize help messages and log output (ar/tr/zh) |
| P6-3 | Speech-to-text use cases | transcription of lectures/sohbet, subtitle generation |
| P6-4 | Video-to-text use cases | meeting recording understanding |
| P6-5 | `image-to-text` generalization | expand OCR pipeline to general image understanding |
| P6-6 | Evaluate replacing viper | direct YAML + os.Getenv would eliminate 9 indirect deps |

## Done

Items completed and committed.

### P0 — Wrong results

| ID | Issue | Commit |
|----|-------|--------|
| P0-1 | `searchString` byte-level slicing — replaced with `strings.Contains` | `1186a6f` |
| P0-2 | Config changes do not invalidate solve outputs — added `ws.ConfigPath()` to rebuild inputs | `1186a6f` |
| P0-4 | Partial read page written before failure — moved 0-region check before save | `1186a6f` |
| P0-5 | Filename padding varies with batch size — compute from max page number | `1186a6f` |
| P0-7 | `append` aliasing on knowledge paths — explicit slice construction | `1186a6f` |
| P0-9 | "JSON array" vs "JSON object" contradiction — fixed user prompts | `1186a6f` |
| P0-10 | `max_tokens: 4096` hardcoded — increased to 8192 | `1186a6f` |
| P0-11 | Output filename `"book"` hardcoded — use input stem | `1186a6f` |
| P0-3 | Interrupted cut treated as complete — added report.json completion marker | `9faddd6` |
| P0-6 | `auto` prerequisite check too weak — check report.json instead of dirHasEntries | `9faddd6` |
| P0-12 | Docker volume mounts use OS-native paths — added `filepath.ToSlash` | `9faddd6` |

### P1 — Reliability and LLM quality

| ID | Issue | Commit |
|----|-------|--------|
| P1-1 | OCR container not stopped on error exits — use `defer` | `fec36ac` |
| P1-2 | OCR container orphaned on context cancellation — call `stopContainer` | `fec36ac` |
| P1-3 | Client goroutine leak on "no usable providers" — call `cleanup()` | `5fee16f` |
| P1-4 | Enable JSON mode on OpenAI — set `response_format: json_object` | `fec36ac` |
| P1-5 | Glossary duplicated in system + user prompt — kept only in user | `fec36ac` |
| P1-6 | Context duplicated in system + user prompt — kept only in user | `fec36ac` |
| P1-7 | No delimiters around region text — added triple-quote delimiters | `fec36ac` |
| P1-8 | Empty context placeholder wastes tokens — return empty string | `fec36ac` |
| P1-9 | Pre-strip tashkeel on glossary forms once at startup | `fec36ac` |
| P1-10 | Build glossary portion of system prompt once per run | `fec36ac` |

### P2 — Security, error handling, observability, CLI UX

| ID | Issue | Commit |
|----|-------|--------|
| P2-1 | Gemini API key in URL query — moved to `X-Goog-Api-Key` header | `8c7482f` |
| P2-2 | Validate `direction` parameter against allowlist | `8c7482f` |
| P2-3 | Validate `languages` parameter | `8c7482f` |
| P2-4 | File/dir permissions (gosec) — workspace/init.go | `6537392` |
| P2-5 | `math/rand` for jitter — already uses `math/rand/v2` with nolint | `5fee16f` |
| P2-7 | `buildInputPageMap` silently swallows parse errors — log warning | `8c7482f` |
| P2-8 | `processTranslatePage` records failure with no log — added message | `8c7482f` |
| P2-9 | `writePhaseReport` swallows marshal error — log warning | `8c7482f` |
| P2-10 | `workspace.Init` writes config non-atomically — atomic write | `6537392` |
| P2-13 | No timeout on docker pull/build — 10-minute timeout | `5fee16f` |
| P2-17 | Log file open failure silently discards — warn on stderr | `8c7482f` |
| P2-19 | Retry logged at Info, not Warn — changed to Warn | `8c7482f` |
| P2-20 | Solve phase context cancellation silent — added log | `8c7482f` |
| P2-23 | `"err"` vs `"error"` key inconsistency — standardized | `6537392` |
| P2-26 | Missing API key error invisible — show on stderr | `8c7482f` |
| P2-27 | `config` command ignores workspace discovery — fixed | `b642238` |
| P2-28 | `status` returns exit 0 when workspace not found — returns error | `b642238` |
| P2-29 | `report.json` inflates status counts — exclude from count | `b642238` |
| P2-30 | `status` references nonexistent `reports/` — fixed | `b642238` |
| P2-31 | Page range error doesn't show provided value — added to message | `ad0eebd` |
| P2-32 | `--auto` runs prerequisites with no indication — print to stderr | `ad0eebd` |

### P3 — Testing and CI/CD

| ID | Issue | Commit |
|----|-------|--------|
| P3-5 | No-assertion tests — added proper assertions | `7055398` |
| P3-6 | Missing tests: `SourceLanguages()` / `SourceLanguagesForStem()` | `b8167f0` |
| P3-8 | OCRPage/OCRRegion JSON round-trip tests | `bd3b3e0` |
| P3-11 | Hardcoded `/tmp` in tests — replaced with `t.TempDir()` | `7055398` |
| P3-13 | No release workflow — created release.yml | `abd2c85` |
| P3-14 | Docker workflow has no PR trigger — added | `abd2c85` |
| P3-15 | `pandoc/Dockerfile` uses floating `latest` tag — pinned | `abd2c85` |
| P3-16 | `xelatex/Dockerfile` uses floating `latest` tag — pinned | `abd2c85` |
| P3-19 | `golangci-lint-action` uses `version: latest` — pinned | `abd2c85` |
| P3-22 | `docker-all` task missing `qari-ocr` — added | `abd2c85` |
| P3-23 | `run` task uses `$@` — use `{{.CLI_ARGS}}` | `abd2c85` |
| P3-24 | `dist` task missing `-ldflags="-s -w"` — already present | `abd2c85` |

### P4 — Code quality, performance, docs

| ID | Issue | Commit |
|----|-------|--------|
| P4-1 | `os.Exit(1)` in non-main package — `Execute()` returns error | `6537392` |
| P4-4 | `RegistryPrefix` is mutable global — made constant | `ceda446` |
| P4-7 | `countRegionType` duplicated — deduplicated to util.go | `6537392` |
| P4-8 | `atomicWrite` dead wrapper — removed | `6537392` |
| P4-12 | Docker subprocess gosec warnings — nolint with justification | `ceda446` |
| P4-14 | Hoist `latexEscape` replacer to package level | `6537392` |
| P4-15 | Build region ID map once in renderers | `bebc22d` |
| P4-35 | DECISIONS.md stale entries — fixed | `ceda446` |
| P4-36 | README `--auto` description missing `ocr` phase — added | `ceda446` |
| P4-37 | README missing `completion` command — added | `ceda446` |
| P4-38 | CLAUDE.md deps list missing `x/image` — added | `ceda446` |
