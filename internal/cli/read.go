package cli

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/mmdemirbas/mutercim/internal/apiclient"
	"github.com/mmdemirbas/mutercim/internal/config"
	"github.com/mmdemirbas/mutercim/internal/input"
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
		dpi          int
	)

	cmd := &cobra.Command{
		Use:   "read",
		Short: "Read structured data from page images (Phase 1)",
		Long:  "Sends page images to an AI vision model and reads structured JSON with entries, footnotes, and metadata.",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Discover workspace
			ws, err := workspace.Discover(".")
			if err != nil {
				return fmt.Errorf("workspace: %w", err)
			}

			// Load config
			configPath := cfgFile
			if configPath == "" {
				configPath = ws.ConfigPath()
			}
			cfg, err := config.Load(configPath)
			if err != nil {
				return fmt.Errorf("config: %w", err)
			}

			// Apply CLI flag overrides
			if readProvider != "" {
				cfg.Read.Provider = readProvider
			}
			if readModel != "" {
				cfg.Read.Model = readModel
			}
			if concurrency > 0 {
				cfg.Read.Concurrency = concurrency
			}
			if dpi > 0 {
				cfg.DPI = dpi
			}

			// Preflight: check pdftoppm if any input is PDF
			for _, inp := range cfg.Inputs {
				resolved := cfg.ResolvePath(ws.Root, inp.Path)
				if config.IsPDF(resolved) {
					if err := input.CheckPdftoppm(); err != nil {
						return err
					}
					break
				}
			}

			// Resolve API key
			apiKey, err := resolveAPIKey(cfg.Read.Provider)
			if err != nil {
				return err
			}

			// Create API client
			clientCfg := apiclient.ClientConfig{
				Timeout:           clientTimeout(cfg.Read.Provider),
				MaxRetries:        cfg.Retry.MaxAttempts,
				BaseBackoff:       time.Duration(cfg.Retry.BackoffSeconds) * time.Second,
				RequestsPerMinute: cfg.RateLimit.RequestsPerMinute,
			}
			logger := slog.Default()
			client := apiclient.NewClient(clientCfg, logger)
			defer client.Close()

			// Create provider
			p, err := createProvider(cfg.Read.Provider, client, apiKey, cfg.Read.Model)
			if err != nil {
				return err
			}

			// Determine page range: CLI flag > config > all
			pageSpec := cfg.Pages // from config
			if pages != "" {
				pageSpec = pages // CLI flag overrides
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
			return pipeline.Read(cmd.Context(), pipeline.ReadOptions{
				Workspace: ws,
				Config:    cfg,
				Provider:  p,
				Tracker:   tracker,
				Pages:     pagesToProcess,
				Logger:    logger,
			})
		},
	}

	cmd.Flags().StringVar(&readProvider, "read-provider", "", "provider: gemini, claude, openai, ollama, surya (default: from config)")
	cmd.Flags().StringVar(&readModel, "read-model", "", "model for reading (default: from config)")
	cmd.Flags().IntVar(&concurrency, "concurrency", 0, "parallel read workers (default: from config)")
	cmd.Flags().IntVar(&dpi, "dpi", 0, "DPI for PDF-to-image conversion (default: from config)")

	return cmd
}

func resolveAPIKey(providerName string) (string, error) {
	var envVar string
	switch providerName {
	case "gemini":
		envVar = "GEMINI_API_KEY"
	case "claude":
		envVar = "ANTHROPIC_API_KEY"
	case "openai":
		envVar = "OPENAI_API_KEY"
	case "ollama":
		return "", nil // Ollama doesn't need an API key
	default:
		return "", fmt.Errorf("unknown provider %q", providerName)
	}

	key := os.Getenv(envVar)
	if key == "" {
		return "", fmt.Errorf("%s environment variable is not set (required for %s provider)", envVar, providerName)
	}
	return key, nil
}

// clientTimeout returns an appropriate HTTP timeout for the given provider.
// Local models (ollama) need much longer timeouts for vision processing.
func clientTimeout(providerName string) time.Duration {
	if providerName == "ollama" {
		return 10 * time.Minute
	}
	return 120 * time.Second
}

func createProvider(name string, client *apiclient.Client, apiKey, modelName string) (provider.Provider, error) {
	switch name {
	case "gemini":
		return provider.NewGeminiProvider(client, apiKey, modelName), nil
	case "claude":
		return provider.NewClaudeProvider(client, apiKey, modelName), nil
	case "openai":
		return provider.NewOpenAIProvider(client, apiKey, modelName), nil
	case "ollama":
		return provider.NewOllamaProvider(client, modelName), nil
	default:
		return nil, fmt.Errorf("unknown provider %q", name)
	}
}
