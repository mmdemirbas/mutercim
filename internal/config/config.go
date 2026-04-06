package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mmdemirbas/mutercim/internal/model"
	"github.com/spf13/viper"
)

// InputSpec describes a single input file with optional per-input page range and source languages.
type InputSpec struct {
	Path      string   `yaml:"path" mapstructure:"path" json:"path"`
	Pages     string   `yaml:"pages,omitempty" mapstructure:"pages" json:"pages,omitempty"`
	Languages []string `yaml:"languages" mapstructure:"languages" json:"languages"`
}

// Config represents the full workspace configuration.
type Config struct {
	Inputs    []InputSpec     `yaml:"inputs" mapstructure:"inputs" json:"inputs"`
	Output    string          `yaml:"output" mapstructure:"output" json:"output"`
	LogLevel  string          `yaml:"log_level,omitempty" mapstructure:"log_level" json:"log_level,omitempty"`
	Cut       CutConfig       `yaml:"cut" mapstructure:"cut" json:"cut"`
	Layout    LayoutConfig    `yaml:"layout" mapstructure:"layout" json:"layout"`
	OCR       OCRConfig       `yaml:"ocr" mapstructure:"ocr" json:"ocr"`
	Read      ReadConfig      `yaml:"read" mapstructure:"read" json:"read"`
	Solve     SolveConfig     `yaml:"solve" mapstructure:"solve" json:"solve"`
	Translate TranslateConfig `yaml:"translate" mapstructure:"translate" json:"translate"`
	Write     WriteConfig     `yaml:"write" mapstructure:"write" json:"write"`
	Knowledge []string        `yaml:"knowledge" mapstructure:"knowledge" json:"knowledge"`
}

// CutConfig holds page-generation settings (PDF to images).
type CutConfig struct {
	DPI int `yaml:"dpi" mapstructure:"dpi" json:"dpi"`
}

// ModelSpec describes a single AI model in a failover chain.
type ModelSpec struct {
	Provider string `yaml:"provider" mapstructure:"provider" json:"provider"`
	Model    string `yaml:"model" mapstructure:"model" json:"model"`
	RPM      int    `yaml:"rpm,omitempty" mapstructure:"rpm" json:"rpm,omitempty"`                // requests per minute (0 = use provider default)
	Vision   *bool  `yaml:"vision,omitempty" mapstructure:"vision" json:"vision,omitempty"`       // nil = auto-detect from provider
	BaseURL  string `yaml:"base_url,omitempty" mapstructure:"base_url" json:"base_url,omitempty"` // override base URL
}

// LayoutConfig holds layout detection phase settings.
type LayoutConfig struct {
	Tool   string         `yaml:"tool,omitempty" mapstructure:"tool" json:"tool,omitempty"`       // "doclayout-yolo" (default), "surya", or "" (disabled)
	Debug  bool           `yaml:"debug,omitempty" mapstructure:"debug" json:"debug,omitempty"`    // when true, write debug overlay images
	Params map[string]any `yaml:"params,omitempty" mapstructure:"params" json:"params,omitempty"` // tool-specific tuning parameters
}

// OCRConfig holds OCR phase settings.
type OCRConfig struct {
	Tool string `yaml:"tool,omitempty" mapstructure:"tool" json:"tool,omitempty"` // "qari" or "" (disabled, skip OCR phase)
}

// ReadConfig holds read-phase settings.
type ReadConfig struct {
	Models      []ModelSpec     `yaml:"models" mapstructure:"models" json:"models"`
	Concurrency int             `yaml:"concurrency" mapstructure:"concurrency" json:"concurrency"` // reserved for future parallel processing
	Retry       RetryConfig     `yaml:"retry,omitempty" mapstructure:"retry" json:"retry,omitempty"`
	RateLimit   RateLimitConfig `yaml:"rate_limit,omitempty" mapstructure:"rate_limit" json:"rate_limit,omitempty"`
}

