# Roadmap — Public Release

Tracking everything needed before mutercim becomes a public open-source project.
Items are ordered by priority within each tier. Check off as completed.

---

## P0 — Must ship

- [x] License — MIT, LICENSE file added
- [x] Region-based translate phase — translate v2.0 region schema, not old entries/footnotes
- [x] Region-based write phase — render from v2.0 regions, not old schema
- [x] DocLayout-YOLO as default layout tool
- [x] Glossary refactor to unified schema
- [x] Error threshold — `retry.max_fail_percent` (default 10%), aborts read/translate if exceeded
- [x] Codebase cleanup — removed dead code (maxPageNum, estimateTotalPages), fixed stale comments
- [x] Test coverage gaps — context cancellation, failover callbacks, docx error path, layout tool
  fallback, non-retryable status codes, config validation, huge page ranges
- [x] Fix known dashboard bug — cursor-up count correctly tracks line count across phase changes

## P1 — Should ship

- [x] CI pipeline — GitHub Actions: build, vet, test, test -race on push/PR to main
- [ ] Pre-built binaries — goreleaser: linux/amd64, darwin/arm64, darwin/amd64
- [ ] Publish Docker images to ghcr.io — poppler, doclayout-yolo, xelatex, pandoc; fall back to
  local build if pull fails
- [x] Sample workspace — example/ with PDF, mutercim.yaml, glossary, full pipeline output
- [x] Config validation — Validate() in config.Load(), checks input paths and page ranges
- [x] `mutercim init` — scaffolds workspace with mutercim.yaml, input/, knowledge/, glossary.yaml
- [x] `.env.example` — at config/.env.example with API key var names and signup URLs
- [x] CONTRIBUTING.md — dev setup, build workflow, conventions, references to CLAUDE.md and GO-CONVENTIONS.md
- [x] Graceful Ctrl+C — signal.NotifyContext + display.Finish() for clean terminal state

## P2 — After launch

- [ ] Parallel processing for pages and read phases
- [ ] Side-by-side bilingual LaTeX output (ar+tr on same page)
- [ ] System-wide config at `~/.config/mutercim/` for API keys and default models
- [ ] Shell completions — `mutercim completion bash/zsh/fish`
- [ ] `mutercim init --from-url` — download PDF directly

## Out of scope

These were discussed and intentionally rejected:

- Web UI — scope creep, this is a CLI tool
- Plugin system — premature abstraction
- Windows native support — WSL2 + Docker covers it
- Python in the pipeline — all AI interaction via HTTP APIs from Go
- Source abbreviation auto-extraction — complex, let AI handle contextually
- X-Y cut / classical OpenCV for column detection — replaced by DocLayout-YOLO