package cli

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/mmdemirbas/mutercim/internal/apiclient"
	"github.com/mmdemirbas/mutercim/internal/config"
	"github.com/mmdemirbas/mutercim/internal/display"
	"github.com/mmdemirbas/mutercim/internal/model"
	"github.com/mmdemirbas/mutercim/internal/pipeline"
	"github.com/mmdemirbas/mutercim/internal/progress"
	"github.com/mmdemirbas/mutercim/internal/provider"
	"github.com/mmdemirbas/mutercim/internal/workspace"
	"github.com/spf13/cobra"
)

func newReadCmd() *cobra.Command {
	var (
		readProvider string
		readModel    string
		concurrency  int
	)

	cmd := &cobra.Command{
		Use:   "read",
		Short: "Read structured data from page images (OCR)",
		Long:  "Sends page images to an AI vision model and reads structured JSON with entries, footnotes, and metadata. Images must exist in midstate/images/ (run 'mutercim pages' first for PDFs).",
		RunE: func(cmd *cobra.Command, args []string) error {
			ws, err := workspace.Discover(".")
			if err != nil {
				return fmt.Errorf("workspace: %w", err)
			}

			configPath := cfgFile
			if configPath == "" {
				configPath = ws.ConfigPath()
			}
			cfg, err := config.Load(configPath)
			if err != nil {
				return fmt.Errorf("config: %w", err)
			}

			// Apply CLI flag overrides — override the models list
			if readProvider != "" || readModel != "" {
				p := readProvider
				if p == "" && len(cfg.Read.Models) > 0 {
					p = cfg.Read.Models[0].Provider
				}
				m := readModel
				if m == "" && len(cfg.Read.Models) > 0 {
					m = cfg.Read.Models[0].Model
				}
				cfg.Read.Models = []config.ModelSpec{{Provider: p, Model: m}}
			}
			if concurrency > 0 {
				cfg.Read.Concurrency = concurrency
			}

			logger := slog.Default()

			chain, err := createProviderChain(cfg.Read.Models, cfg.Retry, logger)
			if err != nil {
				return err
			}
			defer chain.Close()

			// Determine page range: CLI flag > config > all
			pageSpec := cfg.Pages
			if pages != "" {
				pageSpec = pages
			}

			var pagesToProcess []int
			if pageSpec != "" && pageSpec != "all" {
				ranges, err := model.ParsePageRanges(pageSpec)
				if err != nil {
					return fmt.Errorf("parse pages: %w", err)
				}
				pagesToProcess = model.ExpandPages(ranges)
			}

			// Load progress tracker
			tracker := progress.NewTracker(ws.ProgressPath())
			if err := tracker.Load(); err != nil {
				return fmt.Errorf("load progress: %w", err)
			}

			// Run read pipeline
			_, err = pipeline.Read(cmd.Context(), pipeline.ReadOptions{
				Workspace: ws,
				Config:    cfg,
				Provider:  chain,
				Tracker:   tracker,
				Pages:     pagesToProcess,
				Logger:    logger,
				Display:   display.FromContext(cmd.Context()),
			})
			return err
		},
	}

	cmd.Flags().StringVar(&readProvider, "read-provider", "", "provider: gemini, claude, openai, groq, mistral, openrouter, xai, ollama (default: from config)")
	cmd.Flags().StringVar(&readModel, "read-model", "", "model for reading (default: from config)")
	cmd.Flags().IntVar(&concurrency, "concurrency", 0, "parallel read workers (default: from config)")

	return cmd
}

// resolveAPIKey returns the API key for the given provider from environment variables.
func resolveAPIKey(providerName string) (string, error) {
	envVar := apiKeyEnvVar(providerName)
	if envVar == "" {
		return "", nil // Local providers (ollama, surya) don't need a key
	}

	key := os.Getenv(envVar)
	if key == "" {
		return "", fmt.Errorf("%s environment variable is not set (required for %s provider)", envVar, providerName)
	}
	return key, nil
}

