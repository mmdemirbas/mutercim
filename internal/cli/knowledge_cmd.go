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
	cmd.AddCommand(newKnowledgeDiffCmd())
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

			knowledgeDir := cfg.ResolvePath(ws.Root, cfg.KnowledgeDir)
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

func newKnowledgeDiffCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "diff",
		Short: "Show what staged knowledge adds or overrides on top of persistent knowledge",
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

			// Load persistent knowledge only (embedded + workspace, no staged)
			knowledgeDir := cfg.ResolvePath(ws.Root, cfg.KnowledgeDir)
			persistent, err := knowledge.Load(knowledgeDir, "")
			if err != nil {
				return fmt.Errorf("load persistent knowledge: %w", err)
			}

			// Load all three layers (including staged)
			merged, err := knowledge.Load(knowledgeDir, ws.StagedDir())
			if err != nil {
				return fmt.Errorf("load merged knowledge: %w", err)
			}

			// Diff sources
			newSources, overriddenSources := diffSources(persistent.Sources, merged.Sources)

			if len(newSources) == 0 && len(overriddenSources) == 0 {
				fmt.Println("No differences. Staged knowledge is empty or identical to persistent.")
				return nil
			}

			if len(newSources) > 0 {
				fmt.Printf("New sources from staging (%d):\n", len(newSources))
				for _, s := range newSources {
					fmt.Printf("  + %s = %s (%s)\n", s.Code, s.NameTr, s.NameAr)
				}
			}

			if len(overriddenSources) > 0 {
				fmt.Printf("Overridden sources (%d):\n", len(overriddenSources))
				for _, s := range overriddenSources {
					fmt.Printf("  ~ %s = %s (%s) [was: layer %s]\n", s.Code, s.NameTr, s.NameAr, s.Layer)
				}
			}

			return nil
		},
	}
}

// diffSources compares persistent and merged source lists, returning new and overridden entries.
func diffSources(persistent, merged []knowledge.Source) (newEntries, overridden []knowledge.Source) {
	persistentSet := make(map[string]knowledge.Source)
	for _, s := range persistent {
		persistentSet[s.Code] = s
	}

	for _, s := range merged {
		if s.Layer != "staged" {
			continue
		}
		if _, exists := persistentSet[s.Code]; exists {
			overridden = append(overridden, s)
		} else {
			newEntries = append(newEntries, s)
		}
	}
	return
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
