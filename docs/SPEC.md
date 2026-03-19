# Mutercim — Specification & Claude Code Prompt

## Project Identity

**Name**: `mutercim` (مترجم — "the translator", classical Ottoman Turkish scholarly title)
**Purpose**: A CLI tool that translates Arabic Islamic scholarly books into Turkish, preserving layout, structure, and domain-specific terminology. Designed for hadith collections but generalizable to tafsir, fiqh, sirah, and other Islamic literature.
**Language**: Go
**License**: MIT

### Language Decision Rationale

Go was chosen over Python and Kotlin after evaluating the actual workload:

- **Mechanical enforcement over self-discipline**: AI coding agents generate code for this project. Go's compiler catches type mismatches, unused variables, and interface violations at build time. Python's type checking is advisory; Kotlin's compiler is equally strict but Gradle adds disproportionate build system complexity for a CLI tool.
- **JSON strictness is a feature**: AI models occasionally deviate from requested schemas. Go's strict unmarshaling surfaces these deviations immediately (triggering retry), rather than silently coercing types. For unattended 600-page batch processing, fail-fast is correct.
- **HTTP client code is a one-time cost**: The ~500 lines of API client infrastructure (shared HTTP client + 70 lines per provider) replaces what Python SDKs provide. This code is written once, rarely changes, and gives full control over retry/rate-limiting behavior.
- **Distribution**: Single binary with zero runtime dependencies. No Python venv, no JRE, no GraalVM native-image configuration.
- **Dependency sanity**: `go.mod` with 3 dependencies (cobra, viper, yaml.v3). No version conflicts, no transitive dependency surprises.

---

## Core Principles

1. **Dual output**: Every book produces two valuable artifacts — a clean digital Arabic text (from OCR) and its Turkish translation.
2. **Model-agnostic**: Reading and translation use swappable AI backends. Free models default, premium models available.
3. **Incremental & resumable**: Results are saved per-page immediately. Processing can stop and resume at any point.
4. **Domain-aware**: Islamic scholarly terminology, honorifics, person names, and source abbreviations are handled correctly via pluggable knowledge modules.
5. **Best-effort with transparency**: Errors don't halt processing. Every anomaly is logged and flagged for human review.

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                        mutercim CLI                         │
│  mutercim read | solve | translate | write | make           │
└─────┬───────────┬──────────────┬─────────────┬──────────────┘
      │           │              │             │
      ▼           ▼              ▼             ▼
┌──────────┐ ┌──────────┐ ┌───────────┐ ┌───────────┐
│ Phase 1  │ │ Phase 2  │ │  Phase 3  │ │  Phase 4  │
│ READ     │ │ SOLVE    │ │ TRANSLATE │ │  WRITE    │
│          │ │          │ │           │ │           │
│ Image →  │ │ Validate │ │ Arabic →  │ │ JSON →    │
│ Struct.  │ │ Resolve  │ │ Turkish   │ │ Markdown  │
│ JSON     │ │ Merge    │ │ JSON      │ │ LaTeX→PDF │
│          │ │ Flag     │ │           │ │ DOCX      │
│ AI-based │ │ Local    │ │ AI-based  │ │ Local     │
│ Parallel │ │ Sequent. │ │ Parallel  │ │ Local     │
└──────────┘ └──────────┘ └───────────┘ └───────────┘
```

### Phase 1: READ (parallelizable, AI-based)

**Input**: Page images (PNG/JPG) or PDF pages converted to images.
**Output**: One JSON file per page in `midstate/read/page_NNN.json`.
**Model**: Gemini 2.0 Flash (free default) / Claude Sonnet / Qwen2.5-VL (local) / Surya (local OCR + AI structural parse)

The read prompt asks the vision model to:
1. Identify all visual zones on the page (header, body entries, separator, footnotes, page number, margin notes, etc.)
2. Extract Arabic text with full tashkeel preservation
3. Classify each entry by type (hadith, athar, commentary, chapter heading, etc.)
4. Extract entry numbers
5. Detect cross-page continuations (entry starts mid-sentence, no number at top)
6. Extract footnote text with source abbreviation codes
7. Return structured JSON

**No cross-page reasoning happens here.** Each page is processed independently. This makes Phase 1 embarrassingly parallel.

**Read prompt strategy**: The system prompt should NOT describe a specific book layout. Instead, it should instruct the model to analyze the page as a general Islamic scholarly text and identify structural elements. Example instruction areas:
- "Identify numbered entries — these are typically hadith or athar"
- "Detect separator lines (asterisks, horizontal rules) that divide main text from footnotes"
- "Recognize source abbreviation codes in parentheses within footnotes"
- "Flag if the first entry on the page appears to be a continuation (no number, starts mid-sentence)"
- "Preserve all diacritical marks (tashkeel/harakat) exactly as they appear"

### Phase 2: SOLVE (sequential, local, no API calls)

**Input**: All read JSONs + knowledge YAML files.
**Output**: Solved JSONs in `midstate/solved/page_NNN.json`.

This phase performs:
1. **Abbreviation resolution**: Match source codes (ت، ن، حم) against the source abbreviation table (from knowledge YAML or auto-detected from early book pages)
2. **Cross-page merging**: If page N ends with an incomplete entry and page N+1 starts with a continuation, link them with a `continues_from` / `continues_on` field
3. **Hadith number validation**: Check sequential numbering, flag gaps or duplicates
4. **Structural consistency**: Verify entry types, flag anomalies
5. **Knowledge injection**: Attach relevant glossary entries that the translator will need for this page's content

**Output adds fields**: `sources_resolved`, `continuation_info`, `validation_warnings`, `translation_context` (relevant glossary terms for this page).

### Phase 3: TRANSLATE (parallelizable in chunks, AI-based)

**Input**: Solved JSONs + knowledge YAMLs + sliding context window.
**Output**: One JSON file per page in `midstate/translated/page_NNN.json`.
**Model**: Gemini 2.0 Flash (free default) / Claude Sonnet / Claude Opus (premium) / local models

The translation system prompt includes:
- Full domain knowledge (honorifics, person name mappings, terminology glossary)
- Source abbreviation table with Turkish equivalents
- Instructions for meaning-first translation (not word-by-word)
- Rules for handling Arabic honorific formulas (صلى الله عليه وسلم → sallallâhu aleyhi ve sellem)
- Context from previous 1-2 pages (sliding window)
- The page's `translation_context` from solve phase

**Parallelization strategy**: Process in sequential chunks of 10-20 pages. Within each chunk, process sequentially (for context). Different chunks can run in parallel with 1-2 page overlap at boundaries. For v1, just process all pages sequentially.

**Output structure mirrors input**: Same JSON schema but with `turkish_text`, `translated_footnote`, `translated_header` fields added alongside the Arabic originals.

### Phase 4: WRITE (local, no API calls)

**Input**: Translated JSONs + output templates.
**Output**: Final documents in `output/`.

Renderers:
1. **Markdown renderer**: Produces `output/ar/book.md` and `output/tr/book.md`. Quick review format. One file per book with page breaks as horizontal rules.
2. **LaTeX renderer**: Produces `output/tr/latex/book.tex`. Uses XeLaTeX with `polyglossia` + `bidi` for proper Arabic/Turkish mixed typesetting. Compiles to PDF inside a Docker container.
3. **DOCX renderer** (optional, via pandoc from Markdown or direct generation): `output/tr/book.docx`.

Per-page incremental output: Each renderer also writes per-page files (`output/tr/pages/page_NNN.md`) as soon as translation completes, so the user can review while processing continues.

---

## Workspace Model

mutercim separates the **tool** (installed binary + embedded defaults) from **book workspaces** (one directory per book). Each book is an independent workspace with its own config, knowledge, and outputs.

### Workspace Directory Structure

```
~/translations/el-camius-sagir/      # One workspace per book
├── mutercim.yaml                    # Book config (sections, models, language pair)
├── knowledge/                       # Book-specific knowledge (persistent, user-reviewed)
│   ├── sources.yaml                 # THIS book's abbreviation table
│   └── custom.yaml                  # Any overrides or additions
├── input/                           # Source material
│   └── book.pdf                     # Or a directory of scanned images
├── midstate/                        # All intermediate artifacts
│   ├── images/                      # PDF pages converted to PNG
│   ├── read/                        # Phase 1 output: page_NNN.json
│   ├── solved/                      # Phase 2 output: page_NNN.json
│   ├── translated/                  # Phase 3 output: page_NNN.json
│   └── staged/                      # Auto-detected knowledge (pending review)
│       └── sources_pages_6-8.yaml   # e.g., abbreviation table detected from pages 6-8
├── output/                          # Final deliverables
│   ├── ar/                          # Reconstructed Arabic text
│   │   ├── book.md
│   │   └── pages/                   # Per-page files written incrementally
│   ├── tr/                          # Translated Turkish text
│   │   ├── book.md
│   │   ├── pages/
│   │   ├── latex/
│   │   │   ├── book.tex
│   │   │   └── book.pdf
│   │   └── book.docx
├── reports/                         # Per-phase reports
│   ├── read_report.json
│   ├── solve_report.json
│   └── translate_report.json
└── progress.json                    # Checkpoint state
```

### Knowledge Layering

Knowledge loads in three layers. Later layers override earlier on key conflicts:

```
Layer 1: Embedded defaults (compiled into the binary via go:embed)
  → Common honorifics, 50+ person names, core Islamic terminology, common places

