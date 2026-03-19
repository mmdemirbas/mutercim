package cli

import (
	"fmt"
	"log/slog"

	"github.com/mmdemirbas/mutercim/internal/config"
	"github.com/mmdemirbas/mutercim/internal/display"
	"github.com/mmdemirbas/mutercim/internal/input"
	"github.com/mmdemirbas/mutercim/internal/model"
	"github.com/mmdemirbas/mutercim/internal/pipeline"
	"github.com/mmdemirbas/mutercim/internal/workspace"
	"github.com/spf13/cobra"
)

func newPagesCmd() *cobra.Command {
	var dpi int

	cmd := &cobra.Command{
		Use:   "pages",
		Short: "Convert PDF inputs to per-page images",
		Long:  "Converts PDF files to per-page PNG images in midstate/images/. No-op if the input is already a directory of images.",
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

			logger := slog.Default()

			return pipeline.Pages(cmd.Context(), pipeline.PagesOptions{
				Workspace: ws,
				Config:    cfg,
				Pages:     pagesToProcess,
				Logger:    logger,
				Display:   display.FromContext(cmd.Context()),
			})
		},
	}

	cmd.Flags().IntVar(&dpi, "dpi", 0, "DPI for PDF-to-image conversion (default: from config)")

	return cmd
}
