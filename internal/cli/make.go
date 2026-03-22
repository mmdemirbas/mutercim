package cli

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/mmdemirbas/mutercim/internal/config"
	"github.com/mmdemirbas/mutercim/internal/display"
	"github.com/mmdemirbas/mutercim/internal/docker"
	"github.com/mmdemirbas/mutercim/internal/knowledge"
	"github.com/mmdemirbas/mutercim/internal/model"
	"github.com/mmdemirbas/mutercim/internal/pipeline"
	"github.com/mmdemirbas/mutercim/internal/workspace"
	"github.com/spf13/cobra"
)

func newAllCmd() *cobra.Command {
	return &cobra.Command{
		Use:       "all [formats...]",
		Short:     "(Phase *) Run all phases: cut -> layout -> read -> solve -> translate -> write",
		Long:      "Executes the full pipeline sequentially. Validates system dependencies before starting.\n\nOptional format arguments override the write phase output formats:\n  mutercim all pdf\n  mutercim all md docx",
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

			applyOutputDir(ws, cfg)

			// Override write formats if positional args provided
			if len(args) > 0 {
				fmts, err := normalizeFormats(args)
				if err != nil {
					return err
				}
				cfg.Write.Formats = fmts
			}

			// Preflight: check Docker upfront (all external tools run in containers)
			if err := docker.CheckAvailable(cmd.Context()); err != nil {
				return err
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
					InputName:   inputName,
					PageRange:   pageSpec,
					SourceLangs: cfg.SourceLanguages(),
					TargetLangs: cfg.Translate.Languages,
				})
			}
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

			// Phase 1: Cut (PDF → images)
			logger.Info("=== Phase 1: CUT ===")
			if err := pipeline.Cut(cmd.Context(), pipeline.CutOptions{
				Workspace: ws,
				Config:    cfg,
				Pages:     pagesToProcess,
				Force:     force,
				Logger:    logger,
				Display:   disp,
			}); err != nil {
				return fmt.Errorf("cut: %w", err)
			}

			// Phase 2: Layout
			if cfg.Layout.Tool == "" {
				logger.Info("layout tool disabled, skipping layout phase")
			} else {
				logger.Info("=== Phase 2: LAYOUT ===")
				if _, err := pipeline.Layout(cmd.Context(), pipeline.LayoutOptions{
					Workspace: ws,
					Config:    cfg,
					Pages:     pagesToProcess,
					Force:     force,
					Logger:    logger,
					Display:   disp,
				}); err != nil {
					return fmt.Errorf("layout: %w", err)
				}
			}

			// Phase 3: Read
			logger.Info("=== Phase 3: READ ===")
			readChain, err := createProviderChain(cfg.Read.Models, cfg.Read.Retry, logger)
			if err != nil {
				return fmt.Errorf("create read providers: %w", err)
			}
			defer readChain.Close()

			readResult, err := pipeline.Read(cmd.Context(), pipeline.ReadOptions{
				Workspace: ws,
				Config:    cfg,
				Provider:  readChain,
				Pages:     pagesToProcess,
				Force:     force,
				Logger:    logger,
				Display:   disp,
			})
			if err != nil {
				return fmt.Errorf("read: %w", err)
			}
			if readResult.IsEmpty() {
				logger.Error("stopping pipeline: read produced 0 pages")
				return fmt.Errorf("stopping pipeline: read produced 0 pages")
			}

			// Phase 4: Solve
			logger.Info("=== Phase 4: SOLVE ===")
			k, err := knowledge.Load(cfg.ResolveKnowledgePaths(ws.Root), ws.MemoryDir())
			if err != nil {
				return fmt.Errorf("load knowledge: %w", err)
			}

			sourceLang := cfg.PrimarySourceLang()

			solveResult, err := pipeline.Solve(cmd.Context(), pipeline.SolveOptions{
				Workspace:      ws,
				Knowledge:      k,
				KnowledgePaths: cfg.ResolveKnowledgePaths(ws.Root),
				SourceLang:     sourceLang,
				Pages:          pagesToProcess,
				Force:          force,
				Logger:         logger,
				Display:        disp,
			})
			if err != nil {
				return fmt.Errorf("solve: %w", err)
			}
			if solveResult.IsEmpty() {
				logger.Error("stopping pipeline: solve produced 0 pages")
				return fmt.Errorf("stopping pipeline: solve produced 0 pages")
			}

			// Phase 5: Translate
			logger.Info("=== Phase 5: TRANSLATE ===")
			translateChain, err := createProviderChain(cfg.Translate.Models, cfg.Translate.Retry, logger)
			if err != nil {
				return fmt.Errorf("create translate providers: %w", err)
			}
			defer translateChain.Close()

			translateResult, err := pipeline.Translate(cmd.Context(), pipeline.TranslateOptions{
				Workspace: ws,
				Config:    cfg,
				Provider:  translateChain,
				Knowledge: k,
				Pages:     pagesToProcess,
				Force:     force,
				Logger:    logger,
				Display:   disp,
			})
			if err != nil {
				return fmt.Errorf("translate: %w", err)
			}
			if translateResult.IsEmpty() {
				logger.Error("stopping pipeline: translate produced 0 pages")
				return fmt.Errorf("stopping pipeline: translate produced 0 pages")
			}

			// Phase 6: Write
			logger.Info("=== Phase 6: WRITE ===")
			if err := pipeline.Write(cmd.Context(), pipeline.WriteOptions{
				Workspace: ws,
				Config:    cfg,
				Pages:     pagesToProcess,
				Force:     force,
				Logger:    logger,
				Display:   disp,
			}); err != nil {
				return fmt.Errorf("write: %w", err)
			}

			logger.Info("=== All phases complete ===")

			// Print completion summary to console
			colors := display.NewStatusColors(os.Stderr)
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, colors.Green("\u2713 Done."))
			printPhaseSummary(os.Stderr, colors, "Read", readResult)
			printPhaseSummary(os.Stderr, colors, "Solve", solveResult)
			printPhaseSummary(os.Stderr, colors, "Translate", translateResult)
			fmt.Fprintf(os.Stderr, "  %s %s\n", colors.Cyan("Output:"), ws.WriteDir())
			fmt.Fprintln(os.Stderr)
			return nil
		},
	}
}

func printPhaseSummary(w io.Writer, colors display.StatusColors, name string, result pipeline.PhaseResult) {
	detail := fmt.Sprintf("%d pages", result.Completed)
	if result.Failed > 0 {
		detail += colors.Red(fmt.Sprintf(" (%d failed)", result.Failed))
	}
	fmt.Fprintf(w, "  %-12s %s\n", colors.Cyan(name+":"), detail)
}
