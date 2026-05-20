---
date: 2026-05-12
topic: Synthesis of 2026-05-12 research → architecture direction
inputs:
  - notes/research-2026-05-12-end-to-end-doc-parsing.md
  - notes/research-2026-05-12-multiphase-stack.md
  - notes/research-2026-05-12-local-translation.md
  - notes/research-2026-05-12-host-language-and-rendering.md
priority_order: quality > simplicity >> speed > maintainability
constraints:
  - local-first must always work (cloud is optional quality boost)
  - language-agnostic core (Arabic/RTL/Qari are plugins, not the path)
  - both layout modes (strict-PDF + reflow), per input
---

# Direction synthesis — 2026-05-12

## TL;DR

Four findings, three real options, one recommendation:

1. **One open model now does what cut+layout+ocr+read does today**, with better
   public benchmarks. MinerU 2.5-Pro (Apache-2.0-equivalent, 1.2B, 8 GB VRAM,
   Apple Silicon native, 109 langs).
2. **Translation specialists don't cover Arabic/Turkish.** The right local
   path is a general multilingual LLM with ar+tr in pretraining. Gemma 3 27B
   IT Q4 (T2), Gemma 3 12B IT QAT-Q4 (T1). Apache-2.0 alts: Qwen3 32B/8B.
3. **Don't rewrite in Python.** Real Go LoC is 12.4K non-test (not the 50K I
   guessed). Pivot cost 10–16 person-weeks. The hybrid — Go + uv-managed
   Python helpers — captures the Python upside without the rewrite.
4. **Drop Docker mostly.** Three Python-wrapping images go away with uv. The
   three system-binary images (XeLaTeX, pandoc, Poppler) can go via
   `pypandoc-binary` + system `pdftoppm` + optionally Typst replacing
   XeLaTeX. Net: 6 images → 1 or 0 in ~2 weeks.

Recommended path: **Option B — aggressive collapse + uv** (below).

---

## What current architecture gets right (keep)

These survive every option and shouldn't be touched:

- Region-based JSON schema v2.0 (header / entry / footnote / table / etc.) —
  MinerU's native output maps onto this almost directly. The schema is a
  quality asset.
- Failover chain with per-model rate limits and 60s rolling recovery —
  applies to the new local providers too.
- Knowledge / glossary layering (workspace YAML + auto-memory) — directly
  drives translation quality; keep as-is.
- Atomic writes, smart timestamp-based rebuilds, per-page incremental output
  — these are why P0–P3 are done.
- Go orchestrator with strict-typed config, cobra subcommand tree, slog,
  signal-aware shutdown — paid-for craftsmanship, no reason to throw it away.

## What current architecture gets wrong (change)

- **7 phases when 4 suffice**: cut + layout + ocr + read are doing what one
  2026 model does in one pass, often better.
- **Docker for everything**: 3 of 6 images are pure Python — uv solves this
  with native GPU access, faster dev loop, smaller install.
- **Arabic-specific defaults bleeding into core**: Qari as default OCR,
  Adab.md as default prompt corpus. Move to language-profile plugins per the
  "language-agnostic core" constraint.
- **No local translation path that's actually competitive**: Ollama fallback
  today is "it runs", not "it's good". With Gemma 3 27B Q4 and llama.cpp /
  MLX, local can now match cloud on ar+tr quality for most inputs.

## What current architecture gets wrong but already mitigated (or unfixable)

- **Arabic two-column RTL reading order is broken on every open VLM** —
  including current DocLayout-YOLO + Qari-OCR. Not a multi-phase-vs-collapsed
  decision; either way you need a geometry-based RTL fixup. Cheap, bounded,
  deterministic.

---

## Strategic options

### Option A — Conservative refactor (~2 weeks)

- Drop 3 Python-wrapping Docker images (Surya / DocLayout-YOLO / Qari-OCR);
  manage via uv-managed venvs invoked as subprocesses.
