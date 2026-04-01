# TASK.md

## P0 — Pre-release blockers

- [ ] **Pre-built binaries** — add goreleaser config: linux/amd64, darwin/arm64, darwin/amd64
- [ ] **Publish Docker images to ghcr.io** — poppler, doclayout-yolo, xelatex, pandoc; fall back
  to local build if pull fails
- [ ] **Fix unchecked errors in display layer** — `display/line.go:67,92,103`,
  `display/render.go:68`, `display/status.go:26,40`, `display/tty.go:205,266` all use `_, _ =
  fmt.Fprintf(...)`. Either panic on write error or accumulate errors from `Display.Finish()`.
- [ ] **Refactor `readOneInput` and `translateOneInput`** — cognitive complexity 103/65; extract
  `readPageWithOCRAndLayout()`, `readPageVisionOnly()`, `readPageWithRetry()`,
  `setupDisplayCallbacks()` to reduce loop body to ~30 lines
- [ ] **Extend config validation** — `Validate()` in `config/config.go` checks paths and page
  ranges but not: non-empty `read.models`/`translate.models`/`write.formats`, recognized provider
  names, valid layout tool name, non-negative context window

## P1 — Code quality

- [ ] **Log context cancellation in phase loops** — `pipeline/{read,translate,write}.go` silently
  `break` when `ctx.Err() != nil`; log at info: phase name, pages processed, pages remaining
- [ ] **Fix inconsistent failover chain `Name()`** — `provider/failover.go` returns concatenated
  names ("gemini/claude/…"); either return active provider name or document that clients should use
  `ActiveModel()` for per-request context
- [ ] **Log cleanup errors in OCR client** — `ocr/qari.go:159,222,289` (resp.Body.Close) and
  `:208,275` (writer.Close) silently ignore errors; log at warn level
- [ ] **Unify default model constant** — scaffold in `workspace/init.go:87` uses
  `gemini-2.0-flash`; README suggests `gemini-2.5-flash-lite`; pin to one constant shared by
  `config.go` and `init.go`, add note to CLAUDE.md to update on model releases
- [ ] **Add integration tests for CLI entry points** — `cmd/mutercim/main.go` and
  `cmd/gen-schema/main.go` are at 0% coverage; test `Execute()` error path and output
- [ ] **Add nolint comment for math/rand jitter** — `apiclient/client.go:212` uses `math/rand`
  for backoff jitter; add `//nolint:gosec // G404: jitter is not security-sensitive`
- [ ] **Standardize log message capitalization** — use lowercase throughout except proper nouns
  (scan `pipeline/`, `provider/`, `cli/`)
- [ ] **Document `_extra_test.go` naming pattern** — add one paragraph to `CONTRIBUTING.md`
  explaining intent (separation of table-driven vs. integration tests)

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

## P3 — Long-term / exploratory

- [ ] **Multi-language docs** — AR/TR/ZH translations of README and user-facing docs
- [ ] **Multi-language app strings** — localize help messages and log output (ar/tr/zh)
- [ ] **Speech-to-text use cases** — transcription of lectures/sohbet, subtitle generation from
  meeting recordings
- [ ] **Video-to-text use cases** — meeting recording understanding (screenshots + synced speech)
- [ ] **`image-to-text` generalization** — expand OCR pipeline to handle stock photo metadata
  extraction and general image understanding beyond document digitization
