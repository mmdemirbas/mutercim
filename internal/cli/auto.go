package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/mmdemirbas/mutercim/internal/config"
	"github.com/mmdemirbas/mutercim/internal/display"
	"github.com/mmdemirbas/mutercim/internal/docker"
	"github.com/mmdemirbas/mutercim/internal/knowledge"
	"github.com/mmdemirbas/mutercim/internal/ocr"
	"github.com/mmdemirbas/mutercim/internal/pipeline"
	"github.com/mmdemirbas/mutercim/internal/workspace"
)

// phase represents a pipeline phase in execution order.
type phase int

const (
	phaseCut       phase = iota
	phaseLayout    phase = iota
	phaseOCR       phase = iota
	phaseRead      phase = iota
	phaseSolve     phase = iota
	phaseTranslate phase = iota
	phaseWrite     phase = iota
)

// hasPhaseOutput checks whether a phase has completed successfully.
// For phases that write report.json on completion, we check for that marker
// instead of just checking for any directory entries. This prevents interrupted
// phases from being treated as complete.
//
//nolint:cyclop // dispatch over phase enum with per-phase special cases
func hasPhaseOutput(p phase, ws *workspace.Workspace, cfg *config.Config) bool {
	switch p {
	case phaseCut:
		return hasReport(ws.CutDir())
	case phaseLayout:
		return hasReport(ws.LayoutDir())
	case phaseOCR:
		if cfg.OCR.Tool == "" {
			return true // OCR disabled counts as "output present" — skip it
		}
		return hasReport(ws.OcrDir())
	case phaseRead:
		return hasReport(ws.ReadDir())
	case phaseSolve:
		return hasReport(ws.SolveDir())
	case phaseTranslate:
		for _, lang := range cfg.Translate.Languages {
			if hasReport(filepath.Join(ws.TranslateDir(), lang)) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

// hasReport checks if any subdirectory of dir contains a report.json file,
// which serves as a completion marker for pipeline phases.
func hasReport(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() {
			if _, err := os.Stat(filepath.Join(dir, e.Name(), "report.json")); err == nil {
				return true
			}
		}
		// Cut phase writes report.json directly in the images dir (no subdirectory nesting)
		if !e.IsDir() && e.Name() == "report.json" {
			return true
		}
	}
	return false
}

// runPrerequisites runs all pipeline phases needed before targetPhase.
// It finds the first missing phase and runs from there through targetPhase-1.
//
//nolint:cyclop,gocognit,funlen // orchestrates all pipeline phases; complexity is inherent
func runPrerequisites(ctx context.Context, targetPhase phase, ws *workspace.Workspace, cfg *config.Config, pagesToProcess []int, disp display.Display) error {
	// Find first missing prerequisite
	startPhase := targetPhase
	for p := phase(0); p < targetPhase; p++ {
		if !hasPhaseOutput(p, ws, cfg) {
			startPhase = p
			break
		}
	}

	if startPhase == targetPhase {
		return nil // all prerequisites satisfied
	}

	logger := slog.Default()
	logger.Info("auto-running prerequisite phases", "from", phaseName(startPhase), "to", phaseName(targetPhase-1))

	if startPhase <= phaseCut && phaseCut < targetPhase {
		if err := docker.CheckAvailable(ctx); err != nil {
			return err
		}

		logger.Info("=== AUTO: CUT ===")
		if err := pipeline.Cut(ctx, pipeline.CutOptions{
			Workspace: ws, Config: cfg, Pages: pagesToProcess, Logger: logger, Display: disp,
		}); err != nil {
			return fmt.Errorf("auto cut: %w", err)
		}
	}

	if startPhase <= phaseLayout && phaseLayout < targetPhase {
		if cfg.Layout.Tool == "" {
			logger.Info("layout tool disabled, skipping layout phase")
		} else {
			logger.Info("=== AUTO: LAYOUT ===")
			if _, err := pipeline.Layout(ctx, pipeline.LayoutOptions{
				Workspace: ws, Config: cfg, Pages: pagesToProcess, Logger: logger, Display: disp,
			}); err != nil {
				return fmt.Errorf("auto layout: %w", err)
			}
		}
	}

	if startPhase <= phaseOCR && phaseOCR < targetPhase {
		if cfg.OCR.Tool == "" {
			logger.Info("ocr tool disabled, skipping ocr phase")
		} else {
			logger.Info("=== AUTO: OCR ===")
			ocrTool := ocr.NewTool(cfg.OCR.Tool)
			if ocrTool == nil {
				return fmt.Errorf("unknown OCR tool: %q", cfg.OCR.Tool)
			}
			if _, err := pipeline.OCR(ctx, pipeline.OCROptions{
				Workspace: ws, Config: cfg, Tool: ocrTool,
				Pages: pagesToProcess, Logger: logger, Display: disp,
			}); err != nil {
				return fmt.Errorf("auto ocr: %w", err)
			}
		}
	}

	if startPhase <= phaseRead && phaseRead < targetPhase {
		readChain, err := createProviderChain(cfg.Read.Models, cfg.Read.Retry, logger)
		if err != nil {
			return fmt.Errorf("auto create read providers: %w", err)
		}
		defer readChain.Close()

		logger.Info("=== AUTO: READ ===")
		result, err := pipeline.Read(ctx, pipeline.ReadOptions{
			Workspace: ws, Config: cfg, Provider: readChain,
			Pages: pagesToProcess, Logger: logger, Display: disp,
		})
		if err != nil {
			return fmt.Errorf("auto read: %w", err)
		}
		if result.IsEmpty() {
			return fmt.Errorf("auto read produced 0 pages")
		}
	}

	if startPhase <= phaseSolve && phaseSolve < targetPhase {
		k, err := knowledge.Load(cfg.ResolveKnowledgePaths(ws.Root), ws.MemoryDir())
		if err != nil {
			return fmt.Errorf("auto load knowledge: %w", err)
		}

		sourceLang := cfg.PrimarySourceLang()

		logger.Info("=== AUTO: SOLVE ===")
		result, err := pipeline.Solve(ctx, pipeline.SolveOptions{
			Workspace: ws, Knowledge: k, KnowledgePaths: cfg.ResolveKnowledgePaths(ws.Root),
			SourceLang: sourceLang, Pages: pagesToProcess, Logger: logger, Display: disp,
		})
		if err != nil {
			return fmt.Errorf("auto solve: %w", err)
		}
		if result.IsEmpty() {
			return fmt.Errorf("auto solve produced 0 pages")
		}
	}

	if startPhase <= phaseTranslate && phaseTranslate < targetPhase {
		k, err := knowledge.Load(cfg.ResolveKnowledgePaths(ws.Root), ws.MemoryDir())
		if err != nil {
			return fmt.Errorf("auto load knowledge: %w", err)
		}

		translateChain, err := createProviderChain(cfg.Translate.Models, cfg.Translate.Retry, logger)
		if err != nil {
			return fmt.Errorf("auto create translate providers: %w", err)
		}
		defer translateChain.Close()

		logger.Info("=== AUTO: TRANSLATE ===")
		result, err := pipeline.Translate(ctx, pipeline.TranslateOptions{
			Workspace: ws, Config: cfg, Provider: translateChain, Knowledge: k,
			Pages: pagesToProcess, Logger: logger, Display: disp,
		})
		if err != nil {
			return fmt.Errorf("auto translate: %w", err)
		}
		if result.IsEmpty() {
			return fmt.Errorf("auto translate produced 0 pages")
		}
	}

	return nil
}

// phaseName returns a human-readable name for a phase.
func phaseName(p phase) string {
	switch p {
	case phaseCut:
		return "cut"
	case phaseLayout:
		return "layout"
	case phaseOCR:
		return "ocr"
	case phaseRead:
		return "read"
	case phaseSolve:
		return "solve"
	case phaseTranslate:
		return "translate"
	case phaseWrite:
		return "write"
	default:
		return "unknown"
	}
}
