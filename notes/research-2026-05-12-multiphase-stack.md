---
date: 2026-05-12
topic: Multi-phase document translator stack — best-in-class for Layout / OCR / Read-structure
audience: mutercim maintainers
scope: Local-only, open-weight, multilingual (Arabic, Turkish, English, French, CJK)
hardware-budget: 24-48 GB GPU OR Apple Silicon OR CPU fallback
---

# TL;DR

For each phase keep the best independent open-weight model. As of May 2026 the
strongest stack that satisfies local + multilingual (incl. Arabic + CJK) is:
**A: DocLayout-YOLO (or PP-DocLayoutV2 inside PP-StructureV3) → B: PaddleOCR
PP-OCRv5 with Qari-OCR-v0.3 dispatch for Arabic-heavy pages → C: dots.ocr 1.7B
as a unified VLM that can also serve as a fallback for A+B+C combined, with
olmOCR-2-7B / Qwen3-VL-8B as a heavier alternative.** The unified-VLM
candidates (dots.ocr, PaddleOCR-VL-1.5, MinerU2.5) are now strong enough that
keeping the multi-phase pipeline is justified mainly by per-language tuning,
hot-swap of components, and CPU-only operability — quality alone no longer
demands separate phases.

---

# Phase A — Layout detection

Input: page image. Output: bboxes with region types (header, paragraph,
figure, table, caption, footnote) and reading order.

## Candidates

| Tool | Latest version / date | Active? | Strengths | Weaknesses |
|---|---|---|---|---|
| DocLayout-YOLO (opendatalab) | YOLOv12 backbone 2025-03-10, YOLOv26 backbone 2026-03-10 | Yes, opendatalab actively maintains | Fast (real-time, CPU-viable), trained on DocSynth-300K with global-to-local receptive module; works on diverse layouts; small footprint | Class set is generic; reading-order not built-in (need separate sort) |
| Surya layout (datalab-to/surya) | v0.17.1 (2026-01-30); v0.17.0 (2025-09-23) | Yes, very active | Line-level bboxes; layout + reading order + table recognition in one pipeline; 90+ languages including Arabic/CJK | GPL code + custom RAIL-M weights (commercial licensing required); needs GPU for speed |
| PP-StructureV3 / PP-DocLayoutV2 (PaddleOCR 3.0) | PaddleOCR 3.0 (2025); arXiv 2507.05595 | Yes, very active | Integrated with PP-OCRv5; chart→table; 106 languages; OmniDocBench-tuned | Paddle runtime overhead; recent move to PaddleOCR-VL family |
| Docling Heron layout (IBM) | Heron model 2025-12; docling-ibm-models 3.13.2 (2026-04-23) | Yes, IBM-backed | Enterprise polish; Apache-2.0; integrates with Granite-Docling-258M for end-to-end | Newer, less independent third-party benchmarking |
| layout-parser (Allen et al.) | No PyPI release in >12 months | Inactive — declared inactive by Snyk | Historical reference impl. | Stale; do not adopt for new work |
| RT-DETRv2 / YOLOv12 (generic) | 2025 releases | Active as generic detectors | Strong COCO numbers | No document-specific pretraining; you'd retrain |

## Recommendation