- Add local-translation provider: Gemma 3 12B/27B and Qwen3 8B/32B via
  llama.cpp (Linux/Windows) and MLX-LM (Apple Silicon).
- Keep all 7 pipeline phases unchanged.
- Keep Docker for XeLaTeX / pandoc / Poppler.
- Quality: same as today (no model upgrade).
- Simplicity: 6 Docker images → 3. ~1-2 KLoC of Go change.
- Risk: very low. Each piece is independently revertible.

**When to pick:** if you want the smallest, safest move and don't believe
the OmniDocBench numbers transfer to your real workload.

### Option B — Aggressive collapse + uv (~4-6 weeks) ⭐ recommended

Everything in A, plus:

- Replace `cut + layout + ocr + read` with a single `parse` phase backed by
  **MinerU 2.5-Pro** (primary) or **dots.mocr** (runner-up). Region-based
  JSON output flows straight into existing solve/translate/write.
- Keep the old four phases retained behind a `--multi-phase` mode for the
  pathological inputs where the unified VLM regresses (we'll know which from
  the spike).
- Engineer a **bbox-geometry RTL reading-order fixup** that runs on the
  parse output for Arabic-detected pages. Same fixup works for both
  collapsed and multi-phase paths.
- Drop XeLaTeX as required path: start a parallel **Typst 0.14** lane for
  PDF (kashida + character-level justification + RTL fixed in 0.14). Keep
  XeLaTeX as the reference renderer until Typst proves equivalent on real
  Arabic input.
- Drop pandoc Docker via `pypandoc-binary` (1.17, Mar 2026, ships pandoc in
  the wheel).
- Drop Poppler Docker — system `pdftoppm` or PyMuPDF directly (PyMuPDF may
  obviate the cut phase entirely since MinerU accepts PDFs).
- Move Adab / Qari / RTL to per-language profile plugins under
  `internal/lang/<code>/`. Default profile is empty.

End state: 4 phases (cut → parse → translate → write), 6 Docker images → 1
or 0. Quality goes up on most layouts (MinerU 95.69 OmniDocBench v1.6 beats
the current pipeline), at least matches on Arabic single-column, and ties on
Arabic two-column where the geometry fixup carries.

**Risk: medium.** The MinerU output needs validation on real workspace
inputs. The Arabic 2-col fixup is bounded but new code. Estimated 4-6
person-weeks full-time-solo; calendar 1.5-3 months part-time.

### Option C — Python rewrite + collapse (10-16 weeks)

A full rewrite of the orchestrator in Python + uv + pydantic v2 +
cyclopts + asyncio.

- Same model picks as B.
- In-process ML calls, no subprocess boundary.
- Loses: single static binary distribution, compile-time strict types, the
  `context.Context` shutdown ergonomics.
- Gains: shared language with ML tools, notebook-driven debugging of the
  layout/parse pipeline, ~30% denser source code for I/O glue.
- Risk-concentrated pieces: provider failover chain, single-binary
  distribution.

**Not recommended.** The rewrite's strongest selling point (in-process ML)
is captured by B's uv hybrid for ~10× less effort. Pick C only if
notebook-driven debugging of the parse pipeline becomes the primary dev
loop — which the current spec doesn't require.

---

## Recommendation: Option B

Why B and not A:
- **Quality**: B picks the model that beats the current pipeline on every
  public benchmark and matches the worst case (Arabic 2-col, where the
  geometry fixup carries either path). A keeps current quality.
- **Simplicity**: B collapses 7 phases to 4 and 6 Docker images to 1.
  That's the bigger simplicity win.
- The Arabic 2-col fixup is required for **either** path — it's not a B
  cost, it's a pipeline cost.

Why B and not C:
- The Python upside C buys (in-process ML calls) is irrelevant for an
  LLM-API-bound workload. Document translation cost is in API latency, not
  in subprocess overhead.
- The Go strengths the user already paid for in P0-P3 (signal handling,
  atomic writes, strict-typed schemas, single-binary deploy) cost real
  effort to recreate in Python.
