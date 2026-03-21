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
	"github.com/mmdemirbas/mutercim/internal/pipeline"
	"github.com/mmdemirbas/mutercim/internal/workspace"
)

// phase represents a pipeline phase in execution order.
type phase int

const (
	phasePages     phase = iota
	phaseRead      phase = iota
	phaseSolve     phase = iota
	phaseTranslate phase = iota
	phaseWrite     phase = iota
)

// hasPhaseOutput checks whether a phase has already produced output.
func hasPhaseOutput(p phase, ws *workspace.Workspace, cfg *config.Config) bool {
	switch p {
	case phasePages:
		return dirHasEntries(ws.PagesDir())
	case phaseRead:
		return dirHasEntries(ws.ReadDir())
	case phaseSolve:
		return dirHasEntries(ws.SolveDir())
	case phaseTranslate:
		for _, lang := range cfg.Translate.Languages {
			if dirHasEntries(filepath.Join(ws.TranslateDir(), lang)) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

// dirHasEntries returns true if the directory exists and has at least one entry.
func dirHasEntries(dir string) bool {
	entries, err := os.ReadDir(dir)
	return err == nil && len(entries) > 0
}

// runPrerequisites runs all pipeline phases needed before targetPhase.
// It finds the first missing phase and runs from there through targetPhase-1.
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

	if startPhase <= phasePages && phasePages < targetPhase {
		if err := docker.CheckAvailable(ctx); err != nil {
			return err
		}

		logger.Info("=== AUTO: PAGES ===")
		if err := pipeline.Pages(ctx, pipeline.PagesOptions{
			Workspace: ws, Config: cfg, Pages: pagesToProcess, Logger: logger, Display: disp,
		}); err != nil {
			return fmt.Errorf("auto pages: %w", err)
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
	case phasePages:
		return "pages"
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
