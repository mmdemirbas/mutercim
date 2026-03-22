# mutercim (مترجم)

Layout-preserving document translator.

## What it does

Feed a PDF in one language, get structured output in another with the same layout. An AI-powered
CLI tool that reads page images, extracts structured data, translates it, and renders the result as
Markdown, LaTeX, PDF, or DOCX. Works with any language pair supported by the configured AI models.
The glossary system is general-purpose — provide your own terminology files for any domain.

## Pipeline overview

```
PDF/images ──► cut ──► layout ──► ocr ──► read ──► solve ──► translate ──► write
               │          │         │        │        │          │             │
               │          │         │        │        │          │             ▼
               │          │         │        │        │          │         .md .tex .pdf .docx
               │          │         │        │        │          ▼
               │          │         │        │        │      translate/{lang}/{input}/NNN.json
               │          │         │        │        ▼
               │          │         │        │    solve/{input}/NNN.json
               │          │         │        ▼
               │          │         │    read/{input}/NNN.json
               │          │         ▼
               │          │     ocr/{input}/NNN.json
               │          ▼
               │      layout/{input}/NNN.json
               ▼
           cut/{input}/NNN.png
```

- **cut** — Convert PDF inputs to per-page PNG images (via pdftoppm)
- **layout** — Detect document regions with bounding boxes (via DocLayout-YOLO or Surya)
- **read** — Extract structured text from page images using AI vision models
- **solve** — Resolve abbreviations, inject glossary context, validate structure (local, no API)
- **translate** — Translate solved pages into target languages using AI models
- **write** — Render translated data into final output files (Markdown, LaTeX, PDF, DOCX)

## Sample output

TODO: add before/after screenshots

## Prerequisites

- **Go 1.23+** (the project uses Go 1.26, but 1.23+ should work) — for building from source
- **Docker** — all external tools (pdftoppm, pandoc, DocLayout-YOLO, XeLaTeX) run in containers.
  Container images are built automatically on first use from the `docker/` directory.