- 10-16 weeks is the wrong place to spend the next month.

---

## Concrete plan for Option B

### Phase 0 — Spikes (2-3 days, must do before committing)

Pre-existing workspace has Arabic books in `input/`. Use them as the
benchmark, not synthetic data.

1. **MinerU 2.5-Pro on 5 single-column + 5 two-column Arabic pages, plus 5
   mixed-language pages.** Compare to current pipeline output for:
   - region count and types
   - text accuracy (manual diff on 3 sample pages)
   - reading order (Arabic 2-col specifically)
   - latency per page
2. **Gemma 3 27B Q4 via llama.cpp** on 10 already-solved pages from the
   workspace. Compare its ar→tr output to current Gemini/Groq output by
   side-by-side reading. Measure local-vs-cloud quality gap.
3. **Typst 0.14 with Arabic test fixture** — does the existing write phase's
   LaTeX template have a Typst equivalent with acceptable output?

**Decision gate**: if MinerU regresses on real workspace Arabic input
(quality or latency), demote Option B to Option A and revisit in 3 months.

### Phase 1 — Generic uv-managed Python tool helper (~1 week)

- `internal/pyhelper/`: install/start/stop/communicate with a uv-managed
  Python tool. One place for env bootstrap, version pinning,
  uv.lock-per-tool.
- Migrate existing Surya / DocLayout-YOLO / Qari-OCR Docker wrappers to use
  it. Old Docker paths retained behind a `--use-docker-python` flag for
  exactly one release for rollback.
- Taskfile.yml updates for uv installation and tool bootstrap.

### Phase 2 — Parse phase via MinerU (~1.5 weeks)

- New `internal/parse/` package with two backends: `vlm` (MinerU 2.5-Pro
  via vLLM or MLX) and `pipeline` (MinerU's pipeline backend, used for tail
  languages and CPU-only).
- New config block `parse: { backend, model, ... }`. Old `layout`, `ocr`,
  `read` blocks become legacy/multi-phase-only.
- Output schema = existing region-based JSON. MinerU's
  `bbox + category + text + reading_order` JSON maps directly. Categories
  may need a thin remap if MinerU's set doesn't match the current
  `header / entry / footnote / ...` enum exactly.
- New CLI commands `mutercim parse` and updated `mutercim all`.
- `--multi-phase` flag re-engages the old `layout → ocr → read` path for
  comparison and pathological inputs.

### Phase 3 — Arabic 2-col RTL geometry fixup (~3-5 days)

- Per-region script classifier (cheap Unicode-histogram on extracted text).
- When ≥2 column-grouped regions are Arabic script: reverse the column
  order. Bbox geometry tells you the columns; script classifier tells you
  the direction.
- Runs as a post-process on parse output. Works for both `vlm` and
  `pipeline` backends, and for `--multi-phase` mode (since the same bug
  exists in current pipeline). One implementation, three uses.
- Test corpus: collect 10-15 known-good Arabic 2-col pages with manually
  verified reading order.

### Phase 4 — Local translation path (~1 week)

- New provider: `local-llamacpp` (Linux/Windows/Mac CPU+CUDA) and
  `local-mlx` (Apple Silicon). Both implement the existing `Provider`
  interface so the failover chain works unchanged.
- Default model recommendations baked into config templates per platform:
  - T1 (8 GB): `gemma-3-12b-it-qat-q4_0` via llama.cpp / `mlx-community/gemma-3-12b-it-4bit` via MLX
  - T2 (24 GB): `gemma-3-27b-it-q4_K_M` / `mlx-community/gemma-3-27b-it-4bit`
  - T2 Apache-2.0 alt: `qwen3-32b-q4_K_M` / `mlx-community/Qwen3-32B-4bit`
- Chunking to ≤2k input tokens regardless of context window (the
  long-context degradation finding from the research).
