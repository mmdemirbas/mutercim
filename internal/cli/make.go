package cli

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/mmdemirbas/mutercim/internal/apiclient"
	"github.com/mmdemirbas/mutercim/internal/config"
	"github.com/mmdemirbas/mutercim/internal/display"
	"github.com/mmdemirbas/mutercim/internal/input"
	"github.com/mmdemirbas/mutercim/internal/knowledge"
	"github.com/mmdemirbas/mutercim/internal/model"
	"github.com/mmdemirbas/mutercim/internal/pipeline"
	"github.com/mmdemirbas/mutercim/internal/progress"
	"github.com/mmdemirbas/mutercim/internal/renderer"
	"github.com/mmdemirbas/mutercim/internal/workspace"
	"github.com/spf13/cobra"
)

func newMakeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "make",
		Short: "Run all phases: pages -> read -> solve -> translate -> write",
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
				resolved := cfg.ResolvePath(ws.Root, inp.Path)
				if config.IsPDF(resolved) {
					if err := input.CheckPdftoppm(); err != nil {
						return err
					}
					break
				}
			}
			for _, f := range cfg.Write.Formats {
				switch f {
				case "latex":
					if !cfg.Write.SkipPDF {
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
			disp := display.FromContext(cmd.Context())
			if disp != nil {
				defer disp.Finish()
			}

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

			// Phase 0: Pages (PDF → images)
			logger.Info("=== PAGES ===")
			if err := pipeline.Pages(cmd.Context(), pipeline.PagesOptions{
				Workspace: ws,
				Config:    cfg,
				Pages:     pagesToProcess,
				Logger:    logger,
				Display:   disp,
			}); err != nil {
				return fmt.Errorf("pages: %w", err)
			}

			// Phase 1: Read
			logger.Info("=== Phase 1: READ ===")
			readAPIKey, err := resolveAPIKey(cfg.Read.Provider)
			if err != nil {
				return fmt.Errorf("read API key: %w", err)
			}
			readClient := apiclient.NewClient(apiclient.ClientConfig{
				Timeout:           clientTimeout(cfg.Read.Provider),
				MaxRetries:        cfg.Retry.MaxAttempts,
				BaseBackoff:       time.Duration(cfg.Retry.BackoffSeconds) * time.Second,
				RequestsPerMinute: cfg.RateLimit.RequestsPerMinute,
			}, logger)
			defer readClient.Close()

			readProvider, err := createProvider(cfg.Read.Provider, readClient, readAPIKey, cfg.Read.Model)
			if err != nil {
				return fmt.Errorf("create read provider: %w", err)
			}

			if err := pipeline.Read(cmd.Context(), pipeline.ReadOptions{
				Workspace: ws,
				Config:    cfg,
				Provider:  readProvider,
				Tracker:   tracker,
				Pages:     pagesToProcess,
				Logger:    logger,
				Display:   disp,
			}); err != nil {
				return fmt.Errorf("read: %w", err)
			}

			// Phase 2: Solve
			logger.Info("=== Phase 2: SOLVE ===")
			knowledgeDir := cfg.ResolvePath(ws.Root, cfg.Knowledge.Dir)
			k, err := knowledge.Load(knowledgeDir, ws.StagedDir())
			if err != nil {
				return fmt.Errorf("load knowledge: %w", err)
			}

			if err := pipeline.Solve(cmd.Context(), pipeline.SolveOptions{
				Workspace: ws,
				Knowledge: k,
				Tracker:   tracker,
				Pages:     pagesToProcess,
				Logger:    logger,
				Display:   disp,
			}); err != nil {
				return fmt.Errorf("solve: %w", err)
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
				Display:   disp,
			}); err != nil {
				return fmt.Errorf("translate: %w", err)
			}

			// Phase 4: Write
			logger.Info("=== Phase 4: WRITE ===")
			if err := pipeline.Write(cmd.Context(), pipeline.WriteOptions{
				Workspace: ws,
				Config:    cfg,
				Tracker:   tracker,
				Pages:     pagesToProcess,
				Logger:    logger,
				Display:   disp,
			}); err != nil {
				return fmt.Errorf("write: %w", err)
			}

			logger.Info("=== All phases complete ===")
			return nil
		},
	}
}
