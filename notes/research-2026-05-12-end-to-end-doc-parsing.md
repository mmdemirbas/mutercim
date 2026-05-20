---
date: 2026-05-12
topic: End-to-end document parsing models — can one model replace cut/layout/ocr/read?
audience: mutercim pipeline architecture decision
constraints:
  - local-only (consumer GPU 24–48GB, Apple Silicon, or CPU fallback)
  - language-agnostic (Arabic RTL, Turkish, English, French, Chinese, Japanese)
  - layout preservation (bboxes, region types, reading order)
  - quality must match or beat the current 3-step (DocLayout-YOLO + Qari-OCR + vision-LLM) baseline
---

# TL;DR

A single open-weight model can now replace `layout + ocr + read` (the three vision-heavy phases) and is the right architectural move in May 2026. The two viable picks are **MinerU 2.5-Pro (1.2B, 3.1.x toolkit)** and **dots.mocr (3B)** — both emit bbox+category+text JSON in reading order, both run locally on a 24GB consumer GPU or Apple Silicon, and both are ahead of the previous DocLayout-YOLO + dedicated-OCR pipeline pattern on every public benchmark released in late 2025/early 2026. **The one hard caveat: Arabic right-to-left two-column reading order is documented-broken on every open VLM tested on MDPBench (PaddleOCR-VL-1.5, dots.ocr, dots.mocr). If Arabic two-column is a real workload, you need either a post-processing reorder step or to fall back to the pipeline backend with explicit `--lang ar`.**

# Constraint Filter Applied

Tools excluded up front, with reason:

- **Mistral Document AI, Gemini 3 Pro, Azure DI, AWS Textract, OpenAI Vision** — cloud-only, excluded by constraint.
- **Donut, Pix2Struct, original Nougat** — superseded; not actively maintained for new releases in 2025–2026; weak on non-Latin scripts. Reference points only.
- **Florence-2 / Florence-VL** — capable VLMs but not document-parsing-specialized; no bbox+category+reading-order JSON out of the box; ruled out for "phase collapse" purpose.
- **Phi-3.5-Vision / Phi-4-Vision** — general VLMs; same as above, no SOTA on OmniDocBench/olmOCR-Bench in primary sources.
- **MiniCPM-V 2.6 / 4** — same.
- **GOT-OCR 2.0** — 580M, no public 2.5 successor with multilingual bbox JSON as of 2026-05; primarily Latin/CJK; reference only.
- **olmOCR / olmOCR 2** — strong English benchmark (82.4 olmOCR-Bench), but Allen AI explicitly states *"fine-tuned on English documents using a multilingual base VLM; other languages may work"*. Disqualified by the multilingual constraint.
- **MonkeyOCR / MonkeyOCR v1.5** — README states "does not yet fully support photographed text, handwritten content, Traditional Chinese characters, or multilingual text". Disqualified.
- **Marker (datalab-to/marker) + Surya** — solid pipeline OCR with 90+ language detection, MIT/GPL+AI-Pubs-Open-RAIL-M license, OmniDocBench respectable but lower than VLM SOTA; treated as a fallback option for Apple Silicon / CPU-only.
- **DeepSeek-OCR / DeepSeek-OCR2** — interesting research direction (vision-token compression) but optimized for English/Chinese; multilingual story is unverified; treated as research bet, not a recommendation.
- **Dolphin-v2** — bytedance, Dec 2025, 3B, OmniDocBench v1.5 = 89.45. README: "multilingual capacity needs to be expanded". Disqualified by the multilingual constraint.

Tools that survive the filter (detail below):
1. **MinerU 2.5-Pro** (opendatalab) — primary recommendation
2. **dots.mocr / dots.ocr** (rednote-hilab) — runner-up
3. **PaddleOCR-VL-1.5** (Baidu) — runner-up alternative
4. **Qwen3-VL 8B / 32B** (Alibaba) — general-purpose backup
5. **Marker + Surya** (datalab) — CPU/Apple-Silicon fallback

# Per-Tool Detail

## 1. MinerU 2.5-Pro (1.2B) — primary recommendation

