package cli

import (
	"fmt"
	"log/slog"

	"github.com/mmdemirbas/mutercim/internal/config"
	"github.com/mmdemirbas/mutercim/internal/display"
	"github.com/mmdemirbas/mutercim/internal/docker"
	"github.com/mmdemirbas/mutercim/internal/model"
	"github.com/mmdemirbas/mutercim/internal/ocr"
	"github.com/mmdemirbas/mutercim/internal/pipeline"
	"github.com/mmdemirbas/mutercim/internal/workspace"
	"github.com/spf13/cobra"
)

//nolint:cyclop,gocognit // cobra command with pipeline dispatch and flag wiring
func newOCRCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ocr",
		Short: "(Phase 3) Extract text from page images using OCR",
		Long:  "Runs OCR on page images using layout regions if available. Requires Docker for the Qari-OCR tool.",
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

			if cfg.OCR.Tool == "" {
				fmt.Println("OCR is disabled (ocr.tool not configured). The read phase will use vision LLM for text extraction.")
				return nil
			}

			// Preflight: check Docker (OCR tools run in containers)
			if err := docker.CheckAvailable(cmd.Context()); err != nil {
				return err
			}

			// Determine page range: CLI flag > all
			pageSpec := pages

			var pagesToProcess []int
			if pageSpec != "" && pageSpec != "all" {
				ranges, err := model.ParsePageRanges(pageSpec)
				if err != nil {
					return fmt.Errorf("parse pages %q: %w", pageSpec, err)
				}
				pagesToProcess, err = model.ExpandPages(ranges)
				if err != nil {
					return fmt.Errorf("expand pages: %w", err)
				}
			}

			// Auto-run prerequisites if needed
			if auto {
				if err := runPrerequisites(cmd.Context(), phaseOCR, ws, cfg, pagesToProcess, display.FromContext(cmd.Context())); err != nil {
					return err
				}
			}

			logger := slog.Default()

			// Create OCR tool
			tool := ocr.NewTool(cfg.OCR.Tool)
			if tool == nil {
				return fmt.Errorf("unknown OCR tool: %q", cfg.OCR.Tool)
			}
			_, err = pipeline.OCR(cmd.Context(), pipeline.OCROptions{
				Workspace: ws,
				Config:    cfg,
				Tool:      tool,
				Pages:     pagesToProcess,
				Force:     force,
				Logger:    logger,
				Display:   display.FromContext(cmd.Context()),
			})
			return err
		},
	}

	return cmd
}
