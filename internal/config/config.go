package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mmdemirbas/mutercim/internal/model"
	"github.com/spf13/viper"
)

// Config represents the full workspace configuration.
type Config struct {
	Book      model.Book      `yaml:"book" mapstructure:"book" json:"book"`
	Input     string          `yaml:"input" mapstructure:"input" json:"input,omitempty"`    // single input (backward compat)
	Inputs    []string        `yaml:"inputs" mapstructure:"inputs" json:"inputs,omitempty"` // multiple inputs
	Pages     string          `yaml:"pages" mapstructure:"pages" json:"pages,omitempty"`
	Output    string          `yaml:"output" mapstructure:"output" json:"output"`
	CacheDir  string          `yaml:"cache_dir" mapstructure:"cache_dir" json:"cache_dir"`
	DPI       int             `yaml:"dpi" mapstructure:"dpi" json:"dpi"`
	Sections  []model.Section `yaml:"sections" mapstructure:"sections" json:"sections"`
	Read      ReadConfig      `yaml:"read" mapstructure:"read" json:"read"`
	Translate TranslateConfig `yaml:"translate" mapstructure:"translate" json:"translate"`
	Write     WriteConfig     `yaml:"write" mapstructure:"write" json:"write"`
	Knowledge KnowledgeConfig `yaml:"knowledge" mapstructure:"knowledge" json:"knowledge"`
	Retry     RetryConfig     `yaml:"retry" mapstructure:"retry" json:"retry"`
	RateLimit RateLimitConfig `yaml:"rate_limit" mapstructure:"rate_limit" json:"rate_limit"`
}

// ReadConfig holds read-phase settings.
type ReadConfig struct {
	Provider    string `yaml:"provider" mapstructure:"provider" json:"provider"`
	Model       string `yaml:"model" mapstructure:"model" json:"model"`
	Concurrency int    `yaml:"concurrency" mapstructure:"concurrency" json:"concurrency"`
}

// TranslateConfig holds translation-phase settings.
type TranslateConfig struct {
	Provider      string `yaml:"provider" mapstructure:"provider" json:"provider"`
	Model         string `yaml:"model" mapstructure:"model" json:"model"`
	ContextWindow int    `yaml:"context_window" mapstructure:"context_window" json:"context_window"`
}

// WriteConfig holds write-phase settings.
type WriteConfig struct {
	Formats          []string `yaml:"formats" mapstructure:"formats" json:"formats"`
	ExpandSources    bool     `yaml:"expand_sources" mapstructure:"expand_sources" json:"expand_sources"`
	LaTeXDockerImage string   `yaml:"latex_docker_image" mapstructure:"latex_docker_image" json:"latex_docker_image"`
	SkipPDF          bool     `yaml:"skip_pdf" mapstructure:"skip_pdf" json:"skip_pdf"`
}

// KnowledgeConfig holds knowledge directory settings.
type KnowledgeConfig struct {
	Dir string `yaml:"dir" mapstructure:"dir" json:"dir"`
}

// RetryConfig holds retry settings.
type RetryConfig struct {
	MaxAttempts    int `yaml:"max_attempts" mapstructure:"max_attempts" json:"max_attempts"`
	BackoffSeconds int `yaml:"backoff_seconds" mapstructure:"backoff_seconds" json:"backoff_seconds"`
}

// RateLimitConfig holds rate limiting settings.
type RateLimitConfig struct {
	RequestsPerMinute int `yaml:"requests_per_minute" mapstructure:"requests_per_minute" json:"requests_per_minute"`
}

// SetDefaults configures default values in viper.
func SetDefaults(v *viper.Viper) {
	v.SetDefault("book.source_lang", "ar")
	v.SetDefault("book.target_lang", "tr")
	v.SetDefault("input", "./input")
	v.SetDefault("output", "./output")
	v.SetDefault("cache_dir", "./cache")
	v.SetDefault("dpi", 300)

	v.SetDefault("read.provider", "gemini")
	v.SetDefault("read.model", "gemini-2.0-flash")
	v.SetDefault("read.concurrency", 1)

	v.SetDefault("translate.provider", "gemini")
	v.SetDefault("translate.model", "gemini-2.0-flash")
	v.SetDefault("translate.context_window", 2)

	v.SetDefault("write.formats", []string{"md", "latex"})
	v.SetDefault("write.expand_sources", true)
	v.SetDefault("write.latex_docker_image", "mutercim/xelatex:latest")
	v.SetDefault("write.skip_pdf", false)

	v.SetDefault("knowledge.dir", "./knowledge")

	v.SetDefault("retry.max_attempts", 3)
	v.SetDefault("retry.backoff_seconds", 2)

	v.SetDefault("rate_limit.requests_per_minute", 14)
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

	return &cfg, nil
}

