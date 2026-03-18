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

func newExtractCmd() *cobra.Command {
	var (
		extractProvider string
		extractModel    string
		concurrency     int
		dpi             int
	)

	cmd := &cobra.Command{
		Use:   "extract",
		Short: "Extract structured data from page images (Phase 1)",
		Long:  "Sends page images to an AI vision model and extracts structured JSON with entries, footnotes, and metadata.",
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
			if extractProvider != "" {
				cfg.Extract.Provider = extractProvider
			}
			if extractModel != "" {
				cfg.Extract.Model = extractModel
			}
			if concurrency > 0 {
				cfg.Extract.Concurrency = concurrency
			}
			if dpi > 0 {
				cfg.DPI = dpi
			}

			// Preflight: check pdftoppm if input is PDF
			if cfg.InputIsPDF() {
				if err := input.CheckPdftoppm(); err != nil {
					return err
				}
			}

			// Resolve API key
			apiKey, err := resolveAPIKey(cfg.Extract.Provider)
			if err != nil {
				return err
			}

			// Create API client
			clientCfg := apiclient.ClientConfig{
				Timeout:           120 * time.Second,
				MaxRetries:        cfg.Retry.MaxAttempts,
				BaseBackoff:       time.Duration(cfg.Retry.BackoffSeconds) * time.Second,
				RequestsPerMinute: cfg.RateLimit.RequestsPerMinute,
			}
			logger := slog.Default()
			client := apiclient.NewClient(clientCfg, logger)
			defer client.Close()

			// Create provider
			p, err := createProvider(cfg.Extract.Provider, client, apiKey, cfg.Extract.Model)
			if err != nil {
				return err
			}

			// Parse page range
			var pagesToProcess []int
			if pages != "all" && pages != "" {
				ranges, err := model.ParsePageRanges(pages)
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

			// Run extraction pipeline
			return pipeline.Extract(cmd.Context(), pipeline.ExtractOptions{
				Workspace: ws,
				Config:    cfg,
				Provider:  p,
				Tracker:   tracker,
				Pages:     pagesToProcess,
				Logger:    logger,
			})
		},
	}

	cmd.Flags().StringVar(&extractProvider, "extract-provider", "", "provider: gemini, claude, openai, ollama, surya (default: from config)")
	cmd.Flags().StringVar(&extractModel, "extract-model", "", "model for extraction (default: from config)")
	cmd.Flags().IntVar(&concurrency, "concurrency", 0, "parallel extraction workers (default: from config)")
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

func createProvider(name string, client *apiclient.Client, apiKey, modelName string) (provider.Provider, error) {
	switch name {
	case "gemini":
		return provider.NewGeminiProvider(client, apiKey, modelName), nil
	default:
		return nil, fmt.Errorf("provider %q is not yet implemented", name)
	}
}
