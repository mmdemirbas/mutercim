package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mmdemirbas/mutercim/internal/config"
	"github.com/mmdemirbas/mutercim/internal/display"
	"github.com/mmdemirbas/mutercim/internal/progress"
	"github.com/mmdemirbas/mutercim/internal/workspace"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show processing progress and any flagged issues",
		RunE:  runStatus,
	}
}

func runStatus(cmd *cobra.Command, args []string) error {
	ws, err := workspace.Discover(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, "No workspace found. Run 'mutercim init' first.")
		return nil
	}

	configPath := cfgFile
	if configPath == "" {
		configPath = ws.ConfigPath()
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	tracker := progress.NewTracker(ws.ProgressPath())
	if err := tracker.Load(); err != nil {
		return fmt.Errorf("load progress: %w", err)
	}
	state := tracker.State()

	// Discover inputs
	inputs := discoverInputs(ws)
	if len(inputs) == 0 && len(cfg.Inputs) > 0 {
		for _, inp := range cfg.Inputs {
			inputs = append(inputs, filepath.Base(strings.TrimSuffix(inp.Path, filepath.Ext(inp.Path))))
		}
	}

	// Count total images
	totalImages := 0
	for _, stem := range inputs {
		totalImages += countFiles(filepath.Join(ws.ImagesDir(), stem))
	}

	// Build input name from config
	inputName := ""
	if len(cfg.Inputs) > 0 {
		inputName = filepath.Base(cfg.Inputs[0].Path)
		if len(cfg.Inputs) > 1 {
			inputName += fmt.Sprintf(" (+%d more)", len(cfg.Inputs)-1)
		}
	}

	// Build phase rows
	rows := buildPhaseRows(state, inputs, totalImages, cfg.Book.TargetLangs)

	// Collect warnings and errors from failed pages
	var warnings, errors []string
	for _, phaseName := range sortedPhaseNames(state) {
		ps := state.Phases[phaseName]
		for _, p := range ps.Failed {
			errors = append(errors, fmt.Sprintf("page %d — failed in %s", p, string(phaseName)))
		}
	}

	// Log file info
	logPath := "mutercim.log"
	var logSize int64
	if info, err := os.Stat(filepath.Join(ws.Root, logPath)); err == nil {
		logSize = info.Size()
	}

	data := display.StatusData{
		BookTitle:   cfg.Book.Title,
		BookAuthor:  cfg.Book.Author,
		InputName:   inputName,
		InputPages:  totalImages,
		PageRange:   cfg.Pages,
		SourceLangs: cfg.Book.SourceLangs,
		TargetLangs: cfg.Book.TargetLangs,
		Phases:      rows,
		Warnings:    warnings,
		Errors:      errors,
		LogPath:     logPath,
		LogSize:     logSize,
	}

	colors := display.NewStatusColors(os.Stdout)
	display.RenderStatus(os.Stdout, data, colors)
	return nil
}

