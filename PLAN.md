# PLAN

Prioritized by impact: wrong output first, then reliability, then everything else.

## P0 — Fix now: silent wrong output or data loss

Items that produce incorrect results the user may not notice, or lose work.

### Wrong results

| ID | Issue | Location |
|----|-------|----------|
| P0-1 | `searchString` byte-level slicing on UTF-8 Arabic text — reimplements `strings.Contains` with naive byte offsets; slicing mid-rune produces wrong glossary matches | `solver/solver.go:170-181` |
| P0-2 | Config changes do not invalidate solve outputs — omits `ws.ConfigPath()` from `rebuildInputs`; stale solve results served after config change | `pipeline/solve.go:109` |
| P0-3 | Interrupted cut treated as complete — skips cut if `imagesDir` has any entries and mtime is newer than PDF; partial Ctrl+C leaves 50/200 images; downstream phases silently process only those | `pipeline/cut.go:87-98` |
| P0-4 | Partial read page written before failure recorded — saves page to disk then records failure; incremental runs see the file and skip re-processing | `pipeline/read.go:269-278` |
| P0-5 | Solve/translate filename padding varies with batch size — uses `len(pages)` (filtered count) for padding; `--pages 1-10` produces `01.json`, full run produces `001.json`; orphaned files confuse rebuild | `pipeline/solve.go:108` |
| P0-6 | `auto` prerequisite check too weak — uses `dirHasEntries`; one page from an interrupted read makes `--auto translate` skip re-running read entirely | `cli/auto.go:34-58` |
| P0-7 | `append` aliasing on knowledge paths — appends to caller's slice; spare capacity silently mutates backing array across loop iterations | `pipeline/translate.go:241`, `solve.go:109` |
| P0-8 | Glossary lookup misses tashkeel variants — stores `forms[0]` but `LookupByForm` uses exact match; model receives incomplete glossary entries | `solver/solver.go:130-134` |

### Wrong LLM output

| ID | Issue | Location |
|----|-------|----------|
| P0-9 | "JSON array" vs "JSON object" contradiction in read prompts — user prompt says "Return JSON array" but system prompt defines object schema; model may return `[...]` which fails unmarshal | `reader/region_prompts.go:167-178` |
| P0-10 | `max_tokens: 4096` hardcoded — dense Arabic pages with 20+ regions can exceed this; truncated JSON unrecoverable; make configurable, default to 8192+ | `provider/claude.go:74,104` |

### Data loss

| ID | Issue | Location |
|----|-------|----------|
| P0-11 | Output filename `"book"` hardcoded — multiple inputs in one workspace overwrite each other's output files | `pipeline/write.go:83,247` |
| P0-12 | Docker volume mounts use OS-native paths — Windows backslash paths cause Docker mount failure; use `filepath.ToSlash` | `input/pdf.go:38`, `layout/doclayout.go:187`, `layout/surya.go:139`, `renderer/latex.go:169`, `renderer/docx.go:30` |

## P1 — Fix soon: reliability and LLM quality

Resource leaks, prompt issues that waste money or degrade translation quality.

### Resource leaks

| ID | Issue | Location |
|----|-------|----------|
| P1-1 | OCR container not stopped on error exits in `all` command — `ocrTool.Stop()` only runs on happy path; use `defer` | `cli/make.go:236-239` |
| P1-2 | OCR container orphaned on context cancellation during startup — returns `ctx.Err()` without calling `stopContainer`; Ctrl+C leaks container | `ocr/qari.go:113-117` |
| P1-3 | Client goroutine leak on "no usable providers" path — returns error without calling `cleanup()` | `cli/read.go:260` |

### LLM prompt quality