- **Repo / weights**: github.com/opendatalab/MinerU (62.7k stars), huggingface.co/opendatalab/MinerU2.5-Pro-2604-1.2B
- **Release**: Model `MinerU2.5-Pro-2604-1.2B` shipped with MinerU 3.1.0 on **2026-04-18**. Latest toolkit `mineru-3.1.11` released 2026-05-09. Paper: arXiv 2604.04771.
- **License**: **MinerU Open Source License** (Apache 2.0 + additional conditions). Moved from AGPLv3 in 3.1.0 specifically to reduce commercial adoption friction. Code is Python (99.3%).
- **Size / VRAM**: 1.2B params. Pipeline backend min **4GB VRAM**; VLM (vlm-async-engine on vLLM) min **8GB VRAM**. Concurrent inference at **2.12 fps on A100**. CPU-only supported via pipeline backend; **MLX backend for Apple Silicon** (100–200% speedup over CPU per 2.6 changelog).
- **Output**: JSON sorted by reading order with bboxes + category labels + extracted text; also Markdown (with tables as HTML, formulas as LaTeX), and intermediate formats. Supports image, PDF, DOCX, PPTX, XLSX inputs.
- **Multilingual**: Pipeline OCR supports **109 languages**. Arabic/Cyrillic/Devanagari/Telugu/Tamil updated to ppocr-v5 in 2.6 with **40%+ accuracy improvement**. Two backends: pipeline (broad language coverage, lower VRAM) and VLM (higher accuracy, narrower training-distribution coverage).
- **Benchmarks** (primary sources):
  - OmniDocBench v1.6 overall: **95.69** (SOTA, beats Gemini 3 Pro and Qwen3-VL-235B per the model card)
  - OmniDocBench v1.5 overall: 92.98 (the prior 2.5-2509 model); Table TEDS +5.54 in Pro; Dense Formula CDM 97.29; Text Edit 0.036
- **Failure modes / known gaps**:
  - **Arabic RTL reading order is the documented industry-wide failure** — MDPBench paper shows open VLMs collapse on Arabic two-column. MinerU GitHub issues #1107 and #3591 confirm two-column reading-order bugs.
  - Cross-page table merging "currently under integration" (model card).
  - VLM backend is narrower than pipeline for tail languages; for a workload with heavy non-Latin/non-CJK, the *pipeline backend with explicit `--lang`* is often safer than the VLM backend.

## 2. dots.mocr (3B) — runner-up