// buildPhaseRows creates the status table rows from progress state.
func buildPhaseRows(state progress.State, inputs []string, totalImages int, targetLangs []string) []display.ProgressRow {
	var rows []display.ProgressRow

	// Aggregate across all inputs for each phase
	pagesCompleted := aggregateCompleted(state, "pages", inputs)
	readCompleted, readFailed, readWarnings := aggregateAll(state, "read", inputs)
	solveCompleted, solveFailed, _ := aggregateAll(state, "solve", inputs)

	// Pages row
	pagesTotal := totalImages
	if pagesTotal == 0 {
		pagesTotal = pagesCompleted // fallback
	}
	rows = append(rows, display.ProgressRow{
		Phase: display.PhasePages, Completed: pagesCompleted, Total: pagesTotal,
		Done: pagesCompleted > 0 && pagesCompleted >= pagesTotal,
	})

	// Read row
	readTotal := totalImages
	rows = append(rows, display.ProgressRow{
		Phase: display.PhaseRead, Completed: readCompleted, Failed: readFailed,
		Total: readTotal, Warnings: readWarnings,
		Done: readTotal > 0 && readCompleted+readFailed >= readTotal,
	})

	// Solve row — total is based on successful reads
	solveTotal := readCompleted
	rows = append(rows, display.ProgressRow{
		Phase: display.PhaseSolve, Completed: solveCompleted, Failed: solveFailed,
		Total: solveTotal,
		Done:  solveTotal > 0 && solveCompleted+solveFailed >= solveTotal,
	})

	// Translate/write rows — per target language
	for _, lang := range targetLangs {
		transCompleted, transFailed, _ := aggregateAllLang(state, "translate", lang, inputs)
		transTotal := solveCompleted
		rows = append(rows, display.ProgressRow{
			Phase: display.PhaseTranslate, Completed: transCompleted, Failed: transFailed,
			Total: transTotal, Lang: lang,
			Done: transTotal > 0 && transCompleted+transFailed >= transTotal,
		})
	}

	for _, lang := range targetLangs {
		writeCompleted, writeFailed, _ := aggregateAllLang(state, "write", lang, inputs)
		transCompleted, _, _ := aggregateAllLang(state, "translate", lang, inputs)
		writeTotal := transCompleted
		rows = append(rows, display.ProgressRow{
			Phase: display.PhaseWrite, Completed: writeCompleted, Failed: writeFailed,
			Total: writeTotal, Lang: lang,
			Done: writeTotal > 0 && writeCompleted+writeFailed >= writeTotal,
		})
	}

	return rows
}

// aggregateCompleted sums completed pages across all inputs for a phase prefix.
func aggregateCompleted(state progress.State, prefix string, inputs []string) int {
	total := 0
	for _, stem := range inputs {
		name := progress.PhaseName(prefix + ":" + stem)
		if ps, ok := state.Phases[name]; ok {
			total += len(ps.Completed)
		}
	}
	return total
}

// aggregateAll sums completed, failed, and warning counts across inputs.
func aggregateAll(state progress.State, prefix string, inputs []string) (completed, failed, warnings int) {
	for _, stem := range inputs {
		name := progress.PhaseName(prefix + ":" + stem)
		if ps, ok := state.Phases[name]; ok {
			completed += len(ps.Completed)
			failed += len(ps.Failed)
		}
	}
	return
}

// aggregateAllLang sums across inputs for a lang-specific phase (translate:lang:stem, write:lang:stem).
func aggregateAllLang(state progress.State, prefix, lang string, inputs []string) (completed, failed, warnings int) {
	for _, stem := range inputs {
		name := progress.PhaseName(prefix + ":" + lang + ":" + stem)
		if ps, ok := state.Phases[name]; ok {
			completed += len(ps.Completed)
			failed += len(ps.Failed)
		}
	}
	return
}

// discoverInputs finds input stems by scanning midstate subdirectories.
func discoverInputs(ws *workspace.Workspace) []string {
	seen := make(map[string]bool)
	for _, dir := range []string{ws.ImagesDir(), ws.ReadDir(), ws.SolvedDir()} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				seen[e.Name()] = true
			}
		}
	}
	// Also check translated dir (has per-lang subdirs)
	entries, err := os.ReadDir(ws.TranslatedDir())
	if err == nil {
		for _, langDir := range entries {
			if !langDir.IsDir() {
				continue
			}
			subEntries, err := os.ReadDir(filepath.Join(ws.TranslatedDir(), langDir.Name()))
			if err != nil {
				continue
			}
			for _, e := range subEntries {
				if e.IsDir() {
					seen[e.Name()] = true
				}
			}
		}
	}

	stems := make([]string, 0, len(seen))
	for name := range seen {
		stems = append(stems, name)
	}
	sort.Strings(stems)
	return stems
}

// countFiles counts non-directory entries in a directory.
func countFiles(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() {
			count++
		}
	}
	return count
}

func sortedPhaseNames(state progress.State) []progress.PhaseName {
	names := make([]progress.PhaseName, 0, len(state.Phases))
	for name := range state.Phases {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		return string(names[i]) < string(names[j])
	})
	return names
}
