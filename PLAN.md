# PLAN

Unified project plan. Ordered by priority: quality first, features second, release last.

Items marked [done] were resolved in prior commits and are kept for tracking only.

## P0 — Code quality and technical debt

- [ ] **Refactor `readOneInput`** — ~300 lines, cognitive complexity suppressed via nolint; extract
  `setupDisplayCallbacks()`, `readPageWithOCR()`, `readPageVision()`, `readPageWithRetry()` to
  reduce loop body to ~30 lines
- [ ] **Refactor `translateOneInput`** — same pattern; extract display setup and per-page dispatch
- [ ] **Log cleanup errors in OCR client** — `ocr/qari.go` silently ignores `resp.Body.Close`,
  `f.Close`, `l.Close` errors; log at warn level
- [ ] **Rename `model/section.go` to `model/pagerange.go`** — file contains `ParsePageRanges` and
  `ExpandPages`; "section" concept was removed; rename test file too
- [ ] **Fix `Validate()` cognitive complexity** — golangci-lint reports 29 (threshold 15); split
  into `validateInputs()`, `validateModels()`, `validateTools()`
- [ ] **Fix `TestBuildTranslateContext` length** — golangci-lint reports 86 lines (threshold 80);
  extract test helper or split sub-tests
- [ ] **Update DECISIONS.md** — remove or correct stale entries (e.g. "Default: gemini-2.5-flash-lite"
  should match actual `DefaultModel` constant `gemini-2.0-flash`)
- [ ] **Standardize log message capitalization** — scan `pipeline/`, `provider/`, `cli/` for
  uppercase log messages; use lowercase throughout except proper nouns
- [ ] **Extend config validation** — `Validate()` does not check: non-empty `read.models`, non-empty
  `translate.models`, non-empty `write.formats`; add these checks
- [x] ~~Fix unchecked errors in display layer~~ — resolved; I/O writes use `warnWrite()`
- [x] ~~Log context cancellation in phase loops~~ — resolved in read.go, translate.go
- [x] ~~Add nolint comment for math/rand jitter~~ — resolved in client.go
- [x] ~~Fix inconsistent failover chain Name()~~ — documented; callers directed to `ActiveModel()`
- [x] ~~Unify default model constant~~ — `DefaultModel` introduced in config.go

## P1 — Testing and CI

- [ ] **Add integration tests for CLI entry points** — `cmd/mutercim` and `cmd/gen-schema` are at
  0% coverage; test `Execute()` error path and basic output
- [ ] **Improve coverage: docker package** — 18.4%; test `CheckAvailable`, `EnsureImage` error
  paths, `FindDockerDir` with `t.TempDir()`
- [ ] **Improve coverage: cli package** — 27.0%; test `newAllCmd` flag parsing, config override
  paths, clean command targets
- [ ] **Improve coverage: ocr package** — 48.4%; test Qari client HTTP paths with
  `httptest.NewServer`, noop tool behavior
- [ ] **Improve coverage: pipeline package** — 50.6%; test layout/ocr pipeline functions,
  `readOneInput` dispatch paths, write format error paths
- [ ] **Add golangci-lint to CI** — `.golangci.yml` exists but `ci.yml` only runs build/vet/test;
  add a lint step
- [ ] **Investigate slow pipeline tests** — `pipeline` package takes ~200s; identify which tests are
  slow and whether they can be parallelized or simplified
- [ ] **Document `_extra_test.go` naming pattern** — add one paragraph to `CONTRIBUTING.md`
  explaining intent (separation of table-driven vs. integration/extra tests)

## P2 — Features

- [ ] **Three-layer prompt customization** — built-in `adab.md` embedded in binary + per-workspace
  `knowledge/prompt.md` + inline `extra_prompt` in config; applies to read and translate phases;
  built-in ships with Islamic scholarly etiquette (salawat, honorifics)
- [ ] **Decouple source expansion from write phase** — `write.expand_sources` is hadith-specific;
  replace with an optional `source_expansion` step insertable after translate or solve; decouple
  from formatting logic
- [ ] **Move `knowledge` to translate step** — currently global; scope it to the translate config
  block
- [ ] **Optimize token usage** — shorter JSON keys in phase output schema; shorter system prompts
- [ ] **Tashkeel fixing/completion** — optional post-process step to fix or complete Arabic
  diacritics in read output
- [ ] **Parallel processing** — concurrent page processing within a phase; concurrent read phases
  across inputs
- [ ] **Side-by-side bilingual LaTeX output** — ar+tr on same page in write phase
- [ ] **System-wide config** — `~/.config/mutercim/` for API keys and default models
- [ ] **`mutercim init --from-url`** — download PDF directly before scaffolding workspace
- [ ] **Consider `unoffice` for docx generation** — evaluate as replacement or fallback for
  current docx writer

## P3 — Release preparation

- [ ] **Pre-built binaries** — add goreleaser config: linux/amd64, linux/arm64, darwin/arm64,
  darwin/amd64, windows/amd64
- [ ] **Publish Docker images to ghcr.io** — poppler, doclayout-yolo, xelatex, pandoc; fall back
  to local build if pull fails

## P4 — Long-term / exploratory

- [ ] **Multi-language docs** — AR/TR/ZH translations of README and user-facing docs
- [ ] **Multi-language app strings** — localize help messages and log output (ar/tr/zh)
- [ ] **Speech-to-text use cases** — transcription of lectures/sohbet, subtitle generation from
  meeting recordings
- [ ] **Video-to-text use cases** — meeting recording understanding (screenshots + synced speech)
- [ ] **`image-to-text` generalization** — expand OCR pipeline to handle stock photo metadata
  extraction and general image understanding beyond document digitization
