package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/mmdemirbas/mutercim/internal/workspace"
	"github.com/spf13/cobra"
)

var (
	initNonInteractive bool
	initSourceLangs    string
	initTargetLangs    string
)

func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new book workspace in current directory",
		Long:  `Creates workspace directory structure (input/, knowledge/, log/, memory/, cut/, read/, solve/, translate/, write/) and a default mutercim.yaml config file.`,
		RunE:  runInit,
	}

	cmd.Flags().BoolVar(&initNonInteractive, "non-interactive", false, "scaffold workspace with defaults, no prompts")
	cmd.Flags().StringVar(&initSourceLangs, "source-langs", "ar", "source languages, comma-separated (e.g. ar,fa)")
	cmd.Flags().StringVar(&initTargetLangs, "target-langs", "tr", "target languages, comma-separated (e.g. tr,en)")

	return cmd
}

func runInit(cmd *cobra.Command, args []string) error {
	opts := workspace.InitOptions{
		SourceLangs: initSourceLangs,
		TargetLangs: initTargetLangs,
	}

	if !initNonInteractive {
		// Interactive mode: prompt for inputs
		reader := bufio.NewReader(os.Stdin)

		fmt.Printf("Source languages (comma-separated) [%s]: ", initSourceLangs)
		line, _ := reader.ReadString('\n')
		if sl := strings.TrimSpace(line); sl != "" {
			opts.SourceLangs = sl
		}

		fmt.Printf("Target languages (comma-separated) [%s]: ", initTargetLangs)
		line, _ = reader.ReadString('\n')
		if tl := strings.TrimSpace(line); tl != "" {
			opts.TargetLangs = tl
		}
	}

	ws, err := workspace.Init(opts)
	if err != nil {
		return err
	}

	fmt.Printf("Workspace initialized at %s\n", ws.Root)
	fmt.Println("Next steps:")
	fmt.Println("  1. Place your PDF or page images in input/")
	fmt.Println("  2. Edit mutercim.yaml to configure models and languages")
	fmt.Println("  3. Run: mutercim cut && mutercim read")
	return nil
}