Layer 2: Workspace knowledge/ directory (persistent, user-reviewed)
  → Book-specific sources, custom overrides, promoted staged entries

Layer 3: Staged midstate/staged/ (auto-detected, pending review)
  → Abbreviation tables, terminology detected during read phase
  → Used during solve/translation but marked as "staged" in output
  → NOT written to persistent knowledge unless user explicitly promotes
```

### Staging Area — Knowledge Auto-Detection Lifecycle

When Phase 1 reads a page with `type: reference_table` (e.g., an abbreviation key), the detected key-value pairs are written to the staging area:

```
1. Read phase detects abbreviation table on pages 6-8
   → Writes midstate/staged/sources_pages_6-8.yaml

2. Solve phase loads all three knowledge layers
   → Staged sources are used for abbreviation resolution
   → Solved output marks resolved sources with "source: staged" flag

3. User reviews staged files:
   mutercim knowledge staged         # List staged files with summaries
   mutercim knowledge diff           # Show staged vs persistent differences
   mutercim knowledge promote <file> # Merge into knowledge/sources.yaml

4. After promotion, re-running solve/translate uses the persistent version
   → "source: staged" flags become "source: workspace"
```

The staged files use the exact same YAML format as persistent knowledge files, so promotion is a merge operation — no format conversion. The user can also hand-edit before promoting.

---

## CLI Interface

```
mutercim <command> [flags]

Workspace Commands:
  init          Initialize a new book workspace in current directory
  status        Show processing progress and any flagged issues
  config        Show effective configuration (merged config + flags + defaults)

Pipeline Commands:
  read          Read text and structure from book pages (Phase 1)
  solve         Validate, resolve, and enhance read data (Phase 2)
  translate     Translate solved data to target language (Phase 3)
  write         Render translated data to output formats (Phase 4)
  make          Execute all phases sequentially (read → solve → translate → write)
  validate      Run validation checks on read/translated data without processing

Knowledge Commands:
  knowledge list       Show all loaded knowledge (embedded + workspace + staged) with layer info
  knowledge staged     List staged knowledge files with entry counts and source pages
  knowledge diff       Show what staged entries add/override on top of persistent knowledge
  knowledge promote    Merge a staged file into the persistent workspace knowledge directory

Common Flags (all pipeline commands):
  --config, -c        Path to config file (default: ./mutercim.yaml)
  --pages, -p         Page range: "1-50", "1,5,10-20", "all" (default: "all")
  --log-level         Log verbosity: debug, info, warn, error (default: info)
  --log-file          Also write logs to this file

Init Flags:
  --interactive, -i   Prompt for book title, author, language pair (default: true)
  --non-interactive   Scaffold workspace with defaults, no prompts
  --title             Book title (non-interactive mode)
  --author            Book author (non-interactive mode)
  --source-lang       Source language (default: ar)
  --target-lang       Target language (default: tr)

Read-specific:
  --read-model        Model for reading (default: gemini-2.0-flash)
  --read-provider     Provider: gemini, claude, openai, ollama, surya (default: gemini)
  --concurrency       Parallel read workers (default: 1)
  --dpi               DPI for PDF-to-image conversion (default: 300)

