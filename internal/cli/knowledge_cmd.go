package cli

import (
	"fmt"

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
			k, err := knowledge.Load(knowledgeDir, ws.MemoryDir())
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
