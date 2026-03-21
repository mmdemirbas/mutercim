package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mmdemirbas/mutercim/internal/display"
	"github.com/mmdemirbas/mutercim/internal/workspace"
	"github.com/spf13/cobra"
)

// cleanablePhases defines the phase ordering for "+" suffix expansion.
// The order matches the pipeline: log, memory, pages, read, solve, translate, write.
var cleanablePhases = []string{"log", "memory", "pages", "read", "solve", "translate", "write"}

// phaseDir returns the workspace directory for a cleanable phase.
func phaseDir(ws *workspace.Workspace, phase string) string {
	switch phase {
	case "log":
		return ws.LogDir()
	case "memory":
		return ws.MemoryDir()
	case "pages":
		return ws.PagesDir()
	case "read":
		return ws.ReadDir()
	case "solve":
		return ws.SolveDir()
	case "translate":
		return ws.TranslateDir()
	case "write":
		return ws.WriteDir()
	default:
		return ""
	}
}

// expandTargets resolves target arguments into a deduplicated list of phases.
// "all" expands to all phases, "+" suffix expands to the phase and all downstream.
func expandTargets(args []string) ([]string, error) {
	seen := make(map[string]bool)
	var result []string

	for _, arg := range args {
		if arg == "all" {
			return append([]string(nil), cleanablePhases...), nil
		}

		cascade := strings.HasSuffix(arg, "+")
		name := strings.TrimSuffix(arg, "+")

		idx := phaseIndex(name)
		if idx < 0 {
			return nil, fmt.Errorf("unknown clean target %q (valid: %s, all)", arg, strings.Join(cleanablePhases, ", "))
		}

		phases := cleanablePhases[idx:]
		if !cascade {
			phases = cleanablePhases[idx : idx+1]
		}

		for _, p := range phases {
			if !seen[p] {
				seen[p] = true
				result = append(result, p)
			}
		}
	}
	return result, nil
}

// phaseIndex returns the index of a phase name in cleanablePhases, or -1.
func phaseIndex(name string) int {
	for i, p := range cleanablePhases {
		if p == name {
			return i
		}
	}
	return -1
}

// dirSize returns the total size of all files in a directory tree.
func dirSize(path string) int64 {
	var total int64
	entries, err := os.ReadDir(path)
	if err != nil {
		return 0
	}
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		if e.IsDir() {
			total += dirSize(filepath.Join(path, e.Name()))
		} else {
			total += info.Size()
		}
	}
	return total
}

// formatSize returns a human-readable size string.
func formatSize(bytes int64) string {
	if bytes == 0 {
		return "empty"
	}
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	}
	if bytes < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
}

func newCleanCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clean <targets...>",
		Short: "Delete generated data for specified phases",
		Long: `Delete generated directories and reset progress tracking.

Targets: log, memory, pages, read, solve, translate, write, all

Use "+" suffix to include downstream phases:
  mutercim clean read+       # read/ solve/ translate/ write/
  mutercim clean pages+      # pages/ read/ solve/ translate/ write/
  mutercim clean all         # everything (except input/ and knowledge/)

Multiple targets:
  mutercim clean log read solve

NEVER deletes: input/, knowledge/, mutercim.yaml, .env`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || (len(args) == 1 && args[0] == "help") {
				return cmd.Help()
			}
			ws, err := workspace.Discover(".")
			if err != nil {
				return fmt.Errorf("workspace: %w", err)
			}

			phases, err := expandTargets(args)
			if err != nil {
				return err
			}

			// Collect directories to delete with sizes
			type target struct {
				phase string
				dir   string
				size  int64
			}
			var targets []target
			for _, phase := range phases {
				dir := phaseDir(ws, phase)
				if dir == "" {
					continue
				}
				if _, err := os.Stat(dir); os.IsNotExist(err) {
					continue
				}
				targets = append(targets, target{phase: phase, dir: dir, size: dirSize(dir)})
			}

			if len(targets) == 0 {
				fmt.Println("Nothing to clean.")
				return nil
			}

			colors := display.NewStatusColors(os.Stdout)

			// Print what will be deleted
			for _, t := range targets {
				fmt.Printf("  %s\t%s\n", colors.Red(t.phase+"/"), colors.Dim(formatSize(t.size)))
			}

			// Delete directories (truncate log instead of removing)
			for _, t := range targets {
				if t.phase == "log" {
					f, err := os.OpenFile(ws.LogPath(), os.O_TRUNC|os.O_WRONLY, 0644)
					if err == nil {
						f.Close()
					}
					continue // don't RemoveAll
				}
				if err := os.RemoveAll(t.dir); err != nil {
					return fmt.Errorf("remove %s: %w", t.dir, err)
				}
			}

			fmt.Printf("%s %d directories.\n", colors.Green("\u2713"), len(targets))
			return nil
		},
	}
}
