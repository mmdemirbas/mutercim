package cli

import (
	"fmt"
	"strings"

	"github.com/mmdemirbas/mutercim/internal/config"
	"github.com/mmdemirbas/mutercim/internal/display"
	"github.com/mmdemirbas/mutercim/internal/model"
	"github.com/mmdemirbas/mutercim/internal/pipeline"
	"github.com/mmdemirbas/mutercim/internal/workspace"
	"github.com/spf13/cobra"
)

func newWriteCmd() *cobra.Command {
	var (
		formats string
	)

	cmd := &cobra.Command{
		Use:       "write [formats...]",
		Short:     "(Phase 4) Write translated pages into final output",
		Long:      "Generates output files from translated JSON. Supported formats: md, latex (tex), pdf, docx.\n\nFormats can be specified as positional arguments to override the config:\n  mutercim write pdf\n  mutercim write md docx",
		ValidArgs: []string{"md", "latex", "tex", "pdf", "docx"},
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

			// Apply output directory override
			applyOutputDir(ws, cfg)

			// Apply format overrides: positional args > --format flag > config
			if len(args) > 0 {
				fmts, err := normalizeFormats(args)
				if err != nil {
					return err
				}
				cfg.Write.Formats = fmts
			} else if formats != "" {
				fmts, err := normalizeFormats(strings.Split(formats, ","))
				if err != nil {
					return err
				}
				cfg.Write.Formats = fmts
			}
			// Determine page range
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
				if err := runPrerequisites(cmd.Context(), phaseWrite, ws, cfg, pagesToProcess, display.FromContext(cmd.Context())); err != nil {
					return err
				}
			}

			return pipeline.Write(cmd.Context(), pipeline.WriteOptions{
				Workspace: ws,
				Config:    cfg,
				Pages:     pagesToProcess,
				Force:     force,
				Display:   display.FromContext(cmd.Context()),
			})
		},
	}

	cmd.Flags().StringVar(&formats, "format", "", "output formats, comma-separated: md,latex,pdf,docx (overridden by positional args)")

	return cmd
}