// applyDefaults fills in zero-valued fields with their defaults.
func applyDefaults(cfg *Config) {
	if cfg.Book.SourceLang == "" {
		cfg.Book.SourceLang = "ar"
	}
	if cfg.Book.TargetLang == "" {
		cfg.Book.TargetLang = "tr"
	}
	// Migrate singular input to inputs list
	if len(cfg.Inputs) == 0 && cfg.Input != "" {
		cfg.Inputs = []string{cfg.Input}
	}
	if len(cfg.Inputs) == 0 {
		cfg.Inputs = []string{"./input"}
	}
	if cfg.Output == "" {
		cfg.Output = "./output"
	}
	if cfg.CacheDir == "" {
		cfg.CacheDir = "./cache"
	}
	if cfg.DPI == 0 {
		cfg.DPI = 300
	}
	if cfg.Read.Provider == "" {
		cfg.Read.Provider = "gemini"
	}
	if cfg.Read.Model == "" {
		cfg.Read.Model = "gemini-2.0-flash"
	}
	if cfg.Read.Concurrency == 0 {
		cfg.Read.Concurrency = 1
	}
	if cfg.Translate.Provider == "" {
		cfg.Translate.Provider = "gemini"
	}
	if cfg.Translate.Model == "" {
		cfg.Translate.Model = "gemini-2.0-flash"
	}
	if cfg.Translate.ContextWindow == 0 {
		cfg.Translate.ContextWindow = 2
	}
	if len(cfg.Write.Formats) == 0 {
		cfg.Write.Formats = []string{"md", "latex"}
	}
	if cfg.Write.LaTeXDockerImage == "" {
		cfg.Write.LaTeXDockerImage = "mutercim/xelatex:latest"
	}
	if cfg.Knowledge.Dir == "" {
		cfg.Knowledge.Dir = "./knowledge"
	}
	if cfg.Retry.MaxAttempts == 0 {
		cfg.Retry.MaxAttempts = 3
	}
	if cfg.Retry.BackoffSeconds == 0 {
		cfg.Retry.BackoffSeconds = 2
	}
	if cfg.RateLimit.RequestsPerMinute == 0 {
		cfg.RateLimit.RequestsPerMinute = 14
	}
}

// IsPDF returns true if the given path points to a PDF file.
func IsPDF(path string) bool {
	return filepath.Ext(path) == ".pdf"
}

// InputIsPDF returns true if the first input path points to a PDF file.
// Deprecated: use IsPDF with individual paths from Inputs instead.
func (c *Config) InputIsPDF() bool {
	if len(c.Inputs) > 0 {
		return IsPDF(c.Inputs[0])
	}
	return IsPDF(c.Input)
}

// ResolvePath resolves a relative path against the workspace root.
func (c *Config) ResolvePath(base, rel string) string {
	if filepath.IsAbs(rel) {
		return rel
	}
	return filepath.Join(base, rel)
}

// SectionForPage returns the section that contains the given page number.
// If no section matches, returns a section with type "auto".
func (c *Config) SectionForPage(page int) model.Section {
	for _, s := range c.Sections {
		ranges, err := model.ParsePageRanges(s.Pages)
		if err != nil {
			continue
		}
		if model.PageInRanges(page, ranges) {
			return s
		}
	}
	return model.Section{
		Name:      "auto",
		Type:      model.SectionAuto,
		Translate: true,
	}
}

// Validate checks the config for errors.
func (c *Config) Validate() error {
	for i, s := range c.Sections {
		if !s.Type.IsValid() {
			return fmt.Errorf("section %d (%s): invalid type %q", i, s.Name, s.Type)
		}
		if _, err := model.ParsePageRanges(s.Pages); err != nil {
			return fmt.Errorf("section %d (%s): invalid pages %q: %w", i, s.Name, s.Pages, err)
		}
	}

	// Check input paths exist
	for _, inp := range c.Inputs {
		if _, err := os.Stat(inp); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("input path %q: %w", inp, err)
		}
	}

	// Validate pages if set
	if c.Pages != "" {
		if _, err := model.ParsePageRanges(c.Pages); err != nil {
			return fmt.Errorf("pages: %w", err)
		}
	}

	return nil
}
