---
date: 2026-05-12
author: research run
topic: host language (Go vs Python) + Docker-vs-uv + RTL rendering
scope: mutercim
status: draft for user decision
---

# Host language, Docker-vs-uv, and RTL rendering — May 2026

## Actual codebase size (correcting the prompt)

The prompt assumes "~50 KLoC Go". Measured via `wc -l` on `*.go` (excluding `vendor/`):

| Bucket | Lines |
|---|---|
| Non-test Go | **12,413** |
| Test Go | 14,209 |
| Total Go | 26,622 |
| Source files | 140 |

So this is a ~12 KLoC orchestrator with a ~14 KLoC test corpus. That changes the migration arithmetic in Part 5. The user should anchor the conversation on real numbers, not the "50 KLoC" figure.

## TL;DR

uv is production-ready and lets you drop Docker for the orchestrator's *Python* deps (Surya, DocLayout-YOLO, Qari-OCR) on Linux/macOS, but Docker should stay for the *non-Python* binaries with hard system requirements (XeLaTeX, pandoc, Poppler) — drop the Python-wrapping containers, not the toolchain ones. A full Python rewrite is **not justified**: the Go orchestrator is 12 KLoC of mostly typed I/O glue, and the things Go does well here (single binary, signal handling, atomic writes, strict-typed config) are exactly the things Python regresses on. The lowest-risk path is **Go orchestrator + native Python via uv + Typst (not WeasyPrint) for RTL PDF + pandoc subprocess for DOCX**.

---

## Part 1 — uv state, May 2026 — drop Docker for Python deps?

### What uv solves vs Docker