| ID | Issue | Location |
|----|-------|----------|
| P1-4 | Enable JSON mode on OpenAI — does not set `response_format`; models wrap JSON in markdown fences; Gemini already uses `responseMimeType` | `provider/openai.go` |
| P1-5 | Glossary duplicated in system + user prompt — same terms appear twice wasting tokens; keep only page-specific injection in user message | `translation/prompts.go:20-22,124-130` |
| P1-6 | Context duplicated in system + user prompt — injects context from different sources in both; confusing and redundant | `translation/prompts.go:23,131-133` |
| P1-7 | No delimiters around region text in translate prompt — inlines raw OCR text without quoting; reader prompts use triple-quotes | `translation/prompts.go:116-122` |
| P1-8 | Empty context placeholder wastes tokens — returns `"(No previous context available)"` when empty; omit section; saves ~10k tokens per bilingual book | `translation/prompts.go:139-144` |

### Performance (hot path)

| ID | Issue | Location |
|----|-------|----------|
| P1-9 | Pre-strip tashkeel on glossary forms once at startup — ~2M redundant `strings.Map` allocations for 500 pages with 200 glossary entries; cache in `NewSolver` | `solver/solver.go:116-139` |
| P1-10 | Build glossary portion of system prompt once per run — rebuilds identical glossary string on every page; separate static from dynamic parts | `translation/translator.go:55-62` |

## P2 — Harden: security, error handling, observability

### Security

| ID | Issue | Location |
|----|-------|----------|
| P2-1 | Move Gemini API key from URL query to header — use `X-Goog-Api-Key` header instead | `provider/gemini.go:122` |
| P2-2 | Validate `direction` parameter against allowlist — only `"ltr"` and `"rtl"` are valid | `layout/doclayout.go:204` |
| P2-3 | Validate `languages` parameter — validate against `[a-zA-Z,]+` | `layout/surya.go:144` |
| P2-4 | File/dir permissions (gosec G301/G302/G306) | `cli/root.go:69`, `workspace/init.go:52,59,65`, `pipeline/cut.go:99`, `reader/debug.go:92` |
| P2-5 | `math/rand` for jitter — G404; use `crypto/rand` or nolint | `apiclient/client.go:212` |
| P2-6 | Log injection via taint — G706 | `cli/root.go:83` |

### Error handling

| ID | Issue | Location |
|----|-------|----------|
| P2-7 | `buildInputPageMap` silently swallows parse errors — log a warning | `pipeline/read.go:83-88`, `cut.go:52-57` |
| P2-8 | `processTranslatePage` records failure with no log — increments counter but writes no message | `pipeline/translate.go:249-253` |
| P2-9 | `writePhaseReport` swallows marshal error — returns silently on failure | `pipeline/read.go:407`, `solve.go:171`, `ocr.go:301` |
| P2-10 | `workspace.Init` writes config non-atomically | `workspace/init.go:61-68` |
| P2-11 | FailoverChain TOCTOU on `nextName` | `provider/failover.go:189-228` |
| P2-12 | Port TOCTOU in `freePort` — add retry on bind failure | `ocr/qari.go:398-408` |
| P2-13 | No timeout on `docker pull` / `docker build` | `docker/docker.go:78,93` |
| P2-14 | Unchecked `resp.Body.Close` in OCR client | `ocr/qari.go:159,222,289`, `:208,275`, `:364` |
| P2-15 | Unchecked errors in display package | `display/line.go:67,92,103`, `render.go:68`, `status.go:26,40`, `tty.go:205,266` |
| P2-16 | Unchecked `os.Remove` / `f.Close` | `pipeline/atomic.go:9`, `reader/debug.go:101,104` |

### Observability

| ID | Issue | Location |
|----|-------|----------|
| P2-17 | Log file open failure silently discards all logs — falls back to `io.Discard` with no stderr warning | `cli/root.go:70-74` |
| P2-18 | AI response body never logged on parse failure — page failures not debuggable from logs | `reader/region_reader.go:77-103`, `translation/translator.go:76-83` |
| P2-19 | Retry logged at Info, not Warn — consolidate with failure log into single Warn with URL | `apiclient/client.go:110,139` |
| P2-20 | Solve phase context cancellation silent — read and translate phases log this correctly | `pipeline/solve.go:103-106` |
| P2-21 | Layout/OCR logs missing `"input"` attribute — multi-input runs cannot identify which input failed | `pipeline/layout.go:247,252,263`, `pipeline/ocr.go:255,260,270` |
| P2-22 | Translate success log missing metrics — no region count, elapsed time, or character count | `pipeline/translate.go:310` |
| P2-23 | `"err"` vs `"error"` key inconsistency — standardize | `display/render.go:37`, `ocr/qari.go:162,233` vs `pipeline/read.go:71`, `knowledge/loader.go:72` |
| P2-24 | OCR error response body logged without truncation | `ocr/qari.go:239,312` |

