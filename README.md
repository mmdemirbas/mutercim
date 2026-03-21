# mutercim (مترجم)

Layout-preserving document translator.

## What it does

Feed a PDF in one language, get structured output in another with the same layout. An AI-powered
CLI tool that reads page images, extracts structured data, translates it, and renders the result as
Markdown, LaTeX, PDF, or DOCX. Works with any language pair supported by the configured AI models.
Ships with an embedded glossary for Arabic Islamic scholarly texts (honorifics, hadith terminology,
companion names) but the glossary system is general-purpose.

## Pipeline overview

```
PDF/images ──► pages ──► read ──► solve ──► translate ──► write
               │          │        │          │             │
               │          │        │          │             ▼
               │          │        │          │         .md .tex .pdf .docx
               │          │        │          ▼
               │          │        │      translate/{lang}/{input}/NNN.json
               │          │        ▼
               │          │    solve/{input}/NNN.json
               │          ▼
               │      read/{input}/NNN.json
               ▼
           pages/{input}/NNN.png
```

- **pages** — Convert PDF inputs to per-page PNG images (via pdftoppm)
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

# Edit mutercim.yaml to configure your book
# (at minimum, set the title and input path)

# Run the full pipeline
mutercim all

# Or run phases individually
mutercim pages
mutercim read
mutercim solve
mutercim translate
mutercim write

# Find output in write/{lang}/
ls write/en/
```

Minimal `mutercim.yaml`:

```yaml
book:
  title: "My Book"
  source_langs: [ ar ]
  target_langs: [ en ]

inputs:
  - path: ./input/book.pdf
```

Everything else uses sensible defaults (Gemini Flash for reading and translation,
DocLayout-YOLO for layout detection, Markdown + PDF output).

## CLI reference

### Pipeline Commands

| Command     | Description                                                        |
|-------------|--------------------------------------------------------------------|
| `all`       | Run all phases sequentially (pages, read, solve, translate, write) |
| `pages`     | Convert PDF inputs to per-page images                              |
| `read`      | Read structured data from page images via AI vision                |
| `solve`     | Resolve abbreviations and knowledge context                        |
| `translate` | Translate solved pages into target languages                       |
| `write`     | Render translated data to output formats                           |

### Workspace Commands

| Command  | Description                                                     |
|----------|-----------------------------------------------------------------|
| `init`   | Initialize a new book workspace in current directory            |
| `status` | Show processing progress and validation warnings                |
| `config` | Show effective configuration (merged config + flags + defaults) |
| `clean`  | Delete generated data for specified phases                      |

### Common flags

```
--config, -c    Path to config file (default: ./mutercim.yaml)
--pages, -p     Page range: "1-50", "1,5,10-20" (default: from config or all)
--log-level     Log verbosity: debug, info, warn, error (default: info)
--auto          Auto-run missing prerequisite phases before the requested phase
--force         Force re-processing of already completed pages
```

### Clean command

Targets: `log`, `memory`, `pages`, `read`, `solve`, `translate`, `write`, `all`

```bash
mutercim clean read # delete only read/
mutercim clean read+ # delete read/ and all downstream (solve/, translate/, write/)
mutercim clean pages+ # delete pages/ through write/
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
# Book metadata
book:
  title: "My Book"
  source_langs: [ ar ]           # ISO 639-1 codes
  target_langs: [ tr, en ]       # translate into multiple languages

# Input files — PDF or directories of images
# Each can have an optional per-input page range
inputs:
  - path: ./input/book.pdf
    pages: "1-100"             # process only pages 1-100 of this PDF
  - path: ./input/volume2.pdf  # no page restriction

# PDF-to-image conversion DPI (default: 300)
dpi: 300

# Read phase — AI vision OCR
read:
  # Layout detection tool: "doclayout-yolo" (default), "surya", or "" (AI-only)
  layout_tool: "doclayout-yolo"
  # Ordered model failover chain — tries first, falls over on 429
  models:
    - { provider: gemini, model: gemini-2.5-flash-lite }
    - { provider: gemini, model: gemini-2.5-flash }
    - { provider: groq, model: llama-3.2-90b-vision-preview }
    - { provider: ollama, model: qwen2.5vl:7b }
  concurrency: 1               # parallel workers (reserved for future use)

# Translate phase
translate:
  models:
    - { provider: groq, model: llama-3.3-70b-versatile }
    - { provider: gemini, model: gemini-2.5-flash-lite }
    - { provider: mistral, model: mistral-small-latest }
    - { provider: openrouter, model: meta-llama/llama-3.3-70b-instruct:free }
    - { provider: ollama, model: qwen3:14b }
  context_window: 2            # number of previous pages as context (default: 2)

# Write phase — output rendering
write:
  formats: [ md, pdf ]           # output formats (default: [md, pdf])
    # options: md, latex, pdf, docx
    # "pdf" implies LaTeX generation + Docker compilation
  # "latex" generates only .tex
  expand_sources: true         # expand source abbreviations in output
  latex_docker_image: mutercim/xelatex:latest

# Knowledge: list of YAML files and/or directories (default: [./knowledge])
# Directories include all .yaml/.yml files.
knowledge:
  - ./knowledge
  # - ./extra-terms.yaml

# Retry settings for API calls
retry:
  max_attempts: 3              # retries per API call (default: 3)
  backoff_seconds: 2           # base exponential backoff (default: 2)

# Global rate limit fallback (per-model RPM in ModelSpec takes precedence)
rate_limit:
  requests_per_minute: 14
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
├── log/
│   └── mutercim.log           # [generated] activity log
├── pages/                     # [generated] page images from PDF conversion
│   └── book/                  #   organized by input file stem
│       ├── 001.png
│       ├── 002.png
│       └── ...
├── read/                      # [generated] structured JSON from AI vision
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
        ├── el-Câmius-Sağîr.md
        ├── el-Câmius-Sağîr.tex
        ├── el-Câmius-Sağîr.pdf
        └── el-Câmius-Sağîr.docx
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
| groq       | OpenAI-compat    | No*    | 30          | `GROQ_API_KEY`                                    |
| mistral    | OpenAI-compat    | No*    | 60          | `MISTRAL_API_KEY`                                 |
| openrouter | OpenAI-compat    | No*    | 200         | `OPENROUTER_API_KEY`                              |
| xai        | OpenAI-compat    | No*    | 60          | `XAI_API_KEY`                                     |
| ollama     | Ollama native    | Yes    | 1000        | `OLLAMA_HOST` (default: `http://localhost:11434`) |

\* Vision support can be enabled per-model with `vision: true` in the model spec.

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
read:
  layout_tool: "doclayout-yolo"  # or "surya" or ""
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
`mutercim translate --auto` will run `pages`, `read`, and `solve` first if their
outputs don't exist.

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