- **At least one AI provider API key** — see [Provider authentication](#provider-authentication)

## Quick start

```bash
# Build from source
git clone https://github.com/mmdemirbas/mutercim.git
cd mutercim
go build -o build/mutercim ./cmd/mutercim
# optionally: go install ./cmd/mutercim

# Create a workspace
mkdir ~/my-book && cd ~/my-book
mutercim init

# Place your PDF in input/
cp /path/to/book.pdf input/

# Set API key(s)
echo 'GEMINI_API_KEY=your-key-here' >.env

# Edit mutercim.yaml (at minimum, set input path and source language)

# Run the full pipeline
mutercim all

# Or run phases individually
mutercim cut
mutercim layout
mutercim read
mutercim solve
mutercim translate
mutercim write

# Find output in write/{lang}/
ls write/en/
```

Minimal `mutercim.yaml`:

```yaml
inputs:
  - path: ./input/book.pdf
    languages: [ ar ]

translate:
  languages: [ en ]
```

Everything else uses sensible defaults (Gemini Flash for reading and translation,
DocLayout-YOLO for layout detection, Markdown + PDF output).

## CLI reference

### Pipeline Commands

| Command     | Description                                                              |
|-------------|--------------------------------------------------------------------------|
| `all`       | Run all phases sequentially (cut, layout, ocr, read, solve, translate, write) |
| `cut`       | Convert PDF inputs to per-page images                                         |
| `layout`    | Detect document layout regions on page images                                 |
| `ocr`       | Extract text from page images using OCR (optional)                            |
| `read`      | Read structured data from page images or OCR output via AI                    |
| `solve`     | Resolve abbreviations and knowledge context                                   |
| `translate` | Translate solved pages into target languages                                  |
| `write`     | Render translated data to output formats                                      |

### Workspace Commands

| Command  | Description                                                     |
|----------|-----------------------------------------------------------------|
| `init`   | Initialize a new book workspace in current directory            |
| `status` | Show processing progress and validation warnings                |
| `config` | Show effective configuration (merged config + flags + defaults) |
| `clean`  | Delete generated data for specified phases                      |

### Common flags

```
-c, --config      Path to config file (default: ./mutercim.yaml)
-p, --pages       Page range: "1-50", "1,5,10-20" (default: from config or all)
-o, --output      Output directory (default: .)
-l, --log-level   Log verbosity: debug, info, warn, error (default: from config or info)
-a, --auto        Auto-run missing prerequisite phases before the requested phase
-f, --force       Force re-processing of already completed pages
```

### Per-phase flags

Each phase command accepts flags that override config file settings:

```
cut:        -d, --dpi           DPI for PDF-to-image conversion
layout:     -t, --tool          Layout tool: doclayout-yolo, surya, or ""
                --debug         Write debug overlay images
read:       -P, --provider      AI provider name
            -m, --model         AI model name
            -n, --concurrency   Parallel read workers
translate:  -P, --provider      AI provider name
            -m, --model         AI model name
            -w, --context-window Previous pages for context
write:      -F, --format        Output formats (comma-separated: md,latex,pdf,docx)
```

### Clean command

Targets: `log`, `memory`, `cut`, `layout`, `ocr`, `read`, `solve`, `translate`, `write`, `all`

```bash
mutercim clean read # delete only read/
mutercim clean read+ # delete read/ and all downstream (solve/, translate/, write/)
mutercim clean cut+ # delete cut/ through write/
mutercim clean all # delete everything except input/, knowledge/, mutercim.yaml, .env
mutercim clean log read # delete multiple specific targets
```

### Output format arguments

`write` and `all` accept positional format arguments that override the config:

```bash
mutercim write md # only Markdown
mutercim all pdf docx # only PDF and DOCX
mutercim write latex # only .tex (no PDF compilation)
```

## Configuration

Full annotated `mutercim.yaml`:

```yaml
# Log verbosity (default: info). Can be overridden with -l flag.
log_level: info                  # debug, info, warn, error

# Input files — PDF or directories of images
# Each declares its own source languages and optional page range
inputs:
  - path: ./input/book.pdf
    languages: [ ar ]            # ISO 639-1 source language codes
    pages: "1-100"               # process only pages 1-100 of this PDF
  - path: ./input/volume2.pdf    # no page restriction
    languages: [ ar ]

# PDF-to-image conversion
cut:
  dpi: 300                       # DPI (default: 300, min: 72)

# Layout detection (runs before read phase)
layout:
  tool: doclayout-yolo           # "doclayout-yolo" (default), "surya", or "" (disabled)
  debug: false                   # write overlay PNGs to layout/<input>/debug/
  # params:
  #   confidence: 0.2            # min detection score (default 0.2)
  #   iou: 0.7                   # NMS threshold (default 0.7)
  #   image_size: 1024           # inference resolution (default 1024)

# OCR phase — text extraction (optional, runs between layout and read)
# When enabled, the read phase can use text-only LLMs instead of vision models
ocr:
  tool: ""                       # "qari" (Qari-OCR v0.3, Arabic-specialized) or "" (disabled)

# Read phase — structural analysis (uses OCR text and/or page images)
read:
  models: # ordered model failover chain
    - { provider: gemini, model: gemini-2.5-flash-lite }
    - { provider: gemini, model: gemini-2.5-flash }
    - { provider: groq, model: llama-3.2-90b-vision-preview }
    - { provider: ollama, model: qwen2.5vl:7b }
  concurrency: 1
  retry:
    max_attempts: 3
    backoff_seconds: 2

# Translate phase
translate:
  languages: [ tr, en ]          # target languages — each gets its own output
  models:
    - { provider: groq, model: llama-3.3-70b-versatile }
    - { provider: gemini, model: gemini-2.5-flash-lite }
    - { provider: mistral, model: mistral-small-latest }
    - { provider: openrouter, model: meta-llama/llama-3.3-70b-instruct:free }
    - { provider: ollama, model: qwen3:14b }
  context_window: 2

# Write phase — output rendering
write:
  formats: [ md, pdf ]           # options: md, latex, pdf, docx
  expand_sources: true

# Knowledge: list of YAML files and/or directories (default: [./knowledge])
knowledge:
  - ./knowledge
```

### ModelSpec fields

Each model in a failover chain can configure:

```yaml
- provider: gemini             # required: provider name
  model: gemini-2.5-flash      # required: model identifier
  rpm: 30                      # optional: requests/minute override (0 = provider default)
  vision: true                 # optional: vision support override (auto-detected if omitted)
  base_url: https://custom.api # optional: custom API endpoint
```

## Workspace layout

```
my-book/                       # workspace root
├── mutercim.yaml              # [user] book configuration
├── .env                       # [user] API keys (KEY=value, one per line)
├── input/                     # [user] source PDFs or image directories
│   └── book.pdf
├── knowledge/                 # [user] glossary YAML files (never deleted by clean)
│   └── glossary.yaml
├── memory/                    # [generated] auto-extracted knowledge from solve phase
├── mutercim.log               # [generated] activity log
├── cut/                       # [generated] page images from PDF conversion
│   └── book/                  #   organized by input file stem
│       ├── 001.png
│       ├── 002.png
│       └── ...
├── layout/                    # [generated] layout detection regions
│   └── book/
│       ├── 001.json
│       ├── debug/             #   overlay images (when layout.debug: true)
│       │   └── 001_layout.png
│       └── ...
├── ocr/                       # [generated] OCR text extraction (when ocr.tool configured)
│   └── book/
│       ├── 001.json
│       ├── report.json
│       └── ...
├── read/                      # [generated] structured JSON from AI
│   └── book/
│       ├── 001.json
│       └── ...
├── solve/                     # [generated] enriched JSON with glossary context
│   └── book/
│       ├── 001.json
│       └── ...
├── translate/                 # [generated] translated JSON per language
│   └── tr/
│       └── book/
│           ├── 001.json
│           └── ...
└── write/                     # [generated] final output files per language
    └── tr/
        ├── book.md
        ├── book.tex
        ├── book.pdf
        └── book.docx
```

Directories marked `[user]` are provided by the user and never deleted by `clean`.
Directories marked `[generated]` are created by the tool and can be cleaned with `mutercim clean`.

## Glossary system

Knowledge files use a unified YAML schema with ISO 639-1 language code keys.
Values can be a string or a list of strings (first is canonical, rest are variants).

```yaml
entries:
  # Simple entry
  - ar: "حديث"
    tr: "hadîs-i şerîf"
    en: "hadith"

  # Entry with variants
  - ar: [ "صلى الله عليه وسلم", "ﷺ", "صلعم" ]
    tr: [ "sallallâhu aleyhi ve sellem", "s.a.v." ]
    en: [ "peace be upon him", "PBUH" ]
    note: "Salawat. Must appear after every mention of the Prophet."

  # Minimal — just two languages
  - ar: "فقه"
    tr: "fıkıh"
```

The `note` field is optional guidance for the AI translator.

### Knowledge layers

Knowledge loads in two layers (later layers override earlier on key conflicts):

1. **Workspace `knowledge/`** — user-provided YAML files specific to this book.
   Any `.yaml` file in the directory is loaded and merged. A comprehensive Arabic/Turkish/English
   glossary is available at `config/glossary.yaml` in the repository — copy it to your workspace
   `knowledge/` directory as a starting point.
2. **Auto-extracted `memory/`** — knowledge extracted by the solve phase during processing.
   Can be reset with `mutercim clean memory`.

## Model failover

Models are configured as ordered lists. When a model returns HTTP 429 (quota exhaustion),
it is marked as exhausted and the next model in the chain is tried automatically. Exhausted
models recover after 60 seconds.

Non-vision models are automatically skipped during the read phase (which requires image input).

Example failover chain:

```yaml
read:
  models:
    - { provider: gemini, model: gemini-2.5-flash-lite }   # try first (free)
    - { provider: gemini, model: gemini-2.5-flash }         # fallback
    - { provider: groq, model: llama-3.2-90b-vision-preview }
```

### Provider architecture

| Provider   | API format       | Vision | Default RPM | Env var                                           |
|------------|------------------|--------|-------------|---------------------------------------------------|
| gemini     | Gemini native    | Yes    | 10          | `GEMINI_API_KEY`                                  |
| claude     | Anthropic native | Yes    | 50          | `ANTHROPIC_API_KEY`                               |
| openai     | OpenAI           | Yes    | 500         | `OPENAI_API_KEY`                                  |
| groq       | OpenAI-compat    | Auto*  | 30          | `GROQ_API_KEY`                                    |
| mistral    | OpenAI-compat    | Auto*  | 60          | `MISTRAL_API_KEY`                                 |
| openrouter | OpenAI-compat    | Auto*  | 200         | `OPENROUTER_API_KEY`                              |
| xai        | OpenAI-compat    | Auto*  | 60          | `XAI_API_KEY`                                     |
| ollama     | Ollama native    | Yes    | 1000        | `OLLAMA_HOST` (default: `http://localhost:11434`) |

\* Vision auto-detected from model name (patterns: `vision`, `vl`, `scout`, `gemma-3`, `pixtral`).
Override with `vision: true/false` in the model spec.

### Provider authentication

API keys are read from environment variables. You can set them in a `.env` file in the
workspace root:

```
GEMINI_API_KEY=your-gemini-key
GROQ_API_KEY=your-groq-key
MISTRAL_API_KEY=your-mistral-key
```

The `.env` file supports `KEY=value`, `export KEY=value`, quoted values, and inline comments.

## Layout detection

Layout detection identifies document structure (columns, headers, footnotes, tables) before
sending pages to the AI for text extraction. Three options:

| Tool                       | How it works                                                                      | When to use                                                  |
|----------------------------|-----------------------------------------------------------------------------------|--------------------------------------------------------------|
| `doclayout-yolo` (default) | Docker-based YOLO model. Detects regions with bounding boxes and types (no text). | Best for most documents. Detects columns, tables, footnotes. |
| `surya`                    | Docker-based OCR. Detects text lines with bounding boxes and preliminary text.    | When you need text-line-level detection.                     |
| `""` (empty/AI-only)       | No layout tool. AI analyzes the full page image directly.                         | Simpler documents, or when Docker is unavailable.            |

Configure in `mutercim.yaml`:

```yaml
layout:
  tool: doclayout-yolo  # or "surya" or ""
  debug: true           # overlay images for visual verification
  params: # tool-specific tuning
    confidence: 0.15
```

Docker images are built automatically on first use when running from the repo directory.
To pre-build all images:

```bash
task docker-all
```

## Smart rebuilds

Each phase compares output file timestamps against all its input timestamps. If any input
is newer than the output, the page is reprocessed. Otherwise it is skipped. There is no
`progress.json` — the filesystem is the source of truth.

Use `--force` to bypass timestamp checks and reprocess everything.

Use `--auto` to automatically run missing prerequisite phases. For example,
`mutercim translate --auto` will run `cut`, `layout`, `read`, and `solve` first if
their outputs don't exist.

## Development

### Build and test

This project uses [Task](https://taskfile.dev) for automation (not Make):

```bash
task build # build for current platform
task vet # run static analysis
task test # run tests (normal + race detector)
task install # install binary + zsh completion
task all # build + vet + test + install
task dist # cross-compile for linux/windows/darwin
task schema # regenerate config JSON schema
```

### After every change

```bash
go build ./...
go vet ./...
go test ./...
```

### Project conventions

- `CLAUDE.md` — agent instructions and coding rules
- `docs/DECISIONS.md` — spec overrides (source of truth over `docs/SPEC.md`)
- `docs/GO-CONVENTIONS.md` — Go coding patterns
- Table-driven tests, `httptest.NewServer` for HTTP tests, `t.TempDir()` for file I/O
- No global mutable state, no `init()` functions, no `panic`/`os.Exit` outside `main.go`
- `log/slog` for logging, `net/http` for HTTP, minimal dependencies

## License

[MIT](LICENSE)