### CLI UX

| ID | Issue | Location |
|----|-------|----------|
| P2-25 | TTY not restored on interrupt — defers `Finish()` only in `all`; other commands leave terminal broken on Ctrl+C | `cli/make.go:62-63` |
| P2-26 | Missing API key error invisible to user — logs skipped models to file only; user sees generic "no usable providers" | `cli/read.go:231-235` |
| P2-27 | `config` command ignores workspace discovery — running from subdirectory finds no config | `cli/config_cmd.go:20` |
| P2-28 | `status` returns exit 0 when workspace not found | `cli/status.go:31-33` |
| P2-29 | `report.json` inflates status counts — shows "201/200" for a 200-page book | `cli/status.go:239` |
| P2-30 | `status` references nonexistent `reports/` directory | `display/status.go:86` |
| P2-31 | Page range error doesn't show the provided value | `cli/layout.go:63` and all phase cmds |
| P2-32 | `--auto` runs prerequisites with no terminal indication — logs to file only | `cli/auto.go:86` |

## P3 — Strengthen: testing and CI/CD

### Testing gaps

| ID | Issue | Location |
|----|-------|----------|
| P3-1 | Zero coverage: `pipeline/layout.go` — `LayoutRegionsToModelRegions` is pure and trivially testable | `pipeline/layout.go` |
| P3-2 | Zero coverage: `pipeline/ocr.go` — test `loadOCRPage` round-trip and skip behavior | `pipeline/ocr.go` |
| P3-3 | Zero coverage: `cmd/mutercim`, `cmd/gen-schema` — test `Execute()` error path | `cmd/` |
| P3-4 | `time.Sleep` in rate limiter tests — inject clock interface | `apiclient/ratelimit_test.go:41` |
| P3-5 | No-assertion tests — discard errors instead of asserting | `docker/docker_test.go:11-19`, `pipeline/translate_extra_test.go` |
| P3-6 | Missing tests: `config.SourceLanguages()` / `SourceLanguagesForStem()` | `config/config.go:243,258` |
| P3-7 | `TestReadPipelineSkipsCompleted` never asserts skip counter | `pipeline/read_test.go` |
| P3-8 | OCRPage/OCRRegion JSON round-trip tests missing | `model/ocr.go` |
| P3-9 | Improve coverage: ocr package — 48.4% | `ocr/` |
| P3-10 | Improve coverage: pipeline package — 50.6% | `pipeline/` |
| P3-11 | Hardcoded `/tmp` in test files — `go test ./...` fails on Windows | `layout/doclayout_test.go`, `layout/layout_test.go`, `workspace/workspace_test.go` |
| P3-12 | Unchecked errors in test code — spread across 15+ test files | various `*_test.go` |

### CI/CD

| ID | Issue | Location |
|----|-------|----------|
| P3-13 | No release workflow — `.goreleaser.yml` exists but no CI job invokes it | missing `.github/workflows/release.yml` |
| P3-14 | Docker workflow has no PR trigger — broken Dockerfiles discovered only after merge | `.github/workflows/docker.yml:5-8` |
| P3-15 | `pandoc/Dockerfile` uses floating `latest` tag | `docker/pandoc/Dockerfile:1` |
| P3-16 | `xelatex/Dockerfile` uses floating `latest` tag | `docker/xelatex/Dockerfile:1` |
| P3-17 | Python pip packages not pinned in Dockerfiles — rebuilds produce different images | `docker/doclayout-yolo/`, `docker/surya/`, `docker/qari-ocr/` |
| P3-18 | No coverage threshold in CI | `.github/workflows/ci.yml` |
| P3-19 | `golangci-lint-action` uses `version: latest` — pin to specific version | `ci.yml:33` |
| P3-20 | Actions pinned by version tag, not SHA — supply-chain risk | `.github/workflows/*.yml` |
| P3-21 | No `govulncheck` in CI | `.github/workflows/ci.yml` |
| P3-22 | `docker-all` task missing `qari-ocr` | `Taskfile.yml:74` |
| P3-23 | `run` task uses `$@` — use `{{.CLI_ARGS}}` | `Taskfile.yml:106` |
| P3-24 | `dist` task missing `-ldflags="-s -w"` | `Taskfile.yml:56-59` |

