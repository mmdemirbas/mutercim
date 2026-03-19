package cli

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/mmdemirbas/mutercim/internal/apiclient"
	"github.com/mmdemirbas/mutercim/internal/config"
	"github.com/mmdemirbas/mutercim/internal/input"
	"github.com/mmdemirbas/mutercim/internal/knowledge"
	"github.com/mmdemirbas/mutercim/internal/model"
	"github.com/mmdemirbas/mutercim/internal/pipeline"
	"github.com/mmdemirbas/mutercim/internal/progress"
	"github.com/mmdemirbas/mutercim/internal/renderer"
	"github.com/mmdemirbas/mutercim/internal/workspace"
	"github.com/spf13/cobra"
)

func newRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run",
		Short: "Run all phases: extract → enrich → translate → compile",
		Long:  "Executes the full pipeline sequentially. Validates system dependencies before starting.",
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

			// Preflight: check all dependencies upfront
			for _, inp := range cfg.Inputs {
				resolved := cfg.ResolvePath(ws.Root, inp)
				if config.IsPDF(resolved) {
					if err := input.CheckPdftoppm(); err != nil {
						return err
					}
					break
				}
			}
			for _, f := range cfg.Compile.Formats {
				switch f {
				case "latex":
					if !cfg.Compile.SkipPDF {
						if err := renderer.CheckDocker(); err != nil {
							return err
						}
					}
				case "docx":
					if err := renderer.CheckPandoc(); err != nil {
						return err
					}
				}
			}

			logger := slog.Default()

			// Resolve page range
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

			// Phase 1: Extract
			logger.Info("=== Phase 1: EXTRACT ===")
			extractAPIKey, err := resolveAPIKey(cfg.Extract.Provider)
			if err != nil {
				return fmt.Errorf("extract API key: %w", err)
			}
			extractClient := apiclient.NewClient(apiclient.ClientConfig{
				Timeout:           clientTimeout(cfg.Extract.Provider),
				MaxRetries:        cfg.Retry.MaxAttempts,
				BaseBackoff:       time.Duration(cfg.Retry.BackoffSeconds) * time.Second,
				RequestsPerMinute: cfg.RateLimit.RequestsPerMinute,
			}, logger)
			defer extractClient.Close()

			extractProvider, err := createProvider(cfg.Extract.Provider, extractClient, extractAPIKey, cfg.Extract.Model)
			if err != nil {
				return fmt.Errorf("create extract provider: %w", err)
			}

			if err := pipeline.Extract(cmd.Context(), pipeline.ExtractOptions{
				Workspace: ws,
				Config:    cfg,
				Provider:  extractProvider,
				Tracker:   tracker,
				Pages:     pagesToProcess,
				Logger:    logger,
			}); err != nil {
				return fmt.Errorf("extract: %w", err)
			}

			// Phase 2: Enrich
			logger.Info("=== Phase 2: ENRICH ===")
			knowledgeDir := cfg.ResolvePath(ws.Root, cfg.Knowledge.Dir)
			k, err := knowledge.Load(knowledgeDir, ws.StagedDir())
			if err != nil {
				return fmt.Errorf("load knowledge: %w", err)
			}

			if err := pipeline.Enrich(cmd.Context(), pipeline.EnrichOptions{
				Workspace: ws,
				Knowledge: k,
				Tracker:   tracker,
				Pages:     pagesToProcess,
				Logger:    logger,
			}); err != nil {
				return fmt.Errorf("enrich: %w", err)
			}

			// Phase 3: Translate
			logger.Info("=== Phase 3: TRANSLATE ===")
			translateAPIKey, err := resolveAPIKey(cfg.Translate.Provider)
			if err != nil {
				return fmt.Errorf("translate API key: %w", err)
			}
			translateClient := apiclient.NewClient(apiclient.ClientConfig{
				Timeout:           clientTimeout(cfg.Translate.Provider),
				MaxRetries:        cfg.Retry.MaxAttempts,
				BaseBackoff:       time.Duration(cfg.Retry.BackoffSeconds) * time.Second,
				RequestsPerMinute: cfg.RateLimit.RequestsPerMinute,
			}, logger)
			defer translateClient.Close()

			translateProvider, err := createProvider(cfg.Translate.Provider, translateClient, translateAPIKey, cfg.Translate.Model)
			if err != nil {
				return fmt.Errorf("create translate provider: %w", err)
			}

			if err := pipeline.Translate(cmd.Context(), pipeline.TranslateOptions{
				Workspace: ws,
				Config:    cfg,
				Provider:  translateProvider,
				Knowledge: k,
				Tracker:   tracker,
				Pages:     pagesToProcess,
				Logger:    logger,
			}); err != nil {
				return fmt.Errorf("translate: %w", err)
			}

			// Phase 4: Compile
			logger.Info("=== Phase 4: COMPILE ===")
			if err := pipeline.Compile(cmd.Context(), pipeline.CompileOptions{
				Workspace: ws,
				Config:    cfg,
				Tracker:   tracker,
				Pages:     pagesToProcess,
				Logger:    logger,
			}); err != nil {
				return fmt.Errorf("compile: %w", err)
			}

			logger.Info("=== All phases complete ===")
			return nil
		},
	}
}