**Primary: DocLayout-YOLO** (open weights, fast, CPU-viable, document-specific
pretraining on DocSynth-300K). Use a separate reading-order step (XY-cut, or
Surya's reading-order model) because DocLayout-YOLO returns regions but not
order.

**Alternative when you can pay Paddle runtime cost: PP-StructureV3 layout
component** — it's tuned against OmniDocBench and ships with reading order +
table cell detection.

**Skip:** layout-parser (dead), generic RT-DETR (no doc pretraining).

---

# Phase B — OCR (multilingual)

Input: page or region image. Output: text in reading order, with confidence
and per-word/line boxes if available.

## Candidates

| Engine | Version / date | Latin | Arabic | CJK | Notes |
|---|---|---|---|---|---|
| PaddleOCR PP-OCRv5 | 3.0 (mid-2025) | Strong | Supported (106-lang multilingual model) | Unified CN/EN/JA model | Arxiv 2507.05595; specific Arabic accuracy numbers not yet published independently; strongest practical multilingual coverage |
| Surya OCR | 0.17.1 (2026-01-30) | Strong | Supported but error rates flagged as high in E-ARMOR 2025 study (multilingual eval) | Supported | Line-level, GPL code + commercial-licensable weights |
| Tesseract 5.5.1 | 2025-05 | Strong | Supported, but classical LSTM, weaker than VLMs on dense Arabic | Supported | KITAB-Bench: VLM-based OCR beats Tesseract by ~60% avg CER on Arabic |
| EasyOCR | Active, JaidedAI | Decent | 80+ langs incl. Arabic | Supported | Balanced but not SOTA; reasonable baseline |
| TrOCR (Microsoft) | Maintained but old | Strong on handwriting | Limited | Limited | Mostly handwriting niche; not the right choice for general multilingual doc OCR |
| **Qari-OCR-v0.3** (NAMAA-Space) | June 2025 paper, HF model | n/a | **SOTA open-source Arabic** — WER 0.160, CER 0.061, BLEU 0.737 on diacritics-heavy text; v0.3 adds structural preservation (HTML tags) | n/a | 2B-param Qwen2-VL fine-tune; Apache compatible (check HF) |
| Baseer (Misraj) | Sep 2025 (arXiv 2509.18174) | n/a | Arabic SOTA, WER 0.25, TEDS 66 | n/a | Qwen2.5-VL-3B fine-tune; document-to-Markdown focus |
| olmOCR-2 (AllenAI) | 2025-10, 7B Qwen2.5-VL fine-tune | Strong (82.4 on olmOCR-Bench) | Limited | Limited | Apache-2.0; MLX 4-bit / 6-bit ports for Apple Silicon — best local option for English-heavy CPU/Mac users |
| DeepSeek-OCR | 2025-10-20 | Strong | ~100 langs claimed | Supported | Vision-token compression; author-reported only; third-party eval thin |

## Recommendation

**Per-language dispatch is the right choice for highest quality.** The
KITAB-Bench finding ("VLM-based OCR beats traditional engines by ~60% avg CER
on Arabic") is decisive: a single uniform OCR cannot serve Arabic well in
2026.

Recommended dispatch:
- **Arabic-detected pages → Qari-OCR-v0.3** (or Baseer as alternative). Both
  are Qwen2-VL fine-tunes; ~6 GB VRAM each in 4-bit.
- **Latin / Turkish / French / general → PaddleOCR PP-OCRv5**. Single model,
  106 langs, fastest pipeline, runs on CPU or modest GPU.
- **CJK-dense → PP-OCRv5** (unified CN/EN/JA model is competitive) — no
  separate CJK specialist is currently necessary.
- **Apple-Silicon / Mac-only deployment of English-heavy work → olmOCR-2**
  (MLX builds available, 82.4 on olmOCR-Bench).

**Skip:** Tesseract for Arabic (loses ~60% CER vs VLM OCR). TrOCR for general
docs (handwriting niche). Surya OCR for Arabic (E-ARMOR flagged elevated
error rates) — but Surya is still excellent for **layout + reading order**, so
keep it in Phase A even if you drop it from Phase B.

Language detection ahead of dispatch: a 50-ms script-classifier on the
layout-detected region (Unicode script histogram from a cheap initial OCR
pass, or a tiny CNN script-classifier) is sufficient. Don't run the heavy
VLM-OCR on every page.

---

# Phase C — Read-structure (vision LLM)

Input: page image + layout bboxes + OCR text. Output: structured JSON with
typed regions, reading order, and per-region text.

## Candidates (24-48 GB VRAM tier)

| Model | Params | Released | OmniDocBench (where available) | Local-friendly | Multilingual |
|---|---|---|---|---|---|
| **dots.ocr** (rednote-hilab) | 1.7B | 2025-07-30; base 2025-10-31; arXiv 2512.02498 | SOTA on text/tables/reading-order (per paper); strong vs Doubao-1.5 / gemini-2.5-pro on formulas | Yes — MLX port exists | 100+ langs incl. Arabic and Tamil; "decisive advantage on low-resource langs" per paper |
| **PaddleOCR-VL-1.5** | 0.9B | Late 2025 follow-up to PaddleOCR-VL 2025-10-16 | **94.93 on OmniDocBench v1.6_full** (SOTA tier) | Yes — runs even on CPU; tiny VRAM | 109 langs |
| **MinerU2.5-Pro** | 1.2B | 2025-09 (arXiv 2509.22186) | **95.75 on OmniDocBench v1.6_full — current #1** | Yes | Multilingual; decoupled coarse-to-fine architecture |
| Qwen2.5-VL-7B | 7B | 2025-01-28 | Officially aligned baseline in OmniDocBench | Yes; ~17 GB FP16, ~6 GB 4-bit | Strong; 32B variant adds quality but doubles VRAM |
| Qwen3-VL-8B Instruct | 8.8B | 2025-10 | Successor; benchmark numbers still settling | Yes; similar footprint to 2.5-VL-7B | Improved over 2.5 |
| InternVL3-78B | 78B | 2025-04-11 (arXiv 2504.10479) | 72.2 MMMU; SOTA among open MLLMs at release | Needs 48 GB+ at low quant or multi-GPU | Strong multilingual |
| MiniCPM-o 4.5 | 9B | 2026-04-ish | Claims SOTA end-to-end English doc parsing, beats Gemini-3 Flash & GPT-5 (author-reported) | Yes — llama.cpp, Ollama, vLLM, GGUF int4 | 30+ langs |
| Granite-Docling-258M (IBM) | 258M | 2026-01 | Compact; integrates with Docling pipeline | Yes — extremely small | Limited language coverage compared to dots.ocr |
| Phi-4 Multimodal | 5.6B | 2025 | General VLM; doc-parsing not its focus | Yes | Decent |

## Recommendation

**Primary: dots.ocr** — it is the only open-weight VLM in the 1-2B class that
is (a) document-parsing-focused, (b) explicitly multilingual including Arabic
with a published in-house multilingual benchmark, (c) emits stable bbox+type+
text JSON natively, and (d) has an MLX port for Apple Silicon. At 1.7B it
fits comfortably in 8-12 GB VRAM and is CPU-viable.

**Alternative for English / CN / JA heavy workloads: MinerU2.5-Pro** (current
OmniDocBench #1, 1.2B, decoupled architecture, 2.12 pages/s end-to-end). Less
proven on Arabic.

**Alternative when you want a single 7-8B generalist VLM** (so the same model
can do downstream reasoning, not just doc parsing): **Qwen3-VL-8B**.
~17-18 GB FP16, ~6 GB 4-bit, broad multilingual.

**Skip in this tier:**
- Qwen2.5-VL-72B / InternVL3-78B for local: VRAM cost is not justified given
  dots.ocr / MinerU2.5 dominate doc-parsing benchmarks at <2B.
- Granite-Docling-258M: too small for Arabic / less-resourced languages
  despite IBM's polish.
- MiniCPM-o 4.5: strong but most claims are author-reported and English-
  centric.

---

# Cross-cutting failure modes

| Failure mode | Where it bites | Mitigation in recommended stack |
|---|---|---|
| **Arabic diacritics (tashkeel)** | Tesseract, EasyOCR, Surya OCR all lose accuracy on diacritic-rich text | Dispatch to Qari-OCR-v0.3 (specifically trained on diacritic-heavy text; BLEU 0.737) for Arabic pages |
| **RTL reading order at column/figure boundaries** | Generic layout detectors emit LTR-order bboxes | Use Surya reading-order model, or post-process via right-to-left XY-cut when Arabic script detected by the script classifier |
| **Multi-column scientific layouts** | DocLayout-YOLO handles columns but reading-order across columns is brittle | dots.ocr / MinerU2.5 / PP-StructureV3 all include explicit reading-order modeling — fall back to one of these for >2-column pages |
| **Tables with merged cells / nested tables** | All non-VLM OCR pipelines (Tesseract, EasyOCR) fail | Use PP-StructureV3 chart/table parser, or dots.ocr/MinerU2.5; never trust pure-OCR table reconstruction |
| **Math (inline + display)** | Classical OCR loses math entirely | dots.ocr emits LaTeX; MinerU2.5 emits LaTeX; PaddleOCR-VL-1.5 added formula improvements in 1.5 |
| **CJK vertical text** | Older OCRs assume horizontal | PaddleOCR PP-OCRv5's unified CN/EN/JA model handles vertical; verify on Japanese tategaki samples |
| **Low-resolution scans** | Affects everything | Surya is reportedly strong on low-res; dots.ocr's NaViT-style dynamic resolution helps |
| **Handwriting** | Out of scope but often appears as marginalia | Skip / mark as untranslatable; or route to TrOCR for English handwriting |
| **Mixed scripts on same line (e.g., Arabic + English citation)** | Per-language dispatch model is wrong-fit | Run Qari-OCR-v0.3 for the line — it preserves embedded Latin runs; or route the line through dots.ocr |

---

# VRAM budget for recommended stack

Two deployment shapes:

## Shape 1 — Sequential phases (recommended for 12-24 GB consumer GPUs)

Phases run one at a time; only one model resident at a time.

| Phase | Model | VRAM (4-bit/INT4) | VRAM (FP16) |
|---|---|---|---|
| A — Layout | DocLayout-YOLO | ~1 GB | ~2 GB |
| B — OCR (general) | PaddleOCR PP-OCRv5 | ~2 GB | ~3-4 GB |
| B — OCR (Arabic) | Qari-OCR-v0.3 (2B) | ~2-3 GB | ~6 GB |
| C — Read-structure | dots.ocr (1.7B) | ~2-3 GB | ~5-6 GB |
| **Peak (sequential)** | — | **~3 GB** | **~6 GB** |

This fits an 8 GB consumer GPU (RTX 4060, M-series base) easily.

## Shape 2 — All resident simultaneously (24-48 GB GPU server)

| Component | FP16 | 4-bit |
|---|---|---|
| DocLayout-YOLO | 2 GB | 1 GB |
| PaddleOCR PP-OCRv5 | 4 GB | 2 GB |
| Qari-OCR-v0.3 (2B) | 6 GB | 3 GB |
| dots.ocr (1.7B) | 6 GB | 3 GB |
| **Total resident** | **~18 GB** | **~9 GB** |

Headroom for activations, KV cache, and batch ≥ 2 makes 24 GB the comfortable
minimum at FP16; a 12 GB GPU is enough at 4-bit.

## Shape 3 — CPU / Apple Silicon

- DocLayout-YOLO: ONNX runtime, real-time on CPU.
- PaddleOCR PP-OCRv5: native CPU support, ~0.5-2 s/page.
- Qari-OCR-v0.3 / dots.ocr: MLX 4-bit on Apple Silicon; ~5-15 s/page on M2/M3.
- olmOCR-2-7B-MLX-4bit: 6-7 GB, ~3-8 s/page on Apple Silicon; substitute as
  Phase B+C combined for English-heavy work.

---

# Going-forward decision points

1. **Replace multi-phase with unified VLM?** If your evaluation finds dots.ocr
   or MinerU2.5-Pro alone outperforms A+B+C in your real document mix on
   accuracy AND latency, collapse the pipeline. Re-evaluate quarterly — these
   1-2B VLMs are improving faster than the underlying classical components.
2. **License watch.** Surya weights are RAIL-M (commercial restrictions);
   PaddleOCR / DocLayout-YOLO / dots.ocr / olmOCR-2 are Apache-2.0 or
   permissive. Verify Qari-OCR-v0.3's license on HF before shipping.
3. **Benchmarks to keep watching:** OmniDocBench v1.6+, olmOCR-Bench, KITAB-
   Bench (Arabic), Misraj-DocOCR (Arabic), Real5-OmniDocBench.

---

# Sources

Phase A — Layout:
- DocLayout-YOLO repo: https://github.com/opendatalab/DocLayout-YOLO (last commit 2026-03)
- DocLayout-YOLO paper, arXiv 2410.12628 (2024-10-16): https://arxiv.org/abs/2410.12628
- Surya repo: https://github.com/datalab-to/surya (v0.17.1, 2026-01-30)
- PP-StructureV3 docs: https://paddlepaddle.github.io/PaddleOCR/main/en/version3.x/algorithm/PP-StructureV3/PP-StructureV3.html
- PaddleOCR 3.0 Technical Report, arXiv 2507.05595: https://arxiv.org/abs/2507.05595
- IBM Granite-Docling announcement (2026-01): https://www.ibm.com/new/announcements/granite-docling-end-to-end-document-conversion
- docling-ibm-models 3.13.2 (2026-04-23): https://pypi.org/project/docling-ibm-models/
- layout-parser status (Snyk advisor, "inactive"): https://snyk.io/advisor/python/layoutparser

Phase B — OCR:
- Tesseract 5.5.1 release notes (2025-05): https://tesseract-ocr.github.io/tessdoc/ReleaseNotes.html
- PaddleOCR PP-OCRv5 (PaddleOCR 3.0 report): https://arxiv.org/abs/2507.05595
- QARI-OCR paper, arXiv 2506.02295 (2025-06): https://arxiv.org/abs/2506.02295
- QARI-OCR-v0.3 model card: https://huggingface.co/NAMAA-Space/Qari-OCR-v0.3-VL-2B-Instruct
- Baseer (Arabic), arXiv 2509.18174 (2025-09): https://arxiv.org/abs/2509.18174
- KITAB-Bench, arXiv 2502.14949 (ACL 2025): https://arxiv.org/abs/2502.14949
- E-ARMOR multilingual OCR study, arXiv 2509.03615 (2025-09): https://arxiv.org/html/2509.03615v1
- olmOCR 2 paper, arXiv 2510.19817 (2025-10): https://arxiv.org/abs/2510.19817
- olmOCR 2 blog (Allen AI): https://allenai.org/blog/olmocr-2
- olmOCR-2 MLX 6-bit port: https://huggingface.co/richardyoung/olmOCR-2-7B-1025-MLX-6bit
- DeepSeek-OCR repo: https://github.com/deepseek-ai/DeepSeek-OCR (2025-10-20)
- EasyOCR repo: https://github.com/JaidedAI/EasyOCR

Phase C — Read-structure / VLM:
- dots.ocr paper, arXiv 2512.02498 (2025-12): https://arxiv.org/abs/2512.02498
- dots.ocr repo: https://github.com/rednote-hilab/dots.ocr (released 2025-07-30, base 2025-10-31)
- PaddleOCR-VL paper, arXiv 2510.14528 (2025-10-16): https://arxiv.org/abs/2510.14528
- PaddleOCR-VL-1.5 blog: https://ernie.baidu.com/blog/posts/paddleocr-vl-1.5/
- MinerU2.5 paper, arXiv 2509.22186 (2025-09): https://arxiv.org/html/2509.22186v1
- MinerU2.5-Pro-2604 HF model: https://huggingface.co/opendatalab/MinerU2.5-Pro-2604-1.2B
- OmniDocBench (CVPR 2025): https://github.com/opendatalab/OmniDocBench
- OmniDocBench leaderboard (CodeSOTA): https://www.codesota.com/browse/computer-vision/document-parsing/omnidocbench
- Qwen2.5-VL Technical Report, arXiv 2502.13923 (2025-02): https://arxiv.org/abs/2502.13923
- Qwen3-VL repo: https://github.com/QwenLM/Qwen3-VL (2025-10)
- InternVL3 paper, arXiv 2504.10479 (2025-04): https://arxiv.org/abs/2504.10479
- MiniCPM-V/o 4.5 docs: https://github.com/OpenBMB/MiniCPM-o/blob/main/docs/minicpm_v4dot5_en.md
- Qwen VRAM analysis: https://apxml.com/posts/gpu-system-requirements-qwen-models
