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

//nolint:cyclop,gocognit // cobra command with pipeline dispatch and flag wiring
func newCutCmd() *cobra.Command {
	var dpi int

	cmd := &cobra.Command{
		Use:   "cut",
		Short: "(Phase 1) Convert PDF inputs to per-page images",
		Long:  "Converts PDF files to per-page PNG images in cut/. No-op if the input is already a directory of images.",
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

			if dpi > 0 {
				cfg.Cut.DPI = dpi
			}

			// Preflight: check Docker if any input is PDF
			for _, inp := range cfg.Inputs {
				resolved := cfg.ResolvePath(ws.Root, inp.Path)
				if config.IsPDF(resolved) {
					if err := docker.CheckAvailable(cmd.Context()); err != nil {
						return err
					}
					break
				}
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

			logger := slog.Default()

			return pipeline.Cut(cmd.Context(), pipeline.CutOptions{
				Workspace: ws,
				Config:    cfg,
				Pages:     pagesToProcess,
				Force:     force,
				Logger:    logger,
				Display:   display.FromContext(cmd.Context()),
			})
		},
	}

	cmd.Flags().IntVarP(&dpi, "dpi", "d", 0, "DPI for PDF-to-image conversion (default: from config)")

	return cmd
}