// SolveConfig holds solve-phase settings.
type SolveConfig struct {
	Retry     RetryConfig     `yaml:"retry,omitempty" mapstructure:"retry" json:"retry,omitempty"`
	RateLimit RateLimitConfig `yaml:"rate_limit,omitempty" mapstructure:"rate_limit" json:"rate_limit,omitempty"`
}

// TranslateConfig holds translation-phase settings.
type TranslateConfig struct {
	Languages     []string        `yaml:"languages" mapstructure:"languages" json:"languages"`
	Models        []ModelSpec     `yaml:"models" mapstructure:"models" json:"models"`
	ContextWindow int             `yaml:"context_window" mapstructure:"context_window" json:"context_window"`
	Retry         RetryConfig     `yaml:"retry,omitempty" mapstructure:"retry" json:"retry,omitempty"`
	RateLimit     RateLimitConfig `yaml:"rate_limit,omitempty" mapstructure:"rate_limit" json:"rate_limit,omitempty"`
}

// WriteConfig holds write-phase settings.
type WriteConfig struct {
	Formats       []string `yaml:"formats" mapstructure:"formats" json:"formats"`
	ExpandSources bool     `yaml:"expand_sources" mapstructure:"expand_sources" json:"expand_sources"`
}

// RetryConfig holds retry settings.
type RetryConfig struct {
	MaxAttempts    int `yaml:"max_attempts" mapstructure:"max_attempts" json:"max_attempts"`
	BackoffSeconds int `yaml:"backoff_seconds" mapstructure:"backoff_seconds" json:"backoff_seconds"`
	MaxFailPercent int `yaml:"max_fail_percent" mapstructure:"max_fail_percent" json:"max_fail_percent"` // 0 = no limit
}

// RateLimitConfig holds rate limiting settings.
type RateLimitConfig struct {
	RequestsPerMinute int `yaml:"requests_per_minute" mapstructure:"requests_per_minute" json:"requests_per_minute"`
}

// SetDefaults configures default values in viper.
func SetDefaults(v *viper.Viper) {
	v.SetDefault("output", ".")
	v.SetDefault("log_level", "info")
	v.SetDefault("cut.dpi", 300)

	v.SetDefault("layout.tool", "doclayout-yolo")

	v.SetDefault("read.concurrency", 1)
	v.SetDefault("read.retry.max_attempts", 3)
	v.SetDefault("read.retry.backoff_seconds", 2)
	v.SetDefault("read.retry.max_fail_percent", 10)

	v.SetDefault("solve.retry.max_attempts", 3)
	v.SetDefault("solve.retry.backoff_seconds", 2)
	v.SetDefault("solve.retry.max_fail_percent", 10)

	v.SetDefault("translate.languages", []string{"tr"})
	v.SetDefault("translate.context_window", 2)
	v.SetDefault("translate.retry.max_attempts", 3)
	v.SetDefault("translate.retry.backoff_seconds", 2)
	v.SetDefault("translate.retry.max_fail_percent", 10)

	v.SetDefault("write.formats", []string{"md", "pdf"})
	v.SetDefault("write.expand_sources", true)

	v.SetDefault("knowledge", []string{"./knowledge"})
}

// Load reads the config file at the given path and returns a Config.
// If configPath is empty, it looks for mutercim.yaml in the current directory.
func Load(configPath string) (*Config, error) {
	v := viper.New()
	SetDefaults(v)

	v.SetConfigType("yaml")

	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("mutercim")
		v.AddConfigPath(".")
	}

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("read config: %w", err)
		}
		// Config file not found — use defaults only
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	// Apply defaults for fields that viper's nested unmarshal may miss
	applyDefaults(&cfg)

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return &cfg, nil
}