// apiKeyEnvVar returns the environment variable name for a provider's API key.
// Returns empty string for providers that don't need a key.
func apiKeyEnvVar(providerName string) string {
	switch providerName {
	case "gemini":
		return "GEMINI_API_KEY"
	case "claude":
		return "ANTHROPIC_API_KEY"
	case "openai":
		return "OPENAI_API_KEY"
	case "groq":
		return "GROQ_API_KEY"
	case "mistral":
		return "MISTRAL_API_KEY"
	case "openrouter":
		return "OPENROUTER_API_KEY"
	case "xai":
		return "XAI_API_KEY"
	case "ollama", "surya":
		return ""
	default:
		return strings.ToUpper(providerName) + "_API_KEY"
	}
}

// clientTimeout returns an appropriate HTTP timeout for the given provider.
func clientTimeout(providerName string) time.Duration {
	if providerName == "ollama" {
		return 10 * time.Minute
	}
	return 120 * time.Second
}

// defaultRPM returns the default rate limit for a provider.
func defaultRPM(providerName string) int {
	switch providerName {
	case "gemini":
		return 10
	case "claude":
		return 50
	case "openai":
		return 500
	case "groq":
		return 30
	case "mistral":
		return 60
	case "openrouter":
		return 200
	case "xai":
		return 60
	case "ollama":
		return 1000 // local, effectively unlimited
	default:
		return 14
	}
}

// defaultVision returns whether a provider supports vision by default.
func defaultVision(providerName string) bool {
	switch providerName {
	case "gemini", "claude", "openai", "ollama":
		return true
	case "groq", "mistral", "openrouter", "xai":
		return false
	default:
		return false
	}
}

// createProviderChain builds a failover chain from a list of model specs.
// Each model gets its own apiclient.Client with its own rate limiter.
func createProviderChain(models []config.ModelSpec, retryCfg config.RetryConfig, logger *slog.Logger) (*provider.FailoverChain, error) {
	var providers []provider.Provider
	var clients []*apiclient.Client

	cleanup := func() {
		for _, c := range clients {
			c.Close()
		}
	}

	for _, spec := range models {
		apiKey, err := resolveAPIKey(spec.Provider)
		if err != nil {
			cleanup()
			return nil, err
		}

		rpm := spec.RPM
		if rpm == 0 {
			rpm = defaultRPM(spec.Provider)
		}

		client := apiclient.NewClient(apiclient.ClientConfig{
			Timeout:           clientTimeout(spec.Provider),
			MaxRetries:        retryCfg.MaxAttempts,
			BaseBackoff:       time.Duration(retryCfg.BackoffSeconds) * time.Second,
			RequestsPerMinute: rpm,
		}, logger)
		clients = append(clients, client)

		p, err := buildSingleProvider(spec, client, apiKey)
		if err != nil {
			cleanup()
			return nil, err
		}
		providers = append(providers, p)
	}

	return provider.NewFailoverChain(providers, clients, 60*time.Second, logger), nil
}

// buildSingleProvider creates one provider from a ModelSpec.
func buildSingleProvider(spec config.ModelSpec, client *apiclient.Client, apiKey string) (provider.Provider, error) {
	vision := defaultVision(spec.Provider)
	if spec.Vision != nil {
		vision = *spec.Vision
	}

	switch spec.Provider {
	case "gemini":
		return provider.NewGeminiProvider(client, apiKey, spec.Model), nil
	case "claude":
		return provider.NewClaudeProvider(client, apiKey, spec.Model), nil
	case "ollama":
		return provider.NewOllamaProvider(client, spec.Model), nil
	case "surya":
		return nil, fmt.Errorf("surya provider is not yet implemented")
	default:
		// OpenAI-compatible providers
		baseURL := spec.BaseURL
		if baseURL == "" {
			if preset, ok := provider.OpenAICompatPresets[spec.Provider]; ok {
				baseURL = preset
			} else {
				return nil, fmt.Errorf("unknown provider %q (no base_url configured)", spec.Provider)
			}
		}
		return provider.NewOpenAICompatProvider(client, spec.Provider, apiKey, spec.Model, baseURL, vision), nil
	}
}
