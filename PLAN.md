# PLAN

Unified project plan. Ordered by priority: quality first, features second, release last.

Items marked [done] were resolved and are kept for tracking only.

## P0 — Code quality and technical debt

- [x] ~~Refactor `readOneInput`~~ — extracted `readContext` with composable methods
- [x] ~~Refactor `translateOneInput`~~ — extracted `translateContext` with composable methods
- [x] ~~Log cleanup errors in OCR client~~ — warn-level logging for Close errors in qari.go
- [x] ~~Rename `model/section.go` to `model/pagerange.go`~~ — file and test renamed
- [x] ~~Fix `Validate()` cognitive complexity~~ — split into validateInputs/Models/Tools
- [x] ~~Fix `TestBuildTranslateContext` length~~ — extracted package-level test helpers
- [x] ~~Update DECISIONS.md~~ — corrected DefaultModel entry, removed stale references
- [x] ~~Standardize log message capitalization~~ — already lowercase throughout
- [x] ~~Extend config validation~~ — non-empty read.models, translate.models, write.formats
- [x] ~~Fix unchecked errors in display layer~~ — I/O writes use `warnWrite()`
- [x] ~~Log context cancellation in phase loops~~ — resolved in read.go, translate.go
- [x] ~~Add nolint comment for math/rand jitter~~ — resolved in client.go
- [x] ~~Fix inconsistent failover chain Name()~~ — documented; callers directed to `ActiveModel()`
- [x] ~~Unify default model constant~~ — `DefaultModel` introduced in config.go

## P1 — Testing and CI

- [x] ~~Add golangci-lint to CI~~ — added lint step to ci.yml
- [x] ~~Investigate slow pipeline tests~~ — ~6s actual (was stale cache); 4 Docker-check tests ~1-2s each, acceptable
- [x] ~~Improve coverage: docker package~~ — 18% → 61% (imageShortName, FindDockerDir, isDockerfileDir)
- [x] ~~Improve coverage: cli package~~ — added apiKeyEnvVar and formatSize tests
- [x] ~~Document `_extra_test.go` naming pattern~~ — added to CONTRIBUTING.md
- [ ] **Add integration tests for CLI entry points** — `cmd/mutercim` and `cmd/gen-schema` at 0%
  coverage; test `Execute()` error path and basic output
- [ ] **Improve coverage: ocr package** — 48.4%; test Qari client HTTP paths with
  `httptest.NewServer`, noop tool behavior
- [ ] **Improve coverage: pipeline package** — 50.6%; test layout/ocr pipeline functions,
  dispatch paths, write format error paths

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

- [x] ~~Pre-built binaries~~ — goreleaser config added (linux/darwin/windows, amd64/arm64)
- [x] ~~Publish Docker images to ghcr.io~~ — CI workflow + pull-before-build in EnsureImage

## P4 — Long-term / exploratory

- [ ] **Multi-language docs** — AR/TR/ZH translations of README and user-facing docs
- [ ] **Multi-language app strings** — localize help messages and log output (ar/tr/zh)
- [ ] **Speech-to-text use cases** — transcription of lectures/sohbet, subtitle generation from
  meeting recordings
- [ ] **Video-to-text use cases** — meeting recording understanding (screenshots + synced speech)
- [ ] **`image-to-text` generalization** — expand OCR pipeline to handle stock photo metadata
  extraction and general image understanding beyond document digitization
