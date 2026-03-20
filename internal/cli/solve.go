package cli

import (
	"fmt"

	"github.com/mmdemirbas/mutercim/internal/config"
	"github.com/mmdemirbas/mutercim/internal/display"
	"github.com/mmdemirbas/mutercim/internal/knowledge"
	"github.com/mmdemirbas/mutercim/internal/model"
	"github.com/mmdemirbas/mutercim/internal/pipeline"
	"github.com/mmdemirbas/mutercim/internal/workspace"
	"github.com/spf13/cobra"
)

func newSolveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "solve",
		Short: "(Phase 2) Solve read pages with knowledge resolution",
		Long:  "Resolves source abbreviations, detects cross-page continuations, validates structure, and injects translation context.",
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

			// Load knowledge from all three layers
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

			// Auto-run prerequisites if needed
			if auto {
				if err := runPrerequisites(cmd.Context(), phaseSolve, ws, cfg, pagesToProcess, display.FromContext(cmd.Context())); err != nil {
					return err
				}
			}

			_, err = pipeline.Solve(cmd.Context(), pipeline.SolveOptions{
				Workspace: ws,
				Knowledge: k,
				Pages:     pagesToProcess,
				Force:     force,
				Display:   display.FromContext(cmd.Context()),
			})
			return err
		},
	}
}
