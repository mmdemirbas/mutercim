package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mmdemirbas/mutercim/internal/model"
	"github.com/spf13/viper"
)

// InputSpec describes a single input file with optional per-input page range.
type InputSpec struct {
	Path  string `yaml:"path" mapstructure:"path" json:"path"`
	Pages string `yaml:"pages,omitempty" mapstructure:"pages" json:"pages,omitempty"`
}

// Config represents the full workspace configuration.
type Config struct {
	Book         model.Book      `yaml:"book" mapstructure:"book" json:"book"`
	Inputs       []InputSpec     `yaml:"inputs" mapstructure:"inputs" json:"inputs"`
	DPI          int             `yaml:"dpi" mapstructure:"dpi" json:"dpi"`
	Read         ReadConfig      `yaml:"read" mapstructure:"read" json:"read"`
	Translate    TranslateConfig `yaml:"translate" mapstructure:"translate" json:"translate"`
	Write        WriteConfig     `yaml:"write" mapstructure:"write" json:"write"`
	KnowledgeDir string          `yaml:"knowledge_dir" mapstructure:"knowledge_dir" json:"knowledge_dir"`
	Retry        RetryConfig     `yaml:"retry" mapstructure:"retry" json:"retry"`
	RateLimit    RateLimitConfig `yaml:"rate_limit" mapstructure:"rate_limit" json:"rate_limit"`
}

// ModelSpec describes a single AI model in a failover chain.
type ModelSpec struct {
	Provider string `yaml:"provider" mapstructure:"provider" json:"provider"`
	Model    string `yaml:"model" mapstructure:"model" json:"model"`
	RPM      int    `yaml:"rpm,omitempty" mapstructure:"rpm" json:"rpm,omitempty"`                // requests per minute (0 = use provider default)
	Vision   *bool  `yaml:"vision,omitempty" mapstructure:"vision" json:"vision,omitempty"`       // nil = auto-detect from provider
	BaseURL  string `yaml:"base_url,omitempty" mapstructure:"base_url" json:"base_url,omitempty"` // override base URL
}

// ReadConfig holds read-phase settings.
type ReadConfig struct {
	LayoutTool  string      `yaml:"layout_tool,omitempty" mapstructure:"layout_tool" json:"layout_tool,omitempty"` // "surya" or "" (disabled)
	Models      []ModelSpec `yaml:"models" mapstructure:"models" json:"models"`
	Concurrency int         `yaml:"concurrency" mapstructure:"concurrency" json:"concurrency"` // reserved for future parallel processing
}

// TranslateConfig holds translation-phase settings.
type TranslateConfig struct {
	Models        []ModelSpec `yaml:"models" mapstructure:"models" json:"models"`
	ContextWindow int         `yaml:"context_window" mapstructure:"context_window" json:"context_window"`
}

// WriteConfig holds write-phase settings.
type WriteConfig struct {
	Formats          []string `yaml:"formats" mapstructure:"formats" json:"formats"`
	ExpandSources    bool     `yaml:"expand_sources" mapstructure:"expand_sources" json:"expand_sources"`
	LaTeXDockerImage string   `yaml:"latex_docker_image" mapstructure:"latex_docker_image" json:"latex_docker_image"`
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
	v.SetDefault("book.source_langs", []string{"ar"})
	v.SetDefault("book.target_langs", []string{"tr"})
	v.SetDefault("dpi", 300)

	v.SetDefault("read.concurrency", 1)

	v.SetDefault("translate.context_window", 2)

	v.SetDefault("write.formats", []string{"md", "pdf"})
	v.SetDefault("write.expand_sources", true)
	v.SetDefault("write.latex_docker_image", "mutercim/xelatex:latest")

	v.SetDefault("knowledge_dir", "./knowledge")

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
	if len(cfg.Book.SourceLangs) == 0 {
		cfg.Book.SourceLangs = []string{"ar"}
	}
	if len(cfg.Book.TargetLangs) == 0 {
		cfg.Book.TargetLangs = []string{"tr"}
	}
	if len(cfg.Inputs) == 0 {
		cfg.Inputs = []InputSpec{{Path: "./input"}}
	}
	if cfg.DPI == 0 {
		cfg.DPI = 300
	}
	if len(cfg.Read.Models) == 0 {
		cfg.Read.Models = []ModelSpec{{Provider: "gemini", Model: "gemini-2.0-flash"}}
	}
	if cfg.Read.Concurrency == 0 {
		cfg.Read.Concurrency = 1
	}
	if len(cfg.Translate.Models) == 0 {
		cfg.Translate.Models = []ModelSpec{{Provider: "gemini", Model: "gemini-2.0-flash"}}
	}
	if cfg.Translate.ContextWindow == 0 {
		cfg.Translate.ContextWindow = 2
	}
	if len(cfg.Write.Formats) == 0 {
		cfg.Write.Formats = []string{"md", "latex", "docx", "pdf"}
	}
	if cfg.Write.LaTeXDockerImage == "" {
		cfg.Write.LaTeXDockerImage = "mutercim/xelatex:latest"
	}
	if cfg.KnowledgeDir == "" {
		cfg.KnowledgeDir = "./knowledge"
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

// InputPaths returns just the paths from all input specs.
func (c *Config) InputPaths() []string {
	paths := make([]string, len(c.Inputs))
	for i, inp := range c.Inputs {
		paths[i] = inp.Path
	}
	return paths
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

// Validate checks the config for errors.
func (c *Config) Validate() error {
	// Check input paths exist and validate per-input pages
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
