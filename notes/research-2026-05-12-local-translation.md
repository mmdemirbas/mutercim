---
date: 2026-05-12
topic: Local-first translation model and serving stack evaluation
audience: mutercim project (local-first language-agnostic document translator)
primary_lang_pairs: ar↔tr, ar↔en (en/tr/fr/zh/ja sanity check)
tiers:
  T1: 8 GB VRAM / 8 GB unified Apple Silicon
  T2: 24 GB VRAM / 32-48 GB unified Apple Silicon
  T3: 48-80 GB VRAM / 64-128 GB unified Apple Silicon
---

# Local-first translation: 2026-05 state of the art

## TL;DR

For the user's stated **Arabic↔Turkish + Arabic↔English** primary pairs, the
specialized-translation field is incomplete: Tower+ (the strongest open
specialist) excludes Arabic and Turkish entirely from its 22-language list, and
Qwen-MT / TranslateGemma either keep weights closed, or restrict the
production-tier language set. The best local-first quality is achieved with a
**multilingual general LLM that has Arabic+Turkish in its training mix** —
specifically **Gemma 3 27B IT** (T2/T3), **Babel-9B-Chat** or **Qwen3 14B/32B**
(T2), and **Gemma 3 12B QAT-int4** or **Qwen3 8B** (T1). Serve via **MLX-LM on
Apple Silicon**, **llama.cpp / vLLM on Linux+NVIDIA**, and **llama.cpp on CPU**.

---

## 1. General-purpose LLM candidates (Arabic+Turkish capable, open weights, May 2026)

### Table 1 — Multilingual generalists with Arabic AND Turkish coverage

| Model | Params | Active | License | ar | tr | Ctx | Notes |
|---|---|---|---|---|---|---|---|
| **Gemma 3 27B IT** | 27B | 27B (dense) | Gemma TOS | yes | yes | 128k | 140+ langs pre-trained, 35+ supported, strong WMT24++ baseline (TranslateGemma 12B was tuned to beat it). |
| **Gemma 3 12B IT** | 12B | 12B | Gemma TOS | yes | yes | 128k | Sweet spot for T1+ via Q4 QAT (~7 GB). |
| **Gemma 3 4B IT** | 4B | 4B | Gemma TOS | yes | yes | 128k | T1 floor; runs on 8 GB VRAM Q4. |
| **Babel-9B-Chat** | 9B | 9B | seallm (custom — review!) | yes | yes | 4k (Gemma2 base) | Top 25 langs by speakers incl. ar+tr; Flores-200 = 55.1 vs Gemma2-9B 53.2, Qwen2.5-7B 45.5. |
| **Babel-83B-Chat** | 83B | 83B | seallm (custom) | yes | yes | 4k | Flores-200 = 58.8 vs Llama3.1-70B 57.4, Qwen2.5-72B 53.1. T3 only (~50 GB Q4). |
| **Qwen3 32B** | 32B | 32B (dense) | Apache 2.0 | yes | yes | 32k+ (RoPE-extendable) | 119 langs, ≤1 COMET pt loss at 4-bit. |
| **Qwen3 14B** | 14B | 14B | Apache 2.0 | yes | yes | 32k | T2 fit; sane fallback when Gemma TOS is an issue. |
| **Qwen3 8B** | 8B | 8B | Apache 2.0 | yes | yes | 32k | T1 sweet spot (~4.6 GB Q4). |
| **Qwen3-235B-A22B** | 235B | 22B | Apache 2.0 | yes | yes | 32k+ | MoE — ~140 GB at Q4. T3 high-end only. |
| **Mistral Small 3.1 24B** | 24B | 24B | Apache 2.0 | yes | yes | 128k | Both ar and tr in declared language list; vision + text. |
| **Aya Expanse 32B** | 32B | 32B | **CC-BY-NC** (research only) | yes | yes | 8k | 23 langs incl. ar+tr; 58.8 chrF++ on translation. Non-commercial license — **disqualifies** for any commercial mutercim use; OK for personal/research. |
| **Aya Expanse 8B** | 8B | 8B | CC-BY-NC | yes | yes | 8k | Same restriction. |
| **Llama 4 Scout 17B-16E** | 109B total | 17B | Llama 4 license | yes | **NO** | 10M | Officially supports 12 langs incl. ar, but **Turkish is not listed**. Can translate tr de facto but quality is unverified. |
| **DeepSeek V3.2** | 671B | 37B | DeepSeek (open weights) | yes | yes | 128k | 100+ langs claimed. Strong on zh↔en; tr/ar quality mixed per Slator panel. T3 only. |

