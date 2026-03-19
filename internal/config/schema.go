package config

import (
	"encoding/json"
	"reflect"
	"strings"

	"github.com/mmdemirbas/mutercim/internal/model"
)

// schemaMeta provides JSON Schema metadata for a config field.
// Fields are keyed by dot-separated path (e.g. "book.title", "sections[].type").
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

func sectionTypeEnums() []string {
	result := make([]string, len(model.ValidSectionTypes))
	for i, st := range model.ValidSectionTypes {
		result[i] = string(st)
	}
	return result
}

// schemaAnnotations maps field paths to their schema metadata.
// This is the single source of truth for descriptions, defaults, enums, and constraints.
// Structure (field names, types, nesting) comes from reflection on the Go types.
var schemaAnnotations = map[string]schemaMeta{
	// book
	"book":              {Description: "Book metadata."},
	"book.title":        {Description: "Book title."},
	"book.author":       {Description: "Book author."},
	"book.source_langs": {Description: `Source language codes (e.g. ["ar"]).`, Default: []string{"ar"}},
	"book.target_langs": {Description: `Target language codes (e.g. ["tr"]).`, Default: []string{"tr"}},

	// inputs
	"inputs":         {Description: "Input files with optional per-input page ranges.", Default: []map[string]string{{"path": "./input"}}},
	"inputs[]":       {Required: []string{"path"}},
	"inputs[].path":  {Description: "Path to the input file (relative to workspace root)."},
	"inputs[].pages": {Description: `Page range for this input (e.g. "1-50", "1,5,10-20", "all").`},

	// top-level
	"output":       {Description: "Output directory (relative to workspace root).", Default: "./output"},
	"midstate_dir": {Description: "Intermediate state directory (relative to workspace root).", Default: "./midstate"},
	"dpi":          {Description: "DPI for PDF-to-image conversion.", Default: 300, Minimum: intPtr(72)},

	// sections
	"sections":             {Description: "Book section definitions. Pages not covered default to type: auto.", Default: []any{}},
	"sections[]":           {Required: []string{"name", "pages", "type"}},
	"sections[].name":      {Description: "Section name (for display/logging)."},
	"sections[].pages":     {Description: `Page range (e.g. "1-50", "1,5,10-20").`},
	"sections[].type":      {Description: "Section layout type.", Enum: sectionTypeEnums()},
	"sections[].translate": {Description: "Whether to translate this section.", Default: true},

	// read
	"read":                   {Description: "Read phase (OCR/vision) settings."},
	"read.models":            {Description: "Ordered failover chain of models. First model is primary; on 429/quota exhaustion, fails over to next."},
	"read.models[]":          {},
	"read.models[].provider": {Description: "AI provider name.", Enum: []string{"gemini", "claude", "openai", "groq", "mistral", "openrouter", "xai", "ollama"}},
	"read.models[].model":    {Description: "Model name."},
	"read.models[].rpm":      {Description: "Requests per minute (0 = use provider default)."},
	"read.models[].vision":   {Description: "Whether this model supports vision. Null = auto-detect from provider."},
	"read.models[].base_url": {Description: "Override base URL for OpenAI-compatible providers."},
	"read.concurrency":       {Description: "Reserved for future parallel processing.", Default: 1, Minimum: intPtr(1)},

	// translate
	"translate":                   {Description: "Translation phase settings."},
	"translate.models":            {Description: "Ordered failover chain of models for translation."},
	"translate.models[]":          {},
	"translate.models[].provider": {Description: "AI provider name.", Enum: []string{"gemini", "claude", "openai", "groq", "mistral", "openrouter", "xai", "ollama"}},
	"translate.models[].model":    {Description: "Model name."},
	"translate.models[].rpm":      {Description: "Requests per minute (0 = use provider default)."},
	"translate.models[].vision":   {Description: "Whether this model supports vision."},
	"translate.models[].base_url": {Description: "Override base URL for OpenAI-compatible providers."},
	"translate.context_window":    {Description: "Number of previous pages to include as context.", Default: 2, Minimum: intPtr(0)},

	// write
	"write":                    {Description: "Write/compilation phase settings."},
	"write.formats":            {Description: "Output formats to generate.", Default: []string{"md", "latex", "docx", "pdf"}, ItemEnum: []string{"md", "latex", "pdf", "docx"}},
	"write.expand_sources":     {Description: "Expand source abbreviations in output.", Default: true},
	"write.latex_docker_image": {Description: "Docker image for LaTeX/PDF compilation.", Default: "mutercim/xelatex:latest"},

	// knowledge
	"knowledge_dir": {Description: "Knowledge YAML files directory (relative to workspace root).", Default: "./knowledge"},

	// retry
	"retry":                 {Description: "Retry settings for API calls."},
	"retry.max_attempts":    {Description: "Maximum number of retry attempts.", Default: 3, Minimum: intPtr(0)},
	"retry.backoff_seconds": {Description: "Base backoff duration in seconds (exponential: 2s, 4s, 8s).", Default: 2, Minimum: intPtr(1)},

	// rate_limit
	"rate_limit":                     {Description: "Rate limiting settings for API calls."},
	"rate_limit.requests_per_minute": {Description: "Maximum requests per minute.", Default: 14, Minimum: intPtr(1)},
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
