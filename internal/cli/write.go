package cli

import (
	"fmt"
	"strings"

	"github.com/mmdemirbas/mutercim/internal/config"
	"github.com/mmdemirbas/mutercim/internal/display"
	"github.com/mmdemirbas/mutercim/internal/model"
	"github.com/mmdemirbas/mutercim/internal/pipeline"
	"github.com/mmdemirbas/mutercim/internal/progress"
	"github.com/mmdemirbas/mutercim/internal/renderer"
	"github.com/mmdemirbas/mutercim/internal/workspace"
	"github.com/spf13/cobra"
)

func newWriteCmd() *cobra.Command {
	var (
		formats          string
		latexDockerImage string
	)

	cmd := &cobra.Command{
		Use:   "write",
		Short: "(Phase 4) Write translated pages into final output",
		Long:  "Generates output files from translated JSON. Supported formats: md, latex, pdf, docx.",
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
			if formats != "" {
				cfg.Write.Formats = strings.Split(formats, ",")
			}
			if latexDockerImage != "" {
				cfg.Write.LaTeXDockerImage = latexDockerImage
			}
			// Preflight checks
			for _, f := range cfg.Write.Formats {
				switch f {
				case "pdf":
					if err := renderer.CheckDocker(); err != nil {
						return err
					}
				case "docx":
					if err := renderer.CheckPandoc(); err != nil {
						return err
					}
				}
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
				if err := runPrerequisites(cmd.Context(), phaseWrite, ws, cfg, pagesToProcess, display.FromContext(cmd.Context())); err != nil {
					return err
				}
			}

			return pipeline.Write(cmd.Context(), pipeline.WriteOptions{
				Workspace: ws,
				Config:    cfg,
				Tracker:   tracker,
				Pages:     pagesToProcess,
				Display:   display.FromContext(cmd.Context()),
			})
		},
	}

	cmd.Flags().StringVar(&formats, "format", "", "output formats, comma-separated: md,latex,pdf,docx (default: from config)")
	cmd.Flags().StringVar(&latexDockerImage, "latex-docker-image", "", "Docker image for LaTeX compilation (default: from config)")

	return cmd
}