// applyDefaults fills in zero-valued fields with their defaults.
//
//nolint:cyclop // sequential defaulting of many independent fields
func applyDefaults(cfg *Config) {
	if len(cfg.Translate.Languages) == 0 {
		cfg.Translate.Languages = []string{"tr"}
	}
	if len(cfg.Inputs) == 0 {
		cfg.Inputs = []InputSpec{{Path: "./input"}}
	}
	if cfg.Output == "" {
		cfg.Output = "."
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}
	if cfg.Cut.DPI == 0 {
		cfg.Cut.DPI = 300
	}
	if len(cfg.Read.Models) == 0 {
		cfg.Read.Models = []ModelSpec{{Provider: "gemini", Model: DefaultModel}}
	}
	if cfg.Read.Concurrency == 0 {
		cfg.Read.Concurrency = 1
	}
	applyRetryDefaults(&cfg.Read.Retry)
	applyRetryDefaults(&cfg.Solve.Retry)
	if len(cfg.Translate.Models) == 0 {
		cfg.Translate.Models = []ModelSpec{{Provider: "gemini", Model: DefaultModel}}
	}
	if cfg.Translate.ContextWindow == 0 {
		cfg.Translate.ContextWindow = 2
	}
	applyRetryDefaults(&cfg.Translate.Retry)
	if len(cfg.Write.Formats) == 0 {
		cfg.Write.Formats = []string{"md", "latex", "docx", "pdf"}
	}
	if len(cfg.Knowledge) == 0 {
		cfg.Knowledge = []string{"./knowledge"}
	}
}

// applyRetryDefaults fills in zero-valued retry fields with their defaults.
func applyRetryDefaults(r *RetryConfig) {
	if r.MaxAttempts == 0 {
		r.MaxAttempts = 3
	}
	if r.BackoffSeconds == 0 {
		r.BackoffSeconds = 2
	}
	if r.MaxFailPercent == 0 {
		r.MaxFailPercent = 10
	}
}

// InputPaths returns just the paths from all input specs.
func (c *Config) InputPaths() []string {
	paths := make([]string, len(c.Inputs))
	for i, inp := range c.Inputs {
		paths[i] = inp.Path
	}
	return paths
}

// PrimarySourceLang returns the primary source language from the first input.
// Falls back to "ar" if no inputs or no languages configured.
func (c *Config) PrimarySourceLang() string {
	for _, inp := range c.Inputs {
		if len(inp.Languages) > 0 {
			return inp.Languages[0]
		}
	}
	return "ar"
}

// SourceLanguages returns unique source languages across all inputs, preserving order.
func (c *Config) SourceLanguages() []string {
	seen := make(map[string]bool)
	var result []string
	for _, inp := range c.Inputs {
		for _, lang := range inp.Languages {
			if !seen[lang] {
				seen[lang] = true
				result = append(result, lang)
			}
		}
	}
	return result
}

// SourceLanguagesForStem returns source languages for the input matching the given stem name.
// Falls back to the first input's languages or empty if not found.
func (c *Config) SourceLanguagesForStem(stem string) []string {
	for _, inp := range c.Inputs {
		if fileStem(inp.Path) == stem {
			return inp.Languages
		}
	}
	// Fallback: return first input's languages
	if len(c.Inputs) > 0 {
		return c.Inputs[0].Languages
	}
	return nil
}

// fileStem returns the filename without extension.
func fileStem(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	if ext != "" {
		return base[:len(base)-len(ext)]
	}
	return base
}

// ResolveOutputDir resolves the output directory against the workspace root.
func (c *Config) ResolveOutputDir(base string) string {
	return c.ResolvePath(base, c.Output)
}

// ResolveKnowledgePaths resolves all knowledge paths against the workspace root.
func (c *Config) ResolveKnowledgePaths(base string) []string {
	resolved := make([]string, len(c.Knowledge))
	for i, p := range c.Knowledge {
		resolved[i] = c.ResolvePath(base, p)
	}
	return resolved
}

