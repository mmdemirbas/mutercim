package cli

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"sort"
	"strings"

	"github.com/mmdemirbas/mutercim/internal/config"
	"github.com/mmdemirbas/mutercim/internal/display"
	"github.com/mmdemirbas/mutercim/internal/docker"
	"github.com/mmdemirbas/mutercim/internal/knowledge"
	"github.com/mmdemirbas/mutercim/internal/model"
	"github.com/mmdemirbas/mutercim/internal/ocr"
	"github.com/mmdemirbas/mutercim/internal/pipeline"
	"github.com/mmdemirbas/mutercim/internal/workspace"
	"github.com/spf13/cobra"
)

//nolint:cyclop,gocognit,funlen // full pipeline orchestration in a single cobra command
func newAllCmd() *cobra.Command {
	return &cobra.Command{
		Use:       "all [formats...]",
		Short:     "(Phase *) Run all phases: cut -> layout -> ocr -> read -> solve -> translate -> write",
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
				disp.SetHeader(display.HeaderData{
					PageRange:    pageSpec,
					LogLevel:     logLevel,
					OutputDir:    ws.OutputDir,
					Inputs:       resolveInputPaths(ws, cfg),
					Knowledge:    cfg.ResolveKnowledgePaths(ws.Root),
					PhaseConfigs: buildPhaseConfigs(cfg),
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

			// Phase 3: OCR
			if cfg.OCR.Tool == "" {
				logger.Info("ocr tool disabled, skipping ocr phase")
			} else {
				logger.Info("=== Phase 3: OCR ===")
				ocrTool := ocr.NewTool(cfg.OCR.Tool)
				if ocrTool == nil {
					return fmt.Errorf("unknown OCR tool: %q", cfg.OCR.Tool)
				}
				defer func() {
					if err := ocrTool.Stop(cmd.Context()); err != nil {
						logger.Warn("failed to stop ocr container", "error", err)
					}
				}()
				if _, err := pipeline.OCR(cmd.Context(), pipeline.OCROptions{
					Workspace: ws,
					Config:    cfg,
					Tool:      ocrTool,
					Pages:     pagesToProcess,
					Force:     force,
					Logger:    logger,
					Display:   disp,
				}); err != nil {
					return fmt.Errorf("ocr: %w", err)
				}
			}

			// Phase 4: Read
			logger.Info("=== Phase 4: READ ===")
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

			// Phase 5: Solve
			logger.Info("=== Phase 5: SOLVE ===")
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

			// Phase 6: Translate
			logger.Info("=== Phase 6: TRANSLATE ===")
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

			// Phase 7: Write
			logger.Info("=== Phase 7: WRITE ===")
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

// buildPhaseConfigs creates per-phase config summaries for the live dashboard and status.
func buildPhaseConfigs(cfg *config.Config) []display.PhaseConfig {
	var configs []display.PhaseConfig

	// Cut
	configs = append(configs, display.PhaseConfig{
		Phase: display.PhaseCut,
		Info:  fmt.Sprintf("dpi=%d", cfg.Cut.DPI),
	})

	// Layout
	if cfg.Layout.Tool != "" {
		layoutInfo := cfg.Layout.Tool
		if len(cfg.Layout.Params) > 0 {
			var parts []string
			for k, v := range cfg.Layout.Params {
				parts = append(parts, fmt.Sprintf("%s=%v", k, v))
			}
			sort.Strings(parts)
			layoutInfo += " (" + strings.Join(parts, ", ") + ")"
		}
		configs = append(configs, display.PhaseConfig{
			Phase: display.PhaseLayout,
			Info:  layoutInfo,
		})
	}

	// OCR
	if cfg.OCR.Tool != "" {
		configs = append(configs, display.PhaseConfig{
			Phase: display.PhaseOCR,
			Info:  cfg.OCR.Tool,
		})
	}

	// Read
	var readModels []string
	for _, m := range cfg.Read.Models {
		readModels = append(readModels, m.Provider+"/"+m.Model)
	}
	configs = append(configs, display.PhaseConfig{
		Phase:    display.PhaseRead,
		SubItems: readModels,
	})

	// Translate
	var transModels []string
	for _, m := range cfg.Translate.Models {
		transModels = append(transModels, m.Provider+"/"+m.Model)
	}
	transInfo := "\u2192 " + strings.Join(cfg.Translate.Languages, ", ")
	configs = append(configs, display.PhaseConfig{
		Phase:    display.PhaseTranslate,
		Info:     transInfo,
		SubItems: transModels,
	})

	// Write
	configs = append(configs, display.PhaseConfig{
		Phase: display.PhaseWrite,
		Info:  strings.Join(cfg.Write.Formats, ", "),
	})

	return configs
}

func printPhaseSummary(w io.Writer, colors display.StatusColors, name string, result pipeline.PhaseResult) {
	detail := fmt.Sprintf("%d pages", result.Completed)
	if result.Failed > 0 {
		detail += colors.Red(fmt.Sprintf(" (%d failed)", result.Failed))
	}
	_, _ = fmt.Fprintf(w, "  %-12s %s\n", colors.Cyan(name+":"), detail)
}