Translate-specific:
  --translate-model    Model for translation (default: gemini-2.0-flash)
  --translate-provider Provider: gemini, claude, openai, ollama (default: gemini)
  --context-window     Number of previous pages to include as context (default: 2)

Write-specific:
  --format             Output formats, comma-separated: md,latex,docx (default: md,latex)
  --latex-docker-image Docker image for LaTeX compilation (default: mutercim/xelatex:latest)
  --skip-pdf           Generate .tex but don't compile to PDF
  --expand-sources     Expand source abbreviations to full names (default: true)

Knowledge Flags:
  knowledge promote <file>  --merge    Merge entries into existing file (default)
  knowledge promote <file>  --replace  Replace entire file

Provider Authentication (via environment variables):
  GEMINI_API_KEY       Google AI API key (for Gemini models)
  ANTHROPIC_API_KEY    Anthropic API key (for Claude models)
  OPENAI_API_KEY       OpenAI API key (for GPT models)
  OLLAMA_HOST          Ollama server address (default: http://localhost:11434)
```

### Init Examples

```bash
# Interactive — prompts for title, author, language pair
mkdir el-camius-sagir && cd el-camius-sagir
mutercim init

# Non-interactive — useful for scripting or when piped from another tool
mutercim init --non-interactive --title "el-Câmiu's-Sağîr" --author "İmam Süyûtî"

# Minimal — just scaffold with defaults, edit mutercim.yaml later
mutercim init --non-interactive
```

### Config File (mutercim.yaml)

```yaml
book:
  title: "el-Câmiu's-Sağîr"
  author: "İmam Süyûtî"
  source_lang: ar
  target_lang: tr

# Paths (relative to workspace root)
input: ./input
output: ./output
midstate_dir: ./midstate
dpi: 300

# Sections define the book's internal layout structure.
# Pages not covered by any section default to type: auto.
# If no sections are defined at all, entire book is type: auto.
sections:
  - name: front_matter
    pages: "1-2"
    type: skip                 # Don't process

  - name: introduction
    pages: "3-5"
    type: prose                # Free-form paragraphs
    translate: true

  - name: abbreviation_table
    pages: "6-8"
    type: reference_table      # Key-value pairs → auto-staged to knowledge
    translate: false

  - name: table_of_contents
    pages: "9-12"
    type: toc                  # Chapter listing with page numbers
    translate: true

  - name: hadith_entries
    pages: "13-580"
    type: scholarly_entries     # Numbered entries + footnotes with source codes
    translate: true

  - name: indices
    pages: "581-590"
    type: index                # Alphabetical index
    translate: true

  # Pages not matching any section above → type: auto (AI determines layout)

# Section types reference:
#   skip              - Don't process these pages
#   prose             - Continuous paragraphs (introductions, prefaces)
#   scholarly_entries  - Numbered entries + footnotes with source codes (hadith, athar)
#   reference_table   - Key-value reference data (abbreviation keys → auto-staged)
#   toc               - Table of contents
#   index             - Alphabetical index
#   auto              - AI detects layout (default for unconfigured pages)

# Model configuration — different model per phase, mix providers freely
read:
  provider: gemini
  model: gemini-2.0-flash
  concurrency: 1

translate:
  provider: gemini
  model: gemini-2.0-flash
  context_window: 2

write:
  formats: [md, latex]
  expand_sources: true
  latex_docker_image: mutercim/xelatex:latest
  skip_pdf: false

# Knowledge paths (relative to workspace root)
# Files here are Layer 2 (persistent, user-reviewed)
# Embedded defaults (Layer 1) are always loaded automatically
knowledge:
  dir: ./knowledge             # All .yaml files in this directory are loaded

# Processing behavior
retry:
  max_attempts: 3
  backoff_seconds: 2

rate_limit:
  requests_per_minute: 14     # Stay under Gemini free tier 15 RPM
```

---

## Data Schemas

### Read Page (Phase 1 output)

```json
{
  "version": "1.0",
  "page_number": 190,
  "section_type": "scholarly_entries",
  "read_model": "gemini-2.0-flash",
  "read_timestamp": "2026-03-17T14:30:00Z",
  "header": {
    "text": "حرف الألف مع السين",
    "type": "section_title"
  },
  "entries": [
    {
      "number": 1392,
      "type": "hadith",
      "arabic_text": "أَسَرَكَ مَلَكٌ مِنَ المَلائِكَةِ",
      "is_continuation": false,
      "continues_on_next_page": false
    },
    {
      "number": 1393,
      "type": "hadith",
      "arabic_text": "أَسْعَدُ العَجَمِ بِالإِسْلامِ أَهْلُ فارِسَ",
      "is_continuation": false,
      "continues_on_next_page": false
    }
  ],
  "footnotes": [
    {
      "entry_number": 1392,
      "arabic_text": "(بد ، سيرة ابن كثير) كان السائب بن أبي حبيش يحدث أنه كان مع قريش في بدر...",
      "source_codes": ["بد", "سيرة ابن كثير"],
      "additional_references": [
        { "entry_number": 1393, "text": "(كنز 34125)" },
        { "entry_number": 1394, "text": "(كنز 12047 ، طب)" }
      ]
    }
  ],
  "page_footer": "- 190 -",
  "raw_text": "Full page text as fallback...",
  "read_warnings": []
}
```

### Solved Page (Phase 2 output)

Extends the read page with:

```json
{
  "sources_resolved": [
    { "code": "بد", "name_ar": "البداية والنهاية", "name_tr": "el-Bidâye ve'n-Nihâye", "layer": "embedded" },
    { "code": "كنز", "name_ar": "كنز العمال", "name_tr": "Kenzü'l-Ummâl", "number": "34125", "layer": "workspace" },
    { "code": "هب", "name_ar": "شعب الإيمان", "name_tr": "Şuabü'l-Îmân", "layer": "staged" }
  ],
  "unresolved_sources": ["فر"],
  "continuation_info": null,
  "validation": {
    "status": "ok",
    "warnings": [],
    "hadith_number_sequence_valid": true
  },
  "translation_context": {
    "relevant_glossary_terms": ["فارس → Fârisîler (Persler)", "المَلائِكَة → melekler"],
    "previous_page_summary": "Section on letter Alif with Sin. Hadiths 1388-1391 covered."
  }
}
```

The `layer` field on resolved sources indicates where the mapping came from: `embedded` (shipped with binary), `workspace` (user-reviewed persistent), or `staged` (auto-detected, pending review). The `unresolved_sources` array lists any source codes that couldn't be matched against any knowledge layer — these appear in the phase report as actionable items.

### Translated Page (Phase 3 output)

Extends the solved page with:

```json
{
  "translation_model": "gemini-2.0-flash",
  "translation_timestamp": "2026-03-17T15:00:00Z",
  "translated_header": {
    "text": "Elif Harfi - Sin Babı"
  },
  "translated_entries": [
    {
      "number": 1392,
      "turkish_text": "Meleklerden bir melek seni esir aldı.",
      "translator_notes": ""
    }
  ],
  "translated_footnotes": [
    {
      "entry_number": 1392,
      "turkish_text": "(el-Bidâye ve'n-Nihâye, İbn Kesîr Sîresi) Sâib b. Ebî Hubeyş (radıyallâhu anh), Bedir'de Kureyş ile birlikte olduğunu anlatırdı...",
      "sources_expanded": ["el-Bidâye ve'n-Nihâye", "İbn Kesîr Sîresi"]
    }
  ],
  "translation_warnings": []
}
```

### Progress/Checkpoint (progress.json)

```json
{
  "book_id": "el-camius-sagir",
  "total_pages": 600,
  "phases": {
    "read": {
      "completed": [1, 2, 3, 4, 5],
      "failed": [],
      "pending": [6, 7, 8, "..."]
    },
    "solve": {
      "completed": [1, 2, 3, 4, 5],
      "last_run": "2026-03-17T14:35:00Z"
    },
    "translate": {
      "completed": [1, 2, 3],
      "failed": [],
      "pending": [4, 5, "..."]
    },
    "write": {
      "completed": [1, 2, 3],
      "pending": [4, 5, "..."]
    }
  }
}
```

---

## Knowledge Module Format

### honorifics.yaml

```yaml
# Arabic formula → Turkish rendering
# Key is the Arabic text (or common abbreviation), value is Turkish
entries:
  - arabic: "صلى الله عليه وسلم"
    abbreviations: ["ﷺ", "صلعم"]
    turkish: "sallallâhu aleyhi ve sellem"
    context: "Used after mentioning the Prophet"

  - arabic: "رضي الله عنه"
    abbreviations: ["رضه"]
    turkish: "radıyallâhu anh"
    context: "Used after male companion names"

  - arabic: "رضي الله عنها"
    turkish: "radıyallâhu anhâ"
    context: "Used after female companion names"

  - arabic: "رضي الله عنهم"
    turkish: "radıyallâhu anhüm"
    context: "Used after plural companion references"

  - arabic: "رحمه الله"
    turkish: "rahimehullâh"
    context: "Used after deceased scholars"

  - arabic: "عليه السلام"
    turkish: "aleyhisselâm"
    context: "Used after prophet names"
```

### sources.yaml

```yaml
# Source abbreviation codes used in hadith book footnotes
entries:
  - code: "خ"
    name_ar: "صحيح البخاري"
    name_tr: "Sahîh-i Buhârî"
    author_tr: "İmam Buhârî"

  - code: "م"
    name_ar: "صحيح مسلم"
    name_tr: "Sahîh-i Müslim"
    author_tr: "İmam Müslim"

  - code: "ت"
    name_ar: "جامع الترمذي"
    name_tr: "Sünen-i Tirmizî"
    author_tr: "İmam Tirmizî"

  - code: "ن"
    name_ar: "سنن النسائي"
    name_tr: "Sünen-i Nesâî"
    author_tr: "İmam Nesâî"

  - code: "د"
    name_ar: "سنن أبي داود"
    name_tr: "Sünen-i Ebû Dâvûd"
    author_tr: "İmam Ebû Dâvûd"

  - code: "حم"
    name_ar: "مسند أحمد"
    name_tr: "Müsned-i Ahmed"
    author_tr: "Ahmed b. Hanbel"

  - code: "هق"
    name_ar: "السنن الكبرى للبيهقي"
    name_tr: "es-Sünenü'l-Kübrâ"
    author_tr: "İmam Beyhakî"

  - code: "طب"
    name_ar: "المعجم الكبير"
    name_tr: "el-Mu'cemü'l-Kebîr"
    author_tr: "İmam Taberânî"

  - code: "كنز"
    name_ar: "كنز العمال"
    name_tr: "Kenzü'l-Ummâl"
    author_tr: "Müttakî el-Hindî"

  - code: "هب"
    name_ar: "شعب الإيمان"
    name_tr: "Şuabü'l-Îmân"
    author_tr: "İmam Beyhakî"

  - code: "حل"
    name_ar: "حلية الأولياء"
    name_tr: "Hilyetü'l-Evliyâ"
    author_tr: "Ebû Nuaym el-İsfahânî"

  # Add more as encountered in the book...
```

### people.yaml

```yaml
entries:
  - arabic: "أبو هريرة"
    turkish: "Ebû Hüreyre"
    full_name_tr: "Ebû Hüreyre Abdurrahman b. Sahr ed-Devsî"

  - arabic: "عبد الله بن عمر"
    turkish: "Abdullah b. Ömer"

  - arabic: "عبد الله بن مسعود"
    turkish: "Abdullah b. Mes'ûd"

  - arabic: "عائشة"
    turkish: "Hz. Âişe"

  - arabic: "أنس بن مالك"
    turkish: "Enes b. Mâlik"

  - arabic: "عبد الرحمن بن عوف"
    turkish: "Abdurrahman b. Avf"

  - arabic: "السائب بن أبي حبيش"
    turkish: "Sâib b. Ebî Hubeyş"

  # Expand as needed...
```

### terminology.yaml

```yaml
entries:
  - arabic: "حديث"
    turkish: "hadîs-i şerîf"
    context: "When referring to a prophetic tradition"

  - arabic: "سند"
    turkish: "sened"
    context: "Chain of narrators"

  - arabic: "متن"
    turkish: "metin"
    context: "Body text of hadith"

  - arabic: "صحيح"
    turkish: "sahîh"
    context: "Hadith grading - authentic"

  - arabic: "حسن"
    turkish: "hasen"
    context: "Hadith grading - good"

  - arabic: "ضعيف"
    turkish: "zayıf"
    context: "Hadith grading - weak"

  - arabic: "مرسل"
    turkish: "mürsel"
    context: "Hadith with missing link in chain"

  - arabic: "فقه"
    turkish: "fıkıh"

  - arabic: "تفسير"
    turkish: "tefsîr"

  - arabic: "تزكية"
    turkish: "tezkiye"

  # Expand as needed...
```

---

## Go Project Source Structure

This is the source tree for the `mutercim` binary itself — NOT a workspace.

```
mutercim/                            # Go project root (github.com/mmdemirbas/mutercim)
├── cmd/mutercim/
│   └── main.go                      # Entry point, cobra root command
├── internal/
│   ├── cli/
│   │   ├── root.go                  # Root command + common flag registration
│   │   ├── init.go                  # 'init' — workspace scaffolding (interactive + non-interactive)
│   │   ├── read.go                  # 'read' subcommand
│   │   ├── solve.go                 # 'solve' subcommand
│   │   ├── translate.go             # 'translate' subcommand
│   │   ├── write.go                 # 'write' subcommand
│   │   ├── make.go                  # 'make' — full pipeline
│   │   ├── status.go                # 'status' — progress + flagged issues
│   │   ├── validate.go              # 'validate' — dry validation checks
│   │   ├── config_cmd.go            # 'config' — show effective config
│   │   └── knowledge_cmd.go         # 'knowledge' subcommand group (list, staged, diff, promote)
│   ├── workspace/
│   │   ├── workspace.go             # Workspace discovery, path resolution, directory creation
│   │   ├── init.go                  # Workspace scaffolding logic (used by cli/init.go)
│   │   └── staging.go               # Staging area operations (write, list, promote, diff)
│   ├── config/
│   │   ├── config.go                # Config loading: YAML + flags + defaults
│   │   └── sections.go              # Section type definitions, page range parsing, page→section lookup
│   ├── pipeline/
│   │   ├── read.go                  # Phase 1 orchestrator (section-aware prompt selection)
│   │   ├── solve.go                 # Phase 2 orchestrator (loads all 3 knowledge layers)
│   │   ├── translate.go             # Phase 3 orchestrator (section-aware translation)
│   │   └── write.go                 # Phase 4 orchestrator (section-aware template selection)
│   ├── apiclient/
│   │   ├── client.go                # Shared HTTP client: retry, backoff, Retry-After header
│   │   ├── ratelimit.go             # Token bucket rate limiter (per-provider)
│   │   └── response.go              # Extract JSON from AI responses (fences, brace-matching)
│   ├── provider/
│   │   ├── provider.go              # Provider interface definition
│   │   ├── registry.go              # Provider factory: name+model → Provider instance
│   │   ├── gemini.go                # Google Gemini (~70 lines)
│   │   ├── claude.go                # Anthropic Claude (~70 lines)
│   │   ├── openai.go                # OpenAI (~70 lines)
│   │   ├── ollama.go                # Ollama local (~60 lines)
│   │   └── surya.go                 # Surya OCR + text model for structural parsing
│   ├── reader/
│   │   ├── reader.go                # Read logic, section-aware prompt selection
│   │   └── prompts.go               # Prompt templates per section type
│   ├── solver/
│   │   ├── solver.go                # Solve orchestration
│   │   ├── abbreviation.go          # Source code resolution against all knowledge layers
│   │   ├── continuation.go          # Cross-page merging detection
│   │   ├── staging.go               # Auto-stage reference_table reads
│   │   └── validator.go             # Structural validation, numbering checks
│   ├── translation/
│   │   ├── translator.go            # Translation logic, section-aware prompt selection
│   │   └── prompts.go               # Translation prompt templates per section type
│   ├── knowledge/
│   │   ├── loader.go                # Layered loading: embedded → workspace → staged
│   │   ├── embedded.go              # go:embed for internal defaults/ directory
│   │   ├── types.go                 # Knowledge data structures (Honorific, Source, Person, etc.)
│   │   └── glossary.go              # Combined glossary builder for prompt injection
│   ├── renderer/
│   │   ├── renderer.go              # Renderer interface
│   │   ├── markdown.go              # Markdown output (section-aware formatting)
│   │   ├── latex.go                 # LaTeX output (section-aware template selection)
│   │   └── docx.go                  # DOCX output (via pandoc from Markdown)
│   ├── input/
│   │   ├── loader.go                # Input file handling (PDF, images, directories)
│   │   └── pdf.go                   # PDF → image conversion (shells out to pdftoppm)
│   ├── progress/
│   │   └── tracker.go               # Checkpoint/resume: per-page, per-phase state
│   └── model/
│       ├── page.go                  # Page data structures (read, solved, translated)
│       ├── book.go                  # Book-level metadata
│       ├── entry.go                 # Entry, footnote, source data structures
│       └── section.go               # Section type enum, page range model
├── example/                         # Example workspace and reference files
│   ├── defaults/                    # Reference copies of embedded defaults (not used by binary)
│   │   ├── knowledge/               # honorifics, people, terminology, places YAML
│   │   └── templates/               # LaTeX book/page templates
│   └── mutercim.yaml               # Example workspace config
├── docker/
│   └── xelatex/
│       └── Dockerfile               # XeLaTeX compilation container
├── go.mod
├── go.sum
├── Makefile                         # build, test, install, docker-xelatex targets
├── mutercim.yaml.example            # Example workspace config (copied by 'mutercim init')
└── README.md
```

Key structural decisions:
- `internal/knowledge/defaults/` is embedded into the binary via `go:embed` — no external files needed at runtime
- `internal/workspace/` owns all workspace path resolution and staging operations
- `internal/config/sections.go` provides a `SectionForPage(pageNum int) Section` lookup used by all phases
- `provider/registry.go` maps config strings ("gemini", "claude") to concrete Provider instances
- `solver/staging.go` writes auto-read knowledge to the workspace staging area

---

## API Client Package (internal/apiclient)

The `apiclient` package handles all shared HTTP concerns so that each provider implementation is reduced to just endpoint URL + request body construction.

```go
// client.go

// Client is a shared HTTP client with retry, rate limiting, and response parsing.
type Client struct {
    httpClient  *http.Client
    rateLimiter *RateLimiter
    maxRetries  int
    baseBackoff time.Duration
    logger      *slog.Logger
}

// ClientConfig configures a new API client.
type ClientConfig struct {
    Timeout           time.Duration // HTTP request timeout (default: 120s for vision calls)
    MaxRetries        int           // Max retry attempts (default: 3)
    BaseBackoff       time.Duration // Initial backoff duration (default: 2s)
    RequestsPerMinute int           // Rate limit (default: 14 for Gemini free)
}

// NewClient creates a Client with the given configuration.
func NewClient(cfg ClientConfig, logger *slog.Logger) *Client { ... }

// Request represents an AI API request.
type Request struct {
    Method  string            // HTTP method (always POST for AI APIs)
    URL     string            // Full endpoint URL
    Headers map[string]string // Auth headers, content-type, API version headers
    Body    any               // Will be JSON-marshaled
}

// Do executes the request with rate limiting and retry logic.
// Returns the raw response body bytes.
// Retries on: 429 (rate limit), 500, 502, 503, 529 (overloaded).
// Does NOT retry on: 400 (bad request), 401 (auth), 403 (forbidden), 404.
func (c *Client) Do(ctx context.Context, req Request) ([]byte, error) { ... }

// DoJSON executes the request and unmarshals the response into the given type.
func DoJSON[T any](c *Client, ctx context.Context, req Request) (T, error) { ... }

// EncodeImageBase64 reads an image file and returns its base64 encoding and MIME type.
func EncodeImageBase64(imagePath string) (data string, mimeType string, err error) { ... }
```

```go
// ratelimit.go

// RateLimiter implements a token bucket rate limiter.
type RateLimiter struct {
    tokens     chan struct{}
    refillStop chan struct{}
}

// NewRateLimiter creates a rate limiter that allows n requests per minute.
func NewRateLimiter(requestsPerMinute int) *RateLimiter { ... }

// Wait blocks until a token is available or ctx is cancelled.
func (r *RateLimiter) Wait(ctx context.Context) error { ... }

// Close stops the refill goroutine.
func (r *RateLimiter) Close() { ... }
```

```go
// response.go

// SanitizeResponse strips invisible Unicode characters that LLMs occasionally emit
// and that break JSON parsing. Specifically: zero-width spaces (U+200B),
// byte-order marks (U+FEFF), and other zero-width characters.
// Note: U+200C (ZWNJ) and U+200D (ZWJ) are preserved outside of JSON structural
// positions because they have legitimate uses in Arabic typography.
func SanitizeResponse(response string) string { ... }

// ExtractJSON sanitizes the response, then attempts to extract a JSON object.
// Tries in order: direct parse → markdown code block extraction → first { to last }.
// Returns the raw JSON string (not unmarshaled) so the caller can unmarshal into their specific type.
func ExtractJSON(response string) (string, error) { ... }

// ExtractTextContent extracts the text content from provider-specific response structures.
// Each provider has a different response envelope (e.g., Gemini: candidates[0].content.parts[0].text,
// Claude: content[0].text, OpenAI: choices[0].message.content).
// This is NOT in apiclient — each provider implements its own response extraction.
```

### How Providers Use apiclient

Each provider becomes a thin adapter — just request construction and response envelope parsing:

```go
// gemini.go — ~70 lines total

type GeminiProvider struct {
    client *apiclient.Client
    apiKey string
    model  string
}

func (g *GeminiProvider) ReadFromImage(ctx context.Context, image []byte, systemPrompt, userPrompt string) (string, error) {
    body := geminiRequest{
        Contents: []geminiContent{{
            Parts: []geminiPart{
                {InlineData: &geminiInlineData{MimeType: "image/png", Data: base64.StdEncoding.EncodeToString(image)}},
                {Text: userPrompt},
            },
        }},
        SystemInstruction: &geminiContent{Parts: []geminiPart{{Text: systemPrompt}}},
        GenerationConfig:  geminiGenConfig{Temperature: 0, ResponseMIMEType: "application/json"},
    }

    resp, err := apiclient.DoJSON[geminiResponse](g.client, ctx, apiclient.Request{
        Method:  "POST",
        URL:     fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", g.model, g.apiKey),
        Headers: map[string]string{"Content-Type": "application/json"},
        Body:    body,
    })
    if err != nil {
        return "", fmt.Errorf("gemini read: %w", err)
    }
    return resp.Candidates[0].Content.Parts[0].Text, nil
}

// Translate is nearly identical — same endpoint, text-only content instead of image.
```

```go
// claude.go — ~70 lines total

func (c *ClaudeProvider) ReadFromImage(ctx context.Context, image []byte, systemPrompt, userPrompt string) (string, error) {
    body := claudeRequest{
        Model:     c.model,
        MaxTokens: 4096,
        System:    systemPrompt,
        Messages: []claudeMessage{{
            Role: "user",
            Content: []claudeContent{
                {Type: "image", Source: &claudeImageSource{Type: "base64", MediaType: "image/png", Data: base64.StdEncoding.EncodeToString(image)}},
                {Type: "text", Text: userPrompt},
            },
        }},
    }

    resp, err := apiclient.DoJSON[claudeResponse](c.client, ctx, apiclient.Request{
        Method: "POST",
        URL:    "https://api.anthropic.com/v1/messages",
        Headers: map[string]string{
            "Content-Type":      "application/json",
            "x-api-key":         c.apiKey,
            "anthropic-version": "2023-06-01",
        },
        Body: body,
    })
    if err != nil {
        return "", fmt.Errorf("claude read: %w", err)
    }
    return resp.Content[0].Text, nil
}
```

The pattern is clear: each provider is ~70 lines of struct definitions for request/response envelopes + method implementations that construct the body and parse the text. All retry, rate limiting, HTTP plumbing, and error classification lives in `apiclient` once.

**Total HTTP infrastructure**: ~200 lines in `apiclient` + ~70 lines per provider × 4-5 providers = **~500-550 lines**. One-time cost.

---

## Provider Interface

```go
// provider.go

// Provider abstracts AI model interaction for both vision (reading) and text (translation).
type Provider interface {
    // Name returns the provider identifier (e.g., "gemini", "claude", "ollama")
    Name() string

    // ReadFromImage sends an image to a vision model with a system prompt
    // and returns the model's text response (expected to be JSON).
    ReadFromImage(ctx context.Context, image []byte, systemPrompt string, userPrompt string) (string, error)

    // Translate sends text to a language model with a system prompt
    // and returns the model's text response (expected to be JSON).
    Translate(ctx context.Context, systemPrompt string, userPrompt string) (string, error)

    // SupportsVision returns true if this provider can handle image inputs.
    SupportsVision() bool
}
```

Each provider is constructed with an `*apiclient.Client` (configured with the appropriate rate limit for that provider) and provider-specific auth credentials. The Surya provider shells out to the `surya_ocr` CLI and then sends raw OCR text to a text-only model for structural parsing.

### Recommended Models (documented in README and config example)

**Reading (Vision):**
| Priority | Provider | Model | Cost | Quality | Notes |
|----------|----------|-------|------|---------|-------|
| 1 (default) | Gemini | gemini-2.0-flash | Free tier | Good | 15 RPM, 1500 req/day free |
| 2 | Claude | claude-sonnet-4-20250514 | ~$0.03/page | Excellent | Best structural understanding |
| 3 | Ollama | qwen2.5-vl:7b | Free (local) | Decent | Requires ~8GB RAM |
| 4 | Surya | surya_ocr + text model | Free (local) | Good OCR, needs separate parsing | Fastest local option |

**Translation (Text):**
| Priority | Provider | Model | Cost | Quality | Notes |
|----------|----------|-------|------|---------|-------|
| 1 (default) | Gemini | gemini-2.0-flash | Free tier | Good | Same free tier limits |
| 2 | Claude | claude-sonnet-4-20250514 | ~$0.03/page | Excellent | Best for nuanced translation |
| 3 | Claude | claude-opus-4-20250514 | ~$0.13/page | Best | For critical/sensitive passages |
| 4 | Ollama | qwen3:14b | Free (local) | Decent | Arabic→Turkish quality varies |

---

## Key Implementation Details

### System Dependency Validation

Pipeline commands (`read`, `translate`, `write`, `make`) must validate required system dependencies **at startup, before any API calls or file processing**. This prevents discovering a missing tool 200 pages into a run.

```go
// workspace/preflight.go

// Preflight checks system dependencies based on what the current command needs.
// Called at the start of each pipeline command.
func Preflight(cfg *config.Config, command string) error {
    // Always needed for PDF input
    if cfg.InputIsPDF() {
        if _, err := exec.LookPath("pdftoppm"); err != nil {
            return fmt.Errorf("pdftoppm not found in PATH (install: brew install poppler / apt install poppler-utils)")
        }
    }
    // Needed for LaTeX compilation
    if command == "write" || command == "make" {
        if slices.Contains(cfg.Write.Formats, "latex") && !cfg.Write.SkipPDF {
            if _, err := exec.LookPath("docker"); err != nil {
                return fmt.Errorf("docker not found in PATH (required for LaTeX→PDF compilation, or use --skip-pdf)")
            }
        }
    }
    // Needed for DOCX output via pandoc
    if slices.Contains(cfg.Write.Formats, "docx") {
        if _, err := exec.LookPath("pandoc"); err != nil {
            return fmt.Errorf("pandoc not found in PATH (required for DOCX output)")
        }
    }
    return nil
}
```

### Atomic Progress Tracking

The `progress.json` file is a critical checkpoint. A `Ctrl+C` or crash during write must not corrupt it. All progress writes use the atomic write-to-temp-then-rename pattern:

```go
// progress/tracker.go

// Save atomically writes progress state to disk.
// Writes to progress.json.tmp first, then renames to progress.json.
// os.Rename is atomic on POSIX systems (same filesystem).
func (t *Tracker) Save() error {
    data, err := json.MarshalIndent(t.state, "", "  ")
    if err != nil {
        return fmt.Errorf("marshal progress: %w", err)
    }
    tmpPath := t.path + ".tmp"
    if err := os.WriteFile(tmpPath, data, 0644); err != nil {
        return fmt.Errorf("write progress tmp: %w", err)
    }
    if err := os.Rename(tmpPath, t.path); err != nil {
        return fmt.Errorf("rename progress: %w", err)
    }
    return nil
}
```

This same atomic write pattern should be used for any JSON file that serves as state (per-page midstate files, staged knowledge files, report files).

### PDF to Image Conversion

Use `pdftoppm` (from poppler-utils) for PDF → PNG conversion. It's available on all platforms and produces high-quality output.

```go
// pdf.go
func ConvertPDFToImages(pdfPath string, outputDir string, dpi int, pages PageRange) ([]string, error) {
    // pdftoppm -png -r 300 -f 1 -l 10 input.pdf output/page
    args := []string{"-png", "-r", strconv.Itoa(dpi)}
    if pages.First > 0 {
        args = append(args, "-f", strconv.Itoa(pages.First))
    }
    if pages.Last > 0 {
        args = append(args, "-l", strconv.Itoa(pages.Last))
    }
    args = append(args, pdfPath, filepath.Join(outputDir, "page"))
    // exec.Command("pdftoppm", args...)
}
```

### Rate Limiting

Handled by `apiclient.RateLimiter` — token bucket implementation. Each provider gets its own rate limiter instance configured from the config file. Gemini free tier defaults to 14 RPM (headroom under 15 RPM limit). Claude defaults to 50 RPM. Ollama has no rate limit.

### Retry Logic

Handled by `apiclient.Client.Do()` — exponential backoff with jitter. Retries on transient HTTP errors (429, 500, 502, 503, 529). Non-retryable errors (400, 401, 403) fail immediately. The `Do()` method also respects the `Retry-After` header when present (Gemini and Claude both send this on 429).

### JSON Response Parsing

Handled by `apiclient.ExtractJSON()`. AI models sometimes wrap JSON in markdown code blocks or prepend explanatory text. The function tries three strategies in order: direct parse, markdown fence extraction, brace-matching. Once raw JSON is extracted, the caller unmarshals into the strict Go struct — any type mismatch is a validation failure that triggers a retry.

### LaTeX Docker Container

```dockerfile
# docker/xelatex/Dockerfile
FROM texlive/texlive:latest-minimal

RUN tlmgr install \
    polyglossia \
    bidi \
    xetex \
    fontspec \
    arabxetex \
    fancyhdr \
    geometry \
    hyperref \
    bookmark \
    enumitem \
    titlesec \
    amiri      # Amiri font for Arabic

# Install additional Arabic/Turkish fonts
RUN apt-get update && apt-get install -y \
    fonts-amiri \
    fonts-hosny-amiri \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /data
ENTRYPOINT ["xelatex", "-interaction=nonstopmode"]
```

Compilation command:
```bash
docker run --rm -v $(pwd)/output/tr/latex:/data mutercim/xelatex book.tex
```

### Read System Prompt (Core)

```
You are an expert OCR system specialized in classical Arabic Islamic scholarly texts.

Analyze the provided page image and extract ALL text with full structural metadata.

CRITICAL RULES:
1. Preserve ALL diacritical marks (tashkeel/harakat) exactly as they appear: fatḥa, kasra, ḍamma, sukūn, shadda, tanwīn, etc.
2. Do NOT normalize or "clean" the Arabic text. Reproduce it exactly.
3. Identify the structural type of each element on the page.
4. Detect numbered entries and extract their numbers accurately.
5. Recognize footnote/commentary sections (usually separated by a line of asterisks or a horizontal rule from the main text).
6. Extract source abbreviation codes from footnotes (usually single or double Arabic letters in parentheses).
7. If the first entry on the page appears to start mid-sentence without a number, mark it as a continuation.

Return a JSON object with this exact schema:
{
  "page_number": <int or null if not visible>,
  "header": { "text": "<header text>", "type": "section_title|chapter_title|none" } | null,
  "entries": [
    {
      "number": <int or null for continuations>,
      "type": "hadith|athar|commentary|chapter_heading|other",
      "arabic_text": "<full Arabic text with tashkeel>",
      "is_continuation": <bool>,
      "continues_on_next_page": <bool>
    }
  ],
  "footnotes": [
    {
      "entry_numbers": [<int>],
      "arabic_text": "<footnote text>",
      "source_codes": ["<code1>", "<code2>"]
    }
  ],
  "page_footer": "<page number text if present>",
  "warnings": ["<any issues encountered during reading>"]
}

Respond with ONLY the JSON object. No markdown formatting, no explanations.
```

### Translation System Prompt (Core)

```
You are an expert translator of classical Arabic Islamic scholarly texts into Turkish.

TRANSLATION PRINCIPLES:
1. Translate for MEANING, not word-by-word. The Turkish reader should understand the intended message naturally.
2. Use established Turkish Islamic scholarly terminology (see glossary below).
3. Translate Arabic idioms into their Turkish equivalents or explain them naturally — never produce a literal translation that would be cryptic.
4. Preserve the scholarly register and dignity of the text.

HONORIFIC RULES:
{honorifics_section}

PERSON NAME MAPPINGS:
{people_section}

SOURCE ABBREVIATIONS:
{sources_section}

TERMINOLOGY GLOSSARY:
{terminology_section}

CONTEXT FROM PREVIOUS PAGES:
{context_section}

INSTRUCTIONS:
For each entry in the input JSON, produce a Turkish translation.
For footnotes, translate the explanatory text and expand source abbreviations to their full Turkish names.

{expand_sources_instruction}

Input (solved page JSON):
{input_json}

Return a JSON object with this exact schema:
{
  "translated_header": { "text": "<Turkish header>" } | null,
  "translated_entries": [
    {
      "number": <int>,
      "turkish_text": "<Turkish translation>",
      "translator_notes": "<any notes about difficult passages>"
    }
  ],
  "translated_footnotes": [
    {
      "entry_numbers": [<int>],
      "turkish_text": "<Turkish footnote translation>",
      "sources_expanded": ["<full Turkish source name>"]
    }
  ],
  "warnings": ["<any translation difficulties>"]
}

Respond with ONLY the JSON object.
```

---

## Dependencies (go.mod)

```
github.com/spf13/cobra     # CLI framework
github.com/spf13/viper     # Config management (YAML + env + flags)
gopkg.in/yaml.v3           # YAML parsing for knowledge files
```

Minimal dependencies. HTTP clients use Go stdlib `net/http`. JSON parsing uses `encoding/json`. No external logging framework — use `log/slog` (stdlib since Go 1.21).

---

## Build & Run

```makefile
# Makefile
.PHONY: build install test docker-xelatex

build:
	go build -o bin/mutercim ./cmd/mutercim

install:
	go install ./cmd/mutercim

test:
	go test ./...

docker-xelatex:
	docker build -t mutercim/xelatex docker/xelatex/
```

Workspace initialization is handled by `mutercim init`, not the Makefile. Quick start:

```bash
go install github.com/mmdemirbas/mutercim/cmd/mutercim@latest
mkdir my-book && cd my-book
mutercim init
# Copy your PDF into input/
mutercim make
```

---

## Error Handling & Reporting

Every phase produces a `report.json` alongside its outputs:

```json
{
  "phase": "read",
  "timestamp": "2026-03-17T14:30:00Z",
  "total_pages": 600,
  "successful": 595,
  "failed": 2,
  "warnings": 8,
  "details": [
    { "page": 42, "status": "warning", "message": "Hadith number gap: expected 312, found 314" },
    { "page": 187, "status": "failed", "message": "Could not parse JSON after 3 retries", "raw_saved": true },
    { "page": 301, "status": "warning", "message": "Possible continuation detected but confidence low" }
  ]
}
```

Failed pages save whatever raw text was read to `midstate/read/page_NNN.raw.txt` so nothing is lost.

---

## Future Enhancements (Not in v1)

- Parallel chunk processing for Phase 3 (translation) with overlap at chunk boundaries
- Web UI for reviewing and correcting read/translation output
- Support for right-to-left output in DOCX (proper RTL paragraph direction)
- Multiple target languages (Urdu, Malay, English, etc.)
- Fine-tuned local model for Arabic Islamic text OCR
- Integration with existing hadith databases for cross-referencing
- Automatic section type detection from page content (suggest sections config based on initial scan)
- Knowledge sharing across workspaces (a shared knowledge repository that multiple book workspaces can reference)
- Side-by-side Arabic+Turkish output in LaTeX (dual-column or facing-page layout)