### Per-tier recommendation (Arabic+Turkish use case)

| Tier | First pick | Backup | Rationale |
|---|---|---|---|
| **T1 (8 GB)** | **Gemma 3 12B IT @ Q4 QAT** (~7 GB) | **Qwen3 8B @ Q4** (~4.6 GB) | Gemma 3 12B QAT-int4 is purpose-built by Google for consumer GPUs; both ar and tr in pretraining. Qwen3 8B is the safe Apache-2.0 alternative. |
| **T2 (24 GB)** | **Gemma 3 27B IT @ Q4** (~15-17 GB) | **Qwen3 32B @ Q4** (~19 GB) or **Babel-9B-Chat fp16** (~18 GB) | Gemma 3 27B is the strongest open dense generalist with broad multilingual coverage. Qwen3 32B is the Apache-2.0 fallback. Babel-9B fits with headroom for long context and is purpose-built across all 25 top languages including ar+tr. |
| **T3 (48-80 GB)** | **Qwen3-235B-A22B @ Q4** (~140 GB — needs full 80 GB+RAM offload, or dual-GPU) or **Babel-83B @ Q4** (~50 GB) | **DeepSeek V3.2 @ Q4** (~400 GB MoE — needs offload) | At T3 the gap between specialist and generalist shrinks; pick by license and serving stack maturity. Babel-83B is the most credible single-GPU 80 GB target. |

---

## 2. Specialized translation models

### Table 2 — Translation-specialist models

| Model | Params | License | ar | tr | Lang count | Context | Verdict |
|---|---|---|---|---|---|---|---|
| **Tower+ 9B** (Unbabel) | 9B (Gemma 2 base) | CC-BY-NC-4.0 | **NO** | **NO** | 22 European/Asian high-resource | 8k | Strong WMT24++ xComet-xxl scores, beats Gemma 2-9B on IFEval and M-ArenaHard, but **language coverage rules it out** for ar/tr. |
| **Tower+ 2B** | 2B | CC-BY-NC-4.0 | NO | NO | Same 22 | 4k | Same coverage gap; despite outperforming Llama 3.3 on MT, irrelevant for ar/tr. |
| **Tower+ 72B** | 72B (Qwen 2.5 base) | CC-BY-NC-4.0 | NO | NO | Same 22 | 32k | Same coverage gap. |
| **TranslateGemma 27B** | 27B (Gemma 3 base) | Gemma TOS | likely yes (160-lang chat template) | likely yes | 55 "production" / 160 base | **2K input only** | Built on Gemma 3 so ar/tr should work; 12B variant **beats Gemma 3 27B baseline on MetricX/WMT24++**. **2k input is a hard limit for long-form documents** — disqualifies for full-document translation; use as per-paragraph translator. |
| **TranslateGemma 12B** | 12B | Gemma TOS | likely yes | likely yes | 55 / 160 | 2K | Same 2k limit. Best small-model translation quality per Google's own WMT24++ data. |
| **TranslateGemma 4B** | 4B | Gemma TOS | likely yes | likely yes | 55 / 160 | 2K | T1 paragraph-level translator; pairs well with chunking. |
| **Qwen-MT (qwen3-mt-turbo)** | MoE (size undisclosed) | **API-only, weights not released** | yes (multi-dialect) | yes | 92 | API | Excellent quality per Qwen's reports, but **closed weights → fails local-first hard requirement**. Useful as cloud quality boost only. |
| **NLLB-200 distilled 1.3B / 600M** | 1.3B / 600M | CC-BY-NC-4.0 | yes | yes | 200 | **512 tokens** | Still the breadth king for low-resource pairs. **512-token cap and 2022 quality** make it unsuitable as the main translator for long documents but it's an excellent fallback for ultra-low-resource pairs or sentence-by-sentence rescue translation. |
| **MADLAD-400 7B / 10.7B** | 7B / 10.7B | Apache 2.0 | yes | yes | 450 | T5 encoder-decoder, ~512 enc / ~256 dec typical | 2023 model, T5 architecture, breadth not depth. Apache-2.0 is the lone open-license MT specialist that covers ar+tr. Quality below modern LLMs. |
| **ALMA-13B / ALMA-R** | 13B (Llama 2 base) | MIT | NO | NO | 10 directions (en↔de/cs/is/zh/ru only) | 4k | **Does not cover ar or tr** — rules out. |

