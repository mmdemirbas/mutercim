package cli

import (
	"fmt"
	"os"

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

	tracker := progress.NewTracker(ws.ProgressPath())
	if err := tracker.Load(); err != nil {
		return fmt.Errorf("load progress: %w", err)
	}

	state := tracker.State()

	if state.BookID != "" {
		fmt.Printf("Book: %s\n", state.BookID)
	}
	if state.TotalPages > 0 {
		fmt.Printf("Total pages: %d\n", state.TotalPages)
	}

	if len(state.Phases) == 0 {
		fmt.Println("\nNo processing has been done yet.")
		return nil
	}

	fmt.Println()
	phases := []progress.PhaseName{
		progress.PhaseExtract, progress.PhaseEnrich,
		progress.PhaseTranslate, progress.PhaseCompile,
	}

	for _, phase := range phases {
		ps, ok := state.Phases[phase]
		if !ok {
			continue
		}
		fmt.Printf("%s:\n", phase)
		fmt.Printf("  completed: %d\n", len(ps.Completed))
		if len(ps.Failed) > 0 {
			fmt.Printf("  failed:    %d %v\n", len(ps.Failed), ps.Failed)
		}
		if len(ps.Pending) > 0 {
			fmt.Printf("  pending:   %d\n", len(ps.Pending))
		}
		if ps.LastRun != "" {
			fmt.Printf("  last run:  %s\n", ps.LastRun)
		}
	}

	return nil
}