| Concern | uv | Docker |
|---|---|---|
| Pin Python version | Yes — `requires-python` + uv-managed CPython | Yes — base image |
| Pin Python pkg versions w/ hashes | Yes — `uv.lock` with SHA-256 per wheel ([uv docs, "Locking and syncing"](https://docs.astral.sh/uv/concepts/projects/sync/)) | Yes — pip freeze in image |
| Cross-platform lockfile | Yes — single `uv.lock` for all markers ([uv docs](https://docs.astral.sh/uv/concepts/projects/sync/)) | No — per-arch image |
| GPU/CUDA wheel selection | Yes via `sys_platform` markers and explicit indexes — but manual config ([uv pytorch guide](https://docs.astral.sh/uv/guides/integration/pytorch/)) | Yes via base image (e.g. `nvidia/cuda`) |
| Pin native system libs (libGL, fontconfig, libreoffice, tex) | **No** | Yes |
| Pin Python C-ext **runtime** ABI / glibc | Partial (manylinux wheels) | Full |
| Startup cost on cold pull | ~5 s warm cache ([uv docs](https://docs.astral.sh/uv/)) | 30 s – several min per image |
| Disk footprint | Wheels cache (few hundred MB) | GBs per image |
| Cross-host portability | Per-platform lock | Identical image |
| Single-machine reproducibility | Excellent | Excellent |
| Cross-machine reproducibility (glibc/kernel/CUDA driver) | Best-effort | Strong |

### PyTorch + CUDA recipe in 2026

The official uv guide ([uv-pytorch-official-guide](https://docs.astral.sh/uv/guides/integration/pytorch/)) requires manual gating:

```toml
[tool.uv.sources]
torch = [
  { index = "pytorch-cpu",   marker = "sys_platform != 'linux'" },
  { index = "pytorch-cu130", marker = "sys_platform == 'linux'" },
]
```

This is **not the same as "just works"**. PyTorch has *no* CUDA wheels for macOS, so Mac uses MPS via the CPU/Metal wheel; on Linux you pick a specific CUDA version (cu118/cu126/cu128/cu130/rocm7.2/xpu). Per the uv blog ([wheel-variants experimental, Aug 2025](https://astral.sh/blog/wheel-variants)), the "WheelNext" effort with PyTorch/NVIDIA/Quansight is moving toward `uv pip install torch` auto-detecting the GPU, but as of May 2026 this is **experimental**, not the default. Known bug: `--torch-backend=auto` ignores CUDA compute capability on some older cards (e.g. GTX 1080 Ti — [astral-sh/uv#14742](https://github.com/astral-sh/uv/issues/14742)).

### Apple Silicon

PyTorch MPS works via the macOS CPU wheel (PyTorch ships unified macOS wheels that use Metal when available — no separate CUDA-style "+mps" wheel exists). MLX is pip-installable, no special uv handling, but it's pure-CPU/GPU Python with native dylibs — uv pins the Python wheel, which contains the dylib, so reproducibility is per-wheel-hash. Caveat: MLX is Apple-Silicon-only; lockfiles must mark it `sys_platform == 'darwin' and platform_machine == 'arm64'`.

### Multi-Python

`pyproject.toml` with `requires-python = ">=3.12,<3.14"` is honored; uv manages multiple interpreters via `uv python install 3.11 3.12 3.13`. Real issue (HN, [Oct 2025](https://news.ycombinator.com/item?id=45751400)): packages with `setup.py` using legacy `tests_require`/`test_suite`/`use_2to3` still fail to build from sdist (HN comment shows a `blessings==1.6` failure). This is a sdist-build problem, not uv-specific — pip would fail the same way — but `uv build`'s error message is uglier.

### Real failure modes reported in 2025-2026

1. **CUDA driver / kernel skew**: uv pins the wheel, but the host driver isn't pinned. A locked `torch==2.5.0+cu124` works on host A, fails on host B with a different driver. This is exactly what Docker base images fix and uv structurally cannot.
2. **Build-from-sdist for old packages** still requires C/C++ toolchain on host (HN thread above).
3. **Native dylibs vendored in wheels** (libGL, libstdc++, libopenblas) sometimes conflict — e.g. `numpy` + `scipy` + `torch` each bundle different OpenBLAS, harmless on most hosts but breaks deterministically on a few distros.
4. **Heavy C-ext like `transformers`/`flash-attn`**: flash-attn historically required `nvcc` at install time and built from source per machine. As of late 2025 they ship wheels matching CUDA versions, but `uv pip install flash-attn --no-build-isolation` is still the documented incantation, not `uv add flash-attn`.

### Recommendation for mutercim

**Hybrid: drop the Python-wrapping containers, keep the system-binary containers.**

| Current Docker image | What it wraps | Recommendation |
|---|---|---|
| `docker/poppler` | Poppler/pdftoppm (C, system binary) | **Keep**. Or use `homebrew`/`apt` directly. Not a Python concern. |
| `docker/pandoc` | Pandoc (Haskell, system binary) | **Keep**. Or use system pandoc; pypandoc-binary wheel exists ([pypandoc on PyPI](https://pypi.org/project/pypandoc-binary/), v1.17, Mar 2026). |
| `docker/xelatex` | TeX Live (huge system install) | **Keep**, or migrate target → Typst (Part 3). |
| `docker/doclayout-yolo` | Python + ultralytics + torch | **Drop**. Pure Python; manage with uv. |
| `docker/surya` | Python + transformers + torch | **Drop**. Pure Python; manage with uv. |
| `docker/qari-ocr` | Python + transformers + torch | **Drop**. Pure Python; manage with uv. |

This removes 3 of 6 images. The remaining 3 wrap *non-Python* tools where Docker is the actual right answer (TeX Live in particular is 4 GB+; nobody wants to manage that without containers). Result: faster cold-start, simpler dev loop on host, Docker only for the heavyweight system installs.

**When to revisit "all Docker → all uv":** when WheelNext + variant wheels become non-experimental in uv (likely 2026 H2), the CUDA-driver-on-host issue is the main remaining blocker. For mutercim's local-first stance the Docker fallback for prod is fine.

---

## Part 2 — Python orchestrator stack (if we pivot)

This is the stack I'd pick today for a fresh `mutercim` in Python. Each cell is the recommended pick; sibling tools listed for the record.

| Concern | Pick | Runners-up | Notes |
|---|---|---|---|
| CLI | **cyclopts** | typer, click, argparse | cyclopts handles `Union[]`/`Literal[]` natively; Typer pushes you through Click for anything non-trivial ([cyclopts vs typer](https://cyclopts.readthedocs.io/en/latest/vs_typer/README.html)). For mutercim's strict-typed flags, cyclopts is the closest to cobra's ergonomics. |
| Config | **pydantic-settings** | dataclasses+yaml, dynaconf, hydra | Layered loading from env/`.env`/YAML with pydantic v2 validation. Replaces viper's "env > flag > file > default" precedence cleanly. |
| Typed schema | **pydantic v2** | dataclasses+jsonschema, msgspec, attrs+cattrs | Discriminated unions for region kinds (`text` / `image` / `table`) — direct fit for region-based schema. `Field(discriminator='kind')` gives single-pass O(1) dispatch ([pydantic discriminated unions](https://docs.pydantic.dev/latest/concepts/unions/)). msgspec is faster but less ergonomic. |
| HTTP | **httpx async** | aiohttp, requests | httpx has matching sync+async APIs, HTTP/2, native typing. |
| Retry/backoff | **tenacity** | hand-rolled | Decorator-based, async-aware, `wait_exponential` + `retry_if_exception_type(RateLimitError)` is the LLM-API standard ([tenacity 2026 guide](https://johal.in/tenacity-retries-exponential-backoff-decorators-2026/)). |
| Rate limit | **aiolimiter** | asyncio-throttle | Token-bucket with `async with limiter`. Compose with tenacity. |
| Logging | **structlog** | loguru, stdlib | structlog wins for production: ~25% faster JSON than loguru, gets OTel free via stdlib bridge ([Dash0 2026 comparison](https://www.dash0.com/guides/python-logging-libraries)). Loguru is fine if you don't need OTel. |
| Async runtime | **asyncio stdlib** | trio, anyio | asyncio is the default; anyio if you want trio-compat. No reason to deviate. |
| File atomics | **stdlib `os.replace`** + `tempfile` | atomicwrites (unmaintained) | Same pattern as Go: tmp + rename. |
| Signal handling | **`signal` + `asyncio.add_signal_handler`** | — | Manual; less polished than Go's `signal.Notify`. |
| Single-binary dist | **PyInstaller** or **shiv** | PyOxidizer (slow-moving), Nuitka | PyInstaller works but ships 100MB+. Nuitka compiles to C, smaller and faster but compile-time is long. For mutercim distribution, this is a real downgrade from Go's `go build -ldflags="-s -w"` → 20 MB static binary. |

### Concrete loss vs current Go stack

- **Single binary**: Go `go build` → single static binary, no runtime. PyInstaller produces ~100 MB bundles with libpython, slow startup, and OS-specific. This is a real regression for end users installing the tool.
- **Static typing at compile**: pydantic catches at runtime, not at build. mypy/pyright at CI is required to recover this — extra discipline, extra tooling.
- **Concurrency for the orchestrator layer**: Go goroutines + channels for the failover/rate-limit fan-out are very clean. Python asyncio is fine but uglier (no native channel; semaphore + queue + `asyncio.gather`).
- **`context.Context`-style cancellation**: Python's `asyncio.CancelledError` propagation is correct but less well-known; people get it wrong more often.

### Concrete gain vs current Go stack

- **Native ML libs in-process**: no HTTP/Docker boundary. Surya/DocLayout-YOLO/Qari-OCR called as Python imports, zero IPC overhead.
- **Schema validation ergonomics**: pydantic discriminated unions express the region schema in ~30% less code than Go's tagged-union-via-interface pattern.
- **Hot-reload during dev**: shave 5-30 s per iteration; nontrivial over a project lifetime.

### Migration cost estimate (concrete)

- **Source code**: 12.4 KLoC non-test Go → expected ~9-12 KLoC Python (Python is ~30% denser for I/O glue per the 1000:1300 ratio reported in [Medium 2025 rewrite](https://medium.com/@build_break_learn/python-vs-go-the-10x-faster-api-rewrite-that-added-0-value-46f71c06ec68), but pydantic models inflate a bit vs Go structs).
- **Test code**: 14.2 KLoC Go tests → ~12-15 KLoC Python tests. pytest is denser than `testing.T` but parametrize boilerplate adds it back.
- **File count**: ~140 Go files → ~80-110 Python modules (Python packs more per file).
- **Risk areas** (will eat schedule):
  1. Reproducing Go's `context.Context` cancellation semantics in asyncio — non-trivial in the failover-chain code (`internal/apiclient`, `internal/provider`).
  2. Multi-provider HTTP client with per-model rate limits — this is the orchestrator's most subtle code; rewriting it is the highest-risk single piece.
  3. Atomic JSON-state writes — easy to get wrong without explicit fsync (`os.replace` doesn't fsync the dir by default).
  4. CLI parity: cobra subcommand trees and viper precedence don't map 1:1; expect rework on each `cmd/`.
- **Schedule, solo, full-time**: 6-9 weeks for code parity + tests; 2-3 weeks for ecosystem hardening (single-binary build pipeline, CI matrix, distribution). **Total: 8-12 person-weeks.** Add 30% for unknowns → **10-16 person-weeks**.
- **Schedule, solo, part-time (~50%)**: double the above → **5-8 calendar months**.

### Question that decides it

If the goal of the rewrite is "avoid Docker for ML tools", the rewrite is overkill — Part 1's hybrid achieves this without touching the orchestrator. The rewrite is only justified if Python ML calls become the dominant hot path *and* the IPC overhead is measurably hurting throughput. With document translation that's I/O-bound on the LLM API, this is unlikely.

---

## Part 3 — PDF rendering with RTL (Arabic) — option matrix

| Tool | RTL/Bidi | Arabic shaping | Native? | Local-first? | Verdict |
|---|---|---|---|---|---|
| **XeLaTeX + polyglossia** (current) | Excellent | Excellent (HarfBuzz via XeTeX) | System binary, ~4 GB TeX Live | Docker | **Reference quality**. Production-grade for Arabic, used by publishers. ([polyglossia on CTAN](https://ctan.org/pkg/polyglossia)) |
| **Typst 0.14** + typst-py | Good and improving — bidi + kashida + character-level justification added in 0.14, RTL crash fixed in 0.13 ([Typst 0.14 blog, Oct 2025](https://typst.app/blog/2025/typst-0.14/)) | Good (HarfBuzz) | Single ~30 MB binary | Single binary, no Docker | **Strong candidate**. Faster than LaTeX, no 4 GB install. RTL is no longer a known crash. Maturity gap vs XeLaTeX is shrinking. |
| **WeasyPrint 68.x** (HTML/CSS → PDF) | Partial — long-standing partial support since 2013; floated/column layout broken in RTL ([Kozea/WeasyPrint#106](https://github.com/Kozea/WeasyPrint/issues/106), [#574](https://github.com/Kozea/WeasyPrint/issues/574), [#1110](https://github.com/Kozea/WeasyPrint/issues/1110)). Text duplication bug filed in 2024 ([#1686](https://github.com/Kozea/WeasyPrint/issues/1686)). | Glyph shaping OK | Pure Python + Pango/Cairo | Yes | **Not recommended**. Edge cases in RTL still active 13 years after the tracking issue opened. |
| **PyMuPDF (fitz) `insert_htmlbox` / Story** | Yes via HarfBuzz Story class | Yes — HarfBuzz integration ships ([PyMuPDF docs](https://pymupdf.readthedocs.io/en/latest/recipes-text.html)) | C binding | Yes | **Viable for direct box placement**. Not a layout engine — you place runs, it shapes them. Great for region-positioned reassembly (matches mutercim's region pipeline). Not great if you need flowed reflowed multi-page text. |
| **ReportLab 4.4+** + rlbidi | New in 4.4 (Apr 2025) using HarfBuzz, **experimental** ([ReportLab Arabic docs](https://docs.reportlab.com/rl-arabic/)) | New, experimental | Pure Python | Yes | **Wait**. Too new for a quality-first pipeline. Revisit in 12 months. |
| **ReportLab + arabic_reshaper + python-bidi** (legacy) | Manual reshaping then bidi — workaround pattern, not real bidi | Workaround | Pure Python | Yes | **Workaround**. Standard for years but breaks on mixed-direction text. Avoid for production output. |
| **PyLaTeX → XeLaTeX** | Same as XeLaTeX | Same as XeLaTeX | Needs XeLaTeX system binary | Docker still needed | **Same as XeLaTeX with Python templating**. No advantage over current pipeline unless rewriting orchestrator in Python. |
| **LibreOffice headless** | Excellent (LO bidi is mature) | Excellent | 500MB+ install | Subprocess | Heavy. Better for DOCX (Part 4). |

### Recommendation (Part 3)

Primary: **keep XeLaTeX as the reference renderer for now**; it's the production-quality Arabic engine.

Strategic move: **start a parallel Typst lane** for new outputs. Typst 0.14 (Oct 2025) added kashida-based character-level justification and fixed the RTL crash bug — the two pieces that had been blockers. The `typst-py` binding ([messense/typst-py](https://github.com/messense/typst-py), v0.14.x in Jan-Feb 2026) gives you `typst.compile()` from Python with no system install. A/B the two on real Arabic input; if Typst matches XeLaTeX on quality for mutercim's content shape, retire the XeLaTeX Docker image.

Avoid: **WeasyPrint** (RTL still half-baked), **legacy reportlab+arabic_reshaper** (workaround pattern).

For region-positioned PDF assembly (where the layout phase has already produced bounding boxes), **PyMuPDF Story** is the right tool — it's not a competing layout engine, it shapes runs at given positions.

---

## Part 4 — DOCX output — option matrix

| Tool | RTL support | Complex layouts | Local-first | Verdict |
|---|---|---|---|---|
| **Pandoc** (current) | Strong — pandoc handles bidi via reference doc | Strong via reference.docx | System binary | **Keep**. Mature, well-understood. Either as Docker, system install, or `pypandoc-binary` wheel ([PyPI](https://pypi.org/project/pypandoc-binary/) v1.17, Mar 2026, ships pandoc inside the wheel). |
| **python-docx** raw | Partial — `rtl=True` on paragraph works but conflicts with font name setting ([#430](https://github.com/python-openxml/python-docx/issues/430)); no high-level text-direction API ([#349](https://github.com/python-openxml/python-docx/issues/349)) | Manual XML | Pure Python | **Not recommended for from-scratch**. OK for surgical edits of an existing docx. |
| **docxtpl (python-docx-template)** | RichText supports `rtl=False` and `lang` ([docxtpl docs](https://docxtpl.readthedocs.io/)) — built on python-docx, inherits its limits | Jinja2 in a Word template | Pure Python | **Good for template-driven flows**. Not enough for arbitrary layouts. |
| **LibreOffice headless** (`soffice --headless --convert-to docx`) | Excellent | Excellent | 500MB install | **Heavy but bulletproof**. Best path: render to PDF/HTML first, convert via LO. |
| **"unoffice"** (mentioned in PLAN.md) | Could not locate an active project by that name in May 2026. Possibly confusion with "unoconv" (which is itself a LO wrapper, unmaintained since 2022) or "office365" SDK. | — | — | **Treat as not viable**. If user has a specific link, reassess. |

### Recommendation (Part 4)

**Keep pandoc.** It's the right tool. The choice is *how* to invoke it:

- Option A — Pandoc as Docker image (current). Works, slow cold start.
- Option B — System pandoc, called via subprocess. Fastest, requires host install.
- Option C — `pypandoc-binary` wheel — pandoc ships inside Python wheel, no system install, no Docker. Adds ~80 MB to install but everything's in one place.

For mutercim's local-first goal, **Option C if Python pivot, Option B if Go stays.**

Note: "unoffice" in PLAN.md needs verification — I could not find a viable project by that name. If the reference is wrong, drop it from the plan.

---

## Part 5 — Go vs Python: honest recommendation

### What Go does well here that Python regresses on

1. **Single static binary** — `go build` → 20 MB, no runtime. Python → 100 MB PyInstaller bundle, slow startup, glibc-sensitive. For a CLI tool users install, this matters.
2. **Compile-time strict typing** — Go's region-schema with discriminated tagged unions via interfaces, validated at build. Python equivalent (pydantic v2) is runtime-only; recovering compile-time enforcement requires mypy/pyright + CI discipline.
3. **`context.Context` cancellation** — Go's idiom for graceful shutdown is uniform across stdlib. Python's `asyncio.CancelledError` works but is less idiomatic; harder to get right under load.
4. **Concurrency for orchestration** — goroutines + channels for fan-out/fan-in of provider failover with per-model rate limits is very clean in Go. asyncio + semaphores is workable but noisier.
5. **`signal.Notify` graceful shutdown** — already implemented, robust. Python equivalent works but needs more care.
6. **Atomic writes with `os.Rename` + dir fsync** — Go has the dir fsync idiom widely understood; Python's `os.replace` is atomic but the dir fsync step is often skipped.

### What Go does poorly here that Python wins on

1. **In-process ML calls** — every ML tool is Python, so the current architecture has Go-orchestrator → Docker-Python-wrapper → ML-library. Python orchestrator collapses this to one process.
2. **Region-schema authoring** — pydantic v2 discriminated unions express the schema in fewer lines and with better error messages than Go's tagged-interface pattern.
3. **Iteration speed during development** — hot-reload, REPL exploration of intermediate state. Go requires rebuild per change.
4. **Notebook-style debugging of layout/OCR output** — Python wins here decisively because the ML libraries are native Python.

### The honest call

The user's stated priority is **quality > simplicity > speed > maintainability**. With those weights:

- **Quality**: tied. Both can produce the same output. The output quality depends on the ML tools and the prompts, not the orchestrator language.
- **Simplicity**: Python *might* win because it removes the Docker-Python wrapping layer, but loses on distribution (one binary vs PyInstaller bundle). Net: roughly tied, with Python slightly ahead if you don't ship to non-Python users.
- **Speed**: Go wins on cold-start and concurrent fan-out; Python wins on inner-loop ML iteration. For document translation (LLM-API-bound), neither matters.
- **Maintainability**: Go wins on type-safety-at-build, single-binary deploy, signal handling. Python wins on shared language with ML tools, faster iteration. Roughly tied with different failure modes.

**Recommendation: do not rewrite. Adopt the hybrid.**

The Go orchestrator is the right host for a tool that ships as a CLI, must shut down cleanly on signals, and gates external Python tools. The Python pivot's strongest selling point (in-process ML calls) is undermined by the cost of giving up the Go strengths the user already paid for in P0-P3.

Instead, the targeted refactors that get most of the Python upside without the rewrite:

1. **Drop Python-wrapping Docker images** (Surya / DocLayout-YOLO / Qari-OCR) — replace with `uv`-managed virtualenvs invoked as subprocesses. Keep XeLaTeX/pandoc/poppler containers. (Part 1 hybrid.)
2. **Pilot Typst** as a parallel PDF lane to XeLaTeX. If Typst's Arabic quality is acceptable on real input, retire the 4 GB XeLaTeX image. (Part 3.)
3. **Switch DOCX path** to either system pandoc or `pypandoc-binary`-via-Python-subprocess. Drop the pandoc image. (Part 4.)
4. Net effect: 6 Docker images → 1 (or 0) within ~2 weeks of work. No rewrite cost.

**If the user still wants Python orchestrator** (legitimate reasons exist — e.g., they want notebook-driven debugging of the layout pipeline as the primary dev loop), the budget is **10-16 person-weeks** full-time-solo for a parity rewrite + ecosystem hardening. The risk-concentrated pieces are the provider failover chain (Go's concurrency idioms don't port literally to asyncio) and the single-binary distribution (Python regression).

---

## Sources

### uv / packaging
- [uv official docs](https://docs.astral.sh/uv/) — Astral, 2025-2026
- [Using uv with PyTorch — uv official guide](https://docs.astral.sh/uv/guides/integration/pytorch/) — Astral
- [Locking and syncing — uv docs](https://docs.astral.sh/uv/concepts/projects/sync/)
- [Astral blog: PyTorch wheel variants experimental](https://astral.sh/blog/wheel-variants) — Aug 2025
- [astral-sh/uv #14742 — torch-backend=auto ignores CUDA compute capability](https://github.com/astral-sh/uv/issues/14742)
- [HN: A year of uv (2214 points, 1324 comments)](https://news.ycombinator.com/item?id=45751400) — Oct 2025
- [PyTorch blog: Wheel Variants, the Frontier of Python Packaging](https://pytorch.org/blog/pytorch-wheel-variants/)
- [Best Python Package Managers 2026 — scopir.com](https://scopir.com/posts/best-python-package-managers-2026/)

### Typst / PDF
- [Typst 0.14: Now accessible — Typst blog](https://typst.app/blog/2025/typst-0.14/) — Oct 2025 (mentions kashida + character-level justification + RTL improvements)
- [Typst 0.13 blog — RTL crash fix, Arabic CSL updates](https://typst.app/blog/2025/typst-0.13/)
- [Typst 0.14.0 changelog](https://typst.app/docs/changelog/0.14.0/)
- [typst-py GitHub (messense)](https://github.com/messense/typst-py) — v0.14.8 Feb 2026
- [typst-py on PyPI](https://pypi.org/project/typst/)
- [WeasyPrint RTL support issue #106](https://github.com/Kozea/WeasyPrint/issues/106) — open since 2013
- [WeasyPrint RTL Column #574](https://github.com/Kozea/WeasyPrint/issues/574)
- [WeasyPrint RTL floated elements #1110](https://github.com/Kozea/WeasyPrint/issues/1110)
- [WeasyPrint duplicate text RTL #1686](https://github.com/Kozea/WeasyPrint/issues/1686)
- [PyMuPDF Story / insert_htmlbox docs](https://pymupdf.readthedocs.io/en/latest/recipes-text.html)
- [PyMuPDF Mastering insert_htmlbox blog — Artifex](https://artifex.com/blog/mastering-pdf-text-with-pymupdfs-insert-htmlbox-what-you-need-to-know)
- [ReportLab Arabic support docs](https://docs.reportlab.com/rl-arabic/) — 4.4+ experimental, Apr 2025
- [arabic-reshaper PyPI](https://pypi.org/project/arabic-reshaper/)
- [python-bidi PyPI](https://pypi.org/project/python-bidi/)
- [CTAN polyglossia](https://ctan.org/pkg/polyglossia)
- [ieatpdf: Python PDF toolkit for Arabic — 2026-04-27](https://earezki.com/ai-news/2026-04-27-i-built-a-free-pdf-toolkit-that-properly-handles-arabic-documents/)

### DOCX
- [pypandoc PyPI](https://pypi.org/project/pypandoc/)
- [pypandoc-binary PyPI v1.17](https://pypi.org/project/pypandoc-binary/) — Mar 2026
- [python-docx RTL attribute disables font #430](https://github.com/python-openxml/python-docx/issues/430)
- [python-docx no text direction option #349](https://github.com/python-openxml/python-docx/issues/349)
- [docxtpl docs](https://docxtpl.readthedocs.io/)

### Python stack
- [cyclopts vs typer comparison](https://cyclopts.readthedocs.io/en/latest/vs_typer/README.html)
- [Pydantic discriminated unions docs](https://docs.pydantic.dev/latest/concepts/unions/)
- [Dash0: Choosing a Python logging library in 2026](https://www.dash0.com/guides/python-logging-libraries)
- [Tenacity retries 2026 guide](https://johal.in/tenacity-retries-exponential-backoff-decorators-2026/)

### Cost benchmarks
- [Medium: Python vs Go 10x faster API rewrite that added 0 value](https://medium.com/@build_break_learn/python-vs-go-the-10x-faster-api-rewrite-that-added-0-value-46f71c06ec68)
- [PyOxidizer vs PyInstaller comparison](https://pyoxidizer.readthedocs.io/en/stable/pyoxidizer_comparisons.html)