### When to prefer a specialist over a general LLM

- **You need 200+ language breadth and don't have a viable LLM for the pair**: NLLB-200 or MADLAD-400.
- **You need paragraph-level perfection on a high-resource European pair** and you can accept CC-BY-NC: Tower+ 9B at T2, 72B at T3. **Not applicable to the ar/tr use case.**
- **You can chunk to ≤2k tokens** and want the best small-model MT quality: TranslateGemma 12B.
- **For long-form documents (>2k tokens) and the ar/tr primary pairs**: skip specialists entirely. Use a **strong multilingual general LLM with a translation system prompt**.

### Specialist-vs-generalist for ar/tr specifically

Based on Babel's Flores-200 result (55.1 for 9B vs Gemma2-9B 53.2 and Qwen2.5-7B
45.5), and the fact that **no current open-weight specialist (Tower+, ALMA, the
TranslateGemma >2k caveat) covers ar+tr at high quality and unrestricted
license**, the answer for the user's case is unambiguous: **use Babel-9B-Chat or
a recent multilingual LLM (Gemma 3 27B, Qwen3 32B), not a translation
specialist**. A 32B Qwen does not lose to a 13B Tower on Arabic→Turkish for the
trivial reason that Tower doesn't speak those languages.

For en↔de or en↔zh, the answer would flip in favor of Tower+ 9B / Qwen-MT.

---

## 3. Serving stack

### Table 3 — Serving stack per OS

| OS / hardware | Primary | When to use it | Secondary | Notes |
|---|---|---|---|---|
| **Linux + NVIDIA** | **vLLM 0.19+** | Production-style serving, batched throughput, multi-tenant. Async scheduler on by default, Model Runner V2, day-one Gemma 4 support (March-April 2026 releases). | **llama.cpp** | For 5+ concurrent users, Ollama collapses (Towards AI Apr 2026 benchmark) — vLLM is the only serious choice. Single-user dev: llama.cpp via `llama-server`. |
| **Apple Silicon** | **MLX-LM** (built-in OpenAI-compatible server) | Native, fastest path on M1-M5. mlx-community on HF has 4,316+ pre-converted models. M5 Neural Accelerators give 3.5-4× over M4 on prompt processing. | **Ollama with MLX backend** (preview, 2026) | Apple positioned MLX as preferred at WWDC 2025. Rapid-MLX claims 2-4.2× over Ollama. Avoid llama.cpp on Apple Silicon if MLX-quantized weights exist — MLX is faster. |
| **CPU-only** | **llama.cpp** (GGUF) | Highest portability, runs anywhere, GGUF is now ~60% of quantized HF models. | **Llamafile** (Mozilla, single binary) | For Mac Intel / Linux x86 / Windows without GPU: pick the smallest competent quantization (Qwen3 8B Q4_K_M, Gemma 3 4B Q4) and accept 5-15 tok/s. |
| **Cross-platform "just works"** | **Ollama** | Easy onboarding and Modelfile UX; uses MLX on Apple Silicon in preview. | LM Studio | 10-30% throughput overhead vs raw llama.cpp; fine for single-user CLI use. **Do not** use Ollama for the multi-tenant case. |

### Recommendation summary

- **mutercim default local backend**: **llama.cpp / GGUF**. Universal, single
  binary, runs on Linux+NVIDIA, Apple Silicon (Metal), AMD (Vulkan), and CPU.
  GGUFs of Gemma 3, Qwen3, and Babel-9B all exist.
- **mutercim "quality boost" backend on Apple Silicon**: **MLX-LM**. ~2-4× over
  llama.cpp on Mac.
