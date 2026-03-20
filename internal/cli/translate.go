package cli

import (
	"fmt"
	"log/slog"

	"github.com/mmdemirbas/mutercim/internal/config"
	"github.com/mmdemirbas/mutercim/internal/display"
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
		Short: "(Phase 3) Translate solved pages into target languages",
		Long:  "Sends solved page data to an AI model with knowledge-injected prompts and saves translated JSON.",
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
			if translateProvider != "" || translateModel != "" {
				p := translateProvider
				if p == "" && len(cfg.Translate.Models) > 0 {
					p = cfg.Translate.Models[0].Provider
				}
				m := translateModel
				if m == "" && len(cfg.Translate.Models) > 0 {
					m = cfg.Translate.Models[0].Model
				}
				cfg.Translate.Models = []config.ModelSpec{{Provider: p, Model: m}}
			}

			logger := slog.Default()

			chain, err := createProviderChain(cfg.Translate.Models, cfg.Retry, logger)
			if err != nil {
				return err
			}
			defer chain.Close()

			// Load knowledge
			knowledgeDir := cfg.ResolvePath(ws.Root, cfg.KnowledgeDir)
			k, err := knowledge.Load(knowledgeDir, ws.MemoryDir())
			if err != nil {
				return fmt.Errorf("load knowledge: %w", err)
			}

			// Determine page range
			pageSpec := pages
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

			// Auto-run prerequisites if needed
			if auto {
				if err := runPrerequisites(cmd.Context(), phaseTranslate, ws, cfg, pagesToProcess, display.FromContext(cmd.Context())); err != nil {
					return err
				}
			}

			_, err = pipeline.Translate(cmd.Context(), pipeline.TranslateOptions{
				Workspace:     ws,
				Config:        cfg,
				Provider:      chain,
				Knowledge:     k,
				Tracker:       tracker,
				Pages:         pagesToProcess,
				Force:         force,
				ContextWindow: contextWindow,
				Logger:        logger,
				Display:       display.FromContext(cmd.Context()),
			})
			return err
		},
	}

	cmd.Flags().StringVar(&translateProvider, "translate-provider", "", "provider: gemini, claude, openai, groq, mistral, openrouter, xai, ollama (default: from config)")
	cmd.Flags().StringVar(&translateModel, "translate-model", "", "model for translation (default: from config)")
	cmd.Flags().IntVar(&contextWindow, "context-window", 0, "number of previous pages for context (default: from config)")

	return cmd
}