// IsPDF returns true if the given path points to a PDF file.
func IsPDF(path string) bool {
	return filepath.Ext(path) == ".pdf"
}

// ResolvePath resolves a relative path against the workspace root.
func (c *Config) ResolvePath(base, rel string) string {
	if filepath.IsAbs(rel) {
		return rel
	}
	return filepath.Join(base, rel)
}

// DefaultModel is the default AI model used for read and translate phases.
// This is the single source of truth shared with workspace scaffold generation.
const DefaultModel = "gemini-2.0-flash"

// validLayoutTools is the set of recognized layout.tool values.
var validLayoutTools = map[string]bool{"": true, "doclayout-yolo": true, "surya": true}

// validOCRTools is the set of recognized ocr.tool values.
var validOCRTools = map[string]bool{"": true, "qari": true}

// validWriteFormats is the set of recognized write.formats values.
var validWriteFormats = map[string]bool{"md": true, "latex": true, "docx": true, "pdf": true}

// Validate checks the config for errors.
func (c *Config) Validate() error {
	if err := c.validateInputs(); err != nil {
		return err
	}
	if err := c.validateModels(); err != nil {
		return err
	}
	return c.validateTools()
}

// validateInputs checks input paths, languages, and page ranges.
func (c *Config) validateInputs() error {
	hasLanguages := false
	for _, inp := range c.Inputs {
		if len(inp.Languages) > 0 {
			hasLanguages = true
			break
		}
	}
	if !hasLanguages {
		return fmt.Errorf("inputs[].languages is required: at least one input must have source languages")
	}

	for i, inp := range c.Inputs {
		if _, err := os.Stat(inp.Path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("input %d path %q: %w", i, inp.Path, err)
		}
		if inp.Pages != "" {
			if _, err := model.ParsePageRanges(inp.Pages); err != nil {
				return fmt.Errorf("input %d (%s) pages: %w", i, inp.Path, err)
			}
		}
	}
	return nil
}

// validateModels checks that read and translate model lists are non-empty
// and each entry has provider and model fields.
func (c *Config) validateModels() error {
	if len(c.Read.Models) == 0 {
		return fmt.Errorf("read.models is required: configure at least one model for the read phase")
	}
	for i, m := range c.Read.Models {
		if m.Provider == "" {
			return fmt.Errorf("read.models[%d].provider is required", i)
		}
		if m.Model == "" {
			return fmt.Errorf("read.models[%d].model is required", i)
		}
	}

	if len(c.Translate.Models) == 0 {
		return fmt.Errorf("translate.models is required: configure at least one model for the translate phase")
	}
	for i, m := range c.Translate.Models {
		if m.Provider == "" {
			return fmt.Errorf("translate.models[%d].provider is required", i)
		}
		if m.Model == "" {
			return fmt.Errorf("translate.models[%d].model is required", i)
		}
	}
	return nil
}

// validateTools checks layout tool, OCR tool, write formats, and translate settings.
func (c *Config) validateTools() error {
	if !validLayoutTools[c.Layout.Tool] {
		return fmt.Errorf("layout.tool %q is not valid (expected: \"\", \"doclayout-yolo\", or \"surya\")", c.Layout.Tool)
	}

	if !validOCRTools[c.OCR.Tool] {
		return fmt.Errorf("ocr.tool %q is not valid (expected: \"\" or \"qari\")", c.OCR.Tool)
	}

	if len(c.Write.Formats) == 0 {
		return fmt.Errorf("write.formats is required: configure at least one output format")
	}
	for _, f := range c.Write.Formats {
		if !validWriteFormats[f] {
			return fmt.Errorf("write.formats contains unknown format %q (valid: md, latex, docx, pdf)", f)
		}
	}

	if c.Translate.ContextWindow < 0 {
		return fmt.Errorf("translate.context_window must be >= 0, got %d", c.Translate.ContextWindow)
	}

	return nil
}