## P4 — Clean up: code quality, performance, docs

### Code quality

| ID | Issue | Location |
|----|-------|----------|
| P4-1 | `os.Exit(1)` in non-main package — return error from `Execute()` | `cli/root.go:234` |
| P4-2 | Missing `Logger` in `pipeline.Solve` call — latent bug if call order changes | `cli/solve.go:67-75` |
| P4-3 | Global mutable CLI flags — capture via closures | `cli/root.go:19-26`, `cli/init.go:13-17` |
| P4-4 | `RegistryPrefix` is mutable global — make constant or inject | `docker/docker.go:34` |
| P4-5 | `interface{}` in knowledge loader — use typed intermediate | `knowledge/loader.go:101,119,142,146` |
| P4-6 | Type-assertion on `*FailoverChain` in pipeline — extract `ModelTracker` interface | `pipeline/read.go:177`, `translate.go:177` |
| P4-7 | `countRegionType` duplicated across 3 files | `pipeline/solve.go:206`, `read.go:466`, `translate.go:348` |
| P4-8 | `atomicWrite` dead wrapper — remove | `pipeline/write.go:341-343` |
| P4-9 | `readContext`/`translateContext` structural duplication | `pipeline/read.go:155`, `translate.go:155` |
| P4-10 | `layout.go` imports `reader` (inverted dependency) | `pipeline/layout.go:344` |
| P4-11 | `OnFailover` field unsynchronized — safe today but race-detectable if concurrency added | `provider/failover.go:25` |
| P4-12 | Docker subprocess gosec warnings — nolint with justification | `docker/docker.go:41,52,76` |
| P4-13 | Migrate `gopkg.in/yaml.v3` to `go.yaml.in/yaml/v3` — unmaintained path; API identical | `go.mod` |

### Performance (non-critical)

| ID | Issue | Location |
|----|-------|----------|
| P4-14 | Hoist `latexEscape` replacer to package level | `renderer/latex.go:140-154` |
| P4-15 | Build region ID map once in renderers — O(n) scan per reading-order entry | `renderer/markdown.go:109-116` |
| P4-16 | Resolve knowledge paths once before page loop | `pipeline/translate.go:241`, `read.go:317` |
| P4-17 | Compute `buildInputPageMap` once per pipeline run | `pipeline/layout.go:70`, `ocr.go:64`, `read.go:51` |
| P4-18 | Stream OCR multipart body via `io.Pipe` — doubles peak memory | `ocr/qari.go:189-216` |
| P4-19 | Use `sort.Slice` for region ordering | `translation/prompts.go:106-110` |
| P4-20 | Cache `mergeKey` in knowledge entries | `knowledge/loader.go:180-194` |

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

| ID | Issue | Location |
|----|-------|----------|
| P4-35 | DECISIONS.md stale entries — `make`->`all`, `pages`->`cut`, `read.layout_tool`->`layout.tool`, stale dirs, clean targets, wrong log path | `docs/DECISIONS.md` |
| P4-36 | README `--auto` description missing `ocr` phase | `README.md` |
| P4-37 | README missing `completion` command in Workspace Commands table | `README.md` |
| P4-38 | CLAUDE.md deps list missing `x/image` — says "only cobra, viper, yaml.v3" | `CLAUDE.md` |
| P4-39 | GO-CONVENTIONS.md stale pdftoppm example — pdftoppm now runs in Docker | `docs/GO-CONVENTIONS.md` |

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
