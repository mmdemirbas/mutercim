package cli

import (
	"fmt"
	"log/slog"

	"github.com/mmdemirbas/mutercim/internal/config"
	"github.com/mmdemirbas/mutercim/internal/display"
	"github.com/mmdemirbas/mutercim/internal/docker"
	"github.com/mmdemirbas/mutercim/internal/model"
	"github.com/mmdemirbas/mutercim/internal/pipeline"
	"github.com/mmdemirbas/mutercim/internal/workspace"
	"github.com/spf13/cobra"
)

func newLayoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "layout",
		Short: "(Phase L) Detect document layout regions on page images",
		Long:  "Runs layout detection (e.g. DocLayout-YOLO) on page images and writes per-page region JSON to layout/. Requires Docker.",
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
			applyOutputDir(ws, cfg)

			// Preflight: check Docker (layout tools run in containers)
			if err := docker.CheckAvailable(cmd.Context()); err != nil {
				return err
			}

			// Determine page range: CLI flag > all
			pageSpec := pages

			var pagesToProcess []int
			if pageSpec != "" && pageSpec != "all" {
				ranges, err := model.ParsePageRanges(pageSpec)
				if err != nil {
					return fmt.Errorf("parse pages: %w", err)
				}
				pagesToProcess, err = model.ExpandPages(ranges)
				if err != nil {
					return fmt.Errorf("expand pages: %w", err)
				}
			}

			// Auto-run prerequisites if needed
			if auto {
				if err := runPrerequisites(cmd.Context(), phaseLayout, ws, cfg, pagesToProcess, display.FromContext(cmd.Context())); err != nil {
					return err
				}
			}

			logger := slog.Default()

			_, err = pipeline.Layout(cmd.Context(), pipeline.LayoutOptions{
				Workspace: ws,
				Config:    cfg,
				Pages:     pagesToProcess,
				Force:     force,
				Logger:    logger,
				Display:   display.FromContext(cmd.Context()),
			})
			return err
		},
	}
}
