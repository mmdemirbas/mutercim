package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mmdemirbas/mutercim/internal/config"
	"github.com/mmdemirbas/mutercim/internal/knowledge"
	"github.com/mmdemirbas/mutercim/internal/workspace"
	"github.com/spf13/cobra"
)

func newKnowledgeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "knowledge",
		Short: "Manage knowledge layers (sources, terminology, people)",
	}

	cmd.AddCommand(newKnowledgeListCmd())
	cmd.AddCommand(newKnowledgeStagedCmd())
	cmd.AddCommand(newKnowledgePromoteCmd())

	return cmd
}

func newKnowledgeListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Show all loaded knowledge with layer counts",
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

			knowledgeDir := cfg.ResolvePath(ws.Root, cfg.Knowledge.Dir)
			k, err := knowledge.Load(knowledgeDir, ws.StagedDir())
			if err != nil {
				return fmt.Errorf("load knowledge: %w", err)
			}

			fmt.Printf("Honorifics:  %d entries\n", len(k.Honorifics))
			fmt.Printf("Sources:     %d entries\n", len(k.Sources))
			fmt.Printf("People:      %d entries\n", len(k.People))
			fmt.Printf("Terminology: %d entries\n", len(k.Terminology))
			fmt.Printf("Places:      %d entries\n", len(k.Places))

			// Show source layer breakdown
			if len(k.Sources) > 0 {
				layers := make(map[string]int)
				for _, s := range k.Sources {
					layers[s.Layer]++
				}
				fmt.Println("\nSources by layer:")
				for layer, count := range layers {
					fmt.Printf("  %-10s %d\n", layer+":", count)
				}
			}

			return nil
		},
	}
}

func newKnowledgeStagedCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "staged",
		Short: "List staged knowledge files pending review",
		RunE: func(cmd *cobra.Command, args []string) error {
			ws, err := workspace.Discover(".")
			if err != nil {
				return fmt.Errorf("workspace: %w", err)
			}

			files, err := ws.ListStagedFiles()
			if err != nil {
				return fmt.Errorf("list staged: %w", err)
			}

			if len(files) == 0 {
				fmt.Println("No staged knowledge files.")
				return nil
			}

			fmt.Printf("%d staged file(s):\n", len(files))
			for _, f := range files {
				path := filepath.Join(ws.StagedDir(), f)
				info, err := os.Stat(path)
				if err != nil {
					fmt.Printf("  %s\n", f)
					continue
				}
				fmt.Printf("  %s (%d bytes)\n", f, info.Size())
			}

			return nil
		},
	}
}

func newKnowledgePromoteCmd() *cobra.Command {
	var replace bool

	cmd := &cobra.Command{
		Use:   "promote <file>",
		Short: "Promote a staged file to persistent knowledge",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ws, err := workspace.Discover(".")
			if err != nil {
				return fmt.Errorf("workspace: %w", err)
			}

			filename := args[0]
			if err := ws.PromoteStagedFile(filename, replace); err != nil {
				return err
			}

			fmt.Printf("Promoted %s to %s\n", filename, ws.KnowledgeDir())
			return nil
		},
	}

	cmd.Flags().BoolVar(&replace, "replace", false, "replace existing file if present")
	return cmd
}