- Cloud providers stay as the default chain; local moves up the priority
  list when the user is offline (detect once at startup, not per call —
  too expensive).

### Phase 5 — Rendering simplification (~3-5 days)

- `internal/render/docx`: switch from Docker pandoc to
  `pypandoc-binary` via the Phase 1 pyhelper. Removes pandoc image.
- `internal/render/pdf`: add Typst backend alongside existing XeLaTeX.
  Config flag `write.pdf_engine: xelatex | typst`. Default still
  `xelatex` until Typst proven on real Arabic.
- `internal/cut` (if not folded into parse): system `pdftoppm` instead of
  Docker. Removes Poppler image. Or — preferred — let MinerU's parse phase
  consume PDFs directly and retire the cut phase entirely.

### Phase 6 — Language-agnostic plugin surface (~3-5 days)

- `internal/lang/<code>/`: per-language profile with optional script
  detection, OCR override (e.g. Qari for `ar`), prompt corpus (e.g.
  Adab.md for `ar` literary), RTL rendering hints.
- Default profile is empty: no Arabic-specific defaults in core code.
- Workspace `knowledge/lang/<code>/` for user overrides (extends existing
  knowledge layering).

### Phase 7 — Docs + README sweep (~2 days)

- Update README pipeline diagram from 7 phases to 4.
- New CLI commands, new config schema.
- Migration note for existing workspaces (mostly auto-handled — output
  schemas are unchanged; only phase boundaries shift).

**Total estimate**: 4-6 person-weeks full-time, or 6-12 calendar weeks at
half-time. Each phase is independently shippable.

---

## Decisions I need from you before starting

1. **Approve Option B?** (or A, or C, or a hybrid I haven't named.)
2. **License strictness on translation model**: Gemma TOS is permissive but
   not Apache-2.0. Qwen3 is Apache-2.0. If you redistribute output or
   commercialize, default to Qwen3. If personal use, Gemma 3 27B is the
   stronger pick.
3. **Arabic 2-col workload share**: high (do the geometry fixup as P0 inside
   Phase 2) or low (defer to a follow-up after Phase 2 ships)?
4. **Cloud LLM positioning**: do clouds (Gemini/Claude/etc.) stay default
   with local fallback (today's model — easier, "works while online")? Or
   flip to local-default with cloud as quality boost (matches the
   local-first constraint more strictly — but heavier local resource
   commitment per page)?

---

## Things I deliberately did *not* recommend

- **Cloud-only single-shot APIs** (Mistral Document AI, Gemini Files API):
  ruled out by local-first.
- **WeasyPrint for PDF**: 13-year-old RTL tracking issue (Kozea/WeasyPrint
  #106), multiple active RTL bugs in 2025-2026.
- **Tower+, ALMA, Aya Expanse for translation**: either don't cover ar+tr
  or have CC-BY-NC license.
- **Surya OCR for Arabic**: KITAB-Bench and E-ARMOR flagged high error
  rates. Surya stays useful for layout / reading order only.
- **layout-parser**: declared inactive.
- **Tesseract for Arabic**: ~60% worse CER than VLM OCR.
- **PyOxidizer for Python distribution**: too slow-moving.
- **"unoffice"** (from current PLAN.md): could not be verified as an active
  project. Remove from PLAN.

---

## What I would re-research in 3-6 months

- WheelNext / variant wheels in uv: when this lands stable, the
  "drop the last 3 system-binary Docker images" trade-off changes.
- MinerU's Arabic two-column reading-order fix: if upstream lands it, the
  geometry fixup becomes obsolete.
- TranslateGemma 2k input cap: if Google extends it past 8k, it becomes the
  default translation pick.
- Qwen-MT weights release: if Alibaba ever open-weights it, the local-vs-
  cloud gap on ar+tr shrinks dramatically.
- New entrants on OmniDocBench v1.7+: this field shipped 4 new SOTA models
  in the 6 months covered by this research; expect another 2-4 by Nov 2026.
