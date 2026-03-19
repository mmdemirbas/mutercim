package cli

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

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

			logger := slog.Default()
			disp := display.FromContext(cmd.Context())
			if disp != nil {
				defer disp.Finish()
			}

			// Resolve page range (from --pages CLI flag)
			pageSpec := pages

			// Set header on live display
			if disp != nil {
				inputName := ""
				if len(cfg.Inputs) > 0 {
					inputName = filepath.Base(cfg.Inputs[0].Path)
					if len(cfg.Inputs) > 1 {
						inputName += fmt.Sprintf(" (+%d more)", len(cfg.Inputs)-1)
					}
				}
				disp.SetHeader(display.HeaderData{
					BookTitle:   cfg.Book.Title,
					BookAuthor:  cfg.Book.Author,
					InputName:   inputName,
					PageRange:   pageSpec,
					SourceLangs: cfg.Book.SourceLangs,
					TargetLangs: cfg.Book.TargetLangs,
				})
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
			readChain, err := createProviderChain(cfg.Read.Models, cfg.Retry, logger)
			if err != nil {
				return fmt.Errorf("create read providers: %w", err)
			}
			defer readChain.Close()

			readResult, err := pipeline.Read(cmd.Context(), pipeline.ReadOptions{
				Workspace: ws,
				Config:    cfg,
				Provider:  readChain,
				Tracker:   tracker,
				Pages:     pagesToProcess,
				Logger:    logger,
				Display:   disp,
			})
			if err != nil {
				return fmt.Errorf("read: %w", err)
			}
			if readResult.Completed == 0 {
				logger.Error("stopping pipeline: read produced 0 pages")
				return fmt.Errorf("stopping pipeline: read produced 0 pages")
			}

			// Phase 2: Solve
			logger.Info("=== Phase 2: SOLVE ===")
			knowledgeDir := cfg.ResolvePath(ws.Root, cfg.KnowledgeDir)
			k, err := knowledge.Load(knowledgeDir, ws.StagedDir())
			if err != nil {
				return fmt.Errorf("load knowledge: %w", err)
			}

			solveResult, err := pipeline.Solve(cmd.Context(), pipeline.SolveOptions{
				Workspace: ws,
				Knowledge: k,
				Tracker:   tracker,
				Pages:     pagesToProcess,
				Logger:    logger,
				Display:   disp,
			})
			if err != nil {
				return fmt.Errorf("solve: %w", err)
			}
			if solveResult.Completed == 0 {
				logger.Error("stopping pipeline: solve produced 0 pages")
				return fmt.Errorf("stopping pipeline: solve produced 0 pages")
			}

			// Phase 3: Translate
			logger.Info("=== Phase 3: TRANSLATE ===")
			translateChain, err := createProviderChain(cfg.Translate.Models, cfg.Retry, logger)
			if err != nil {
				return fmt.Errorf("create translate providers: %w", err)
			}
			defer translateChain.Close()

			translateResult, err := pipeline.Translate(cmd.Context(), pipeline.TranslateOptions{
				Workspace: ws,
				Config:    cfg,
				Provider:  translateChain,
				Knowledge: k,
				Tracker:   tracker,
				Pages:     pagesToProcess,
				Logger:    logger,
				Display:   disp,
			})
			if err != nil {
				return fmt.Errorf("translate: %w", err)
			}
			if translateResult.Completed == 0 {
				logger.Error("stopping pipeline: translate produced 0 pages")
				return fmt.Errorf("stopping pipeline: translate produced 0 pages")
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

			// Print completion summary to console
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, "Done.")
			fmt.Fprintf(os.Stderr, "  Read:      %d pages (%d failed)\n", readResult.Completed, readResult.Failed)
			fmt.Fprintf(os.Stderr, "  Solve:     %d pages (%d failed)\n", solveResult.Completed, solveResult.Failed)
			fmt.Fprintf(os.Stderr, "  Translate: %d pages (%d failed)\n", translateResult.Completed, translateResult.Failed)
			fmt.Fprintf(os.Stderr, "  Output:    %s\n", filepath.Join(ws.Root, cfg.Output))
			fmt.Fprintln(os.Stderr)
			return nil
		},
	}
}
