package cli

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/mmdemirbas/mutercim/internal/apiclient"
	"github.com/mmdemirbas/mutercim/internal/config"
	"github.com/mmdemirbas/mutercim/internal/knowledge"
	"github.com/mmdemirbas/mutercim/internal/model"
	"github.com/mmdemirbas/mutercim/internal/pipeline"
	"github.com/mmdemirbas/mutercim/internal/progress"
	"github.com/mmdemirbas/mutercim/internal/workspace"
	"github.com/spf13/cobra"
)

func newTranslateCmd() *cobra.Command {
	var (
		translateProvider string
		translateModel    string
		contextWindow     int
	)

	cmd := &cobra.Command{
		Use:   "translate",
		Short: "Translate enriched pages into Turkish (Phase 3)",
		Long:  "Sends enriched page data to an AI model with knowledge-injected prompts and saves translated JSON.",
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

			// Apply CLI flag overrides
			if translateProvider != "" {
				cfg.Translate.Provider = translateProvider
			}
			if translateModel != "" {
				cfg.Translate.Model = translateModel
			}

			// Resolve API key
			apiKey, err := resolveAPIKey(cfg.Translate.Provider)
			if err != nil {
				return err
			}

			// Create API client
			clientCfg := apiclient.ClientConfig{
				Timeout:           clientTimeout(cfg.Translate.Provider),
				MaxRetries:        cfg.Retry.MaxAttempts,
				BaseBackoff:       time.Duration(cfg.Retry.BackoffSeconds) * time.Second,
				RequestsPerMinute: cfg.RateLimit.RequestsPerMinute,
			}
			logger := slog.Default()
			client := apiclient.NewClient(clientCfg, logger)
			defer client.Close()

			// Create provider
			p, err := createProvider(cfg.Translate.Provider, client, apiKey, cfg.Translate.Model)
			if err != nil {
				return err
			}

			// Load knowledge
			knowledgeDir := cfg.ResolvePath(ws.Root, cfg.Knowledge.Dir)
			k, err := knowledge.Load(knowledgeDir, ws.StagedDir())
			if err != nil {
				return fmt.Errorf("load knowledge: %w", err)
			}

			// Determine page range
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

			tracker := progress.NewTracker(ws.ProgressPath())
			if err := tracker.Load(); err != nil {
				return fmt.Errorf("load progress: %w", err)
			}

			return pipeline.Translate(cmd.Context(), pipeline.TranslateOptions{
				Workspace:     ws,
				Config:        cfg,
				Provider:      p,
				Knowledge:     k,
				Tracker:       tracker,
				Pages:         pagesToProcess,
				ContextWindow: contextWindow,
				Logger:        logger,
			})
		},
	}

	cmd.Flags().StringVar(&translateProvider, "translate-provider", "", "provider: gemini, claude, openai, ollama (default: from config)")
	cmd.Flags().StringVar(&translateModel, "translate-model", "", "model for translation (default: from config)")
	cmd.Flags().IntVar(&contextWindow, "context-window", 0, "number of previous pages for context (default: from config)")

	return cmd
}
