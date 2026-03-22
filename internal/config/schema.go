package config

import (
	"encoding/json"
	"reflect"
	"strings"
)

// schemaMeta provides JSON Schema metadata for a config field.
// Fields are keyed by dot-separated path (e.g. "inputs[].path", "sections[].type").
type schemaMeta struct {
	Description string
	Default     any
	Enum        []string
	Minimum     *int
	Deprecated  bool
	ItemEnum    []string // enum constraint for array items
	Required    []string // required properties for object types
}

func intPtr(v int) *int { return &v }

// schemaAnnotations maps field paths to their schema metadata.
// This is the single source of truth for descriptions, defaults, enums, and constraints.
// Structure (field names, types, nesting) comes from reflection on the Go types.
var schemaAnnotations = map[string]schemaMeta{
	// output
	"output":    {Description: "Base directory for all generated output (cut/, read/, solve/, translate/, write/, log/, memory/). Relative to workspace root. Use this to keep generated files separate from the workspace.", Default: "."},
	"log_level": {Description: "Log verbosity level. Can be overridden with --log-level (-l) CLI flag.", Default: "info", Enum: []string{"debug", "info", "warn", "error"}},

	// inputs
	"inputs":             {Description: "Input files or directories. PDFs are converted to page images via pdftoppm; image directories are used as-is. Multiple inputs are processed independently through read/solve/translate, then merged in write.", Default: []map[string]string{{"path": "./input"}}},
	"inputs[]":           {Required: []string{"path", "languages"}},
	"inputs[].path":      {Description: "Path to input PDF or image directory (relative to workspace root). The filename stem becomes the subdirectory name under cut/, read/, solve/."},
	"inputs[].pages":     {Description: `Optional page range to process from this input (e.g. "1-50", "1,5,10-20"). If omitted, all pages are processed. Can also be set globally via the --pages CLI flag.`},
	"inputs[].languages": {Description: "Source language codes for this input (required). First is the primary language used for source markdown output and AI prompt configuration."},

	// cut
	"cut":     {Description: "Page-generation settings (PDF to images)."},
	"cut.dpi": {Description: "DPI for PDF-to-image conversion via pdftoppm. Higher values improve OCR accuracy but increase file size and processing time. 300 is a good balance.", Default: 300, Minimum: intPtr(72)},

	// layout
	"layout":        {Description: "Layout detection phase settings. Detects document regions (headers, columns, tables, footnotes) with bounding boxes before the read phase."},
	"layout.tool":   {Description: "Layout detection tool. 'doclayout-yolo' uses DocLayout-YOLO (default, best for document structure). 'surya' uses Surya OCR. Both require Docker. Empty string disables layout detection (read phase uses AI-only mode).", Default: "doclayout-yolo", Enum: []string{"", "doclayout-yolo", "surya"}},
	"layout.debug":  {Description: "When true, write annotated PNG images showing detected bounding boxes to layout/<input>/debug/. Useful for verifying and tuning layout detection.", Default: false},
	"layout.params": {Description: "Tool-specific tuning parameters passed to the layout tool as key-value pairs. Each tool interprets its own params; unknown keys are logged at WARN and ignored.\n\nFor doclayout-yolo:\n  confidence (float, default 0.2) — minimum confidence score to keep a detection (0.0–1.0). Lower values produce more detections, possibly noisy. Higher values keep only high-certainty regions.\n  iou (float, default 0.7) — IoU threshold for non-maximum suppression (0.0–1.0). Controls overlap merging: lower values merge more aggressively (fewer overlapping boxes), higher values allow more overlap.\n  image_size (int, default 1024) — input image size in pixels for model inference. The model resizes the page to this size internally. Larger values preserve more detail but run slower.\n  max_det (int, default 300) — maximum number of detections per page image.\n\nFor surya:\n  languages (string) — comma-separated OCR language codes for text recognition (e.g. \"ar\", \"ar,fa\", \"en,tr\"). Automatically set from the input's source languages if not specified."},

	// read
	"read":                                {Description: "Read phase settings. The read phase sends page images to an AI vision model to extract structured JSON (entries, footnotes, metadata). If layout data exists, it uses pre-detected regions; otherwise it uses AI-only mode."},
	"read.models":                         {Description: "Ordered failover chain of AI models for OCR. The first model is primary; if it returns 429/quota errors, the next model is tried. All models must support vision for the read phase."},
	"read.models[]":                       {},
	"read.models[].provider":              {Description: "AI provider name. Determines the API endpoint and authentication method.", Enum: []string{"gemini", "claude", "openai", "groq", "mistral", "openrouter", "xai", "ollama"}},
	"read.models[].model":                 {Description: "Model identifier as expected by the provider's API (e.g. 'gemini-2.5-flash-lite', 'claude-sonnet-4-20250514')."},
	"read.models[].rpm":                   {Description: "Requests per minute limit for this model. Overrides the provider default. Set to 0 to use the provider's default RPM."},
	"read.models[].vision":                {Description: "Whether this model supports image input. Required for the read phase. Set explicitly to override auto-detection (e.g. some Groq models support vision but aren't detected automatically)."},
	"read.models[].base_url":              {Description: "Custom API base URL. Only needed for self-hosted or non-standard OpenAI-compatible endpoints. Standard providers use built-in URLs."},
	"read.concurrency":                    {Description: "Number of parallel page-reading workers. Currently only 1 is supported.", Default: 1, Minimum: intPtr(1)},
	"read.retry":                          {Description: "Retry settings for failed read-phase API calls."},
	"read.retry.max_attempts":             {Description: "Maximum number of retry attempts per API call before marking the page as failed.", Default: 3, Minimum: intPtr(0)},
	"read.retry.backoff_seconds":          {Description: "Base backoff duration in seconds. Uses exponential backoff: 2s, 4s, 8s, etc.", Default: 2, Minimum: intPtr(1)},
	"read.rate_limit":                     {Description: "Rate limiting for read-phase API calls."},
	"read.rate_limit.requests_per_minute": {Description: "Maximum requests per minute for read-phase API calls.", Default: 0, Minimum: intPtr(0)},

	// solve
	"solve":                                {Description: "Solve phase settings. Resolves abbreviations, injects glossary context, validates structure."},
	"solve.retry":                          {Description: "Retry settings for solve-phase operations."},
	"solve.retry.max_attempts":             {Description: "Maximum number of retry attempts.", Default: 3, Minimum: intPtr(0)},
	"solve.retry.backoff_seconds":          {Description: "Base backoff duration in seconds.", Default: 2, Minimum: intPtr(1)},
	"solve.rate_limit":                     {Description: "Rate limiting for solve-phase operations."},
	"solve.rate_limit.requests_per_minute": {Description: "Maximum requests per minute.", Default: 0, Minimum: intPtr(0)},

	// translate
	"translate":                                {Description: "Translation phase settings. The translate phase sends structured page data to an AI model with knowledge-enriched prompts to produce translated text."},
	"translate.languages":                      {Description: "Target language codes. Each language gets its own translate/ and write/ subdirectory. Translation runs once per target language.", Default: []string{"tr"}},
	"translate.models":                         {Description: "Ordered failover chain of AI models for translation. Vision support is not required for translation (text-only). Non-vision models like Groq llama are valid here."},
	"translate.models[]":                       {},
	"translate.models[].provider":              {Description: "AI provider name.", Enum: []string{"gemini", "claude", "openai", "groq", "mistral", "openrouter", "xai", "ollama"}},
	"translate.models[].model":                 {Description: "Model identifier as expected by the provider's API."},
	"translate.models[].rpm":                   {Description: "Requests per minute limit for this model. Overrides the provider default."},
	"translate.models[].vision":                {Description: "Whether this model supports vision. Not required for translation."},
	"translate.models[].base_url":              {Description: "Custom API base URL for OpenAI-compatible endpoints."},
	"translate.context_window":                 {Description: "Number of previous pages included in the translation prompt for continuity. Higher values give better cross-page consistency but use more tokens.", Default: 2, Minimum: intPtr(0)},
	"translate.retry":                          {Description: "Retry settings for failed translate-phase API calls."},
	"translate.retry.max_attempts":             {Description: "Maximum number of retry attempts per API call before marking the page as failed.", Default: 3, Minimum: intPtr(0)},
	"translate.retry.backoff_seconds":          {Description: "Base backoff duration in seconds. Uses exponential backoff: 2s, 4s, 8s, etc.", Default: 2, Minimum: intPtr(1)},
	"translate.rate_limit":                     {Description: "Rate limiting for translate-phase API calls."},
	"translate.rate_limit.requests_per_minute": {Description: "Maximum requests per minute for translate-phase API calls.", Default: 0, Minimum: intPtr(0)},

	// write
	"write":                {Description: "Write phase settings. The write phase renders translated data into final output files (markdown, LaTeX, PDF, DOCX) under write/<lang>/."},
	"write.formats":        {Description: "Output formats to generate. 'md' produces markdown, 'latex' produces .tex only, 'pdf' produces .tex and compiles to PDF via Docker, 'docx' converts markdown to Word via pandoc.", Default: []string{"md", "latex", "docx", "pdf"}, ItemEnum: []string{"md", "latex", "pdf", "docx"}},
	"write.expand_sources": {Description: "When true, source abbreviations in footnotes are expanded to full names in the rendered output (e.g. 'خ' becomes 'Sahîh-i Buhârî').", Default: true},

	// knowledge
	"knowledge": {Description: "List of knowledge YAML files and/or directories. Directories include all .yaml/.yml files. Merged with auto-extracted memory/ entries. Relative to workspace root.", Default: []string{"./knowledge"}},
}