- **mutercim "throughput / batch" backend**: **vLLM** when running on
  Linux+NVIDIA with 24 GB+.
- **Ollama**: keep as fallback if user already has it installed. Don't make
  it the default — too much overhead.

---

## 4. Language-pair notes

### Arabic ↔ Turkish (primary)

- **No open specialist covers this pair well**. Tower+ excludes both; ALMA
  excludes both; TranslateGemma supports but is capped at 2k tokens.
- **Best open option**: Babel-9B-Chat (Flores-200 = 55.1, both languages in the
  top-25 mix) or Gemma 3 27B IT (broader coverage, longer context, larger
  model). Babel is specifically tuned on under-represented language families
  including Turkic and Semitic.
- **Style nuance**: Arabic dialect handling matters. Qwen-MT (cloud) supports 10
  Arabic varieties (Standard, Egyptian, Mesopotamian, Moroccan, Najdi, Levantine
  N/S, Ta'izzi-Adeni, Tunisian). Open-weight models default to Modern Standard
  Arabic — instruct via prompt if you need a dialect.
- **Cloud rescue path**: When ar↔tr local output is unsatisfactory, Qwen-MT API
  is the strongest "fast path" per their July 2025 release notes.

### Arabic ↔ English (primary)

- Better-served pair. **Gemma 3 27B**, **Qwen3 32B**, and **Babel-9B-Chat** all
  produce solid output. DeepSeek R1 has been studied specifically on
  Arabic-English literary translation (Tandfonline 2025) — strong on automatic
  metrics, weak on pragmatic coherence and emotional depth in poetry/drama.
- For technical/news/legal Arabic→English at T2: **Gemma 3 27B IT @ Q4** is
  the recommended first pick.
- For literary Arabic→English: **prefer cloud (Qwen-MT or Claude) as quality
  boost**; local Gemma/Qwen will be merely "good", not "literary".

### en/zh/ja sanity check

- **en↔zh**: DeepSeek V3.2 dominates open-weight zh↔en; Qwen3 32B is the
  strong dense alternative. Tower+ 9B is also excellent here (zh is one of
  its 22 languages).
- **en↔ja**: Qwen3 (ja in 119-lang mix), Aya Expanse (ja explicit), and Tower+
  9B (ja explicit) all work. For T1, Qwen3 8B is the cleanest pick.
- **en↔fr**: Solved territory; any model in Table 1 works. Tower+ 2B is the
  cost-efficient specialist at T1 if you don't need ar/tr.

---

## 5. VRAM / RAM budget cheatsheet

| Model | fp16 | Q8 | Q4_K_M / Q4 QAT | Notes |
|---|---|---|---|---|
| Gemma 3 4B | ~8 GB | ~4 GB | **~2.5 GB** | T1 floor, fits anywhere. |
| Qwen3 4B | ~8 GB | ~4 GB | **~2.5 GB** | T1 floor. |
| Qwen3 8B | ~16 GB | ~8 GB | **~4.6 GB** | T1 sweet spot. |
| Gemma 3 12B | ~24 GB | ~12 GB | **~7 GB (QAT)** | T1 ceiling; Google QAT-int4 designed for 8 GB consumer GPUs. |
| TranslateGemma 12B | ~24 GB | ~12 GB | ~7 GB | T1+ — but 2k input only. |
| Babel-9B-Chat | ~18 GB | ~9 GB | **~5.5 GB** | T1+/T2 — fp16 fits on 24 GB. |
| Tower+ 9B | ~18 GB | ~9 GB | ~5.5 GB | (not for ar/tr) |
| Qwen3 14B | ~28 GB | ~14 GB | **~8 GB** | T2. |
| Mistral Small 3.1 24B | ~48 GB | ~24 GB | **~14 GB** | T2 sweet spot. |
| Gemma 3 27B IT | ~54 GB | ~27 GB | **~16 GB** | T2 sweet spot. |
| Qwen3 32B | ~64 GB | ~32 GB | **~19 GB** | T2 sweet spot. |
| Aya Expanse 32B | ~64 GB | ~32 GB | ~19 GB | CC-BY-NC. |
| Tower+ 72B | ~144 GB | ~72 GB | ~42 GB | T3 only. |
| Babel-83B | ~166 GB | ~83 GB | **~50 GB** | T3 single-GPU 80 GB target. |
| Qwen3-235B-A22B | ~470 GB | ~235 GB | **~140 GB (Q4)** | T3 high-end, dual-GPU or CPU offload. |
| DeepSeek V3.2 (671B MoE) | ~1.3 TB | — | **~400 GB (Q4)** | T3 exotic. CPU+SSD offload realistic on Mac Studio. |

For long-form documents you also need KV-cache headroom: budget +20-30% on top of
weights, especially for 32k+ contexts.

---

## 6. Concrete plan for mutercim's translate phase

1. **Local default at T1**: Gemma 3 12B IT @ Q4 QAT (`google/gemma-3-12b-it-qat-q4_0-gguf`)
   served via llama.cpp. Apple Silicon: MLX-quantized equivalent.
2. **Local default at T2**: Gemma 3 27B IT @ Q4_K_M served via llama.cpp / MLX /
   vLLM. Apache-2.0 alternative: Qwen3 32B.
3. **Local default at T3**: Babel-83B-Chat @ Q4 (single 80 GB GPU), or
   Qwen3-235B-A22B @ Q4 if dual-GPU or CPU offload is available.
4. **Cloud quality boost** (optional): Qwen-MT (qwen-mt-turbo) for ar↔tr and
   ar↔en when an internet connection exists. Falls back to local on offline.
5. **Hard fallback for ultra-low-resource pairs not in Gemma's 140-language
   pre-training**: NLLB-200-distilled-1.3B, sentence-by-sentence with 512-token
   chunking.
6. **Chunking strategy**: target ≤2k input tokens per call regardless of model
   context window — recent eval literature (arXiv 2509.17249, BABILong-ITA 2025)
   confirms that long-form translation quality degrades even when the model's
   declared context is much larger. Use sentence/paragraph-boundary chunking
   and translate sequentially with prior translated paragraphs as context.

---

## Sources

### Model cards and primary releases

- [Tower+: Bridging Generality and Translation Specialization in Multilingual LLMs (arXiv 2506.17080, 2025-06-20)](https://arxiv.org/html/2506.17080v1)
- [Unbabel/Tower-Plus-9B model card (HF)](https://huggingface.co/Unbabel/Tower-Plus-9B)
- [Tower+ MarkTechPost coverage (2025-06-27)](https://www.marktechpost.com/2025/06/27/unbabel-introduces-tower-a-unified-framework-for-high-fidelity-translation-and-instruction-following-in-multilingual-llms/)
- [Qwen-MT: Where Speed Meets Smart Translation (Qwen blog, 2025-07-24)](https://qwenlm.github.io/blog/qwen-mt/)
- [Qwen3 Technical Report (arXiv 2505.09388, 2025-05-15)](https://arxiv.org/pdf/2505.09388)
- [Qwen3 blog](https://qwenlm.github.io/blog/qwen3/)
- [CohereLabs/aya-expanse-32b model card](https://huggingface.co/CohereLabs/aya-expanse-32b)
- [Aya Expanse: Combining Research Breakthroughs for a New Multilingual Frontier (arXiv 2412.04261)](https://arxiv.org/abs/2412.04261)
- [TranslateGemma blog (Google, 2026)](https://blog.google/innovation-and-ai/technology/developers-tools/translategemma/)
- [google/translategemma-12b-it model card](https://huggingface.co/google/translategemma-12b-it)
- [google/translategemma-4b-it model card](https://huggingface.co/google/translategemma-4b-it)
- [Welcome Gemma 3 (HF blog, 2025)](https://huggingface.co/blog/gemma3)
- [Gemma 3 27B IT model card](https://huggingface.co/google/gemma-3-27b-it)
- [Gemma 3 QAT Models blog (Google Developers)](https://developers.googleblog.com/en/gemma-3-quantized-aware-trained-state-of-the-art-ai-to-consumer-gpus/)
- [google/gemma-3-12b-it-qat-q4_0-gguf](https://huggingface.co/google/gemma-3-12b-it-qat-q4_0-gguf)
- [Gemma 4 model overview (Google AI for Developers)](https://ai.google.dev/gemma/docs/core)
- [Babel: Open Multilingual LLMs Serving Over 90% of Global Speakers (project site)](https://babel-llm.github.io/babel-llm/)
- [Babel paper (arXiv 2503.00865)](https://arxiv.org/pdf/2503.00865)
- [Tower-Babel/Babel-9B model card](https://huggingface.co/Tower-Babel/Babel-9B)
- [Mistral Small 3.1 (Mistral AI, 2025)](https://mistral.ai/news/mistral-small-3-1)
- [mistralai/Mistral-Small-3.1-24B-Instruct-2503](https://huggingface.co/mistralai/Mistral-Small-3.1-24B-Instruct-2503)
- [meta-llama/Llama-4-Scout-17B-16E (model card)](https://huggingface.co/meta-llama/Llama-4-Scout-17B-16E)
- [Llama 4 blog (Meta AI)](https://ai.meta.com/blog/llama-4-multimodal-intelligence/)
- [DeepSeek V3.2-Exp (GitHub)](https://github.com/deepseek-ai/DeepSeek-V3.2-Exp)
- [DeepSeek-V3 Technical Report (arXiv 2412.19437)](https://arxiv.org/pdf/2412.19437)
- [Assessing DeepSeek R1 and ChatGPT 4.5 in Arabic-English literary translation (Cogent Arts, 2025)](https://www.tandfonline.com/doi/full/10.1080/23311983.2025.2531183)

### Translation-specialist papers

- [A Paradigm Shift in Machine Translation: Boosting LLM Translation Performance — ALMA (arXiv 2309.11674)](https://arxiv.org/abs/2309.11674)
- [haoranxu/ALMA-13B (HF)](https://huggingface.co/haoranxu/ALMA-13B)
- [facebook/nllb-200-distilled-1.3B](https://huggingface.co/facebook/nllb-200-distilled-1.3B)
- [facebook/nllb-200-distilled-600M](https://huggingface.co/facebook/nllb-200-distilled-600M)
- [google/madlad400-7b-mt](https://huggingface.co/google/madlad400-7b-mt)
- [MADLAD-400 paper (arXiv 2309.04662)](https://arxiv.org/abs/2309.04662)

### Long-context and evaluation

- [WMT 2024 General Machine Translation Shared Task findings (ACL Anthology)](https://aclanthology.org/2024.wmt-1.1/)
- [WMT 2025 Model Compression Shared Task](https://www2.statmt.org/wmt25/)
- [Extending Automatic Machine Translation Evaluation to Book-Length Documents (arXiv 2509.17249, 2025-09)](https://arxiv.org/html/2509.17249v1)
- [BABILong-ITA: long-context benchmark (CLIC 2025)](https://clic2025.unica.it/wp-content/uploads/2025/09/104_main_long.pdf)

### Serving stack

- [vLLM releases (GitHub)](https://github.com/vllm-project/vllm/releases)
- [vLLM 2026 release update (fazm.ai)](https://fazm.ai/t/vllm-release-2026-update)
- [vLLM Production Deployment 2026 (Spheron)](https://www.spheron.network/blog/vllm-production-deployment-2026/)
- [ml-explore/mlx-lm (GitHub)](https://github.com/ml-explore/mlx-lm)
- [Exploring LLMs with MLX and M5 Neural Accelerators (Apple ML Research, 2026-01)](https://machinelearning.apple.com/research/exploring-llms-mlx-m5)
- [Ollama is now powered by MLX on Apple Silicon (preview)](https://ollama.com/blog/mlx)
- [ggml-org/llama.cpp](https://github.com/ggml-org/llama.cpp)
- [I Tested Ollama vs vLLM vs llama.cpp at 5 concurrent users (Towards AI, Apr 2026)](https://pub.towardsai.net/i-tested-ollama-vs-vllm-vs-llama-cpp-the-easiest-one-collapses-at-5-concurrent-users-d4f8e0e84886?gi=9f69b6e692dc)
- [What Breaks When You Quantize for Translation? (TheSalt)](https://thesalt.substack.com/p/what-breaks-when-you-quantize-for)
