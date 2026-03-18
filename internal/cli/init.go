package cli

import (
	"fmt"

	"github.com/mmdemirbas/mutercim/internal/workspace"
	"github.com/spf13/cobra"
)

var (
	initNonInteractive bool
	initTitle          string
	initAuthor         string
	initSourceLang     string
	initTargetLang     string
)

func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new book workspace in current directory",
		Long:  `Creates workspace directory structure (input/, output/, cache/, knowledge/, reports/) and a default mutercim.yaml config file.`,
		RunE:  runInit,
	}

	cmd.Flags().BoolVar(&initNonInteractive, "non-interactive", false, "scaffold workspace with defaults, no prompts")
	cmd.Flags().StringVar(&initTitle, "title", "", "book title (non-interactive mode)")
	cmd.Flags().StringVar(&initAuthor, "author", "", "book author (non-interactive mode)")
	cmd.Flags().StringVar(&initSourceLang, "source-lang", "ar", "source language")
	cmd.Flags().StringVar(&initTargetLang, "target-lang", "tr", "target language")

	return cmd
}

func runInit(cmd *cobra.Command, args []string) error {
	opts := workspace.InitOptions{
		Title:      initTitle,
		Author:     initAuthor,
		SourceLang: initSourceLang,
		TargetLang: initTargetLang,
	}

	if !initNonInteractive && initTitle == "" {
		// Interactive mode: prompt for inputs
		fmt.Print("Book title: ")
		fmt.Scanln(&opts.Title)

		fmt.Print("Book author: ")
		fmt.Scanln(&opts.Author)

		fmt.Printf("Source language [%s]: ", initSourceLang)
		var sl string
		fmt.Scanln(&sl)
		if sl != "" {
			opts.SourceLang = sl
		}

		fmt.Printf("Target language [%s]: ", initTargetLang)
		var tl string
		fmt.Scanln(&tl)
		if tl != "" {
			opts.TargetLang = tl
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
	fmt.Println("  3. Run: mutercim extract")
	return nil
}