// GenerateSchema produces a JSON Schema for the mutercim configuration file.
// Structure is derived from reflection on the Config struct; metadata comes from schemaAnnotations.
func GenerateSchema() ([]byte, error) {
	root := buildSchemaObject(reflect.TypeOf(Config{}), "")
	root["$schema"] = "https://json-schema.org/draft/2020-12/schema"
	root["$id"] = "https://github.com/mmdemirbas/mutercim/config/mutercim.schema.json"
	root["title"] = "mutercim configuration"
	root["description"] = "Configuration file for mutercim \u2014 Islamic scholarly book translation tool."

	data, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func buildSchemaObject(t reflect.Type, path string) map[string]any {
	props := make(map[string]any)
	for i := range t.NumField() {
		field := t.Field(i)
		name := yamlFieldName(field)
		if name == "" {
			continue
		}
		fieldPath := joinPath(path, name)
		props[name] = buildSchemaType(field.Type, fieldPath)
	}

	s := map[string]any{
		"type":                 "object",
		"properties":           props,
		"additionalProperties": false,
	}
	applyAnnotations(s, path)
	return s
}

func buildSchemaType(t reflect.Type, path string) map[string]any {
	switch t.Kind() {
	case reflect.Struct:
		return buildSchemaObject(t, path)
	case reflect.String:
		s := map[string]any{"type": "string"}
		applyAnnotations(s, path)
		return s
	case reflect.Int, reflect.Int64:
		s := map[string]any{"type": "integer"}
		applyAnnotations(s, path)
		return s
	case reflect.Bool:
		s := map[string]any{"type": "boolean"}
		applyAnnotations(s, path)
		return s
	case reflect.Slice:
		return buildSchemaSlice(t, path)
	default:
		return map[string]any{}
	}
}

func buildSchemaSlice(t reflect.Type, path string) map[string]any {
	elem := t.Elem()

	items := buildSchemaType(elem, path+"[]")

	// Apply item-level enum from parent annotation
	if meta, ok := schemaAnnotations[path]; ok && len(meta.ItemEnum) > 0 {
		items["enum"] = meta.ItemEnum
	}

	s := map[string]any{
		"type":  "array",
		"items": items,
	}
	applyAnnotations(s, path)
	return s
}

func applyAnnotations(s map[string]any, path string) {
	meta, ok := schemaAnnotations[path]
	if !ok {
		return
	}
	if meta.Description != "" {
		s["description"] = meta.Description
	}
	if meta.Default != nil {
		s["default"] = meta.Default
	}
	if len(meta.Enum) > 0 {
		s["enum"] = meta.Enum
	}
	if meta.Minimum != nil {
		s["minimum"] = *meta.Minimum
	}
	if meta.Deprecated {
		s["deprecated"] = true
	}
	if len(meta.Required) > 0 {
		s["required"] = meta.Required
	}
}

func yamlFieldName(f reflect.StructField) string {
	tag := f.Tag.Get("yaml")
	if tag == "" || tag == "-" {
		return ""
	}
	name, _, _ := strings.Cut(tag, ",")
	return name
}

func joinPath(parent, child string) string {
	if parent == "" {
		return child
	}
	return parent + "." + child
}
