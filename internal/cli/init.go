package cli

import (
	"fmt"

	"github.com/mmdemirbas/mutercim/internal/workspace"
	"github.com/spf13/cobra"
)

var (
	initNonInteractive bool
	initTitle          string
	initSourceLangs    string
	initTargetLangs    string
)

func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new book workspace in current directory",
		Long:  `Creates workspace directory structure (input/, knowledge/, log/, memory/, pages/, read/, solve/, translate/, write/) and a default mutercim.yaml config file.`,
		RunE:  runInit,
	}

	cmd.Flags().BoolVar(&initNonInteractive, "non-interactive", false, "scaffold workspace with defaults, no prompts")
	cmd.Flags().StringVar(&initTitle, "title", "", "book title (non-interactive mode)")
	cmd.Flags().StringVar(&initSourceLangs, "source-langs", "ar", "source languages, comma-separated (e.g. ar,fa)")
	cmd.Flags().StringVar(&initTargetLangs, "target-langs", "tr", "target languages, comma-separated (e.g. tr,en)")

	return cmd
}

func runInit(cmd *cobra.Command, args []string) error {
	opts := workspace.InitOptions{
		Title:       initTitle,
		SourceLangs: initSourceLangs,
		TargetLangs: initTargetLangs,
	}

	if !initNonInteractive && initTitle == "" {
		// Interactive mode: prompt for inputs
		fmt.Print("Book title: ")
		fmt.Scanln(&opts.Title)

		fmt.Printf("Source languages (comma-separated) [%s]: ", initSourceLangs)
		var sl string
		fmt.Scanln(&sl)
		if sl != "" {
			opts.SourceLangs = sl
		}

		fmt.Printf("Target languages (comma-separated) [%s]: ", initTargetLangs)
		var tl string
		fmt.Scanln(&tl)
		if tl != "" {
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
	fmt.Println("  2. Edit mutercim.yaml to configure sections")
	fmt.Println("  3. Run: mutercim pages && mutercim read")
	return nil
}
