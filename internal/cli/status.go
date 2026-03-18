package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mmdemirbas/mutercim/internal/config"
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
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	ws, err := workspace.Discover(cwd)
	if err != nil {
		return fmt.Errorf("not in a workspace: %w", err)
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

	// Print book info
	if cfg.Book.Title != "" {
		fmt.Printf("Book:   %s\n", cfg.Book.Title)
	}
	if cfg.Book.Author != "" {
		fmt.Printf("Author: %s\n", cfg.Book.Author)
	}
	if cfg.Pages != "" {
		fmt.Printf("Pages:  %s\n", cfg.Pages)
	}

	// Discover inputs from filesystem
	inputs := discoverInputs(ws)
	if len(inputs) == 0 && len(cfg.Inputs) > 0 {
		// No files on disk yet, show configured inputs
		for _, inp := range cfg.Inputs {
			inputs = append(inputs, filepath.Base(strings.TrimSuffix(inp, filepath.Ext(inp))))
		}
	}

	if len(inputs) == 0 {
		fmt.Println("\nNo inputs found.")
		return nil
	}

	// Phase directories to scan
	type phaseDir struct {
		label string
		dir   string
	}
	phaseDirs := []phaseDir{
		{"images", ws.ImagesDir()},
		{"extracted", ws.ExtractedDir()},
		{"enriched", ws.EnrichedDir()},
		{"translated", ws.TranslatedDir()},
	}

	// Print per-input status
	for _, stem := range inputs {
		fmt.Printf("\n%s:\n", stem)

		// Count files on disk per phase
		for _, pd := range phaseDirs {
			subdir := filepath.Join(pd.dir, stem)
			count := countFiles(subdir)
			if count > 0 {
				fmt.Printf("  %-12s %d files\n", pd.label+":", count)
			}
		}

		// Show progress tracker data for all matching phases
		for _, phaseName := range sortedPhaseNames(state) {
			name := string(phaseName)
			// Match "extract:Anfas1" to input "Anfas1"
			if !strings.HasSuffix(name, ":"+stem) {
				continue
			}
			prefix := strings.TrimSuffix(name, ":"+stem)
			ps := state.Phases[phaseName]
			printPhaseProgress(prefix, ps)
		}
	}

	// Show any phases not tied to a specific input (legacy or global)
	orphanPhases := findOrphanPhases(state, inputs)
	if len(orphanPhases) > 0 {
		fmt.Println()
		for _, name := range orphanPhases {
			ps := state.Phases[progress.PhaseName(name)]
			fmt.Printf("%s:\n", name)
			printPhaseProgress("", ps)
		}
	}

	return nil
}

func printPhaseProgress(label string, ps *progress.PhaseState) {
	if label != "" {
		label += ": "
	}
	prefix := "  "

	fmt.Printf("%s%scompleted: %d", prefix, label, len(ps.Completed))
	if len(ps.Failed) > 0 {
		fmt.Printf(", failed: %d %v", len(ps.Failed), ps.Failed)
	}
	if len(ps.Pending) > 0 {
		fmt.Printf(", pending: %d", len(ps.Pending))
	}
	fmt.Println()
	if ps.LastRun != "" {
		fmt.Printf("%s%slast run: %s\n", prefix, label, ps.LastRun)
	}
}

// discoverInputs finds input stems by scanning cache subdirectories for per-input folders.
func discoverInputs(ws *workspace.Workspace) []string {
	seen := make(map[string]bool)
	for _, dir := range []string{ws.ImagesDir(), ws.ExtractedDir(), ws.EnrichedDir(), ws.TranslatedDir()} {
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

// findOrphanPhases returns phase names that don't match any known input stem.
func findOrphanPhases(state progress.State, inputs []string) []string {
	inputSet := make(map[string]bool)
	for _, stem := range inputs {
		inputSet[stem] = true
	}

	var orphans []string
	for name := range state.Phases {
		s := string(name)
		if idx := strings.LastIndex(s, ":"); idx >= 0 {
			stem := s[idx+1:]
			if inputSet[stem] {
				continue
			}
		}
		// Phase without ":" or with unknown stem
		orphans = append(orphans, s)
	}
	sort.Strings(orphans)
	return orphans
}