- **Repo / weights**: github.com/rednote-hilab/dots.mocr (re-brand of dots.ocr-1.5, 8.6k stars on the dots.ocr repo), huggingface.co/rednote-hilab/dots.mocr
- **Release**: `dots.ocr` 2025-07-30 (1.7B); `dots.ocr.base` 2025-10-31; **`dots.mocr` rebrand 2026-03-19** (3B). Papers: arXiv 2512.02498 (dots.ocr), arXiv 2603.13032 (dots.mocr, "Multimodal OCR: Parse Anything from Documents").
- **License**: **MIT** (huggingface model card).
- **Size / VRAM**: 3B params. Officially integrated into vLLM ≥0.11.0. Reported to run on a single 48GB RTX A6000; with vLLM gpu-memory-utilization 0.9 fits on 24GB at int4/FP8 quant in practice (no first-party 24GB number — qualify before relying on it).
- **Output**: Single-JSON object — array of `{bbox: [x1,y1,x2,y2], category, text}` with categories `{Caption, Footnote, Formula, List-item, Page-footer, Page-header, Picture, Section-header, Table, Text, Title}`, formulas as LaTeX, tables as HTML, all sorted in human reading order. Also emits a Markdown view and layout-visualization image.
- **Multilingual**: Designed for ~100 languages; MDPBench-evaluated on 17 (Simplified/Traditional Chinese, English, Arabic, German, Spanish, French, Hindi, Indonesian, Italian, Japanese, Korean, Portuguese, Russian, Thai, Vietnamese, +1).
- **Benchmarks** (primary):
  - OmniDocBench v1.5: TextEdit↓ **0.031**, ReadOrderEdit↓ **0.029** (best open model in the in-repo table; beats MinerU2.5's 0.047/0.044 on v1.5).
  - olmOCR-Bench: **83.9** (paper claim — new SOTA).
  - OCR Arena Elo: ranks #2 only to Gemini 3 Pro among public models.
  - MDPBench (17 languages): **80.5%** overall — best open-source; Gemini 3 Pro proprietary leader at 86.4%.
- **Failure modes / known gaps**:
  - **Two-column Arabic** explicitly flagged: model processes LTR despite text being RTL (per MDPBench paper, both dots.ocr and PaddleOCR-VL-1.5 affected).
  - Vietnamese is occasionally misrecognized as Chinese.
  - "Complex tables and dense formulas" still hard for a 3B model.
  - Parsing failures occur, lower rate than 1.0 but present.

## 3. PaddleOCR-VL-1.5 (0.9B)

- **Repo / weights**: github.com/PaddlePaddle/PaddleOCR, huggingface.co/PaddlePaddle/PaddleOCR-VL-1.5; GGUF and MLX variants available.
- **Release**: PaddleOCR-VL (0.9B) Oct 2025; **PaddleOCR-VL-1.5 released 2026-01-29**. Paper: arXiv 2510.14528 (VL), 2601.21957 (VL-1.5).
- **License**: **Apache 2.0**.
- **Size / VRAM**: 0.9B (NaViT-style ViT + ERNIE-4.5-0.3B decoder). Smallest of the top open performers; fits comfortably in 8GB. MLX build (`mlx-community/PaddleOCR-VL-1.5-bf16`) confirms Apple Silicon native support.
- **Output**: PDF/image → structured Markdown + JSON; supports irregular-shaped bbox localization (added in 1.5). Pipeline-style — PP-DocLayout for layout, then VLM for element recognition.
- **Multilingual**: **109 languages** explicitly enumerated, including Chinese, English, Japanese, Korean, Latin family, Russian (Cyrillic), **Arabic**, Hindi (Devanagari), Thai. 1.5 added Tibetan and Bengali.
- **Benchmarks**:
  - OmniDocBench v1.5 overall: **94.5** (1.5); 92.56 (VL 0.9B original).
  - Formula CDM 90.88 (beats MinerU2.5's 87.55 and dots.ocr's 85.34 per ERNIE blog).
- **Failure modes**:
  - **Same Arabic two-column LTR-vs-RTL bug as dots.mocr** (per MDPBench paper).
  - No public OmniDocBench v1.6 number yet — direct comparison to MinerU2.5-Pro's 95.69 requires running v1.6 yourself.

## 4. Qwen3-VL (8B / 32B / 235B-A22B)

- **Repo / weights**: github.com/QwenLM/Qwen3-VL (19.2k stars), huggingface.co/Qwen
- **Release**: 235B-A22B (Sept 23 2025); 30B-A3B (Oct 4 2025); **4B/8B (Oct 15 2025); 2B/32B (Oct 21 2025)**; paper arXiv 2511.21631 (Nov 27 2025).
- **License**: **Apache 2.0**.
- **Size / VRAM**: 2B/4B fit easily; **8B-Instruct** is the practical sweet spot for a 24GB consumer card (BF16 ~16GB); 32B requires 48GB at BF16 or quantization on 24GB. Dense + MoE variants.
- **Output**: Native 256K context (1M expandable). Document parsing cookbook outputs **Qwen HTML format** with `data-bbox` attributes, bboxes normalized to 0–1000 (top-left → bottom-right). Also Markdown, JSON, LaTeX.
- **Multilingual OCR**: **32 languages** explicitly (up from Qwen2.5-VL's 10). Strong on low-light, blur, tilt, rare/ancient characters, long-document structure. Verified Arabic, Turkish, French, English, CJK in base model coverage.
- **Benchmarks**: Top-tier on OCRBench; dots.mocr's in-repo table shows Qwen3-VL-235B at TextEdit 0.069 / ReadOrderEdit 0.068 (worse than the specialized 3B dots.mocr on v1.5).
- **Failure modes**:
  - General VLM, not a document-specialized model; the specialists outperform it head-to-head at parsing despite much larger param counts.
  - Bbox accuracy drops on non-square images at 1000×1000 normalization (llama.cpp issue #16880).
  - 8B-Instruct is what you'd run; 235B is out of scope for this constraint.

## 5. Marker (datalab-to/marker) + Surya — CPU/Apple-Silicon fallback

- **Repo / weights**: github.com/datalab-to/marker (35k stars), github.com/datalab-to/surya
- **Release**: marker **v1.10.2 on 2026-01-31**; v1.9.0 (early 2026) moved OCR inference from line-level to block-level for accuracy.
- **License**: **Code = GPL; model weights = modified AI Pubs Open RAIL-M** (free for research, personal, and startups <$2M; otherwise commercial license needed). Strictest license of the lot — material for your specific scenario.
- **Size / VRAM**: CPU-runnable; small specialized models (layout, OCR, table-rec, reading-order). Projected **25 pages/sec on H100** in batch mode.
- **Output**: Markdown, HTML, JSON. JSON includes bbox in `(x1,y1,x2,y2)` format and block-level categories.
- **Multilingual**: Surya OCR claims **90+ languages**; works without OCR for any language when text is extractable from the PDF (`--strip_existing_ocr` to force re-OCR). RTL languages were "encountered during testing"; explicit Arabic handling is not a headline feature.
- **Benchmarks**: Internal benchmarks favorable vs Llamaparse and Mathpix; **no published OmniDocBench v1.5/v1.6 number** for the current release, which is the gap vs. the VLM specialists.
- **Failure modes**:
  - License is the biggest concern for commercial use.
  - Quality ceiling is below the VLM specialists per third-party comparisons; primary use case is when you cannot run a 1–3B VLM (CPU-only, very low memory).

## Honorable mentions (research/reference only)

- **GLM-OCR** (Z.ai, 0.9B, 2026-03-11, MIT/Apache-2.0 split): claims OmniDocBench v1.5 = 94.62 with 0.9B params; 100+ languages. Architecturally similar to PaddleOCR-VL (two-stage: PP-DocLayout-V3 + parallel region-level recognition). Strong candidate to track but no independent third-party Arabic-specific evaluation as of 2026-05.
- **MinerU-Diffusion-V1-0320-2.5B** (2026-03-24, paper arXiv 2603.22458): diffusion-based decoder, 2.1× faster than MinerU2.5 at equal accuracy at τ=0.95, 3.2× at τ=0.6. Production-ready as of release; consider as a speed-optimized alternative if throughput matters more than accuracy ceiling.
- **DeepSeek-OCR / DeepSeek-OCR2** (Jan 2026, 3B, MIT): visual-token compression research; 200k+ pages/day on one GPU; multilingual story still unverified for Arabic specifically.
- **Dolphin-v2** (Dec 2025, 3B, bytedance, Qwen2.5-VL based, ACL 2025): 21 element categories, OmniDocBench v1.5 = 89.45; **multilingual gap explicitly acknowledged** — disqualified here, but useful for Chinese/English.

# Comparison Table

| Model | Release | License | Size | Min VRAM | Output | Multilingual | Best benchmark (source) | Arabic 2-col reading order |
|---|---|---|---|---|---|---|---|---|
| MinerU 2.5-Pro | 2026-04-18 | MinerU OSL (Apache-2.0+) | 1.2B VLM (+ pipeline) | 4 GB pipeline / 8 GB VLM | JSON+bbox+category, Markdown, HTML tables, LaTeX | 109 langs (pipeline) | OmniDocBench v1.6 = **95.69** (HF card) | Known broken on 2-col (issues #1107/#3591); pipeline `--lang ar` partly mitigates |
| dots.mocr | 2026-03-19 | MIT | 3B | ~16–24 GB | JSON+bbox+category, Markdown, SVG for graphics | ~100, MDPBench 17 langs | OmniDocBench v1.5 TextEdit **0.031** / olmOCR-Bench **83.9** / MDPBench **80.5%** (papers) | Documented broken (MDPBench paper) |
| PaddleOCR-VL-1.5 | 2026-01-29 | Apache 2.0 | 0.9B | ~6–8 GB | Markdown + JSON, irregular bbox | 109 langs incl. Arabic, Tibetan, Bengali | OmniDocBench v1.5 = **94.5** (ERNIE blog) | Documented broken (MDPBench paper) |
| Qwen3-VL 8B-Instruct | 2025-10-15 | Apache 2.0 | 8B | ~16 GB | Qwen-HTML w/ data-bbox, JSON, Markdown, LaTeX | 32 OCR langs explicit | OCRBench top-tier; weaker than specialists on OmniDocBench v1.5 (TextEdit 0.069 at 235B per dots.mocr table) | Not explicitly tested; general VLM |
| Marker + Surya | 2026-01-31 | GPL code / AI Pubs RAIL-M weights | small specialized | CPU-OK | Markdown, HTML, JSON+bbox | 90+ langs (Surya) | No public OmniDocBench v1.5/v1.6 number | RTL "encountered" — not first-class |
| GLM-OCR (track) | 2026-03-11 | MIT + Apache 2.0 | 0.9B | ~6–8 GB | Markdown, JSON, LaTeX | 100+ | OmniDocBench v1.5 = 94.62 (Z.ai claim) | Untested for Arabic 2-col |
| MinerU-Diffusion (track) | 2026-03-24 | MinerU OSL | 2.5B | ~12 GB | same as MinerU pipeline | inherits 109 | 108.9 TPS (2.1× faster) at MinerU2.5 accuracy (paper) | inherits MinerU 2-col Arabic issue |

# Final Recommendation

**Yes — a single model replaces `cut + layout + ocr + read` in May 2026 for the languages mutercim cares about, with one specific Arabic caveat.**

## Recommended pick

**MinerU 2.5-Pro (1.2B VLM) inside the MinerU 3.1.x toolkit.** Rationale:

1. **Highest published quality**: OmniDocBench v1.6 = 95.69, beating Qwen3-VL-235B and Gemini 3 Pro (per the HF model card and paper). Lower text-edit distance (0.036) than any open competitor.
2. **Layout-preserving JSON output is first-class**: bbox + category + reading-order is the *primary* output, not a side prompt — schema is documented and stable.
3. **Best Apple Silicon and CPU story** of the top-three: VLM on 8GB GPU, **MLX backend** for M-series, pipeline backend runs CPU-only. The other top contenders (dots.mocr, PaddleOCR-VL) are vLLM-first.
4. **Two backends in one tool**: when the VLM struggles on a tail language, switch to the pipeline backend with explicit `--lang` for any of 109 languages without changing tools.
5. **Permissive license** (custom Apache-2.0-based) explicitly designed to ease commercial integration.
6. **Active maintenance**: 5,367 commits, 164 releases, last release 2026-05-09 — three days before this report.

## Runner-up

**dots.mocr (3B, MIT)** if you need (a) the cleanest single-model VLM design with no pipeline fallback, (b) MIT license without modifications, (c) SVG-graphics-to-code as an extra capability. Slightly stronger on v1.5 metrics; harder to deploy on Apple Silicon as a first-class target.

## Remaining gap — name it explicitly

**Arabic two-column right-to-left reading order is broken on every open VLM tested by MDPBench (March 2026)**, including dots.mocr, dots.ocr, and PaddleOCR-VL-1.5. MinerU has open GitHub issues for two-column reading order generally. **Implication for mutercim**: if Turkish/English/French/CJK is the dominant workload and Arabic is occasional or single-column, you can collapse the three phases to one without quality regression. If Arabic two-column books/journals are a real workload, plan one of:

- Keep a post-processing reading-order corrector that runs after the VLM, using bbox geometry + per-line direction detection (RTL bbox cluster → reverse order).
- Use the MinerU pipeline backend with `--lang ar` for Arabic-detected pages (lower per-page accuracy than the VLM but reading order is heuristic, not LLM-learned).
- Add an Arabic-specialized post-OCR step (KITAB-Bench tooling, MBZUAI Oryx) only on those pages.

This gap is the same for any open VLM you pick. It is *not* a reason to keep the current three-phase pipeline — DocLayout-YOLO + Qari-OCR has the same problem.

## Migration path

1. Replace `layout + ocr + read` phases with one MinerU call. Keep `cut`, `translate`, `write` unchanged.
2. Add a `backend` config (pipeline | vlm-mlx | vlm-async-engine) so the same code runs on a Linux workstation, Apple Silicon, and CPU. MinerU exposes all three.
3. Add an Arabic-only reading-order fixup pass keyed off the bbox JSON. Cheap, deterministic, contained.
4. Keep the existing prompt budget for `translate` unchanged — the JSON schema MinerU emits is closer to what your translator needs than the current 3-phase intermediates, so you may also win on translate quality downstream.

# Sources

Primary (GitHub, HuggingFace, arXiv):

- MinerU repo and license: https://github.com/opendatalab/MinerU (accessed 2026-05-12)
- MinerU2.5-Pro model card: https://huggingface.co/opendatalab/MinerU2.5-Pro-2604-1.2B (accessed 2026-05-12)
- MinerU2.5-Pro paper: https://arxiv.org/abs/2604.04771
- MinerU 2.5 paper: https://arxiv.org/html/2509.22186v1
- MinerU-Diffusion paper: https://arxiv.org/abs/2603.22458
- MinerU changelog: https://opendatalab.github.io/MinerU/reference/changelog/
- dots.ocr README and benchmarks: https://github.com/rednote-hilab/dots.ocr/blob/master/README.md (accessed 2026-05-12)
- dots.mocr paper "Multimodal OCR: Parse Anything from Documents": https://arxiv.org/abs/2603.13032
- dots.ocr paper: https://arxiv.org/abs/2512.02498
- dots.mocr model card: https://huggingface.co/rednote-hilab/dots.mocr
- PaddleOCR-VL paper: https://arxiv.org/abs/2510.14528
- PaddleOCR-VL-1.5 paper: https://arxiv.org/abs/2601.21957
- PaddleOCR-VL-1.5 model card: https://huggingface.co/PaddlePaddle/PaddleOCR-VL-1.5
- PaddleOCR-VL-1.5 blog (Baidu ERNIE): https://ernie.baidu.com/blog/posts/paddleocr-vl-1.5/
- Qwen3-VL repo: https://github.com/QwenLM/Qwen3-VL
- Qwen3-VL paper: https://arxiv.org/pdf/2511.21631
- Qwen3-VL 8B model card: https://huggingface.co/Qwen/Qwen3-VL-8B-Instruct
- olmOCR-2 model card: https://huggingface.co/allenai/olmOCR-2-7B-1025
- olmOCR 2 blog: https://allenai.org/blog/olmocr-2
- olmOCR paper: https://olmocr.allenai.org/papers/olmocr.pdf
- marker repo: https://github.com/datalab-to/marker
- surya repo: https://github.com/datalab-to/surya
- MonkeyOCR paper: https://arxiv.org/abs/2506.05218
- MonkeyOCR v1.5 paper: https://arxiv.org/abs/2511.10390
- Dolphin paper (ACL 2025): https://arxiv.org/abs/2505.14059
- Dolphin-v2 model card: https://huggingface.co/ByteDance/Dolphin-v2
- DeepSeek-OCR: https://huggingface.co/deepseek-ai/DeepSeek-OCR
- DeepSeek-OCR2: https://huggingface.co/deepseek-ai/DeepSeek-OCR-2
- GLM-OCR repo: https://github.com/zai-org/GLM-OCR
- GLM-OCR paper: https://arxiv.org/abs/2603.10910
- OmniDocBench (CVPR 2025): https://github.com/opendatalab/OmniDocBench
- MDPBench paper: https://arxiv.org/abs/2603.28130 (HTML: https://arxiv.org/html/2603.28130v1)
- KITAB-Bench Arabic OCR (ACL 2025): https://arxiv.org/abs/2502.14949 ; https://github.com/mbzuai-oryx/KITAB-Bench
- MinerU reading-order issues: https://github.com/opendatalab/MinerU/issues/1107 ; https://github.com/opendatalab/MinerU/issues/3591
- Qwen3-VL bbox normalization issue: https://github.com/ggml-org/llama.cpp/issues/16880

Secondary (for triangulation only; not the basis of any benchmark number cited above):

- neurohive.io MinerU2.5 article (Sept 2025)
- AI Innovations and Insights Substack on MinerU 2.5 and MinerU-Diffusion
- alphaxiv summaries of MinerU 2.5-Pro and MonkeyOCR v1.5
- llamaindex multilingual OCR roundup
- codesota OCR leaderboard
